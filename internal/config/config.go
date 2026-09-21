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
	CurrentSchemaVersion = 1
	envNotesDir          = "F_NOTES_DIR"
	legacyEnvNotesDir    = "TERM_NOTES_DIR"
	envTheme             = "F_THEME"
)

type File struct {
	SchemaVersion    int    `yaml:"schema_version"`
	ConfirmRun       bool   `yaml:"confirm_run"`
	NotesDir         string `yaml:"notes_dir"`
	NotesRepo        string `yaml:"notes_repo"`
	Theme            string `yaml:"theme"`
	Layout           string `yaml:"layout"`
	FullScreen       bool   `yaml:"full_screen"`
	PickerHeight     int    `yaml:"picker_height"`
	FocusMode        bool   `yaml:"focus_mode"`
	ShowMatchContext bool   `yaml:"show_match_context"`
	ShowListOnStart  bool   `yaml:"show_list_on_start"`
	TwoLineResults   bool   `yaml:"two_line_results"`
}

type rawFile struct {
	SchemaVersion       int    `yaml:"schema_version"`
	ConfirmRun          *bool  `yaml:"confirm_run"`
	NotesDir            string `yaml:"notes_dir"`
	NotesRepo           string `yaml:"notes_repo"`
	Theme               string `yaml:"theme"`
	Layout              string `yaml:"layout"`
	FullScreen          *bool  `yaml:"full_screen"`
	PickerHeight        *int   `yaml:"picker_height"`
	FocusMode           *bool  `yaml:"focus_mode"`
	ShowMatchContext    *bool  `yaml:"show_match_context"`
	ShowListOnStart     *bool  `yaml:"show_list_on_start"`
	TwoLineResults      *bool  `yaml:"two_line_results"`
	LegacyShowPreview   *bool  `yaml:"show_preview"`
	LegacyShowNotesList *bool  `yaml:"show_notes_on_start"`
}

type SetupRequiredError struct{}

func (e SetupRequiredError) Error() string {
	return strings.TrimSpace(`
notes directory is not configured

Initial setup:
  edit ~/.config/f/config.yaml
  and set notes_dir

Or use commands:
  f set-folder /path/to/notes

Temporary override:
  f --dir /path/to/notes
  F_NOTES_DIR=/path/to/notes f

Check current config:
  f config show
`)
}

