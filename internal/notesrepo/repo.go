package notesrepo

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"aoo/internal/config"
)

type Status struct {
	IsRepo     bool
	RemoteURL  string
	Dirty      bool
	DirtyFiles int
	Ahead      int
	Behind     int
}

func DefaultNotesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "f", "notes"), nil
}

func BootstrapIfNeeded(stdin io.Reader, stdout, stderr io.Writer) (string, error) {
	root, _, err := config.ResolveNotesDir("")
	if err == nil && strings.TrimSpace(root) != "" {
		return root, nil
	}

	if _, ok := err.(config.SetupRequiredError); !ok {
		return "", err
	}

	return SetupSource(stdin, stdout, stderr)
}

func SetupSource(stdin io.Reader, stdout, stderr io.Writer) (string, error) {
	reader := bufio.NewReader(stdin)
	defaultDir, err := DefaultNotesDir()
	if err != nil {
		return "", err
	}

	fmt.Fprintln(stdout, "f notes source")
	fmt.Fprintln(stdout, "1. repo")
	fmt.Fprintln(stdout, "2. local folder")
	fmt.Fprint(stdout, "(1/2?) [1]: ")
	mode, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = "1"
	}

	if mode == "1" {
		fmt.Fprintf(stdout, "repo url: ")
		repoURL, readErr := reader.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return "", readErr
		}
		repoURL = strings.TrimSpace(repoURL)
		if repoURL == "" {
			return "", fmt.Errorf("repo url is required for source type repo")
		}
		if err := cloneRepo(repoURL, defaultDir, stdout, stderr); err != nil {
			pubkeys, _ := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519.pub"))
			if len(pubkeys) == 0 {
				pubkeys, _ = os.ReadFile(filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa.pub"))
			}
			return "", fmt.Errorf("clone notes repo failed\nif repo is private, add this public key to Deploy Keys:\n%s\noriginal error: %w", strings.TrimSpace(string(pubkeys)), err)
		}
		if err := config.SetNotesRepo(repoURL); err != nil {
			return "", err
		}
		if _, err := config.SetNotesDir(defaultDir); err != nil {
			return "", err
		}
		fmt.Fprintf(stdout, "configured notes_dir: %s\n", defaultDir)
		return defaultDir, nil
	}

	fmt.Fprintf(stdout, "notes folder [%s]: ", defaultDir)
	value, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value != "" {
		defaultDir = value
	}
	defaultDir, err = filepath.Abs(defaultDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		return "", err
	}
	if _, err := config.SetNotesDir(defaultDir); err != nil {
		return "", err
	}
	fmt.Fprintf(stdout, "configured notes_dir: %s\n", defaultDir)
	return defaultDir, nil
}

func CheckStatus(dir string) (Status, error) {
	if strings.TrimSpace(dir) == "" || !isGitRepo(dir) {
		return Status{}, nil
	}

	status := Status{IsRepo: true}
	status.RemoteURL = strings.TrimSpace(gitOutput(dir, "remote", "get-url", "origin"))
	porcelain := strings.TrimSpace(gitOutput(dir, "status", "--porcelain"))
	if porcelain != "" {
		status.Dirty = true
		status.DirtyFiles = len(strings.Split(porcelain, "\n"))
	}

	status.Ahead, status.Behind = aheadBehind(dir)
	return status, nil
}

