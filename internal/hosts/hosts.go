package hosts

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"aoo/internal/notes"
	"gopkg.in/yaml.v3"
)

const envHostsFile = "AOO_HOSTS_FILE"

type Host struct {
	Name string   `yaml:"name"`
	Host string   `yaml:"host"`
	User string   `yaml:"user,omitempty"`
	Port int      `yaml:"port,omitempty"`
	Tags []string `yaml:"tags,omitempty"`
	Args string   `yaml:"args,omitempty"`
	Cmd  string   `yaml:"cmd,omitempty"`
	Desc string   `yaml:"desc,omitempty"`
}

type File struct {
	Hosts []Host `yaml:"hosts"`
}

func Path() (string, error) {
	if value := strings.TrimSpace(os.Getenv(envHostsFile)); value != "" {
		return filepath.Abs(value)
	}
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hosts.yaml"), nil
}

func ConfigDir() (string, error) {
	if value := strings.TrimSpace(os.Getenv(envHostsFile)); value != "" {
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
	return filepath.Join(dir, "aoo", "config.d"), nil
}

func Load() ([]Host, error) {
	var out []Host
	for _, path := range hostFiles() {
		if raw, err := os.ReadFile(path); err == nil {
			parsed, err := parse(raw)
			if err != nil {
				return nil, fmt.Errorf("parse %s: %w", path, err)
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
	if value := strings.TrimSpace(os.Getenv(envHostsFile)); value != "" {
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
		label := h.Name
		if label == "" {
			label = h.Host
		}
		detail := hostDetail(h)
		searchParts := []string{h.Name, h.Host, h.User, h.Desc, cmd}
		searchParts = append(searchParts, h.Tags...)
		entries = append(entries, notes.Entry{
			Desc:    label,
			Note:    strings.Join(searchParts, " "),
			Actions: []notes.Action{{Desc: detail, Cmd: cmd}},
		})
	}
	return entries
}

func hostDetail(h Host) string {
	parts := []string{}
	target := h.Host
	if h.User != "" && target != "" {
		target = h.User + "@" + target
	}
	if target != "" {
		if h.Port > 0 {
			target += ":" + strconv.Itoa(h.Port)
		}
		parts = append(parts, target)
	}
	if len(h.Tags) > 0 {
		parts = append(parts, "#"+strings.Join(h.Tags, " #"))
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

func loadSSHConfigFile(path string, visited map[string]bool) []Host {
	path = expandPath(path)
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	if visited[path] {
		return nil
	}
	visited[path] = true

	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	home, _ := os.UserHomeDir()

	var out []Host
	var current []string
	attrs := map[string]string{}
	var pendingTags []string
	flush := func() {
		for _, alias := range current {
			if alias == "" || strings.ContainsAny(alias, "*?") {
				continue
			}
			h := Host{Name: alias, Host: attrs["hostname"], User: attrs["user"], Tags: append([]string{}, pendingTags...)}
			if h.Host == "" {
				h.Host = alias
			}
			if p, _ := strconv.Atoi(attrs["port"]); p > 0 {
				h.Port = p
			}
			// Preserve the exact OpenSSH config semantics, including ProxyJump,
			// LocalForward, RemoteCommand and options unknown to aoo.
			h.Cmd = "ssh " + shellQuote(alias)
			h.Desc = strings.TrimPrefix(path, home)
			out = append(out, normalize(h))
		}
	}
	s := bufio.NewScanner(file)
	for s.Scan() {
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			continue
		}
		if strings.HasPrefix(raw, "#") {
			if tags := parseSSHTags(raw); len(tags) > 0 {
				pendingTags = tags
			}
			continue
		}
		line := strings.TrimSpace(strings.SplitN(raw, "#", 2)[0])
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		value := strings.Join(fields[1:], " ")
		switch key {
		case "include":
			for _, pattern := range fields[1:] {
				for _, include := range expandInclude(pattern, filepath.Dir(path)) {
					out = append(out, loadSSHConfigFile(include, visited)...)
				}
			}
		case "host":
			flush()
			current = fields[1:]
			attrs = map[string]string{}
		case "hostname", "user", "port":
			attrs[key] = value
		}
	}
	flush()
	return out
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

func expandInclude(pattern, baseDir string) []string {
	pattern = expandPath(pattern)
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(baseDir, pattern)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil
	}
	sort.Strings(matches)
	return matches
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
