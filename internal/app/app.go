package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"aoo/internal/bundled"
	"aoo/internal/config"
	"aoo/internal/notecreate"
	"aoo/internal/notes"
	"aoo/internal/notesrepo"
	"aoo/internal/ui"
)

const version = "0.3.0"

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "validate":
			return runValidate(args[1:], stdout, stderr)
		case "duplicates":
			return runDuplicates(args[1:], stdout, stderr)
		case "doctor":
			return runDoctor(args[1:], stdout, stderr)
		case "migrate":
			return runMigrate(stdout)
		case "themes":
			return runThemes(stdout)
		case "config":
			return runConfig(args[1:], stdout, stderr)
		case "set-source":
			return runSetSource(stdin, stdout, stderr)
		case "set-folder":
			return runSetFolder(args[1:], stdout, stderr)
		case "set-theme":
			return runSetTheme(args[1:], stdout, stderr)
		case "add":
			return runAdd(args[1:], stdout, stderr)
		case "upgrade":
			return runUpgrade(args[1:], stdout, stderr)
		case "version", "--version", "-v":
			_, err := fmt.Fprintf(stdout, "%s %s\n", cliName(), version)
			return err
		case "help", "--help", "-h":
			printUsage(stdout)
			return nil
		}
	}

	return runInteractive(args, stdin, stdout, stderr)
}

func runInteractive(args []string, stdin io.Reader, stdout, stderr io.Writer) (err error) {
	fs := flag.NewFlagSet("f", flag.ContinueOnError)
	fs.SetOutput(stderr)

	dir := fs.String("dir", "", "directory with YAML notes")
	query := fs.String("query", "", "initial search query")
	themeFlag := fs.String("theme", "", "theme name")
	strict := fs.Bool("strict", false, "fail when any note file has validation errors")
	yes := fs.Bool("yes", false, "run commands without confirmation")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if message, ok := repoUpdateHint(); ok {
		fmt.Fprintf(stdout, "[f] %s\n", message)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	root, _, err := config.ResolveNotesDir(*dir)
	if err != nil {
		if _, ok := err.(config.SetupRequiredError); ok && strings.TrimSpace(*dir) == "" {
			root = ""
		} else {
			return err
		}
	}

	var syncStream <-chan ui.SyncStatus
	if strings.TrimSpace(root) != "" {
		syncStream = startNotesSync(root)
	}

	themeName, _, err := config.ResolveTheme(*themeFlag)
	if err != nil {
		return err
	}

	uiOptions := ui.Options{
		FullScreen:        cfg.FullScreen,
		Height:            cfg.PickerHeight,
		FocusMode:         cfg.FocusMode,
		ShowMatchContext:  cfg.ShowMatchContext,
		ShowListOnStart:   cfg.ShowListOnStart,
		SingleLineResults: !cfg.TwoLineResults,
		Layout:            cfg.Layout,
		SyncStatusStream:  syncStream,
	}
	if syncStream != nil {
		uiOptions.InitialSync = ui.SyncStatus{State: ui.SyncStateRunning}
	}
	if uiOptions.FullScreen {
		uiOptions.Height = 0
	}

	currentQuery := *query
	hasActiveUI := false
	for {
		result, err := loadInteractiveEntries(root, *strict, stderr)
		if err != nil {
			return err
		}
		if hasActiveUI && !uiOptions.FullScreen {
			clearInteractiveArea(stdout, uiOptions.Height)
		}
		selected, selectedLine, nextQuery, cancelled, editRequested, printOnlyRequested, createKind, err := ui.RunPicker(result.Entries, currentQuery, themeName, uiOptions)
		hasActiveUI = true
		currentQuery = nextQuery
		if err != nil {
			return err
		}
		if createKind != "" {
			if strings.TrimSpace(root) == "" {
				return errors.New("notes_dir is not configured")
			}
			if err := createAndEditDraft(root, notecreate.Kind(createKind), currentQuery, stdout, stderr); err != nil {
				return err
			}
			continue
		}
		if cancelled || selected == nil {
			return nil
		}
		if editRequested {
			if err := openEntryInEditor(*selected, selectedLine, stdout, stderr); err != nil {
				return err
			}
			continue
		}

		if action := selected.QuickAction(); action != nil {
			if printOnlyRequested {
				return printActionCommand(*selected, action, stdout)
			}
			switch {
			case action.IsShow():
				printActionText(*selected, action, stdout)
				return nil
			case action.IsCmd():
				return runCommand(*selected, action, stdin, stdout, stderr, cfg.ConfirmRun && !*yes)
			}
		}

		if !uiOptions.FullScreen {
			clearInteractiveArea(stdout, uiOptions.Height)
		}
		action, cancelled, actionPrintOnly, err := ui.RunActionPicker(*selected, themeName, uiOptions)
		if err != nil {
			return err
		}
		if cancelled || action == nil {
			continue
		}

		if printOnlyRequested || actionPrintOnly {
			return printActionCommand(*selected, action.Action, stdout)
		}

		switch action.Kind {
		case ui.ActionRead:
			printActionText(*selected, action.Action, stdout)
			return nil
		case ui.ActionRun:
			return runCommand(*selected, action.Action, stdin, stdout, stderr, cfg.ConfirmRun && !*yes)
		default:
			return fmt.Errorf("unknown action: %s", action.Kind)
		}
	}
}

func clearInteractiveArea(stdout io.Writer, height int) {
	rows := height
	if rows < 6 {
		rows = 6
	}
	rows += 2
	fmt.Fprintf(stdout, "\r\x1b[%dA\x1b[J", rows)
}

func startNotesSync(root string) <-chan ui.SyncStatus {
	result := make(chan ui.SyncStatus, 1)
	go func() {
		defer close(result)
		if err := notesrepo.Sync(root, io.Discard, io.Discard); err != nil {
			result <- ui.SyncStatus{State: ui.SyncStateError, Message: shortError(err)}
			return
		}
		result <- ui.SyncStatus{State: ui.SyncStateOK}
	}()
	return result
}

func shortError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.TrimSpace(err.Error())
	if text == "" {
		return "unknown error"
	}
	if line := strings.TrimSpace(strings.Split(text, "\n")[0]); line != "" {
		return line
	}
	return text
}

