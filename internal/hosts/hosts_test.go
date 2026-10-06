package hosts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDirUsesFDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("F_HOSTS_FILE", "")
	t.Setenv("AOO_HOSTS_FILE", "")
	dir, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "f", "config.d")
	if dir != want {
		t.Fatalf("config dir = %q, want %q", dir, want)
	}
}

func TestClassifySSHEntryKinds(t *testing.T) {
	tests := []struct {
		name  string
		attrs map[string]string
		want  string
	}{
		{name: "host", attrs: map[string]string{"hostname": "server"}, want: "host"},
		{name: "command", attrs: map[string]string{"remotecommand": "show version"}, want: "cmd"},
		{name: "jump", attrs: map[string]string{"proxyjump": "bastion"}, want: "jump"},
		{name: "proxy command", attrs: map[string]string{"proxycommand": "ssh gateway -W %h:%p"}, want: "jump"},
		{name: "local forward", attrs: map[string]string{"localforward": "8443 service:443"}, want: "fwd"},
		{name: "remote forward", attrs: map[string]string{"remoteforward": "2222 localhost:22"}, want: "fwd"},
		{name: "dynamic forward", attrs: map[string]string{"dynamicforward": "1080"}, want: "fwd"},
		{name: "command through jump", attrs: map[string]string{"remotecommand": "show version", "proxyjump": "bastion"}, want: "cmd/jump"},
		{name: "forward through jump", attrs: map[string]string{"localforward": "8443 service:443", "proxyjump": "bastion"}, want: "fwd/jump"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := entryKind(classifySSHEntry(tt.attrs)); got != tt.want {
				t.Fatalf("entry kind = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassifyLegacyHostSyntaxKinds(t *testing.T) {
	tests := []struct {
		name string
		host Host
		want string
	}{
		{name: "host", host: Host{Cmd: "ssh server"}, want: "host"},
		{name: "command", host: Host{Cmd: "docker ps"}, want: "cmd"},
		{name: "jump", host: Host{Args: "-J bastion"}, want: "jump"},
		{name: "local forward", host: Host{Args: "-L 8443:service:443"}, want: "fwd"},
		{name: "remote forward", host: Host{Args: "-R 2222:localhost:22"}, want: "fwd"},
		{name: "dynamic forward", host: Host{Args: "-D 1080"}, want: "fwd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := entryKind(classifyHostSyntax(tt.host)); got != tt.want {
				t.Fatalf("entry kind = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToEntriesKeepsSSHAddressSeparateFromDescription(t *testing.T) {
	host := Host{Name: "gateway", Host: "192.0.2.10", User: "operator", Port: 2222, Desc: "edge router"}
	entries := ToEntries([]Host{host})
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if got := entries[0].Address; got != "operator@192.0.2.10:2222" {
		t.Fatalf("address = %q, want operator@192.0.2.10:2222", got)
	}
}

func TestToEntriesAddsSearchableKindWithoutChangingCommand(t *testing.T) {
	host := Host{Name: "router-status", Host: "router", Cmd: "ssh router show version", Mode: "commands jumps"}
	entries := ToEntries([]Host{host})
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.Kind != "cmd/jump" {
		t.Fatalf("kind = %q, want cmd/jump", entry.Kind)
	}
	if !strings.Contains(entry.Note, "cmd") || !strings.Contains(entry.Note, "command") || !strings.Contains(entry.Note, "jump") {
		t.Fatalf("kind search aliases missing from note: %q", entry.Note)
	}
	if got := entry.QuickAction().Cmd; got != host.Cmd {
		t.Fatalf("command = %q, want unchanged %q", got, host.Cmd)
	}
}

func TestSSHConfigEntryKeepsSourceLocationInPickerEntry(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "f.conf")
	content := "# managed hosts\n\nHost gateway\n  HostName 192.0.2.10\n  User operator\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	hosts := loadSSHConfigFile(path, map[string]bool{})
	entries := ToEntries(hosts)
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].SourcePath != path || entries[0].SourceLine != 3 {
		t.Fatalf("source = %q:%d, want %q:3", entries[0].SourcePath, entries[0].SourceLine, path)
	}
}
