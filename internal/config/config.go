package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	envNotesDir       = "F_NOTES_DIR"
	legacyAooNotesDir = "AOO_NOTES_DIR"
	legacyEnvNotesDir = "TERM_NOTES_DIR"
)

type File struct {
	NotesDir     string `yaml:"notes_dir"`
	NotesRepo    string `yaml:"notes_repo"`
	UIMode       string `yaml:"ui_mode"`
	Layout       string `yaml:"layout"`
	PickerHeight int    `yaml:"picker_height"`
}

type rawFile struct {
	NotesDir     string `yaml:"notes_dir"`
	NotesRepo    string `yaml:"notes_repo"`
	UIMode       string `yaml:"ui_mode"`
	Layout       string `yaml:"layout"`
	PickerHeight *int   `yaml:"picker_height"`
}

type SetupRequiredError struct{}

func (e SetupRequiredError) Error() string {
	return strings.TrimSpace(`
notes directory is not configured

Initial setup:
  edit ~/.config/f/config.yaml
  and set notes_dir

Temporary override:
  F_NOTES_DIR=/path/to/notes f

Check current config:
  f config show
`)
}

func ConfigPath() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("F_CONFIG_FILE")); custom != "" {
		return filepath.Abs(custom)
	}
	if custom := strings.TrimSpace(os.Getenv("AOO_CONFIG_FILE")); custom != "" {
		return filepath.Abs(custom)
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve user config dir: %w", err)
	}

	return filepath.Join(dir, "f", "config.yaml"), nil
}

func legacyConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "aoo", "config.yaml"), nil
}

func DefaultFile() File {
	return File{
		UIMode:       "compact",
		Layout:       "bottom",
		PickerHeight: 14,
	}
}

func Load() (File, error) {
	path, err := ConfigPath()
	if err != nil {
		return File{}, err
	}

	raw, err := os.ReadFile(path)
	migratedLegacy := false
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return File{}, fmt.Errorf("read config %s: %w", path, err)
		}
		if legacyPath, legacyErr := legacyConfigPath(); legacyErr == nil && legacyPath != path {
			if legacyRaw, readErr := os.ReadFile(legacyPath); readErr == nil {
				raw = legacyRaw
				migratedLegacy = true
				err = nil
			}
		}
		if err != nil {
			cfg := DefaultFile()
			if saveErr := Save(cfg); saveErr != nil {
				return File{}, saveErr
			}
			return cfg, nil
		}
	}

	cfg := DefaultFile()
	var parsed rawFile
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return File{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	cfg.NotesDir = strings.TrimSpace(parsed.NotesDir)
	cfg.NotesRepo = strings.TrimSpace(parsed.NotesRepo)
	if value := strings.TrimSpace(parsed.UIMode); value != "" {
		cfg.UIMode = normalizeUIMode(value)
	}
	if value := strings.TrimSpace(parsed.Layout); value != "" {
		cfg.Layout = normalizeLayout(value)
	}
	if parsed.PickerHeight != nil {
		cfg.PickerHeight = *parsed.PickerHeight
	}
	if cfg.PickerHeight < 6 {
		cfg.PickerHeight = 6
	}
	cfg.Layout = normalizeLayout(cfg.Layout)
	cfg.UIMode = normalizeUIMode(cfg.UIMode)
	if migratedLegacy || configNeedsRewrite(raw, cfg) {
		if err := Save(cfg); err != nil {
			return File{}, err
		}
	}
	return cfg, nil
}

func Save(cfg File) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	if err := os.WriteFile(path, []byte(renderConfig(cfg)), 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}

	return nil
}

func ResolveNotesDir(cliValue string) (string, string, error) {
	if value := strings.TrimSpace(cliValue); value != "" {
		path, err := filepath.Abs(value)
		return path, "flag --dir", err
	}

	if value := strings.TrimSpace(os.Getenv(envNotesDir)); value != "" {
		path, err := filepath.Abs(value)
		return path, envNotesDir, err
	}

	if value := strings.TrimSpace(os.Getenv(legacyAooNotesDir)); value != "" {
		path, err := filepath.Abs(value)
		return path, legacyAooNotesDir, err
	}

	if value := strings.TrimSpace(os.Getenv(legacyEnvNotesDir)); value != "" {
		path, err := filepath.Abs(value)
		return path, legacyEnvNotesDir, err
	}

	cfg, err := Load()
	if err != nil {
		return "", "", err
	}

	if value := strings.TrimSpace(cfg.NotesDir); value != "" {
		path, err := filepath.Abs(value)
		return path, "config", err
	}

	cwd, err := os.Getwd()
	if err == nil {
		matches, globErr := filepath.Glob(filepath.Join(cwd, "*.yaml"))
		if globErr == nil && hasVisibleYAML(matches) {
			return cwd, "current directory", nil
		}
	}

	return "", "", SetupRequiredError{}
}