func runValidate(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "directory with YAML notes")
	if err := fs.Parse(args); err != nil {
		return err
	}

	root, _, err := config.ResolveNotesDir(*dir)
	if err != nil {
		return err
	}

	result := notes.LoadDir(root)
	duplicates := notes.AnalyzeDuplicates(result.Entries)
	for _, duplicate := range duplicates {
		fmt.Fprintf(stderr, "WARNING: duplicate %s %q at %s\n", duplicate.Kind, duplicate.Value, formatLocations(duplicate.Locations))
	}
	if len(result.Errors) == 0 {
		_, err = fmt.Fprintf(stdout, "OK: %d entries loaded from %s\n", len(result.Entries), root)
		return err
	}

	for _, loadErr := range result.Errors {
		fmt.Fprintf(stderr, "ERROR: %v\n", loadErr)
	}
	return fmt.Errorf("validation failed: %d file(s) with errors", len(result.Errors))
}

type duplicateFindingsError struct{ count int }

func (e duplicateFindingsError) Error() string { return fmt.Sprintf("duplicate findings: %d", e.count) }
func IsDuplicateFindings(err error) bool {
	var target duplicateFindingsError
	return errors.As(err, &target)
}

func runDuplicates(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("duplicates", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "directory with YAML notes")
	jsonOutput := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, _, err := config.ResolveNotesDir(*dir)
	if err != nil {
		return err
	}
	result := notes.LoadDir(root)
	if len(result.Errors) > 0 {
		for _, loadErr := range result.Errors {
			fmt.Fprintf(stderr, "ERROR: %v\n", loadErr)
		}
		return fmt.Errorf("validation failed: %d file(s) with errors", len(result.Errors))
	}
	duplicates := notes.AnalyzeDuplicates(result.Entries)
	if duplicates == nil {
		duplicates = []notes.Duplicate{}
	}
	if *jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(duplicates); err != nil {
			return err
		}
	} else if len(duplicates) == 0 {
		fmt.Fprintln(stdout, "No duplicates found.")
	} else {
		for _, duplicate := range duplicates {
			fmt.Fprintf(stdout, "%s: %q\n", duplicate.Kind, duplicate.Value)
			for _, location := range duplicate.Locations {
				fmt.Fprintf(stdout, "  %s:%d\n", location.Path, location.Line)
			}
		}
	}
	if len(duplicates) > 0 {
		return duplicateFindingsError{count: len(duplicates)}
	}
	return nil
}

