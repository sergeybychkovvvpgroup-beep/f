package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestDefaultUpgradeRepoUsesFRepository(t *testing.T) {
	t.Setenv("F_UPGRADE_REPO", "")
	t.Setenv("AOO_UPGRADE_REPO", "")
	const want = "https://github.com/sergeybychkovvvpgroup-beep/f.git"
	if got := defaultUpgradeRepo(); got != want {
		t.Fatalf("defaultUpgradeRepo = %q, want %q", got, want)
	}
}

func TestDefaultUpgradeRepoAllowsExplicitOverride(t *testing.T) {
	const want = "file:///tmp/f-test.git"
	t.Setenv("F_UPGRADE_REPO", want)
	if got := defaultUpgradeRepo(); got != want {
		t.Fatalf("defaultUpgradeRepo override = %q, want %q", got, want)
	}
}

func TestUpdateCacheUsesFDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", root)
	path, err := updateCachePath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "f", "update-check.json")
	if path != want {
		t.Fatalf("update cache path = %q, want %q", path, want)
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

func TestUpdateCacheIsIgnoredWhenItDiffersFromRunningBuild(t *testing.T) {
	cached := updateCheckCache{
		CheckedAt: time.Now(),
		Commit:    "12d39feb52b73a70c907d592206b47458203215a",
	}
	current := "c7b2930c463e2b5599e73de8f7e44c2d5c3374f5"
	if updateCacheUsable(cached, current, time.Now()) {
		t.Fatal("stale mismatched cache would offer a downgrade instead of refreshing the remote HEAD")
	}
}

func TestUpdateCacheIsUsedWhenItMatchesRunningBuild(t *testing.T) {
	current := "c7b2930c463e2b5599e73de8f7e44c2d5c3374f5"
	cached := updateCheckCache{CheckedAt: time.Now(), Commit: current}
	if !updateCacheUsable(cached, current[:8], time.Now()) {
		t.Fatal("fresh cache matching the running build should avoid a network check")
	}
}

func TestRecordInstalledCommitReplacesStaleCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update-check.json")
	if err := writeUpdateCache(path, updateCheckCache{CheckedAt: time.Now(), Commit: "old"}); err != nil {
		t.Fatal(err)
	}
	installed := "7af35d99b0021b7db09b12c72bdf2834d4a80e8b"
	when := time.Now().Add(time.Second)
	if err := recordInstalledCommit(path, installed, when); err != nil {
		t.Fatal(err)
	}
	got, ok := readUpdateCache(path)
	if !ok || got.Commit != installed || !got.CheckedAt.Equal(when) {
		t.Fatalf("installed cache = %#v, readable=%v", got, ok)
	}
}

func TestValidateForwardUpgradeRejectsDowngrade(t *testing.T) {
	dir, older, newer := testUpgradeRepository(t)
	if err := validateForwardUpgrade(dir, older, newer); err != nil {
		t.Fatalf("forward upgrade rejected: %v", err)
	}
	if err := validateForwardUpgrade(dir, newer, older); err == nil || !strings.Contains(err.Error(), "non-forward") {
		t.Fatalf("downgrade error = %v, want non-forward rejection", err)
	}
}

func TestValidateForwardUpgradeRejectsUnknownRunningCommit(t *testing.T) {
	dir, _, newer := testUpgradeRepository(t)
	missing := strings.Repeat("a", 40)
	if err := validateForwardUpgrade(dir, missing, newer); err == nil || !strings.Contains(err.Error(), "not present") {
		t.Fatalf("missing commit error = %v, want not-present rejection", err)
	}
}

func testUpgradeRepository(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	runGitTest(t, dir, "init")
	runGitTest(t, dir, "config", "user.name", "Test Operator")
	runGitTest(t, dir, "config", "user.email", "operator@example.invalid")
	path := filepath.Join(dir, "version.txt")
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, "add", "version.txt")
	runGitTest(t, dir, "commit", "-m", "old")
	older := strings.TrimSpace(runGitTest(t, dir, "rev-parse", "HEAD"))
	if err := os.WriteFile(path, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, "commit", "-am", "new")
	newer := strings.TrimSpace(runGitTest(t, dir, "rev-parse", "HEAD"))
	return dir, older, newer
}

func runGitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}
