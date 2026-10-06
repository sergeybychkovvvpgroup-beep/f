package hosts

import (
	"os"
	"os/exec"
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
	if got := entries[0].Command; got != "ssh -p 2222 operator@192.0.2.10" {
		t.Fatalf("display command = %q, want full composed command", got)
	}
}

func TestImportedSSHEntryDisplaysExpandedCommandButExecutesAlias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.conf")
	raw := "Host gateway\n  HostName 192.0.2.10\n  User operator\n  Port 2222\n  ProxyJump jump.example\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	hosts := loadSSHConfigFile(path, map[string]bool{})
	entries := ToEntries(hosts)
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if got := strings.Join(strings.Fields(entries[0].Command), " "); got != "ssh -o Port=2222 -o ProxyJump=jump.example operator@192.0.2.10" {
		t.Fatalf("display command = %q", got)
	}
	if got := entries[0].QuickAction().Cmd; got != "ssh gateway" {
		t.Fatalf("execution command = %q, want alias-preserving ssh gateway", got)
	}
}

func TestDisplayCommandPreservesQuotedWhitespace(t *testing.T) {
	host := Host{ExpandedCommand: "ssh -o 'RemoteCommand=printf a  b' operator@192.0.2.10"}
	got := displayCommand(host, "")
	if got != host.ExpandedCommand {
		t.Fatalf("display command changed quoted whitespace: %q", got)
	}
}

func TestCustomPreviewDoesNotReplaceDisplayCommand(t *testing.T) {
	host := Host{Name: "gateway", Host: "192.0.2.10", User: "operator", Preview: "connect to the edge router"}
	entry := ToEntries([]Host{host})[0]
	if entry.Command != "ssh operator@192.0.2.10" {
		t.Fatalf("display command = %q, want composed SSH command", entry.Command)
	}
	if entry.QuickAction().Banner != host.Preview {
		t.Fatalf("preview banner was lost: %q", entry.QuickAction().Banner)
	}
}

func TestExpandedSSHCommandKeepsRepeatedDirectivesAndProxyCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.conf")
	raw := "Host gateway\n  HostName 192.0.2.10\n  User operator\n  IdentityFile ~/.ssh/first\n  IdentityFile ~/.ssh/second\n  LocalForward 8443 service:443\n  LocalForward 8080 service:80\n  ProxyCommand ssh jump -W %h:%p\n  ForwardAgent no\n  ServerAliveInterval 30\n  Compression yes\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := ToEntries(loadSSHConfigFile(path, map[string]bool{}))[0]
	command := entry.Command
	for _, want := range []string{"-o 'IdentityFile=~/.ssh/first'", "-o 'IdentityFile=~/.ssh/second'", "-o 'LocalForward=8443 service:443'", "-o 'LocalForward=8080 service:80'", "-o 'ProxyCommand=ssh jump -W %h:%p'", "-o ForwardAgent=no", "-o ServerAliveInterval=30", "-o Compression=yes"} {
		if !strings.Contains(command, want) {
			t.Fatalf("display command %q is missing %q", command, want)
		}
	}
}

func TestExpandedSSHCommandPreservesExplicitDefaultsAndNone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	raw := "Host gateway\n  HostName 192.0.2.10\n  Port 22\n  ProxyJump none\n  IdentityFile none\n  LocalForward none\n  RemoteForward none\n  DynamicForward none\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	command := ToEntries(loadSSHConfigFile(path, map[string]bool{}))[0].Command
	for _, want := range []string{"-o Port=22", "-o ProxyJump=none", "-o IdentityFile=none", "-o LocalForward=none", "-o RemoteForward=none", "-o DynamicForward=none"} {
		if !strings.Contains(command, want) {
			t.Fatalf("display command %q is missing explicit directive %q", command, want)
		}
	}
}

func TestExpandedSSHCommandIsAcceptedByOpenSSH(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not installed")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	raw := "Host gateway\n  HostName 192.0.2.10\n  IdentityFile none\n  LocalForward 8443 service:443\n  RemoteForward 2222 localhost:22\n  DynamicForward 1080\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	command := ToEntries(loadSSHConfigFile(path, map[string]bool{}))[0].Command
	probe := strings.Replace(command, "ssh ", "ssh -G ", 1) + " >/dev/null"
	if output, err := exec.Command("sh", "-c", probe).CombinedOutput(); err != nil {
		t.Fatalf("OpenSSH rejected display command %q: %v\n%s", command, err, output)
	}
}

func TestEqualsSyntaxIsParsed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	raw := "Host = gateway\nHostName = 192.0.2.10\nUser=operator\nLocalForward=8443 service:443\nProxyCommand sh -c \"echo #literal\"\nRemoteCommand sh -c echo\\#literal\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	entries := ToEntries(loadSSHConfigFile(path, map[string]bool{}))
	if len(entries) != 1 || !strings.Contains(entries[0].Command, "-o 'LocalForward=8443 service:443'") || !strings.Contains(entries[0].Command, "#literal") || !strings.HasSuffix(entries[0].Command, "operator@192.0.2.10") {
		t.Fatalf("equals/hash syntax was not preserved: %#v", entries)
	}
	if !SSHSourceDefinesAlias(path, "gateway") {
		t.Fatal("Host = gateway was not recognized by edit validation")
	}
}

func TestMatchBlockDoesNotLeakIntoPreviousHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	raw := "Host gateway\n  HostName 192.0.2.10\nMatch user operator\n  ForwardAgent yes\nHost second\n  HostName 192.0.2.11\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	entries := ToEntries(loadSSHConfigFile(path, map[string]bool{}))
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if strings.Contains(entries[0].Command, "Match") || strings.Contains(entries[0].Command, "ForwardAgent") {
		t.Fatalf("Match directives leaked into gateway: %q", entries[0].Command)
	}
}

func TestIncludeInsideHostIsExpandedInPlace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(filepath.Join(sshDir, "config.d"), 0o700); err != nil {
		t.Fatal(err)
	}
	included := filepath.Join(sshDir, "options file")
	if err := os.WriteFile(included, []byte("ProxyJump jump.example\nServerAliveInterval 20\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sshDir, "config.d", "config")
	raw := "Host gateway\n  HostName 192.0.2.10\n  Include \"options file\"\n  User operator\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := ToEntries(loadSSHConfigFile(path, map[string]bool{}))[0]
	for _, want := range []string{"-o ProxyJump=jump.example", "-o ServerAliveInterval=20", "operator@192.0.2.10"} {
		if !strings.Contains(entry.Command, want) {
			t.Fatalf("expanded include command %q is missing %q", entry.Command, want)
		}
	}
}

func TestSameIncludeExpandsInsideEveryHost(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(filepath.Join(sshDir, "config.d"), 0o700); err != nil {
		t.Fatal(err)
	}
	included := filepath.Join(sshDir, "options")
	if err := os.WriteFile(included, []byte("ForwardAgent yes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sshDir, "config.d", "config")
	raw := "Host first\n  HostName 192.0.2.10\n  Include options\nHost second\n  HostName 192.0.2.11\n  Include options\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	entries := ToEntries(loadSSHConfigFile(path, map[string]bool{}))
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	for _, entry := range entries {
		if !strings.Contains(entry.Command, "-o ForwardAgent=yes") {
			t.Fatalf("repeated include missing from %s: %q", entry.Desc, entry.Command)
		}
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
	path := filepath.Join(root, "config")
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
	if !entries[0].Editable {
		t.Fatal("extensionless OpenSSH source must remain editable")
	}
}
