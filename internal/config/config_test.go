package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultShowsSSHListOnStart(t *testing.T) {
	if !DefaultFile().ShowListOnStart {
		t.Fatal("default picker must show the SSH list so tabs and preview are visible immediately")
	}
}

func TestDefaultUIModeIsFull(t *testing.T) {
	if got := DefaultFile().UIMode; got != "full" {
		t.Fatalf("default ui mode = %q, want full", got)
	}
}

func TestLoadAcceptsLightUIMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("AOO_CONFIG_FILE", path)
	if err := os.WriteFile(path, []byte("ui_mode: light\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UIMode != "light" {
		t.Fatalf("ui mode = %q, want light", cfg.UIMode)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "ui_mode: light") {
		t.Fatalf("rewritten config does not preserve light mode:\n%s", raw)
	}
}
