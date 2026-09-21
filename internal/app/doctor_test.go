package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDoctorReportUsesInjectedPathsDeterministically(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "n.yaml"), []byte("desc: hi\ncmd: echo hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report := buildDoctorReport(doctorOptions{Executable: "/bin/f", Version: "test", ConfigPath: "/cfg/f/config.yaml", NotesDir: dir, NotesSource: "flag --dir", PathValue: ""})
	if report.Executable != "/bin/f" || report.Version != "test" || report.ConfigPath != "/cfg/f/config.yaml" {
		t.Fatalf("unexpected report: %#v", report)
	}
	if report.ValidEntries != 1 || report.ValidationErrors != 0 {
		t.Fatalf("unexpected counts: %#v", report)
	}
	if !strings.Contains(formatDoctorReport(report), "notes source: flag --dir") {
		t.Fatal("missing notes resolution")
	}
}

func TestDoctorReportsStaleAooBinary(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "aoo")
	if err := os.WriteFile(legacy, []byte("#!/bin/sh\nprintf 'aoo 0.2.0\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	report := buildDoctorReport(doctorOptions{PathValue: dir})
	if len(report.LegacyBinaries) != 1 || report.LegacyBinaries[0].Path != legacy {
		t.Fatalf("stale aoo binary was not reported: %#v", report.LegacyBinaries)
	}
	if !strings.Contains(formatDoctorReport(report), "stale aoo") {
		t.Fatalf("formatted report omits stale binary: %q", formatDoctorReport(report))
	}
}

func TestDoctorDoesNotExecutePathBinaries(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "executed")
	script := filepath.Join(dir, "f")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ntouch \""+marker+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	report := buildDoctorReport(doctorOptions{PathValue: dir})
	if len(report.Binaries) != 1 {
		t.Fatalf("expected binary inventory, got %#v", report.Binaries)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("doctor executed an untrusted PATH binary")
	}
}

func TestDoctorDoesNotExecuteGitFromPath(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "git-executed")
	git := filepath.Join(dir, "git")
	if err := os.WriteFile(git, []byte("#!/bin/sh\n: > \""+marker+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	buildDoctorReport(doctorOptions{NotesDir: dir, PathValue: dir})
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("doctor executed an untrusted git from PATH")
	}
}

func TestDoctorDoesNotExecuteRepositoryFsmonitor(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "fsmonitor-executed")
	hook := filepath.Join(dir, "fsmonitor.sh")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\n: > \""+marker+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init"}, {"config", "core.fsmonitor", hook}} {
		cmd := exec.Command("/usr/bin/git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	buildDoctorReport(doctorOptions{NotesDir: dir})
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("doctor executed repository-controlled fsmonitor")
	}
}
