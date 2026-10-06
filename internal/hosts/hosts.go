package hosts

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"f/internal/notes"
	"gopkg.in/yaml.v3"
)

const (
	envHostsFile       = "F_HOSTS_FILE"
	legacyEnvHostsFile = "AOO_HOSTS_FILE"
)

func hostsFileOverride() string {
	if value := strings.TrimSpace(os.Getenv(envHostsFile)); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv(legacyEnvHostsFile))
}

type Host struct {
	Name            string         `yaml:"name"`
	Host            string         `yaml:"host"`
	User            string         `yaml:"user,omitempty"`
	Port            int            `yaml:"port,omitempty"`
	Tags            []string       `yaml:"tags,omitempty"`
	Args            string         `yaml:"args,omitempty"`
	Cmd             string         `yaml:"cmd,omitempty"`
	Desc            string         `yaml:"desc,omitempty"`
	Preview         string         `yaml:"preview,omitempty"`
	ExpandedCommand string         `yaml:"-"`
	Mode            string         `yaml:"-"`
	ForwardRemote   string         `yaml:"-"`
	SSHSource       bool           `yaml:"-"`
	SSHDirectives   []SSHDirective `yaml:"-"`
	SourcePath      string         `yaml:"-"`
	SourceLine      int            `yaml:"-"`
}

type SSHDirective struct {
	Key   string
	Value string
}

type File struct {
	Hosts []Host `yaml:"hosts"`
}

func Path() (string, error) {
	if value := hostsFileOverride(); value != "" {
		return filepath.Abs(value)
	}
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hosts.yaml"), nil
}

func ConfigDir() (string, error) {
	if value := hostsFileOverride(); value != "" {
		abs, err := filepath.Abs(value)
		if err != nil {
			return "", err
		}
		return filepath.Dir(abs), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "f", "config.d"), nil
}

func Load() ([]Host, error) {
	var out []Host
	for _, path := range hostFiles() {
		if raw, err := os.ReadFile(path); err == nil {
			parsed, err := parse(raw)
			if err != nil {
				return nil, fmt.Errorf("parse %s: %w", path, err)
			}
			for i := range parsed {
				parsed[i].SourcePath = path
			}
			out = append(out, parsed...)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	out = merge(out, loadSSHConfig())
	if len(out) == 0 {
		return nil, fmt.Errorf("no hosts found; add one with: f add NAME [user@]HOST [-p PORT] [--tag TAG]")
	}
	return out, nil
}

func hostFiles() []string {
	if value := hostsFileOverride(); value != "" {
		abs, err := filepath.Abs(value)
		if err != nil {
			return nil
		}
		return []string{abs}
	}
	var out []string
	if dir, err := ConfigDir(); err == nil {
		matches, _ := filepath.Glob(filepath.Join(dir, "*.yaml"))
		sort.Strings(matches)
		out = append(out, matches...)
		defaultPath := filepath.Join(dir, "hosts.yaml")
		if !containsPath(out, defaultPath) {
			out = append(out, defaultPath)
		}
	}
	if legacy, err := legacyPath(); err == nil {
		out = append(out, legacy)
	}
	return out
}

func legacyPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "aoo", "hosts.yaml"), nil
}

func containsPath(paths []string, path string) bool {
	for _, p := range paths {
		if p == path {
			return true
		}
	}
	return false
}

func parse(raw []byte) ([]Host, error) {
	var f File
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	if len(f.Hosts) == 0 {
		var list []Host
		if err := yaml.Unmarshal(raw, &list); err != nil {
			return nil, err
		}
		f.Hosts = list
	}
	for i := range f.Hosts {
		f.Hosts[i] = normalize(f.Hosts[i])
		if f.Hosts[i].Name == "" {
			f.Hosts[i].Name = f.Hosts[i].Host
		}
		if f.Hosts[i].Host == "" && f.Hosts[i].Cmd == "" {
			return nil, fmt.Errorf("host entry %d is missing host or cmd", i+1)
		}
	}
	return f.Hosts, nil
}

