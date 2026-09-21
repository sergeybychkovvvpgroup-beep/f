package notes

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var ignoredDirs = map[string]struct{}{
	".git":      {},
	".obsidian": {},
	"_archive":  {},
	"bin":       {},
	"dist":      {},
}

type ValidationError struct {
	Path    string
	Problem string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Path, e.Problem)
}

type LoadResult struct {
	Entries []Entry
	Errors  []error
}

func LoadDir(root string) LoadResult {
	var result LoadResult
	counter := 0

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			result.Errors = append(result.Errors, walkErr)
			return nil
		}

		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			if _, skip := ignoredDirs[d.Name()]; skip {
				return filepath.SkipDir
			}
			return nil
		}

		if !isSupportedNoteFile(path) {
			return nil
		}

		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		entries, err := loadFile(path)
		if err != nil {
			result.Errors = append(result.Errors, err)
			return nil
		}

		for i := range entries {
			counter++
			entries[i].index = counter
			result.Entries = append(result.Entries, entries[i])
		}

		return nil
	})

	sort.Slice(result.Entries, func(i, j int) bool {
		if result.Entries[i].SourceFile == result.Entries[j].SourceFile {
			return result.Entries[i].index < result.Entries[j].index
		}
		return result.Entries[i].SourceFile < result.Entries[j].SourceFile
	})

	return result
}

func LoadBytes(path string, raw []byte) LoadResult {
	var result LoadResult

	entries, err := loadStructuredBytes(path, raw)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}

	for i := range entries {
		entries[i].index = i + 1
	}
	result.Entries = entries
	return result
}

func loadFile(path string) ([]Entry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return loadStructuredBytes(path, raw)
}

func loadStructuredBytes(path string, raw []byte) ([]Entry, error) {
	if isYAMLFile(path) {
		noteLike, detectErr := looksLikeNoteYAML(raw)
		if detectErr != nil {
			return nil, ValidationError{Path: path, Problem: detectErr.Error()}
		}
		if noteLike {
			return loadYAML(path, raw)
		}
	}
	return loadRawFile(path, raw)
}

func loadYAML(path string, raw []byte) ([]Entry, error) {

	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, ValidationError{Path: path, Problem: err.Error()}
	}

	if len(doc.Content) == 0 {
		return nil, nil
	}

	root := doc.Content[0]
	var nodes []*yaml.Node

	switch root.Kind {
	case yaml.SequenceNode:
		nodes = root.Content
	case yaml.MappingNode:
		nodes = []*yaml.Node{root}
	default:
		return nil, ValidationError{
			Path:    path,
			Problem: "top-level YAML must be a note object or a list of note objects",
		}
	}

	entries := make([]Entry, 0, len(nodes))
	for _, node := range nodes {
		if node.Kind != yaml.MappingNode {
			return nil, ValidationError{
				Path:    path,
				Problem: fmt.Sprintf("note at line %d must be a YAML object", node.Line),
			}
		}
		if err := validateEntryFields(path, node); err != nil {
			return nil, err
		}

		var entry Entry
		if err := node.Decode(&entry); err != nil {
			return nil, ValidationError{
				Path:    path,
				Problem: fmt.Sprintf("cannot decode note at line %d: %v", node.Line, err),
			}
		}

		entry = normalizeLegacyEntry(entry)
		if err := validateEntry(path, node.Line, entry); err != nil {
			return nil, err
		}

		entry.SourcePath = path
		entry.SourceFile = filepath.Base(path)
		entry.SourceLine = node.Line
		entry.SourceKind = SourceKindNote
		entry.searchData = weightedFields(entry)
		entries = append(entries, entry)
	}

	return entries, nil
}

func validateEntryFields(path string, node *yaml.Node) error {
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]
		key := strings.TrimSpace(strings.ToLower(keyNode.Value))
		if !isNoteField(key) {
			return ValidationError{
				Path:    path,
				Problem: fmt.Sprintf("note at line %d uses unsupported field %q", keyNode.Line, keyNode.Value),
			}
		}
		switch key {
		case "actions":
			if err := validateActionNodes(path, valueNode); err != nil {
				return err
			}
		case "run":
			if err := validateRunNodes(path, valueNode); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateActionNodes(path string, node *yaml.Node) error {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	for _, item := range node.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(item.Content); i += 2 {
			keyNode := item.Content[i]
			key := strings.TrimSpace(strings.ToLower(keyNode.Value))
			switch key {
			case "desc", "cmd", "text", "banner":
			default:
				return ValidationError{
					Path:    path,
					Problem: fmt.Sprintf("action at line %d uses unsupported field %q", keyNode.Line, keyNode.Value),
				}
			}
		}
	}
	return nil
}