func formatLocations(locations []notes.Location) string {
	parts := make([]string, 0, len(locations))
	for _, location := range locations {
		parts = append(parts, fmt.Sprintf("%s:%d", location.Path, location.Line))
	}
	return strings.Join(parts, ", ")
}

func loadBundledNotes() notes.LoadResult {
	return bundled.Load()
}

func runCommand(entry notes.Entry, action *notes.Action, stdin io.Reader, stdout, stderr io.Writer, confirm bool) error {
	if action == nil || !action.IsCmd() {
		return errors.New("selected action is not a command action")
	}
	command := strings.TrimSpace(action.Cmd)
	banner := strings.TrimSpace(action.Banner)
	desc := runHeader(entry, action)

	if confirm && !promptCommandRun(desc, command, stdin, stdout) {
		fmt.Fprintln(stdout, "cancelled")
		return nil
	}
	if !confirm {
		fmt.Fprintf(stdout, "[run] %s\n\n[command]\n%s\n", desc, command)
	}

	if banner := strings.TrimSpace(banner); banner != "" {
		fmt.Fprintln(stdout, renderBanner(desc, banner))
	}

	cmd := exec.Command("/bin/sh", "-lc", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func runHeader(entry notes.Entry, action *notes.Action) string {
	base := strings.TrimSpace(entry.DisplayName())
	if action == nil {
		return base
	}
	suffix := strings.TrimSpace(action.Desc)
	if suffix == "" || strings.EqualFold(base, suffix) {
		return base
	}
	return fmt.Sprintf("%s :: %s", base, suffix)
}

func promptCommandRun(desc, command string, stdin io.Reader, stdout io.Writer) bool {
	fmt.Fprintf(stdout, "[run] %s\n", desc)
	fmt.Fprintf(stdout, "\n[command]\n%s\n", command)
	fmt.Fprint(stdout, "Run command? [y/N] ")
	var answer string
	if _, err := fmt.Fscanln(stdin, &answer); err != nil {
		return false
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

func runMigrate(stdout io.Writer) error {
	path, err := config.MigrateLegacy()
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "migrated config to %s\n", path)
	fmt.Fprintln(stdout, "notes were not moved; update notes_dir explicitly if you want to relocate them to ~/.local/share/f/notes")
	return nil
}

func runConfig(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return openConfigInEditor(stdout, stderr)
	}

	switch args[0] {
	case "show":
		return runConfigShow(stdout)
	case "set-source":
		return runSetSource(os.Stdin, stdout, stderr)
	case "set-folder":
		return runSetFolder(args[1:], stdout, stderr)
	case "set-theme":
		return runSetTheme(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown config subcommand: %s", args[0])
	}
}

func runSetSource(stdin io.Reader, stdout, stderr io.Writer) error {
	_, err := notesrepo.SetupSource(stdin, stdout, stderr)
	return err
}

func runSetFolder(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("set-folder", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return errors.New("usage: f set-folder /path/to/notes")
	}

	path, err := config.SetNotesDir(fs.Arg(0))
	if err != nil {
		return err
	}

	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(stdout, "configured notes_dir: %s\nconfig file: %s\n", path, configPath)
	return err
}

func runConfigShow(stdout io.Writer) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}

	root, source, resolveErr := config.ResolveNotesDir("")
	if resolveErr != nil {
		if _, ok := resolveErr.(config.SetupRequiredError); ok {
			root = ""
			source = "not configured"
		} else {
			return resolveErr
		}
	}

	themeName, themeSource, themeErr := config.ResolveTheme("")
	if themeErr != nil {
		return themeErr
	}
	fmt.Fprintf(stdout, "schema_version: %d\n", cfg.SchemaVersion)
	fmt.Fprintf(stdout, "confirm_run: %t\n", cfg.ConfirmRun)
	fmt.Fprintf(stdout, "config file: %s\n", configPath)
	fmt.Fprintf(stdout, "notes_dir: %s\n", emptyIfUnset(cfg.NotesDir))
	fmt.Fprintf(stdout, "notes_repo: %s\n", emptyIfUnset(cfg.NotesRepo))
	fmt.Fprintf(stdout, "active dir: %s\n", emptyIfUnset(root))
	fmt.Fprintf(stdout, "active source: %s\n", source)
	fmt.Fprintf(stdout, "theme: %s\n", emptyIfUnset(cfg.Theme))
	fmt.Fprintf(stdout, "active theme: %s\n", themeName)
	fmt.Fprintf(stdout, "theme source: %s\n", themeSource)
	fmt.Fprintf(stdout, "layout: %s\n", cfg.Layout)
	fmt.Fprintf(stdout, "full_screen: %t\n", cfg.FullScreen)
	fmt.Fprintf(stdout, "picker_height: %d\n", cfg.PickerHeight)
	fmt.Fprintf(stdout, "focus_mode: %t\n", cfg.FocusMode)
	fmt.Fprintf(stdout, "show_match_context: %t\n", cfg.ShowMatchContext)
	fmt.Fprintf(stdout, "show_list_on_start: %t\n", cfg.ShowListOnStart)
	fmt.Fprintf(stdout, "two_line_results: %t\n", cfg.TwoLineResults)
	return nil
}

func runSetTheme(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("set-theme", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return errors.New("usage: f set-theme THEME")
	}

	themeName := fs.Arg(0)
	if _, err := ui.ResolveTheme(themeName); err != nil {
		return err
	}

	saved, err := config.SetTheme(themeName)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(stdout, "configured theme: %s\n", saved)
	return err
}

func runThemes(stdout io.Writer) error {
	fmt.Fprintln(stdout, "Themes:")
	for _, name := range ui.ThemeNames() {
		fmt.Fprintf(stdout, "  %s\n", name)
	}
	return nil
}

func runAdd(args []string, stdout, stderr io.Writer) error {
	root, _, err := config.ResolveNotesDir("")
	if err != nil {
		return err
	}

	kind := notecreate.KindNote
	start := 0
	if len(args) > 0 {
		switch notecreate.Kind(args[0]) {
		case notecreate.KindNote, notecreate.KindCommand:
			kind = notecreate.Kind(args[0])
			start = 1
		}
	}
	title := strings.TrimSpace(strings.Join(args[start:], " "))
	return createAndEditDraft(root, kind, title, stdout, stderr)
}

func openEntryInEditor(entry notes.Entry, line int, stdout, stderr io.Writer) error {
	if strings.TrimSpace(entry.SourcePath) == "" {
		return errors.New("selected note has no source file")
	}
	if !filepath.IsAbs(entry.SourcePath) {
		notesDir, _, err := config.ResolveNotesDir("")
		if err != nil {
			return err
		}
		path, err := bundled.Materialize(entry.SourcePath, notesDir)
		if err != nil {
			return err
		}
		targetLine := line
		if targetLine <= 0 {
			targetLine = entry.SourceLine
		}
		return openPathInEditor(path, targetLine, fmt.Sprintf("f: update %s", filepath.Base(path)), stdout, stderr)
	}
	targetLine := line
	if targetLine <= 0 {
		targetLine = entry.SourceLine
	}
	return openPathInEditor(entry.SourcePath, targetLine, fmt.Sprintf("f: update %s", filepath.Base(entry.SourcePath)), stdout, stderr)
}

func openConfigInEditor(stdout, stderr io.Writer) error {
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	return openPathInEditor(configPath, 0, "", stdout, stderr)
}

func openPathInEditor(path string, line int, commitMessage string, stdout, stderr io.Writer) error {
	editor := strings.TrimSpace(os.Getenv("VISUAL"))
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return errors.New("editor command is empty")
	}

	args := []string{}
	if line > 0 {
		args = append(args, "+"+strconv.Itoa(line))
	}
	args = append(args, path)
	args = append(parts[1:], args...)

	fmt.Fprintf(stdout, "[edit] %s\n", path)
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	if err := notesrepo.AutoCommitPushPath(path, commitMessage, stdout, stderr); err != nil {
		return err
	}
	return nil
}

