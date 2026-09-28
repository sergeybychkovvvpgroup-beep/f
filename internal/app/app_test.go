package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aoo/internal/config"
	"aoo/internal/notes"
	"aoo/internal/ui"
)

func TestPromptCommandRunPrintsCommandWithoutConfirmation(t *testing.T) {
	var stdout bytes.Buffer

	if err := promptCommandRun("List files", "ls -la", &stdout); err != nil {
		t.Fatalf("promptCommandRun returned error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "[run] List files") {
		t.Fatalf("expected run header in output, got %q", output)
	}
	if !strings.Contains(output, "[command]\nls -la") {
		t.Fatalf("expected command to be printed, got %q", output)
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
	if got := strings.TrimSpace(stdout.String()); got != "f 0.4.1" {
		t.Fatalf("version = %q, want %q", got, "f 0.4.1")
	}
}

func TestConfigUISelectsLightMode(t *testing.T) {
	t.Setenv("AOO_CONFIG_FILE", filepath.Join(t.TempDir(), "config.yaml"))
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"config", "ui", "light"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UIMode != "light" {
		t.Fatalf("ui mode = %q, want light", cfg.UIMode)
	}
	if !strings.Contains(stdout.String(), "ui mode: light") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestLightModeUsesCompactNonFullscreenPicker(t *testing.T) {
	cfg := config.DefaultFile()
	cfg.UIMode = "light"
	cfg.FullScreen = true
	cfg.PickerHeight = 11
	options := pickerOptions(cfg, ui.SyncStatus{})
	if options.FullScreen {
		t.Fatal("light mode must not use the full-screen alternate buffer")
	}
	if options.Height != 11 {
		t.Fatalf("light mode height = %d, want 11", options.Height)
	}
}