func validateRunNodes(path string, node *yaml.Node) error {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	for _, item := range node.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(item.Content); i += 2 {
			keyNode := item.Content[i]
			key := strings.TrimSpace(strings.ToLower(keyNode.Value))
			switch key {
			case "desc", "run", "banner":
			default:
				return ValidationError{
					Path:    path,
					Problem: fmt.Sprintf("run option at line %d uses unsupported field %q", keyNode.Line, keyNode.Value),
				}
			}
		}
	}
	return nil
}

func validateEntry(path string, line int, entry Entry) error {
	mode := strings.ToLower(strings.TrimSpace(entry.Mode))
	if mode != "" && mode != "run" && mode != "show" {
		return ValidationError{
			Path:    path,
			Problem: fmt.Sprintf("note in %s at line %d has invalid mode %q, expected run or show", filepath.Base(path), line, entry.Mode),
		}
	}

	if strings.TrimSpace(entry.Desc) == "" && strings.TrimSpace(entry.SourceFile) == "" {
		return ValidationError{
			Path:    path,
			Problem: fmt.Sprintf("note in %s at line %d should have desc", filepath.Base(path), line),
		}
	}

	if len(entry.Actions) == 0 {
		return ValidationError{
			Path:    path,
			Problem: fmt.Sprintf("note in %s at line %d must have at least one action", filepath.Base(path), line),
		}
	}

	for i, action := range entry.Actions {
		actionLine := line
		kinds := 0
		if action.IsShow() {
			kinds++
		}
		if action.IsCmd() {
			kinds++
		}
		if kinds == 0 {
			return ValidationError{
				Path:    path,
				Problem: fmt.Sprintf("note in %s action %d at line %d must have text or cmd", filepath.Base(path), i+1, actionLine),
			}
		}
		if kinds > 1 {
			return ValidationError{
				Path:    path,
				Problem: fmt.Sprintf("note in %s action %d at line %d must use only one of text or cmd", filepath.Base(path), i+1, actionLine),
			}
		}
	}

	return nil
}

func normalizeLegacyEntry(entry Entry) Entry {
	if len(entry.Actions) > 0 {
		entry.ActionCmd = strings.TrimSpace(entry.ActionCmd)
		entry.Text = strings.TrimSpace(entry.Text)
		entry.Note = strings.TrimSpace(entry.Note)
		entry.Cmd = strings.TrimSpace(entry.Cmd)
		normalized := make([]Action, 0, len(entry.Actions)+2)
		if entry.ActionCmd != "" {
			normalized = append(normalized, Action{
				Cmd:    entry.ActionCmd,
				Banner: strings.TrimSpace(entry.Banner),
			})
		}
		if entry.Note != "" {
			normalized = append(normalized, Action{
				Desc: inferShowDesc(entry),
				Text: entry.Note,
			})
		}
		if entry.Text != "" {
			normalized = append(normalized, Action{
				Desc: inferShowDesc(entry),
				Text: entry.Text,
			})
		}
		for i := range entry.Actions {
			entry.Actions[i].Desc = strings.TrimSpace(entry.Actions[i].Desc)
			entry.Actions[i].Cmd = strings.TrimSpace(entry.Actions[i].Cmd)
			entry.Actions[i].Text = strings.TrimSpace(entry.Actions[i].Text)
			entry.Actions[i].Banner = strings.TrimSpace(entry.Actions[i].Banner)
			normalized = append(normalized, entry.Actions[i])
		}
		entry.Actions = normalized
		entry.Text = ""
		entry.Note = ""
		return entry
	}

	actions := make([]Action, 0, 1+len(entry.Run))
	entry.ActionCmd = strings.TrimSpace(entry.ActionCmd)
	entry.Text = strings.TrimSpace(entry.Text)
	entry.Cmd = strings.TrimSpace(entry.Cmd)
	legacyDesc := strings.TrimSpace(entry.Desc)
	entry.Lite = true
	if entry.ActionCmd != "" {
		actions = append(actions, Action{
			Cmd:    entry.ActionCmd,
			Banner: strings.TrimSpace(entry.Banner),
		})
	}
	if entry.Cmd != "" {
		actions = append(actions, Action{
			Desc:   legacyActionDesc(legacyDesc, inferActionDesc(entry.Cmd, "run")),
			Cmd:    entry.Cmd,
			Banner: strings.TrimSpace(entry.Banner),
		})
	}
	if entry.Text != "" {
		actions = append(actions, Action{
			Desc: legacyActionDesc(legacyDesc, inferShowDesc(entry)),
			Text: entry.Text,
		})
	}
	if text := strings.TrimSpace(entry.Note); text != "" {
		actions = append(actions, Action{
			Desc: legacyActionDesc(legacyDesc, inferShowDesc(entry)),
			Text: text,
		})
	}
	for i, option := range entry.Run {
		desc := strings.TrimSpace(option.Desc)
		if desc == "" {
			desc = inferActionDesc(option.Run, fmt.Sprintf("run %d", i+1))
		}
		banner := strings.TrimSpace(option.Banner)
		if banner == "" && i == 0 {
			banner = strings.TrimSpace(entry.Banner)
		}
		actions = append(actions, Action{
			Desc:   desc,
			Cmd:    strings.TrimSpace(option.Run),
			Banner: banner,
		})
	}
	entry.Actions = actions
	entry.ActionCmd = ""
	entry.Text = ""
	entry.Note = ""
	entry.Cmd = ""
	entry.Run = nil
	return entry
}