func Save(list []Host) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	sort.SliceStable(list, func(i, j int) bool { return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name) })
	raw, err := yaml.Marshal(File{Hosts: list})
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func Add(h Host) (Host, error) {
	h = normalize(h)
	if h.Name == "" {
		return Host{}, errors.New("name is required")
	}
	if h.Host == "" && h.Cmd == "" {
		return Host{}, errors.New("host or cmd is required")
	}
	path, _ := Path()
	var list []Host
	if raw, err := os.ReadFile(path); err == nil {
		parsed, err := parse(raw)
		if err != nil {
			return Host{}, err
		}
		list = parsed
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Host{}, err
	}
	for i := range list {
		if strings.EqualFold(list[i].Name, h.Name) {
			list[i] = h
			return h, Save(list)
		}
	}
	list = append(list, h)
	return h, Save(list)
}

func ToEntries(list []Host) []notes.Entry {
	entries := make([]notes.Entry, 0, len(list))
	for _, h := range list {
		cmd := h.Command()
		label := DisplayName(h)
		detail := hostDetail(h)
		mode := h.Mode
		if mode == "" {
			mode = classifyHostSyntax(h)
		}
		kind := entryKind(mode)
		kindSearch := entryKindSearch(kind)
		searchParts := []string{h.Name, h.Host, h.User, h.Desc, cmd, kindSearch}
		entries = append(entries, notes.Entry{
			Desc:       label,
			Address:    hostAddress(h),
			Command:    displayCommand(h, cmd),
			Kind:       kind,
			KindSearch: kindSearch,
			Mode:       mode,
			SourcePath: h.SourcePath,
			SourceLine: h.SourceLine,
			Editable:   h.SSHSource && looksLikePlainSSH(cmd),
			Note:       strings.Join(searchParts, " "),
			Actions: []notes.Action{{
				Desc:   detail,
				Cmd:    cmd,
				Banner: h.Preview,
			}},
		})
	}
	return entries
}

func displayCommand(h Host, fallback string) string {
	command := strings.TrimSpace(h.ExpandedCommand)
	if command == "" {
		command = strings.TrimSpace(fallback)
	}
	lines := strings.Split(command, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimSpace(strings.TrimSuffix(line, "\\"))
		lines[i] = line
	}
	return strings.TrimSpace(strings.Join(lines, " "))
}

func DisplayName(h Host) string {
	name := strings.TrimSpace(h.Name)
	if name == "" {
		name = strings.TrimSpace(h.Host)
	}
	// A lot of imported runbook aliases were named ssh-<host>. In a SSH
	// picker that prefix is redundant noise; keep the real alias in Cmd so
	// execution still uses the exact OpenSSH config entry.
	for _, prefix := range []string{"ssh-", "aoo-"} {
		if strings.HasPrefix(name, prefix) {
			name = strings.TrimPrefix(name, prefix)
		}
	}
	name = humanizeRouteSuffix(name)
	if suffix := forwardDisplaySuffix(h); suffix != "" {
		name += " " + suffix
	}
	return name
}

func humanizeRouteSuffix(name string) string {
	type suffixRule struct {
		suffix string
		label  string
	}
	rules := []suffixRule{
		{suffix: "-netbird-emergency", label: "netbird emergency"},
		{suffix: "-netbird", label: "netbird"},
		{suffix: "-router-wh-jump", label: "router wh jump"},
		{suffix: "-jump", label: "jump"},
		{suffix: "-tunnel", label: "tunnel"},
	}
	for _, rule := range rules {
		if strings.HasSuffix(name, rule.suffix) {
			base := strings.TrimSuffix(name, rule.suffix)
			if base == "" {
				break
			}
			name = base + " [" + rule.label + "]"
			break
		}
	}
	return name
}

