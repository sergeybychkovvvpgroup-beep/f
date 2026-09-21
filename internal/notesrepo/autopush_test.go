package notesrepo

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutoCommitPushPathCommitsAndPushes(t *testing.T) {
	remote := filepath.Join(t.TempDir(), "remote.git")
	worktree := filepath.Join(t.TempDir(), "notes")

	runGit(t, "", "init", "--bare", remote)
	runGit(t, "", "clone", remote, worktree)
	runGit(t, worktree, "config", "user.name", "f-test")
	runGit(t, worktree, "config", "user.email", "f-test@example.com")

	notePath := filepath.Join(worktree, "router.yaml")
	if err := os.WriteFile(notePath, []byte("desc: router\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := AutoCommitPushPath(notePath, "f: update router.yaml", &stdout, &stderr); err != nil {
		t.Fatal(err)
	}

	log := strings.TrimSpace(gitOutput(worktree, "log", "--format=%s", "-1"))
	if log != "f: update router.yaml" {
		t.Fatalf("unexpected commit message: %q", log)
	}

	remoteLog := strings.TrimSpace(runGitOutput(t, "", "--git-dir", remote, "log", "--format=%s", "-1"))
	if remoteLog != "f: update router.yaml" {
		t.Fatalf("expected pushed commit, got %q", remoteLog)
	}
}

func TestAutoCommitPushPathRebasesWhenRemoteHasNewCommit(t *testing.T) {
	remote := filepath.Join(t.TempDir(), "remote.git")
	worktree := filepath.Join(t.TempDir(), "notes")
	otherClone := filepath.Join(t.TempDir(), "other")

	runGit(t, "", "init", "--bare", remote)
	runGit(t, "", "clone", remote, worktree)
	runGit(t, worktree, "config", "user.name", "f-test")
	runGit(t, worktree, "config", "user.email", "f-test@example.com")

	notePath := filepath.Join(worktree, "router.yaml")
	if err := os.WriteFile(notePath, []byte("desc: router\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, worktree, "add", "router.yaml")
	runGit(t, worktree, "commit", "-m", "initial")
	branch := strings.TrimSpace(runGitOutput(t, worktree, "branch", "--show-current"))
	runGit(t, worktree, "push", "-u", "origin", branch)

	runGit(t, "", "clone", remote, otherClone)
	runGit(t, otherClone, "config", "user.name", "f-test")
	runGit(t, otherClone, "config", "user.email", "f-test@example.com")
	if err := os.WriteFile(filepath.Join(otherClone, "remote.yaml"), []byte("desc: remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, otherClone, "add", "remote.yaml")
	runGit(t, otherClone, "commit", "-m", "remote update")
	runGit(t, otherClone, "push")

	if err := os.WriteFile(notePath, []byte("desc: router local\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := AutoCommitPushPath(notePath, "f: update router.yaml", &stdout, &stderr); err != nil {
		t.Fatal(err)
	}

	remoteLog := runGitOutput(t, "", "--git-dir", remote, "log", "--format=%s", "-2")
	if !strings.Contains(remoteLog, "f: update router.yaml") || !strings.Contains(remoteLog, "remote update") {
		t.Fatalf("expected rebased push to include both commits, got %q", remoteLog)
	}
}

func TestAutoCommitPushPathAbortsRebaseOnConflict(t *testing.T) {
	remote := filepath.Join(t.TempDir(), "remote.git")
	worktree := filepath.Join(t.TempDir(), "notes")
	otherClone := filepath.Join(t.TempDir(), "other")

	runGit(t, "", "init", "--bare", remote)
	runGit(t, "", "clone", remote, worktree)
	runGit(t, worktree, "config", "user.name", "f-test")
	runGit(t, worktree, "config", "user.email", "f-test@example.com")

	notePath := filepath.Join(worktree, "router.yaml")
	if err := os.WriteFile(notePath, []byte("desc: router\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, worktree, "add", "router.yaml")
	runGit(t, worktree, "commit", "-m", "initial")
	branch := strings.TrimSpace(runGitOutput(t, worktree, "branch", "--show-current"))
	runGit(t, worktree, "push", "-u", "origin", branch)

	runGit(t, "", "clone", remote, otherClone)
	runGit(t, otherClone, "config", "user.name", "f-test")
	runGit(t, otherClone, "config", "user.email", "f-test@example.com")
	if err := os.WriteFile(filepath.Join(otherClone, "router.yaml"), []byte("desc: remote change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, otherClone, "add", "router.yaml")
	runGit(t, otherClone, "commit", "-m", "remote conflict")
	runGit(t, otherClone, "push")

	if err := os.WriteFile(notePath, []byte("desc: local change\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := AutoCommitPushPath(notePath, "f: update router.yaml", &stdout, &stderr)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !strings.Contains(err.Error(), "resolve notes repo conflict manually") {
		t.Fatalf("unexpected conflict error: %v", err)
	}

	status := strings.TrimSpace(gitOutput(worktree, "status", "--short"))
	if strings.Contains(status, "UU ") {
		t.Fatalf("expected rebase to be aborted cleanly, got status %q", status)
	}
	if strings.TrimSpace(gitOutput(worktree, "rebase", "--show-current-patch")) != "" {
		t.Fatal("expected no active rebase after abort")
	}
}

func TestSyncCommitsPullsAndPushesPendingChanges(t *testing.T) {
	remote := filepath.Join(t.TempDir(), "remote.git")
	worktree := filepath.Join(t.TempDir(), "notes")
	otherClone := filepath.Join(t.TempDir(), "other")

	runGit(t, "", "init", "--bare", remote)
	runGit(t, "", "clone", remote, worktree)
	runGit(t, worktree, "config", "user.name", "f-test")
	runGit(t, worktree, "config", "user.email", "f-test@example.com")

	notePath := filepath.Join(worktree, "router.yaml")
	if err := os.WriteFile(notePath, []byte("desc: router\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, worktree, "add", "router.yaml")
	runGit(t, worktree, "commit", "-m", "initial")
	branch := strings.TrimSpace(runGitOutput(t, worktree, "branch", "--show-current"))
	runGit(t, worktree, "push", "-u", "origin", branch)

	runGit(t, "", "clone", remote, otherClone)
	runGit(t, otherClone, "config", "user.name", "f-test")
	runGit(t, otherClone, "config", "user.email", "f-test@example.com")
	if err := os.WriteFile(filepath.Join(otherClone, "remote.yaml"), []byte("desc: remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, otherClone, "add", "remote.yaml")
	runGit(t, otherClone, "commit", "-m", "remote update")
	runGit(t, otherClone, "push")

	if err := os.WriteFile(notePath, []byte("desc: local pending\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := Sync(worktree, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}

	if got := strings.TrimSpace(gitOutput(worktree, "status", "--short")); got != "" {
		t.Fatalf("expected clean worktree after sync, got %q", got)
	}

	log := runGitOutput(t, "", "--git-dir", remote, "log", "--format=%s", "-3")
	if !strings.Contains(log, "f: sync notes") || !strings.Contains(log, "remote update") {
		t.Fatalf("expected synced history to include local and remote commits, got %q", log)
	}
}

func TestSyncPullsRemoteChangesIntoCleanClone(t *testing.T) {
	remote := filepath.Join(t.TempDir(), "remote.git")
	worktree := filepath.Join(t.TempDir(), "notes")
	otherClone := filepath.Join(t.TempDir(), "other")

	runGit(t, "", "init", "--bare", remote)
	runGit(t, "", "clone", remote, worktree)
	runGit(t, worktree, "config", "user.name", "f-test")
	runGit(t, worktree, "config", "user.email", "f-test@example.com")

	notePath := filepath.Join(worktree, "router.yaml")
	if err := os.WriteFile(notePath, []byte("desc: router\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, worktree, "add", "router.yaml")
	runGit(t, worktree, "commit", "-m", "initial")
	branch := strings.TrimSpace(runGitOutput(t, worktree, "branch", "--show-current"))
	runGit(t, worktree, "push", "-u", "origin", branch)

	runGit(t, "", "clone", remote, otherClone)
	runGit(t, otherClone, "config", "user.name", "f-test")
	runGit(t, otherClone, "config", "user.email", "f-test@example.com")
	if err := os.WriteFile(filepath.Join(otherClone, "extra.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, otherClone, "add", "extra.md")
	runGit(t, otherClone, "commit", "-m", "remote extra")
	runGit(t, otherClone, "push")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := Sync(worktree, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(worktree, "extra.md")); err != nil {
		t.Fatalf("expected remote file to be pulled locally: %v", err)
	}
}

func TestNonInteractiveGitEnvDisablesPrompts(t *testing.T) {
	t.Setenv("GIT_SSH_COMMAND", "ssh -F /tmp/test-config")

	env := nonInteractiveGitEnv()
	joined := strings.Join(env, "\n")

	for _, expected := range []string{
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=Never",
		"SSH_ASKPASS=",
		"GIT_ASKPASS=",
		"GIT_SSH_COMMAND=ssh -F /tmp/test-config -o BatchMode=yes",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected env to contain %q, got %q", expected, joined)
		}
	}
}

func TestNonInteractiveGitEnvAutoAddsSSHKeysFromHome(t *testing.T) {
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "fkey"), []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nkey\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "id_ed25519"), []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nkey\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "id_ed25519.pub"), []byte("ssh-ed25519 AAAA\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte("Host *\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("GIT_SSH_COMMAND", "")

	env := nonInteractiveGitEnv()
	joined := strings.Join(env, "\n")

	expected := "GIT_SSH_COMMAND=ssh -i '" + filepath.Join(sshDir, "id_ed25519") + "' -i '" + filepath.Join(sshDir, "fkey") + "' -o BatchMode=yes"
	if !strings.Contains(joined, expected) {
		t.Fatalf("expected env to contain %q, got %q", expected, joined)
	}
}

func TestGitAuthHintExplainsMissingDeployKey(t *testing.T) {
	output := []byte("git@git.example: Permission denied (publickey,password).\nfatal: Could not read from remote repository.\n")

	hint := gitAuthHint(output, errors.New("exit status 128"))
	if !strings.Contains(hint, "deploy key") {
		t.Fatalf("expected deploy key hint, got %q", hint)
	}
	if !strings.Contains(hint, "ssh-agent") {
		t.Fatalf("expected ssh-agent hint, got %q", hint)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmdArgs := args
	if dir != "" {
		cmdArgs = append([]string{"-C", dir}, args...)
	}

	cmd := exec.Command("git", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmdArgs := args
	if dir != "" {
		cmdArgs = append([]string{"-C", dir}, args...)
	}

	cmd := exec.Command("git", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
	return string(output)
}