func legacyActionDesc(legacyDesc, fallback string) string {
	if strings.TrimSpace(legacyDesc) != "" {
		return strings.TrimSpace(legacyDesc)
	}
	return fallback
}

func inferActionDesc(command string, fallback string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return fallback
	}

	if parts := strings.Split(command, "&&"); len(parts) > 1 {
		command = strings.TrimSpace(parts[len(parts)-1])
	}
	command = strings.TrimSpace(strings.TrimSuffix(command, ";"))

	fields := strings.Fields(command)
	if len(fields) == 0 {
		return fallback
	}

	first := fields[0]
	if first == "sudo" && len(fields) > 1 {
		first = fields[1]
		fields = fields[1:]
	}
	switch first {
	case "ssh":
		for i := 1; i < len(fields); i++ {
			if fields[i] == "-L" {
				if i+1 < len(fields) {
					if port := strings.SplitN(fields[i+1], ":", 2)[0]; port != "" {
						return "tunnel " + port
					}
				}
				return "tunnel"
			}
			if strings.HasPrefix(fields[i], "-L") && len(fields[i]) > 2 {
				if port := strings.SplitN(strings.TrimPrefix(fields[i], "-L"), ":", 2)[0]; port != "" {
					return "tunnel " + port
				}
				return "tunnel"
			}
		}
		return "ssh"
	case "grep", "find", "dig", "curl", "make", "git", "docker", "mount", "nmap", "echo", "f":
		return first
	default:
		return fallback
	}
}

func isSupportedNoteFile(path string) bool {
	if isYAMLFile(path) {
		return true
	}
	return isRawTextExtension(path)
}

func isYAMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

func isRawTextExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".txt", ".md", ".conf", ".cfg", ".ini", ".json", ".toml", ".env", ".py", ".sh", ".bash", ".zsh", ".vyos", ".service":
		return true
	default:
		return false
	}
}

func looksLikeNoteYAML(raw []byte) (bool, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return false, err
	}
	if len(doc.Content) == 0 {
		return false, nil
	}

	return nodeLooksLikeNoteDoc(doc.Content[0]), nil
}

func nodeLooksLikeNoteDoc(root *yaml.Node) bool {
	switch root.Kind {
	case yaml.SequenceNode:
		if len(root.Content) == 0 {
			return false
		}
		for _, node := range root.Content {
			if node.Kind != yaml.MappingNode {
				return false
			}
			if !mappingLooksLikeNote(node) {
				return false
			}
		}
		return true
	case yaml.MappingNode:
		return mappingLooksLikeNote(root)
	default:
		return false
	}
}

func mappingLooksLikeNote(node *yaml.Node) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := strings.TrimSpace(strings.ToLower(node.Content[i].Value))
		if isNoteField(key) {
			return true
		}
	}
	return false
}

func isNoteField(key string) bool {
	switch key {
	case "desc", "action", "text", "cmd", "mode", "actions", "run", "note", "banner":
		return true
	default:
		return false
	}
}

func loadRawFile(path string, raw []byte) ([]Entry, error) {
	if isBinaryContent(raw) {
		return nil, nil
	}

	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	entry := Entry{
		Desc:       displayNameFromFile(filepath.Base(path)),
		Actions:    []Action{{Desc: sourceActionLabel(path), Text: text}},
		SourcePath: path,
		SourceFile: filepath.Base(path),
		SourceLine: 1,
		SourceKind: SourceKindRaw,
	}
	entry.searchData = weightedFields(entry)
	return []Entry{entry}, nil
}

func sourceActionLabel(path string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if ext == "" {
		return SourceKindRaw
	}
	return ext
}

func isBinaryContent(raw []byte) bool {
	for _, b := range raw {
		if b == 0 {
			return true
		}
	}
	return false
}