func printActionText(entry notes.Entry, action *notes.Action, stdout io.Writer) {
	if action == nil {
		return
	}
	fmt.Fprintln(stdout, entry.DisplayName())
	fmt.Fprintln(stdout, strings.TrimRight(action.Text, "\n"))
}

func printActionCommand(entry notes.Entry, action *notes.Action, stdout io.Writer) error {
	if action == nil {
		return errors.New("selected action is empty")
	}
	if !action.IsCmd() {
		printActionText(entry, action, stdout)
		return nil
	}
	fmt.Fprintln(stdout, strings.TrimSpace(action.Cmd))
	return nil
}

func runUpgrade(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repoURL := fs.String("repo", defaultUpgradeRepo(), "git repository URL")
	workDir := fs.String("workdir", "", "source checkout directory")
	binPath := fs.String("bin", "", "target binary path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	target := strings.TrimSpace(*binPath)
	if target == "" {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		target, err = filepath.EvalSymlinks(exe)
		if err != nil {
			target = exe
		}
	}
	if strings.TrimSpace(target) == "" {
		return errors.New("cannot detect target binary path")
	}

	dir := strings.TrimSpace(*workDir)
	if dir == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			return err
		}
		dir = filepath.Join(cacheDir, "f", "source")
	}

	fmt.Fprintf(stdout, "[upgrade] repo: %s\n", safeRepoURL(*repoURL))
	fmt.Fprintf(stdout, "[upgrade] source: %s\n", dir)
	fmt.Fprintf(stdout, "[upgrade] target: %s\n", target)

	if err := ensureUpgradeCheckout(dir, *repoURL, stdout, stderr); err != nil {
		return err
	}

	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", target, "./cmd/f")
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build upgrade: %w", err)
	}

	fmt.Fprintf(stdout, "[upgrade] done: %s\n", target)
	return nil
}

