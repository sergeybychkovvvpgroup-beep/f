package notecreate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"aoo/internal/notes"
	"gopkg.in/yaml.v3"
)

type Kind string

const (
	KindNote    Kind = "note"
	KindCommand Kind = "cmd"
)

type Draft struct {
	Path          string
	Line          int
	CommitMessage string
}

func Create(root string, kind Kind, title string) (Draft, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return Draft{}, fmt.Errorf("notes dir is required")
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return Draft{}, fmt.Errorf("create notes dir: %w", err)
	}

	title = strings.TrimSpace(title)
	if title == "" {
		switch kind {
		case KindCommand:
			title = "new command"
		default:
			title = "new note"
		}
	}

	if duplicate := findDraftDuplicate(root, kind, title); duplicate != "" {
		return Draft{}, fmt.Errorf("exact duplicate already exists: %s", duplicate)
	}

	path, err := uniquePath(root, slugify(title))
	if err != nil {
		return Draft{}, err
	}

	content, line, err := scaffold(kind, title)
	if err != nil {
		return Draft{}, err
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return Draft{}, fmt.Errorf("write draft %s: %w", path, err)
	}

	verb := "add note"
	if kind == KindCommand {
		verb = "add command"
	}

	return Draft{
		Path:          path,
		Line:          line,
		CommitMessage: fmt.Sprintf("f: %s %s", verb, filepath.Base(path)),
	}, nil
}

func findDraftDuplicate(root string, kind Kind, title string) string {
	result := notes.LoadDir(root)
	wantTitle := strings.ToLower(strings.Join(strings.Fields(title), " "))
	wantValue := "add note here"
	if kind == KindCommand {
		wantValue = `echo "replace with command"`
	}
	for _, entry := range result.Entries {
		if strings.ToLower(strings.Join(strings.Fields(entry.Desc), " ")) != wantTitle {
			continue
		}
		actions := entry.ActionsList()
		if len(actions) != 1 {
			continue
		}
		value := actions[0].Text
		if kind == KindCommand {
			value = actions[0].Cmd
		}
		if strings.Join(strings.Fields(value), " ") == wantValue {
			return entry.SourcePath
		}
	}
	return ""
}

func scaffold(kind Kind, title string) (string, int, error) {
	desc, err := yamlScalar(title)
	if err != nil {
		return "", 0, err
	}

	switch kind {
	case KindCommand:
		lines := []string{
			"desc: " + desc,
			"cmd: echo \"replace with command\"",
			"",
		}
		return strings.Join(lines, "\n"), 2, nil
	case KindNote:
		fallthrough
	default:
		lines := []string{
			"desc: " + desc,
			"text: |",
			"  add note here",
			"",
		}
		return strings.Join(lines, "\n"), 3, nil
	}
}

func yamlScalar(value string) (string, error) {
	raw, err := yaml.Marshal(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("marshal yaml scalar: %w", err)
	}
	return strings.TrimSpace(string(raw)), nil
}

func uniquePath(root, base string) (string, error) {
	if strings.TrimSpace(base) == "" {
		base = "note"
	}

	for i := 0; i < 1000; i++ {
		name := base
		if i > 0 {
			name = fmt.Sprintf("%s-%d", base, i+1)
		}
		path := filepath.Join(root, name+".yaml")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path, nil
		} else if err != nil {
			return "", fmt.Errorf("check draft path %s: %w", path, err)
		}
	}

	return "", fmt.Errorf("cannot allocate draft filename for %s", base)
}

func slugify(value string) string {
	var b strings.Builder
	lastDash := false

	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "note"
	}
	return out
}
