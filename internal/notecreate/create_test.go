package notecreate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateCommandDraft(t *testing.T) {
	root := t.TempDir()

	draft, err := Create(root, KindCommand, "router jump")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasSuffix(draft.Path, "router-jump.yaml") {
		t.Fatalf("unexpected draft path: %s", draft.Path)
	}
	if draft.Line != 2 {
		t.Fatalf("expected edit line 2, got %d", draft.Line)
	}

	raw, err := os.ReadFile(draft.Path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "desc: router jump") {
		t.Fatalf("expected desc in draft, got %q", text)
	}
	if !strings.Contains(text, "cmd: echo \"replace with command\"") {
		t.Fatalf("expected cmd scaffold, got %q", text)
	}
	if strings.Contains(text, "actions:") {
		t.Fatalf("expected simple scaffold without actions list, got %q", text)
	}
}

func TestCreateUsesUniqueFilename(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "router.yaml")
	if err := os.WriteFile(first, []byte("desc: router\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	draft, err := Create(root, KindNote, "router")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasSuffix(draft.Path, "router-2.yaml") {
		t.Fatalf("unexpected unique path: %s", draft.Path)
	}
}

func TestCreateRejectsExactDraftDuplicateAndPointsToExistingFile(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "router.yaml")
	if err := os.WriteFile(existing, []byte("desc: router\ncmd: echo \"replace with command\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Create(root, KindCommand, " Router ")
	if err == nil || !strings.Contains(err.Error(), existing) {
		t.Fatalf("expected duplicate error naming %s, got %v", existing, err)
	}
	matches, _ := filepath.Glob(filepath.Join(root, "*.yaml"))
	if len(matches) != 1 {
		t.Fatalf("duplicate draft was created: %v", matches)
	}
}