func Sync(dir string, stdout, stderr io.Writer) error {
	if strings.TrimSpace(dir) == "" || !isGitRepo(dir) {
		return nil
	}
	if strings.TrimSpace(gitOutput(dir, "remote", "get-url", "origin")) == "" {
		return nil
	}

	branch := strings.TrimSpace(gitOutput(dir, "branch", "--show-current"))
	if branch == "" {
		return fmt.Errorf("sync notes repo: cannot detect current branch")
	}

	if output, err := gitCombined(dir, "fetch", "--prune", "origin"); err != nil {
		if len(output) > 0 {
			_, _ = stderr.Write(output)
		}
		return compactGitError("git fetch origin", output, err)
	}

	if status, err := CheckStatus(dir); err == nil && status.Dirty {
		fmt.Fprintf(stdout, "[notes] auto-committing %d pending change(s)\n", status.DirtyFiles)
		if err := commitAll(dir, "f: sync notes", stdout, stderr); err != nil {
			return err
		}
	}

	if output, err := gitCombined(dir, "pull", "--rebase", "--autostash", "origin", branch); err != nil {
		if len(output) > 0 {
			_, _ = stdout.Write(output)
		}
		_ = gitRun(dir, io.Discard, io.Discard, "rebase", "--abort")
		return compactGitError(fmt.Sprintf("git pull --rebase origin %s", branch), output, err)
	} else if len(output) > 0 && !bytes.Contains(output, []byte("Already up to date.")) {
		_, _ = stdout.Write(output)
	}

	if ahead, _ := aheadBehind(dir); ahead > 0 {
		if err := pushWithRebaseRetry(dir, stdout, stderr); err != nil {
			return err
		}
	}

	return nil
}

func PrintHint(status Status, stdout io.Writer) {
	if !status.IsRepo {
		return
	}

	switch {
	case status.Dirty:
		fmt.Fprintf(stdout, "[notes] %d local change(s) pending\n", status.DirtyFiles)
	case status.Ahead > 0 && status.Behind > 0:
		fmt.Fprintf(stdout, "[notes] diverged: %d ahead / %d behind\n", status.Ahead, status.Behind)
	case status.Ahead > 0:
		fmt.Fprintf(stdout, "[notes] %d local commit(s) pending push\n", status.Ahead)
	case status.Behind > 0:
		fmt.Fprintf(stdout, "[notes] %d fetched commit(s) available\n", status.Behind)
	}
}

func cloneRepo(repoURL, targetDir string, stdout, stderr io.Writer) error {
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return err
	}
	cmd := gitCommand("", "clone", repoURL, targetDir)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return authAwareGitError("git clone", nil, err)
	}
	return nil
}

func isGitRepo(dir string) bool {
	cmd := gitCommand(dir, "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

func git(dir string, args ...string) error {
	cmd := gitCommand(dir, args...)
	return cmd.Run()
}

func gitOutput(dir string, args ...string) string {
	cmd := gitCommand(dir, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return ""
	}
	return stdout.String()
}

func aheadBehind(dir string) (ahead int, behind int) {
	out := strings.TrimSpace(gitOutput(dir, "rev-list", "--left-right", "--count", "@{upstream}...HEAD"))
	if out == "" {
		return 0, 0
	}
	fmt.Sscanf(out, "%d %d", &behind, &ahead)
	return ahead, behind
}

func commitAll(dir, message string, stdout, stderr io.Writer) error {
	if err := gitRun(dir, stdout, stderr, "add", "-A"); err != nil {
		return fmt.Errorf("git add -A: %w", err)
	}
	if !hasAnyStagedChanges(dir) {
		return nil
	}
	if err := gitRun(dir, stdout, stderr, "commit", "-m", message); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}

func hasAnyStagedChanges(dir string) bool {
	cmd := gitCommand(dir, "diff", "--cached", "--quiet")
	return cmd.Run() != nil
}

func compactGitError(prefix string, output []byte, err error) error {
	message := strings.TrimSpace(firstNonEmptyLine(string(output)))
	if message == "" {
		return fmt.Errorf("%s: %w", prefix, err)
	}
	return fmt.Errorf("%s: %s", prefix, message)
}

func gitCommand(dir string, args ...string) *exec.Cmd {
	cmdArgs := args
	if strings.TrimSpace(dir) != "" {
		cmdArgs = append([]string{"-C", dir}, args...)
	}

	cmd := exec.Command("git", cmdArgs...)
	cmd.Env = nonInteractiveGitEnv()
	return cmd
}

func nonInteractiveGitEnv() []string {
	env := append([]string{}, os.Environ()...)
	env = appendOrReplaceEnv(env, "GIT_TERMINAL_PROMPT=0")
	env = appendOrReplaceEnv(env, "GCM_INTERACTIVE=Never")
	env = appendOrReplaceEnv(env, "SSH_ASKPASS=")
	env = appendOrReplaceEnv(env, "GIT_ASKPASS=")

	sshCommand := strings.TrimSpace(os.Getenv("GIT_SSH_COMMAND"))
	if sshCommand == "" {
		sshCommand = autoSSHCommand()
	} else if !strings.Contains(sshCommand, "BatchMode=yes") {
		sshCommand += " -o BatchMode=yes"
	}
	env = appendOrReplaceEnv(env, "GIT_SSH_COMMAND="+sshCommand)
	return env
}

func autoSSHCommand() string {
	args := []string{"ssh"}
	for _, path := range discoverSSHIdentityFiles() {
		args = append(args, "-i", shellQuote(path))
	}
	args = append(args, "-o", "BatchMode=yes")
	return strings.Join(args, " ")
}

func discoverSSHIdentityFiles() []string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return nil
	}

	sshDir := filepath.Join(home, ".ssh")
	entries, err := os.ReadDir(sshDir)
	if err != nil {
		return nil
	}

	preferred := map[string]int{
		"id_ed25519":    1,
		"id_ecdsa":      2,
		"id_ecdsa_sk":   3,
		"id_ed25519_sk": 4,
		"id_rsa":        5,
		"id_dsa":        6,
		"identity":      7,
	}

	type candidate struct {
		path     string
		priority int
	}

	candidates := make([]candidate, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !looksLikePrivateKeyName(name) {
			continue
		}
		path := filepath.Join(sshDir, name)
		if !isPrivateKeyFile(path) {
			continue
		}
		priority := 100
		if value, ok := preferred[name]; ok {
			priority = value
		}
		candidates = append(candidates, candidate{path: path, priority: priority})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].priority != candidates[j].priority {
			return candidates[i].priority < candidates[j].priority
		}
		return candidates[i].path < candidates[j].path
	})

	paths := make([]string, 0, len(candidates))
	for _, item := range candidates {
		paths = append(paths, item.path)
	}
	return paths
}