func SetNotesDir(dir string) (string, error) {
	path, err := filepath.Abs(strings.TrimSpace(dir))
	if err != nil {
		return "", err
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("check notes dir %s: %w", path, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", path)
	}

	cfg, err := Load()
	if err != nil {
		return "", err
	}
	cfg.NotesDir = path

	if err := Save(cfg); err != nil {
		return "", err
	}

	return path, nil
}

func SetNotesRepo(repo string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.NotesRepo = strings.TrimSpace(repo)
	return Save(cfg)
}

func SetUIMode(mode string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "compact", "light":
		mode = "compact"
	case "full-screen", "fullscreen", "full":
		mode = "full-screen"
	default:
		return "", fmt.Errorf("ui mode must be compact or full-screen")
	}
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	cfg.UIMode = mode
	if err := Save(cfg); err != nil {
		return "", err
	}
	return mode, nil
}

func SetLayout(layout string) (string, error) {
	layout = strings.TrimSpace(strings.ToLower(layout))
	if layout != "top" && layout != "bottom" {
		return "", fmt.Errorf("layout must be top or bottom")
	}
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	cfg.Layout = layout
	if err := Save(cfg); err != nil {
		return "", err
	}
	return layout, nil
}

func hasVisibleYAML(matches []string) bool {
	for _, match := range matches {
		base := filepath.Base(match)
		if strings.HasPrefix(base, ".") {
			continue
		}
		return true
	}
	return false
}

func renderConfig(cfg File) string {
	cfg.NotesDir = strings.TrimSpace(cfg.NotesDir)
	cfg.NotesRepo = strings.TrimSpace(cfg.NotesRepo)
	cfg.UIMode = normalizeUIMode(cfg.UIMode)
	cfg.Layout = normalizeLayout(cfg.Layout)
	if cfg.PickerHeight < 6 {
		cfg.PickerHeight = DefaultFile().PickerHeight
	}
	lines := []string{
		"# f — SSH host picker",
		"# hosts are stored as OpenSSH config in ~/.ssh/config.d/f_hosts/*.conf",
		"# ui_mode: compact | full-screen (both use the same frameless fzf-style UI)",
		"# layout: top | bottom (compact mode only)",
		"ui_mode: " + yamlScalar(cfg.UIMode),
		"layout: " + yamlScalar(cfg.Layout),
		"picker_height: " + strconv.Itoa(cfg.PickerHeight),
		"",
	}
	return strings.Join(lines, "\n")
}

func configNeedsRewrite(raw []byte, cfg File) bool {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	legacyMarkers := []string{
		"app_dir:",
		"last_repo_check:",
		"search_mode:",
		"show_preview:",
		"show_notes_on_start:",
		"notes_dir:",
		"notes_repo:",
		"theme:",
		"# search_mode:",
		"full_screen:",
		"focus_mode:",
		"show_match_context:",
		"show_list_on_start:",
		"two_line_results:",
	}
	for _, marker := range legacyMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}

	if !strings.Contains(text, "ui_mode:") {
		return true
	}

	current := strings.TrimSpace(text)
	expected := strings.TrimSpace(renderConfig(cfg))
	return current != expected
}

func normalizeUIMode(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "compact", "light":
		return "compact"
	case "full-screen", "fullscreen", "full":
		return "full-screen"
	default:
		return DefaultFile().UIMode
	}
}

func normalizeLayout(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "top":
		return "top"
	case "bottom":
		return "bottom"
	default:
		return DefaultFile().Layout
	}
}

func yamlScalar(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return `""`
	}
	raw, err := yaml.Marshal(trimmed)
	if err != nil {
		return `""`
	}
	return strings.TrimSpace(string(raw))
}
