package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveNotesDirUsesConfig(t *testing.T) {
	configHome := t.TempDir()
	notesDir := filepath.Join(t.TempDir(), "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("F_NOTES_DIR", "")
	t.Setenv("TERM_NOTES_DIR", "")

	if _, err := SetNotesDir(notesDir); err != nil {
		t.Fatal(err)
	}

	got, source, err := ResolveNotesDir("")
	if err != nil {
		t.Fatal(err)
	}

	if got != notesDir {
		t.Fatalf("expected %s, got %s", notesDir, got)
	}
	if source != "config" {
		t.Fatalf("expected source=config, got %s", source)
	}
}

func TestResolveNotesDirReturnsSetupErrorWithoutSources(t *testing.T) {
	configHome := t.TempDir()
	workdir := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("F_NOTES_DIR", "")
	t.Setenv("TERM_NOTES_DIR", "")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if err := os.Chdir(workdir); err != nil {
		t.Fatal(err)
	}

	_, _, err = ResolveNotesDir("")
	if err == nil {
		t.Fatal("expected setup error")
	}

	if _, ok := err.(SetupRequiredError); !ok {
		t.Fatalf("expected SetupRequiredError, got %T", err)
	}
}

func TestLoadCreatesCommentedConfigWithDefaults(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PickerHeight != 14 {
		t.Fatalf("expected default picker height 14, got %d", cfg.PickerHeight)
	}
	if cfg.Theme != "fzf-dark" {
		t.Fatalf("expected default theme fzf-dark, got %q", cfg.Theme)
	}
	if cfg.Layout != "bottom" {
		t.Fatalf("expected default layout bottom, got %q", cfg.Layout)
	}
	if !cfg.FullScreen {
		t.Fatal("expected full screen default to be true")
	}
	if cfg.ShowMatchContext {
		t.Fatal("expected show match context default to be false")
	}
	if cfg.FocusMode {
		t.Fatal("expected focus mode default to be false")
	}
	if cfg.ShowListOnStart {
		t.Fatal("expected show list on start default to be false")
	}
	if !cfg.TwoLineResults {
		t.Fatal("expected two line results default to be true")
	}

	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "# themes: fzf-dark, catppuccin-mocha, catppuccin-latte, dracula, nord, solarized-dark, solarized-light") {
		t.Fatalf("expected compact theme list in config, got %q", text)
	}
	if !strings.Contains(text, "theme: fzf-dark") {
		t.Fatalf("expected default theme in config, got %q", text)
	}
	if !strings.Contains(text, "layout: bottom") {
		t.Fatalf("expected default layout in config, got %q", text)
	}
	if strings.Contains(text, "search_mode:") {
		t.Fatalf("expected config without search_mode, got %q", text)
	}
	if !strings.Contains(text, "full_screen: true") {
		t.Fatalf("expected full screen in config, got %q", text)
	}
	if !strings.Contains(text, "focus_mode: false") {
		t.Fatalf("expected focus_mode in config, got %q", text)
	}
	if !strings.Contains(text, "show_match_context: false") {
		t.Fatalf("expected show_match_context in config, got %q", text)
	}
	if !strings.Contains(text, "show_list_on_start: false") {
		t.Fatalf("expected show_list_on_start in config, got %q", text)
	}
	if !strings.Contains(text, "two_line_results: true") {
		t.Fatalf("expected two_line_results in config, got %q", text)
	}
}

func TestLoadDoesNotReintroduceSearchModeIntoConfigFile(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	raw := strings.Join([]string{
		"notes_dir: \"/tmp/notes\"",
		"theme: fzf-dark",
		"layout: bottom",
		"full_screen: true",
		"picker_height: 14",
		"focus_mode: true",
		"show_match_context: false",
		"show_list_on_start: true",
		"two_line_results: false",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Layout != "bottom" {
		t.Fatalf("expected bottom layout, got %q", cfg.Layout)
	}
	if !cfg.FocusMode {
		t.Fatal("expected focus_mode to be loaded from config")
	}
	if !cfg.ShowListOnStart {
		t.Fatal("expected show_list_on_start to be loaded from config")
	}
	if cfg.TwoLineResults {
		t.Fatal("expected two_line_results to be loaded from config")
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(updated), "search_mode:") {
		t.Fatalf("expected config file without search_mode, got %q", string(updated))
	}
	if !strings.Contains(string(updated), "show_match_context: false") {
		t.Fatalf("expected rewritten config to contain show_match_context, got %q", string(updated))
	}
	if !strings.Contains(string(updated), "focus_mode: true") {
		t.Fatalf("expected rewritten config to contain focus_mode, got %q", string(updated))
	}
	if !strings.Contains(string(updated), "show_list_on_start: true") {
		t.Fatalf("expected rewritten config to contain show_list_on_start, got %q", string(updated))
	}
	if !strings.Contains(string(updated), "two_line_results: false") {
		t.Fatalf("expected rewritten config to contain two_line_results, got %q", string(updated))
	}
}

func TestLoadAcceptsLegacyPreviewKeys(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	raw := strings.Join([]string{
		"show_preview: true",
		"show_notes_on_start: true",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ShowMatchContext {
		t.Fatal("expected legacy show_preview to map to show_match_context")
	}
	if !cfg.ShowListOnStart {
		t.Fatal("expected legacy show_notes_on_start to map to show_list_on_start")
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	if strings.Contains(text, "show_preview:") {
		t.Fatalf("expected legacy show_preview key to be removed, got %q", text)
	}
	if strings.Contains(text, "show_notes_on_start:") {
		t.Fatalf("expected legacy show_notes_on_start key to be removed, got %q", text)
	}
	if !strings.Contains(text, "show_match_context: true") {
		t.Fatalf("expected migrated show_match_context key, got %q", text)
	}
	if !strings.Contains(text, "show_list_on_start: true") {
		t.Fatalf("expected migrated show_list_on_start key, got %q", text)
	}
}

func TestConfigPathUsesFDirectory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, filepath.Join("f", "config.yaml")) {
		t.Fatalf("config path = %q", path)
	}
}

func TestLoadMigratesUnversionedFConfigAndAddsSafetyDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, _ := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("notes_dir: /tmp/notes\ntheme: nord\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SchemaVersion != CurrentSchemaVersion || !cfg.ConfirmRun {
		t.Fatalf("unexpected migrated config: %#v", cfg)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "schema_version: 1") || !strings.Contains(string(raw), "confirm_run: true") {
		t.Fatalf("migration was not persisted: %s", raw)
	}
}

func TestLoadRejectsUnknownKeyWithoutRewriting(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, _ := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("schema_version: 1\nhosts: []\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "unknown config key") {
		t.Fatalf("expected unknown-key error, got %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(original) {
		t.Fatal("invalid config was rewritten")
	}
}

func TestLegacyAooConfigRequiresExplicitMigration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	legacy := filepath.Join(home, "aoo", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("notes_dir: /tmp/notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "f migrate") {
		t.Fatalf("expected migration instruction, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, "f", "config.yaml")); !os.IsNotExist(statErr) {
		t.Fatal("legacy config was migrated implicitly")
	}
}

func TestLoadRejectsUnsupportedSchemaVersionWithoutRewriting(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, _ := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("schema_version: 99\nnotes_dir: /tmp/notes\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "unsupported schema_version") {
		t.Fatalf("expected unsupported schema error, got %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(original) {
		t.Fatal("unsupported config was rewritten")
	}
}