func looksLikePrivateKeyName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if strings.HasSuffix(name, ".pub") {
		return false
	}
	switch name {
	case "authorized_keys", "known_hosts", "known_hosts.old", "config":
		return false
	}
	return true
}

func isPrivateKeyFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return false
	}
	line := strings.TrimSpace(firstNonEmptyLine(string(data[:minInt(len(data), 256)])))
	return strings.HasPrefix(line, "-----BEGIN OPENSSH PRIVATE KEY-----") ||
		strings.HasPrefix(line, "-----BEGIN RSA PRIVATE KEY-----") ||
		strings.HasPrefix(line, "-----BEGIN EC PRIVATE KEY-----") ||
		strings.HasPrefix(line, "-----BEGIN DSA PRIVATE KEY-----") ||
		strings.HasPrefix(line, "-----BEGIN PRIVATE KEY-----")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func appendOrReplaceEnv(env []string, pair string) []string {
	key, _, ok := strings.Cut(pair, "=")
	if !ok {
		return env
	}
	prefix := key + "="
	for i, existing := range env {
		if strings.HasPrefix(existing, prefix) {
			env[i] = pair
			return env
		}
	}
	return append(env, pair)
}

func authAwareGitError(prefix string, output []byte, err error) error {
	if hint := gitAuthHint(output, err); hint != "" {
		return fmt.Errorf("%s: %s", prefix, hint)
	}
	return compactGitError(prefix, output, err)
}

func gitAuthHint(output []byte, err error) string {
	text := strings.ToLower(strings.TrimSpace(string(output)))
	if err != nil && text == "" {
		text = strings.ToLower(strings.TrimSpace(err.Error()))
	}

	authMarkers := []string{
		"permission denied",
		"publickey",
		"could not read from remote repository",
		"terminal prompts disabled",
		"batchmode",
		"could not resolve hostname",
		"authentication failed",
		"passphrase",
		"password",
	}
	for _, marker := range authMarkers {
		if strings.Contains(text, marker) {
			return "git auth failed; add the repo deploy key or preload the SSH key in ssh-agent. Password/passphrase prompts are disabled for automatic operations"
		}
	}
	return ""
}

func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