func forwardDisplaySuffix(h Host) string {
	if !strings.Contains(" "+strings.ToLower(h.Mode)+" ", " forwards ") {
		return ""
	}
	remote := strings.TrimSpace(h.ForwardRemote)
	if remote == "" {
		remote = forwardRemoteFromText(h.Args + " " + h.Cmd + " " + h.Preview)
	}
	if remote == "" {
		return ""
	}
	return "[remote " + remote + "]"
}

func forwardRemoteSummary(attrs map[string]string) string {
	for _, key := range []string{"localforward", "remoteforward"} {
		if value := strings.TrimSpace(attrs[key]); value != "" {
			if remote := forwardRemoteFromText(value); remote != "" {
				return remote
			}
		}
	}
	if value := strings.TrimSpace(attrs["dynamicforward"]); value != "" {
		return "socks"
	}
	return ""
}

func forwardRemoteFromText(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return ""
	}
	// OpenSSH LocalForward/RemoteForward: bind/local port first, destination second.
	if len(fields) >= 2 {
		return remotePortLabel(fields[1])
	}
	for _, field := range fields {
		if strings.Contains(field, ":") {
			return remotePortLabel(field)
		}
	}
	return ""
}

func remotePortLabel(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "'\"")
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ":")
	port := parts[len(parts)-1]
	if port == "" {
		return value
	}
	if len(parts) >= 2 {
		host := strings.Join(parts[:len(parts)-1], ":")
		if host != "" && host != "localhost" && host != "127.0.0.1" {
			return host + ":" + port
		}
	}
	return port
}

func hostAddress(h Host) string {
	target := strings.TrimSpace(h.Host)
	if target == "" {
		return ""
	}
	if user := strings.TrimSpace(h.User); user != "" {
		target = user + "@" + target
	}
	if h.Port > 0 {
		target += ":" + strconv.Itoa(h.Port)
	}
	return target
}

func hostDetail(h Host) string {
	parts := []string{}
	if target := hostAddress(h); target != "" {
		parts = append(parts, target)
	}
	if h.Desc != "" && h.Desc != "~/.ssh/config" {
		parts = append(parts, h.Desc)
	}
	return strings.Join(parts, "  ")
}

func (h Host) Command() string {
	if strings.TrimSpace(h.Cmd) != "" {
		return strings.TrimSpace(h.Cmd)
	}
	parts := []string{"ssh"}
	if h.Port > 0 {
		parts = append(parts, "-p", strconv.Itoa(h.Port))
	}
	if h.Args != "" {
		parts = append(parts, strings.Fields(h.Args)...)
	}
	target := h.Host
	if h.User != "" {
		target = h.User + "@" + target
	}
	parts = append(parts, target)
	for i := range parts {
		parts[i] = shellQuote(parts[i])
	}
	return strings.Join(parts, " ")
}

