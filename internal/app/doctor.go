package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"aoo/internal/config"
	"aoo/internal/notes"
)

type doctorOptions struct {
	Executable, Version, ConfigPath, NotesDir, NotesSource, PathValue string
	SchemaVersion                                                     int
}

type doctorBinary struct{ Path, Version string }
type doctorReport struct {
	Executable       string
	Version          string
	Binaries         []doctorBinary
	LegacyBinaries   []doctorBinary
	ConfigPath       string
	SchemaVersion    int
	NotesDir         string
	NotesSource      string
	ValidEntries     int
	ValidationErrors int
	Duplicates       []notes.Duplicate
	GitStatus        string
}

func buildDoctorReport(options doctorOptions) doctorReport {
	r := doctorReport{Executable: options.Executable, Version: options.Version, ConfigPath: options.ConfigPath, SchemaVersion: options.SchemaVersion, NotesDir: options.NotesDir, NotesSource: options.NotesSource}
	result := notes.LoadDir(options.NotesDir)
	r.ValidEntries, r.ValidationErrors = len(result.Entries), len(result.Errors)
	r.Duplicates = notes.AnalyzeDuplicates(result.Entries)
	r.Binaries = findFBinaries(options.PathValue)
	r.LegacyBinaries = findNamedBinaries(options.PathValue, "aoo")
	if options.NotesDir != "" {
		if _, err := os.Stat(filepath.Join(options.NotesDir, ".git")); err == nil {
			r.GitStatus = "repository detected; status not executed"
		} else {
			r.GitStatus = "not a git repository"
		}
	}
	return r
}

func findFBinaries(pathValue string) []doctorBinary {
	return findNamedBinaries(pathValue, "f")
}

func findNamedBinaries(pathValue, name string) []doctorBinary {
	seen := map[string]bool{}
	var out []doctorBinary
	for _, dir := range filepath.SplitList(pathValue) {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			continue
		}
		real, err := filepath.EvalSymlinks(path)
		if err == nil {
			path = real
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, doctorBinary{Path: path, Version: "not executed"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func formatDoctorReport(r doctorReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "executable: %s\nversion: %s\n", r.Executable, r.Version)
	fmt.Fprintf(&b, "config: %s\nschema version: %d\n", r.ConfigPath, r.SchemaVersion)
	fmt.Fprintf(&b, "notes dir: %s\nnotes source: %s\n", r.NotesDir, r.NotesSource)
	fmt.Fprintf(&b, "validation: %d entries, %d errors\nduplicates: %d\n", r.ValidEntries, r.ValidationErrors, len(r.Duplicates))
	for _, binary := range r.Binaries {
		fmt.Fprintf(&b, "PATH f: %s (%s)\n", binary.Path, binary.Version)
	}
	for _, binary := range r.LegacyBinaries {
		fmt.Fprintf(&b, "stale aoo: %s (%s)\n", binary.Path, binary.Version)
	}
	fmt.Fprintf(&b, "git status: %s\n", r.GitStatus)
	return b.String()
}

func runDoctor(args []string, stdout, stderr interface{ Write([]byte) (int, error) }) error {
	_ = args
	_ = stderr
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if real, e := filepath.EvalSymlinks(exe); e == nil {
		exe = real
	}
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	notesDir, source, err := config.ResolveNotesDir("")
	if err != nil {
		if _, ok := err.(config.SetupRequiredError); !ok {
			return err
		}
	}
	r := buildDoctorReport(doctorOptions{Executable: exe, Version: version, ConfigPath: configPath, SchemaVersion: cfg.SchemaVersion, NotesDir: notesDir, NotesSource: source, PathValue: os.Getenv("PATH")})
	_, err = fmt.Fprint(stdout, formatDoctorReport(r))
	return err
}