func ConfigPath() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("F_CONFIG_FILE")); custom != "" {
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

func LegacyConfigPath() (string, error) { return legacyConfigPath() }

func MigrateLegacy() (string, error) {
	legacy, err := legacyConfigPath()
	if err != nil {
		return "", err
	}
	target, err := ConfigPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("refusing to overwrite existing config %s", target)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	raw, err := os.ReadFile(legacy)
	if err != nil {
		return "", fmt.Errorf("read legacy config %s: %w", legacy, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(target, raw, 0o644); err != nil {
		return "", err
	}
	_, err = Load()
	if err != nil {
		_ = os.Remove(target)
		return "", fmt.Errorf("legacy config is not compatible: %w", err)
	}
	return target, nil
}

func DefaultFile() File {
	return File{
		SchemaVersion:    CurrentSchemaVersion,
		ConfirmRun:       true,
		Theme:            "fzf-dark",
		Layout:           "bottom",
		FullScreen:       true,
		PickerHeight:     14,
		FocusMode:        false,
		ShowMatchContext: false,
		ShowListOnStart:  false,
		TwoLineResults:   true,
	}
}

func Load() (File, error) {
	path, err := ConfigPath()
	if err != nil {
		return File{}, err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			legacy, legacyErr := legacyConfigPath()
			if legacyErr == nil {
				if _, statErr := os.Stat(legacy); statErr == nil {
					return File{}, fmt.Errorf("legacy config found at %s; run `f migrate` to migrate it explicitly", legacy)
				}
			}
			cfg := DefaultFile()
			if saveErr := Save(cfg); saveErr != nil {
				return File{}, saveErr
			}
			return cfg, nil
		}
		return File{}, fmt.Errorf("read config %s: %w", path, err)
	}

	cfg := DefaultFile()
	var keys map[string]yaml.Node
	if err := yaml.Unmarshal(raw, &keys); err != nil {
		return File{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	known := map[string]bool{"schema_version": true, "confirm_run": true, "notes_dir": true, "notes_repo": true, "theme": true, "layout": true, "full_screen": true, "picker_height": true, "focus_mode": true, "show_match_context": true, "show_list_on_start": true, "two_line_results": true, "show_preview": true, "show_notes_on_start": true}
	for key := range keys {
		if !known[key] {
			return File{}, fmt.Errorf("unknown config key %q in %s (old SSH-picker configs are not compatible; edit manually)", key, path)
		}
	}
	var parsed rawFile
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return File{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if parsed.SchemaVersion < 0 || parsed.SchemaVersion > CurrentSchemaVersion {
		return File{}, fmt.Errorf("unsupported schema_version %d in %s (current: %d)", parsed.SchemaVersion, path, CurrentSchemaVersion)
	}

	cfg.SchemaVersion = CurrentSchemaVersion
	if parsed.ConfirmRun != nil {
		cfg.ConfirmRun = *parsed.ConfirmRun
	}
	cfg.NotesDir = strings.TrimSpace(parsed.NotesDir)
	cfg.NotesRepo = strings.TrimSpace(parsed.NotesRepo)
	if value := strings.TrimSpace(parsed.Theme); value != "" {
		cfg.Theme = value
	}
	if value := strings.TrimSpace(parsed.Layout); value != "" {
		cfg.Layout = normalizeLayout(value)
	}
	if parsed.FullScreen != nil {
		cfg.FullScreen = *parsed.FullScreen
	}
	if parsed.PickerHeight != nil {
		cfg.PickerHeight = *parsed.PickerHeight
	}
	if parsed.FocusMode != nil {
		cfg.FocusMode = *parsed.FocusMode
	}
	if parsed.ShowMatchContext != nil {
		cfg.ShowMatchContext = *parsed.ShowMatchContext
	} else if parsed.LegacyShowPreview != nil {
		cfg.ShowMatchContext = *parsed.LegacyShowPreview
	}
	if parsed.ShowListOnStart != nil {
		cfg.ShowListOnStart = *parsed.ShowListOnStart
	} else if parsed.LegacyShowNotesList != nil {
		cfg.ShowListOnStart = *parsed.LegacyShowNotesList
	}
	if parsed.TwoLineResults != nil {
		cfg.TwoLineResults = *parsed.TwoLineResults
	}
	if cfg.PickerHeight < 6 {
		cfg.PickerHeight = 6
	}
	cfg.Layout = normalizeLayout(cfg.Layout)
	if configNeedsRewrite(raw, cfg) {
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

func ResolveTheme(cliValue string) (string, string, error) {
	if value := strings.TrimSpace(cliValue); value != "" {
		return value, "flag --theme", nil
	}

	if value := strings.TrimSpace(os.Getenv(envTheme)); value != "" {
		return value, envTheme, nil
	}

	cfg, err := Load()
	if err != nil {
		return "", "", err
	}

	if value := strings.TrimSpace(cfg.Theme); value != "" {
		return value, "config", nil
	}

	return DefaultFile().Theme, "default", nil
}

func SetTheme(theme string) (string, error) {
	theme = strings.TrimSpace(theme)
	if theme == "" {
		return "", errors.New("theme name is required")
	}

	cfg, err := Load()
	if err != nil {
		return "", err
	}
	cfg.Theme = theme

	if err := Save(cfg); err != nil {
		return "", err
	}

	return theme, nil
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
	cfg.Theme = strings.TrimSpace(cfg.Theme)
	cfg.Layout = normalizeLayout(cfg.Layout)
	if cfg.Theme == "" {
		cfg.Theme = DefaultFile().Theme
	}
	if cfg.PickerHeight < 6 {
		cfg.PickerHeight = DefaultFile().PickerHeight
	}
	lines := []string{
		"# f",
		"# themes: fzf-dark, catppuccin-mocha, catppuccin-latte, dracula, nord, solarized-dark, solarized-light",
		"# layout: top | bottom",
		"# focus_mode: hide hotkeys/help footer for a quieter UI",
		"# show_match_context: preview line for selected item",
		"# show_list_on_start: render results when query is empty",
		"# two_line_results: desc on first line, command/text on second line",
		"schema_version: " + strconv.Itoa(CurrentSchemaVersion),
		"confirm_run: " + yamlScalarBool(cfg.ConfirmRun),
		"notes_dir: " + yamlScalar(cfg.NotesDir),
		"notes_repo: " + yamlScalar(cfg.NotesRepo),
		"theme: " + yamlScalar(cfg.Theme),
		"layout: " + yamlScalar(cfg.Layout),
		"full_screen: " + yamlScalarBool(cfg.FullScreen),
		"picker_height: " + strconv.Itoa(cfg.PickerHeight),
		"focus_mode: " + yamlScalarBool(cfg.FocusMode),
		"show_match_context: " + yamlScalarBool(cfg.ShowMatchContext),
		"show_list_on_start: " + yamlScalarBool(cfg.ShowListOnStart),
		"two_line_results: " + yamlScalarBool(cfg.TwoLineResults),
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
		"# search_mode:",
	}
	for _, marker := range legacyMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}

	if !strings.Contains(text, "show_match_context:") {
		return true
	}
	if !strings.Contains(text, "focus_mode:") {
		return true
	}
	if !strings.Contains(text, "show_list_on_start:") {
		return true
	}
	if !strings.Contains(text, "two_line_results:") {
		return true
	}

	current := strings.TrimSpace(text)
	expected := strings.TrimSpace(renderConfig(cfg))
	return current != expected
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

func yamlScalarBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