func normalize(h Host) Host {
	h.Name = strings.TrimSpace(h.Name)
	h.Host = strings.TrimSpace(h.Host)
	h.User = strings.TrimSpace(h.User)
	h.Args = strings.TrimSpace(h.Args)
	h.Cmd = strings.TrimSpace(h.Cmd)
	h.Desc = strings.TrimSpace(h.Desc)
	if strings.Contains(h.Host, "@") && h.User == "" {
		parts := strings.SplitN(h.Host, "@", 2)
		h.User = strings.TrimSpace(parts[0])
		h.Host = strings.TrimSpace(parts[1])
	}
	cleanTags := h.Tags[:0]
	for _, tag := range h.Tags {
		if tag = strings.TrimSpace(tag); tag != "" {
			cleanTags = append(cleanTags, tag)
		}
	}
	h.Tags = cleanTags
	return h
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.IndexFunc(s, func(r rune) bool {
		return !(r == '-' || r == '_' || r == '.' || r == '/' || r == ':' || r == '@' || r == '+' || r == '=' || r == ',' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z')
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func merge(primary, secondary []Host) []Host {
	seen := map[string]bool{}
	out := make([]Host, 0, len(primary)+len(secondary))
	for _, h := range append(primary, secondary...) {
		key := strings.ToLower(h.Name)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, h)
	}
	return out
}

func loadSSHConfig() []Host {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	path := filepath.Join(home, ".ssh", "config")
	visited := map[string]bool{}
	return loadSSHConfigFile(path, visited)
}

type sshConfigLine struct {
	Path   string
	Number int
	Text   string
}

func loadSSHConfigFile(path string, visited map[string]bool) []Host {
	lines := expandSSHConfigLines(path, visited)
	var out []Host
	var current []string
	attrs := map[string]string{}
	var directives []SSHDirective
	var pendingTags []string
	currentPath := ""
	currentLine := 0
	flush := func() {
		for _, alias := range current {
			if alias == "" || strings.ContainsAny(alias, "*?") {
				continue
			}
			h := Host{Name: alias, Host: attrs["hostname"], User: attrs["user"], Tags: append([]string{}, pendingTags...), SSHSource: true, SSHDirectives: append([]SSHDirective(nil), directives...), SourcePath: currentPath, SourceLine: currentLine}
			if h.Host == "" {
				h.Host = alias
			}
			if p, _ := strconv.Atoi(attrs["port"]); p > 0 {
				h.Port = p
			}
			h.Cmd = "ssh " + shellQuote(alias)
			h.Mode = classifySSHEntry(attrs)
			h.ForwardRemote = forwardRemoteSummary(attrs)
			h.ExpandedCommand = expandedSSHCommand(alias, attrs, directives)
			h.Preview = h.ExpandedCommand
			out = append(out, normalize(h))
		}
	}
	for _, source := range lines {
		raw := strings.TrimSpace(source.Text)
		if raw == "" {
			continue
		}
		if strings.HasPrefix(raw, "#") {
			if tags := parseSSHTags(raw); len(tags) > 0 {
				pendingTags = tags
			}
			continue
		}
		line := strings.TrimSpace(stripSSHComment(raw))
		originalKey, value, ok := splitSSHConfigDirective(line)
		if !ok {
			continue
		}
		key := strings.ToLower(originalKey)
		switch key {
		case "host":
			flush()
			current = strings.Fields(value)
			currentPath = source.Path
			currentLine = source.Number
			attrs = map[string]string{}
			directives = nil
		case "match":
			flush()
			current = nil
			currentPath = ""
			currentLine = 0
			attrs = map[string]string{}
			directives = nil
		default:
			if len(current) == 0 {
				continue
			}
			directives = append(directives, SSHDirective{Key: originalKey, Value: value})
			if isRepeatableSSHDirective(key) && attrs[key] != "" {
				attrs[key] += "\n" + value
			} else if attrs[key] == "" {
				attrs[key] = value
			}
		}
	}
	flush()
	return out
}

func expandSSHConfigLines(path string, visited map[string]bool) []sshConfigLine {
	path = expandPath(path)
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if visited[path] {
		return nil
	}
	visited[path] = true
	defer delete(visited, path)
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	var lines []sshConfigLine
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		text := scanner.Text()
		line := strings.TrimSpace(stripSSHComment(text))
		key, value, ok := splitSSHConfigDirective(line)
		if ok && strings.EqualFold(key, "Include") {
			for _, pattern := range splitSSHWords(value) {
				for _, include := range expandInclude(pattern) {
					lines = append(lines, expandSSHConfigLines(include, visited)...)
				}
			}
			continue
		}
		lines = append(lines, sshConfigLine{Path: path, Number: lineNumber, Text: text})
	}
	return lines
}

func SSHSourceHasExecutableMatch(path string) (bool, error) {
	return sshSourceHasExecutableMatch(path, map[string]bool{})
}

func sshSourceHasExecutableMatch(path string, visiting map[string]bool) (bool, error) {
	path = expandPath(path)
	abs, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	path = abs
	if visiting[path] {
		return false, nil
	}
	visiting[path] = true
	defer delete(visiting, path)
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	for {
		text, readErr := reader.ReadString('\n')
		line := strings.TrimSpace(stripSSHComment(text))
		key, value, ok := splitSSHConfigDirective(line)
		if ok && strings.EqualFold(key, "Match") {
			for _, token := range splitSSHWords(value) {
				criterion := strings.ToLower(strings.TrimLeft(token, "!"))
				if criterion == "exec" || strings.HasPrefix(criterion, "exec=") {
					return true, nil
				}
			}
		}
		if ok && strings.EqualFold(key, "Include") {
			for _, pattern := range splitSSHWords(value) {
				for _, include := range expandInclude(pattern) {
					found, includeErr := sshSourceHasExecutableMatch(include, visiting)
					if includeErr != nil {
						return false, includeErr
					}
					if found {
						return true, nil
					}
				}
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return false, nil
			}
			return false, readErr
		}
	}
}

func SSHSourceDefinesAlias(path, alias string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	wanted := strings.TrimSpace(alias)
	reader := bufio.NewReader(file)
	for {
		text, readErr := reader.ReadString('\n')
		line := strings.TrimSpace(stripSSHComment(text))
		key, value, ok := splitSSHConfigDirective(line)
		if ok && strings.EqualFold(key, "Host") {
			for _, candidate := range strings.Fields(value) {
				if candidate == wanted {
					return true
				}
			}
		}
		if readErr != nil {
			return false
		}
	}
}

func stripSSHComment(line string) string {
	var quote rune
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == '#' {
			return line[:i]
		}
	}
	return line
}

func splitSSHConfigDirective(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", "", false
	}
	end := 0
	for end < len(line) && line[end] != '=' && line[end] != ' ' && line[end] != '	' {
		end++
	}
	if end == 0 || end == len(line) {
		return "", "", false
	}
	key = line[:end]
	rest := strings.TrimLeft(line[end:], " 	")
	if strings.HasPrefix(rest, "=") {
		rest = strings.TrimLeft(rest[1:], " 	")
	}
	value = strings.TrimSpace(rest)
	if value == "" {
		return "", "", false
	}
	return key, value, true
}

