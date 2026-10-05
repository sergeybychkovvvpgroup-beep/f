package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"f/internal/config"
	"f/internal/notes"
	"f/internal/ui"
)

func TestPromptCommandRunPrintsCommandWithoutConfirmation(t *testing.T) {
	var stdout bytes.Buffer
	if err := promptCommandRun("List files", "ls -la", &stdout); err != nil {
		t.Fatalf("promptCommandRun returned error: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "[run] List files") || !strings.Contains(output, "[command]\nls -la") {
		t.Fatalf("unexpected output %q", output)
	}
	if strings.Contains(output, "run?") {
		t.Fatalf("expected no confirmation prompt, got %q", output)
	}
}

func TestRunHeaderSkipsDuplicateActionDescription(t *testing.T) {
	entry := notes.Entry{Desc: "Vyos-DHCP Chashnikovo показать subnets"}
	action := &notes.Action{Desc: "Vyos-DHCP Chashnikovo показать subnets"}
	if got := runHeader(entry, action); got != entry.DisplayName() {
		t.Fatalf("expected deduplicated run header, got %q", got)
	}
}

func TestRunHeaderKeepsDistinctActionDescription(t *testing.T) {
	entry := notes.Entry{Desc: "Vyos-DHCP Chashnikovo"}
	action := &notes.Action{Desc: "показать subnets"}
	if got := runHeader(entry, action); got != "Vyos-DHCP Chashnikovo :: показать subnets" {
		t.Fatalf("unexpected run header: %q", got)
	}
}

func TestVersionReportsCurrentRelease(t *testing.T) {
	oldArg0 := os.Args[0]
	os.Args[0] = "f"
	t.Cleanup(func() { os.Args[0] = oldArg0 })
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"version"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "f 0.8.0" {
		t.Fatalf("version = %q, want %q", got, "f 0.8.0")
	}
}

func TestConfigUISelectsCompactMode(t *testing.T) {
	t.Setenv("AOO_CONFIG_FILE", filepath.Join(t.TempDir(), "config.yaml"))
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"config", "ui", "compact"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UIMode != "compact" {
		t.Fatalf("ui mode = %q, want compact", cfg.UIMode)
	}
	if !strings.Contains(stdout.String(), "ui mode: compact") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestCompactModeUsesNonFullscreenPicker(t *testing.T) {
	cfg := config.DefaultFile()
	cfg.UIMode = "compact"
	cfg.PickerHeight = 11
	options := pickerOptions(cfg, ui.SyncStatus{})
	if options.FullScreen {
		t.Fatal("compact mode must not use the full-screen alternate buffer")
	}
	if options.Height != 11 {
		t.Fatalf("compact mode height = %d, want 11", options.Height)
	}
}

func TestFullScreenModeUsesAlternateScreen(t *testing.T) {
	cfg := config.DefaultFile()
	cfg.UIMode = "full-screen"
	options := pickerOptions(cfg, ui.SyncStatus{})
	if !options.FullScreen {
		t.Fatal("full-screen mode must use the alternate screen")
	}
	if options.Height != 0 {
		t.Fatalf("full-screen height = %d, want 0", options.Height)
	}
}

func TestConfigAddressEnablesMutedAddressRows(t *testing.T) {
	t.Setenv("AOO_CONFIG_FILE", filepath.Join(t.TempDir(), "config.yaml"))
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"config", "address", "on"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ShowAddress {
		t.Fatal("show_address is disabled after 'f config address on'")
	}
	if !pickerOptions(cfg, ui.SyncStatus{}).ShowAddress {
		t.Fatal("picker options did not receive show_address")
	}
	if !strings.Contains(stdout.String(), "address mode: on") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestConfigLayoutSelectsBottom(t *testing.T) {
	t.Setenv("AOO_CONFIG_FILE", filepath.Join(t.TempDir(), "config.yaml"))
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"config", "layout", "bottom"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Layout != "bottom" {
		t.Fatalf("layout = %q, want bottom", cfg.Layout)
	}
}

func TestUsageUsesFBrand(t *testing.T) {
	oldArg0 := os.Args[0]
	os.Args[0] = "f"
	t.Cleanup(func() { os.Args[0] = oldArg0 })
	var out bytes.Buffer
	printUsage(&out)
	text := out.String()
	if strings.Contains(strings.ToLower(text), "aoo uses") || strings.Contains(text, "f / aoo") {
		t.Fatalf("usage still advertises the old product name:\n%s", text)
	}
}

func TestEditableSSHBlockUsesFBrand(t *testing.T) {
	block, err := editableSSHBlock("test-alias")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(block, "f uses its dedicated OpenSSH") {
		t.Fatalf("editable block does not use f branding:\n%s", block)
	}
	if strings.Contains(block, "# aoo uses") {
		t.Fatalf("editable block still advertises old product name:\n%s", block)
	}
}

func TestSSHConfigPathsUseFNames(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir, err := sshConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".ssh", "config.d", "f_hosts"); dir != want {
		t.Fatalf("SSH config dir = %q, want %q", dir, want)
	}
	path, err := userSSHConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "f.conf"); path != want {
		t.Fatalf("SSH config path = %q, want %q", path, want)
	}
}

func TestUpsertReplacesLegacyEditMarkers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.conf")
	legacy := "# aoo-edit begin server\nHost server\n  HostName old\n# aoo-edit end server\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := upsertMarkedBlock(path, "server", "Host server\n  HostName new\n"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "aoo-edit") || strings.Count(text, "Host server") != 1 || !strings.Contains(text, "# f-edit begin server") {
		t.Fatalf("legacy markers were not migrated cleanly:\n%s", text)
	}
}
