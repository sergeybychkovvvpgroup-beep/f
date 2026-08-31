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
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "aoo", "hosts.yaml"), nil
}

func Load() ([]Host, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	var out []Host
	if raw, err := os.ReadFile(path); err == nil {
		parsed, err := parse(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		out = append(out, parsed...)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	out = merge(out, loadSSHConfig())
	if len(out) == 0 {
		return nil, fmt.Errorf("no hosts found; add one with: f add NAME [user@]HOST [-p PORT] [--tag TAG]")
	}
	return out, nil
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
		descParts := []string{h.Name, h.Host}
		if h.User != "" {
			descParts = append(descParts, h.User)
		}
		descParts = append(descParts, h.Tags...)
		if h.Cmd != "" {
			descParts = append(descParts, h.Cmd)
		}
		if h.Desc != "" {
			descParts = append(descParts, "—", h.Desc)
		}
		entries = append(entries, notes.Entry{
			Desc:    strings.Join(descParts, " "),
			Actions: []notes.Action{{Desc: "ssh", Cmd: cmd}},
		})
	}
	return entries
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
	file, err := os.Open(filepath.Join(home, ".ssh", "config"))
	if err != nil {
		return nil
	}
	defer file.Close()
	var out []Host
	var current []string
	attrs := map[string]string{}
	flush := func() {
		for _, alias := range current {
			if alias == "" || strings.ContainsAny(alias, "*?") {
				continue
			}
			h := Host{Name: alias, Host: attrs["hostname"], User: attrs["user"]}
			if h.Host == "" {
				h.Host = alias
			}
			if p, _ := strconv.Atoi(attrs["port"]); p > 0 {
				h.Port = p
			}
			h.Desc = "~/.ssh/config"
			out = append(out, normalize(h))
		}
	}
	s := bufio.NewScanner(file)
	for s.Scan() {
		line := strings.TrimSpace(strings.SplitN(s.Text(), "#", 2)[0])
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		value := strings.Join(fields[1:], " ")
		if key == "host" {
			flush()
			current = fields[1:]
			attrs = map[string]string{}
			continue
		}
		attrs[key] = value
	}
	flush()
	return out
}