func classifyHostSyntax(h Host) string {
	text := " " + strings.ToLower(strings.TrimSpace(h.Args+" "+h.Cmd)) + " "
	forward := strings.Contains(text, " -l ") || strings.Contains(text, " -r ") || strings.Contains(text, " -d ") || strings.Contains(text, " localforward ") || strings.Contains(text, " remoteforward ") || strings.Contains(text, " dynamicforward ")
	jump := strings.Contains(text, " -j ") || strings.Contains(text, " proxyjump ") || strings.Contains(text, " proxycommand ")
	command := strings.TrimSpace(h.Cmd) != "" && !looksLikePlainSSH(h.Cmd)
	return entryMode(forward, command, jump)
}

func looksLikePlainSSH(cmd string) bool {
	fields := strings.Fields(strings.TrimSpace(cmd))
	return len(fields) == 2 && fields[0] == "ssh"
}

func classifySSHEntry(attrs map[string]string) string {
	forward := hasAnySSHAttr(attrs, "localforward", "remoteforward", "dynamicforward")
	command := hasAnySSHAttr(attrs, "remotecommand")
	jump := hasAnySSHAttr(attrs, "proxyjump", "proxycommand")
	return entryMode(forward, command, jump)
}

func entryMode(forward, command, jump bool) string {
	parts := make([]string, 0, 3)
	if forward {
		parts = append(parts, "forwards")
	}
	if command {
		parts = append(parts, "commands")
	}
	if jump {
		parts = append(parts, "jumps")
	}
	if len(parts) == 0 {
		return "general"
	}
	return strings.Join(parts, " ")
}

func entryKind(mode string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(mode)))
	has := func(want string) bool {
		for _, field := range fields {
			if field == want {
				return true
			}
		}
		return false
	}
	parts := make([]string, 0, 3)
	if has("forwards") || has("forward") || has("fwd") {
		parts = append(parts, "fwd")
	}
	if has("commands") || has("command") || has("cmd") {
		parts = append(parts, "cmd")
	}
	if has("jumps") || has("jump") {
		parts = append(parts, "jump")
	}
	if len(parts) == 0 {
		return "host"
	}
	return strings.Join(parts, "/")
}