func defaultUpgradeRepo() string {
	if value := strings.TrimSpace(os.Getenv("F_UPGRADE_REPO")); value != "" {
		return value
	}
	return "https://github.com/sergeybychkovvvpgroup-beep/f.git"
}

func safeRepoURL(repoURL string) string {
	if i := strings.Index(repoURL, "://"); i >= 0 {
		schemeEnd := i + len("://")
		rest := repoURL[schemeEnd:]
		if at := strings.Index(rest, "@"); at >= 0 {
			return repoURL[:schemeEnd] + "***@" + rest[at+1:]
		}
	}
	return repoURL
}

func ensureUpgradeCheckout(dir, repoURL string, stdout, stderr io.Writer) error {
	if strings.TrimSpace(repoURL) == "" {
		return errors.New("upgrade repo URL is required")
	}
	if fileExists(filepath.Join(dir, ".git")) {
		if err := runUpgradeGit(dir, stdout, stderr, "remote", "set-url", "origin", repoURL); err != nil {
			return err
		}
		if err := runUpgradeGit(dir, stdout, stderr, "fetch", "--prune", "origin"); err != nil {
			return err
		}
		branch := strings.TrimSpace(upgradeGitOutput(dir, "branch", "--show-current"))
		if branch == "" {
			branch = "main"
		}
		if err := runUpgradeGit(dir, stdout, stderr, "pull", "--ff-only", "origin", branch); err != nil {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("git", "clone", repoURL, dir)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func runUpgradeGit(dir string, stdout, stderr io.Writer, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func upgradeGitOutput(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out strings.Builder
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return out.String()
}

func cliName() string {
	return "f"
}

func renderBanner(title, message string) string {
	lines := strings.Split(strings.TrimSpace(message), "\n")
	width := len(title)
	for _, line := range lines {
		if len(line) > width {
			width = len(line)
		}
	}
	width += 4

	var b strings.Builder
	border := "+" + strings.Repeat("-", width+2) + "+"
	b.WriteString(border + "\n")
	b.WriteString(fmt.Sprintf("| %-*s |\n", width, title))
	b.WriteString("|" + strings.Repeat("-", width+2) + "|\n")
	for _, line := range lines {
		b.WriteString(fmt.Sprintf("| %-*s |\n", width, line))
	}
	b.WriteString(border + "\n")
	return b.String()
}

func printUsage(w io.Writer) {
	name := cliName()
	fmt.Fprintln(w, name)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Terminal notes and command launcher.")
	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "Run `%s` and search for `f-help`.\n", name)
	fmt.Fprintln(w, "Built-in setup/help notes are available on a clean host.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Extra commands:")
	fmt.Fprintf(w, "  %s version\n", name)
	fmt.Fprintf(w, "  %s validate --dir PATH\n", name)
	fmt.Fprintf(w, "  %s duplicates --dir PATH [--json]\n", name)
	fmt.Fprintf(w, "  %s doctor\n", name)
	fmt.Fprintf(w, "  %s migrate\n", name)
	fmt.Fprintf(w, "  %s add [title]\n", name)
	fmt.Fprintf(w, "  %s add cmd [title]\n", name)
	fmt.Fprintf(w, "  %s set-source\n", name)
	fmt.Fprintf(w, "  %s upgrade\n", name)
}

func printConfigUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  f config")
	fmt.Fprintln(w, "  f config show")
	fmt.Fprintln(w, "  f config set-folder PATH")
	fmt.Fprintln(w, "  f config set-theme THEME")
}

func emptyIfUnset(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(not set)"
	}
	return value
}

func mergeEntries(primary, secondary []notes.Entry) []notes.Entry {
	merged := make([]notes.Entry, 0, len(primary)+len(secondary))
	seen := map[string]struct{}{}

	appendEntry := func(entry notes.Entry) {
		key := entry.DisplayName() + "|" + entry.Action() + "|" + entry.DisplayValue()
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		merged = append(merged, entry)
	}

	for _, entry := range primary {
		appendEntry(entry)
	}
	for _, entry := range secondary {
		appendEntry(entry)
	}

	return merged
}

func loadInteractiveEntries(root string, strict bool, stderr io.Writer) (notes.LoadResult, error) {
	result := notes.LoadResult{}
	if strings.TrimSpace(root) != "" {
		result = notes.LoadDir(root)
	}

	if bundledResult := loadBundledNotes(); len(bundledResult.Entries) > 0 || len(bundledResult.Errors) > 0 {
		for _, loadErr := range bundledResult.Errors {
			fmt.Fprintf(stderr, "warn: %v\n", loadErr)
		}
		result.Entries = mergeEntries(result.Entries, bundledResult.Entries)
	}

	if len(result.Errors) > 0 {
		for _, loadErr := range result.Errors {
			fmt.Fprintf(stderr, "warn: %v\n", loadErr)
		}
		if strict {
			return notes.LoadResult{}, errors.New("validation errors found")
		}
	}
	if len(result.Entries) == 0 {
		return notes.LoadResult{}, errors.New("no notes found")
	}
	return result, nil
}

func createAndEditDraft(root string, kind notecreate.Kind, title string, stdout, stderr io.Writer) error {
	draft, err := notecreate.Create(root, kind, title)
	if err != nil {
		return err
	}

	if err := openPathInEditor(draft.Path, draft.Line, draft.CommitMessage, stdout, stderr); err != nil {
		return err
	}
	return nil
}

func repoUpdateHint() (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}

	repoRoot, ok := detectFRepoRoot(cwd)
	if !ok {
		return "", false
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", false
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return "", false
	}

	if isWithinDir(exePath, repoRoot) {
		return "", false
	}

	exeInfo, err := os.Stat(exePath)
	if err != nil {
		return "", false
	}

	checks := []string{
		filepath.Join(repoRoot, "go.mod"),
		filepath.Join(repoRoot, "internal", "bundled", "notes.go"),
	}

	for _, path := range checks {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.ModTime().After(exeInfo.ModTime()) {
			return fmt.Sprintf("repo is newer than installed binary; run `sudo make update` or `./bin/f` from %s", repoRoot), true
		}
	}

	return "", false
}

func detectFRepoRoot(start string) (string, bool) {
	dir := start
	for {
		if fileExists(filepath.Join(dir, "go.mod")) && fileExists(filepath.Join(dir, "internal", "bundled", "notes.go")) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isWithinDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}
