package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultShowsSSHListOnStart(t *testing.T) {
	if !DefaultFile().ShowListOnStart {
		t.Fatal("default picker must show the SSH list immediately")
	}
}

func TestDefaultUIModeIsCompact(t *testing.T) {
	if got := DefaultFile().UIMode; got != "compact" {
		t.Fatalf("default ui mode = %q, want compact", got)
	}
}

func TestLoadMigratesLightUIModeToCompact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("AOO_CONFIG_FILE", path)
	if err := os.WriteFile(path, []byte("ui_mode: light\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UIMode != "compact" {
		t.Fatalf("ui mode = %q, want compact", cfg.UIMode)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "ui_mode: compact") {
		t.Fatalf("rewritten config does not migrate light mode:\n%s", raw)
	}
}

func TestLoadMigratesFullUIModeToFullScreen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("AOO_CONFIG_FILE", path)
	if err := os.WriteFile(path, []byte("ui_mode: full\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UIMode != "full-screen" {
		t.Fatalf("ui mode = %q, want full-screen", cfg.UIMode)
	}
}

func TestSetUIModeRejectsUnknownMode(t *testing.T) {
	t.Setenv("AOO_CONFIG_FILE", filepath.Join(t.TempDir(), "config.yaml"))
	if _, err := SetUIMode("giant"); err == nil {
		t.Fatal("unknown UI mode must be rejected")
	}
}

func TestSetLayoutAcceptsTopAndBottom(t *testing.T) {
	t.Setenv("AOO_CONFIG_FILE", filepath.Join(t.TempDir(), "config.yaml"))
	for _, want := range []string{"top", "bottom"} {
		got, err := SetLayout(want)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("layout = %q, want %q", got, want)
		}
	}
}