func entryKindSearch(kind string) string {
	terms := []string{kind}
	if strings.Contains(kind, "host") {
		terms = append(terms, "host hosts ssh login logins")
	}
	if strings.Contains(kind, "cmd") {
		terms = append(terms, "cmd command commands")
	}
	if strings.Contains(kind, "fwd") {
		terms = append(terms, "fwd forward forwards tunnel tunnels")
	}
	if strings.Contains(kind, "jump") {
		terms = append(terms, "jump jumps proxy bastion")
	}
	return strings.Join(terms, " ")
}

func hasAnySSHAttr(attrs map[string]string, keys ...string) bool {
	for _, key := range keys {
		if strings.TrimSpace(attrs[key]) != "" {
			return true
		}
	}
	return false
}

func expandedSSHCommand(alias string, attrs map[string]string, directives []SSHDirective) string {
	if len(attrs) == 0 {
		return ""
	}
	parts := []string{"ssh"}
	for _, directive := range directives {
		key := strings.TrimSpace(directive.Key)
		value := strings.TrimSpace(directive.Value)
		if key == "" || value == "" {
			continue
		}
		switch strings.ToLower(key) {
		case "hostname", "user":
			continue
		default:
			parts = append(parts, "-o", key+"="+value)
		}
	}

	target := strings.TrimSpace(attrs["hostname"])
	if target == "" {
		target = alias
	}
	if user := strings.TrimSpace(attrs["user"]); user != "" {
		target = user + "@" + target
	}
	parts = append(parts, target)
	return multilineShellCommand(parts)
}

func isRepeatableSSHDirective(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "identityfile", "localforward", "remoteforward", "dynamicforward":
		return true
	default:
		return false
	}
}

func multilineShellCommand(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) <= 3 {
		quoted := make([]string, len(parts))
		for i, part := range parts {
			quoted[i] = shellQuote(part)
		}
		return strings.Join(quoted, " ")
	}
	groups := []string{shellQuote(parts[0])}
	for i := 1; i < len(parts); i++ {
		part := parts[i]
		if (part == "-o" || part == "-J" || part == "-p" || part == "-i" || part == "-L" || part == "-R" || part == "-D") && i+1 < len(parts) {
			groups = append(groups, shellQuote(part)+" "+shellQuote(parts[i+1]))
			i++
			continue
		}
		groups = append(groups, shellQuote(part))
	}
	lines := []string{groups[0] + " \\"}
	for i := 1; i < len(groups); i++ {
		line := "  " + groups[i]
		if i < len(groups)-1 {
			line += " \\"
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func parseSSHTags(line string) []string {
	line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
	lower := strings.ToLower(line)
	if !strings.HasPrefix(lower, "tags:") && !strings.HasPrefix(lower, "tag:") {
		return nil
	}
	_, value, _ := strings.Cut(line, ":")
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	var tags []string
	for _, tag := range parts {
		if tag = strings.Trim(strings.TrimSpace(tag), "#"); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func splitSSHWords(value string) []string {
	var words []string
	var current strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}
	for _, r := range value {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '	' {
			flush()
			continue
		}
		current.WriteRune(r)
	}
	if escaped {
		current.WriteRune('\\')
	}
	flush()
	return words
}

func expandInclude(pattern string) []string {
	pattern = expandPath(pattern)
	if !filepath.IsAbs(pattern) {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		pattern = filepath.Join(home, ".ssh", pattern)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil
	}
	sort.Strings(matches)
	return matches
}

func expandPath(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	if strings.HasPrefix(path, "~") {
		nameAndRest := strings.TrimPrefix(path, "~")
		name, rest, found := strings.Cut(nameAndRest, "/")
		if name != "" {
			if account, err := user.Lookup(name); err == nil {
				if !found || rest == "" {
					return account.HomeDir
				}
				return filepath.Join(account.HomeDir, rest)
			}
		}
	}
	return path
}
