package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCommitsEqualAcceptsFullAndShortHashes(t *testing.T) {
	full := "0123456789abcdef0123456789abcdef01234567"
	if !commitsEqual(full, full) {
		t.Fatal("equal commits were not recognized")
	}
	if !commitsEqual(full[:8], full) {
		t.Fatal("short build commit was not recognized")
	}
	if commitsEqual("unknown", full) {
		t.Fatal("unknown build commit must be treated as upgradeable")
	}
	if commitsEqual("aaaaaaaa", full) {
		t.Fatal("different commits were treated as equal")
	}
}

func TestShortCommit(t *testing.T) {
	if got := shortCommit("0123456789abcdef"); got != "01234567" {
		t.Fatalf("shortCommit = %q", got)
	}
	if got := shortCommit(""); got != "unknown" {
		t.Fatalf("empty shortCommit = %q", got)
	}
}

func TestUpdateCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "update-check.json")
	want := updateCheckCache{CheckedAt: time.Now().UTC().Truncate(time.Second), Commit: "abc123"}
	if err := writeUpdateCache(path, want); err != nil {
		t.Fatal(err)
	}
	got, ok := readUpdateCache(path)
	if !ok {
		t.Fatal("cache was not readable")
	}
	if got.Commit != want.Commit || !got.CheckedAt.Equal(want.CheckedAt) {
		t.Fatalf("cache = %#v, want %#v", got, want)
	}
	if mode := fileMode(t, path); mode.Perm() != 0o600 {
		t.Fatalf("cache permissions = %v, want 0600", mode.Perm())
	}
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode()
}
