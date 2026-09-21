package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aoo/internal/notes"
)

func TestPromptCommandRunRequiresExplicitYes(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		want        bool
	}{
		{"yes", "y\n", true}, {"no", "n\n", false}, {"eof", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			got := promptCommandRun("List files", "ls -la", strings.NewReader(tc.input), &stdout)
			if got != tc.want {
				t.Fatalf("confirmed = %t, want %t", got, tc.want)
			}
			if !strings.Contains(stdout.String(), "Run command? [y/N]") {
				t.Fatalf("missing prompt: %q", stdout.String())
			}
		})
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

func TestDuplicatesCommandReportsJSONAndReturnsFindingError(t *testing.T) {
	dir := t.TempDir()
	raw := "- desc: One\n  cmd: echo hi\n- desc: Two\n  cmd: echo hi\n"
	if err := os.WriteFile(filepath.Join(dir, "notes.yaml"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := runDuplicates([]string{"--dir", dir, "--json"}, &stdout, &stderr)
	if err == nil || !IsDuplicateFindings(err) {
		t.Fatalf("expected duplicate finding error, got %v", err)
	}
	if !strings.Contains(stdout.String(), `"kind": "command"`) || !strings.Contains(stdout.String(), `"line": 1`) {
		t.Fatalf("unexpected JSON: %s", stdout.String())
	}
}

func TestDuplicatesJSONUsesEmptyArrayWhenClean(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.yaml"), []byte("desc: One\ncmd: echo hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := runDuplicates([]string{"--dir", dir, "--json"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "[]" {
		t.Fatalf("clean JSON = %q, want []", got)
	}
}

func TestValidateWarnsOnDuplicatesButSucceeds(t *testing.T) {
	dir := t.TempDir()
	raw := "- desc: One\n  cmd: echo hi\n- desc: Two\n  cmd: echo hi\n"
	if err := os.WriteFile(filepath.Join(dir, "notes.yaml"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := runValidate([]string{"--dir", dir}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "WARNING: duplicate command") {
		t.Fatalf("missing warning: %q", stderr.String())
	}
}

func TestVersionReportsCurrentFRelease(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"version"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "f 0.3.0" {
		t.Fatalf("version = %q, want %q", got, "f 0.3.0")
	}
}
