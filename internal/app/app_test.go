package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"f/internal/config"
	"f/internal/notes"
	"f/internal/ui"
)

func TestPromptCommandRunPrintsCommandWithoutConfirmation(t *testing.T) {
	var stdout bytes.Buffer
	if err := promptCommandRun("List files", "ls -la", &stdout); err != nil {
		t.Fatalf("promptCommandRun returned error: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "[run] List files") || !strings.Contains(output, "[command]\nls -la") {
		t.Fatalf("unexpected output %q", output)
	}
	if strings.Contains(output, "run?") {
		t.Fatalf("expected no confirmation prompt, got %q", output)
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

func TestVersionReportsCurrentRelease(t *testing.T) {
	oldArg0 := os.Args[0]
	os.Args[0] = "f"
	t.Cleanup(func() { os.Args[0] = oldArg0 })
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"version"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "f 0.9.5" {
		t.Fatalf("version = %q, want %q", got, "f 0.9.5")
	}
}

func TestConfigUISelectsCompactMode(t *testing.T) {
	t.Setenv("AOO_CONFIG_FILE", filepath.Join(t.TempDir(), "config.yaml"))
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"config", "ui", "compact"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UIMode != "compact" {
		t.Fatalf("ui mode = %q, want compact", cfg.UIMode)
	}
	if !strings.Contains(stdout.String(), "ui_mode: compact") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestCompactModeUsesNonFullscreenPicker(t *testing.T) {
	cfg := config.DefaultFile()
	cfg.UIMode = "compact"
	cfg.PickerHeight = 11
	options := pickerOptions(cfg, ui.SyncStatus{})
	if options.FullScreen {
		t.Fatal("compact mode must retain compact sizing")
	}
	if options.Height != 11 {
		t.Fatalf("compact mode height = %d, want 11", options.Height)
	}
}

func TestFullScreenModeUsesAlternateScreen(t *testing.T) {
	cfg := config.DefaultFile()
	cfg.UIMode = "full-screen"
	options := pickerOptions(cfg, ui.SyncStatus{})
	if !options.FullScreen {
		t.Fatal("full-screen mode must use the alternate screen")
	}
	if options.Height != 0 {
		t.Fatalf("full-screen height = %d, want 0", options.Height)
	}
}

func TestConfigAddressEnablesMutedAddressRows(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("F_CONFIG_FILE", configPath)
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"config", "address", "on"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ShowAddress {
		t.Fatal("show_address is disabled after 'f config address on'")
	}
	if !pickerOptions(cfg, ui.SyncStatus{}).ShowAddress {
		t.Fatal("picker options did not receive show_address")
	}
	if !strings.Contains(stdout.String(), "show_address: true") || !strings.Contains(stdout.String(), configPath) {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestConfigWithoutSubcommandShowsCurrentSettingsAndCommands(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("F_CONFIG_FILE", configPath)
	if err := config.Save(config.File{UIMode: "compact", Layout: "bottom", PickerHeight: 18, ShowAddress: true}); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"config"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	text := stdout.String()
	for _, want := range []string{
		configPath,
		"ui_mode: compact",
		"layout: bottom",
		"picker_height: 18",
		"show_address: true",
		"f config address on|off",
		"f config height N",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config output is missing %q:\n%s", want, text)
		}
	}
}

func TestConfigHeightUpdatesPickerHeight(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("F_CONFIG_FILE", configPath)
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"config", "height", "20"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PickerHeight != 20 {
		t.Fatalf("picker_height = %d, want 20", cfg.PickerHeight)
	}
	if text := stdout.String(); !strings.Contains(text, "picker_height: 20") || !strings.Contains(text, configPath) {
		t.Fatalf("height update does not identify the key and config file: %q", text)
	}
}

func TestConfigLayoutSelectsBottom(t *testing.T) {
	t.Setenv("AOO_CONFIG_FILE", filepath.Join(t.TempDir(), "config.yaml"))
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"config", "layout", "bottom"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Layout != "bottom" {
		t.Fatalf("layout = %q, want bottom", cfg.Layout)
	}
}

func TestUsageUsesFBrand(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("F_CONFIG_FILE", configPath)
	oldArg0 := os.Args[0]
	os.Args[0] = "f"
	t.Cleanup(func() { os.Args[0] = oldArg0 })
	var out bytes.Buffer
	printUsage(&out)
	text := out.String()
	if strings.Contains(strings.ToLower(text), "aoo uses") || strings.Contains(text, "f / aoo") {
		t.Fatalf("usage still advertises the old product name:\n%s", text)
	}
	for _, want := range []string{configPath, "f config", "f config address on|off", "f config height N", "show_address"} {
		if !strings.Contains(text, want) {
			t.Fatalf("usage is missing configuration guidance %q:\n%s", want, text)
		}
	}
}

func TestEditableSSHBlockUsesFBrand(t *testing.T) {
	block, err := editableSSHBlock("test-alias")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(block, "f uses its dedicated OpenSSH") {
		t.Fatalf("editable block does not use f branding:\n%s", block)
	}
	if strings.Contains(block, "# aoo uses") {
		t.Fatalf("editable block still advertises old product name:\n%s", block)
	}
}

func TestSSHConfigPathsUseFNames(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir, err := sshConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".ssh", "config.d", "f_hosts"); dir != want {
		t.Fatalf("SSH config dir = %q, want %q", dir, want)
	}
	path, err := userSSHConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "f.conf"); path != want {
		t.Fatalf("SSH config path = %q, want %q", path, want)
	}
}

func TestUpsertReplacesLegacyEditMarkers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.conf")
	legacy := "# aoo-edit begin server\nHost server\n  HostName old\n# aoo-edit end server\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := upsertMarkedBlock(path, "server", "Host server\n  HostName new\n"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "aoo-edit") || strings.Count(text, "Host server") != 1 || !strings.Contains(text, "# f-edit begin server") {
		t.Fatalf("legacy markers were not migrated cleanly:\n%s", text)
	}
}

func TestSetupWithoutArgumentPromptsForRepositoryAndExplainsMissingAccess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	publicKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestSetupKey fresh-machine"
	gitPath := filepath.Join(binDir, "git")
	if err := os.WriteFile(gitPath, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	keygenPath := filepath.Join(binDir, "ssh-keygen")
	if err := os.WriteFile(keygenPath, []byte("#!/bin/sh\nprintf '%s\\n' '"+publicKey+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "id_ed25519"), []byte("test-private-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "id_ed25519.pub"), []byte(publicKey+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := runSetup(nil, strings.NewReader("git@git.example:sergeyb/sshconfig.git\n"), &stdout, &stderr)
	if err == nil {
		t.Fatal("setup unexpectedly succeeded without repository access")
	}
	text := stdout.String()
	for _, want := range []string{
		"SSH inventory repository",
		"git@git.example:sergeyb/sshconfig.git",
		"Add this public key",
		publicKey,
		"f setup",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("setup guidance is missing %q:\n%s", want, text)
		}
	}
	if _, statErr := os.Stat(filepath.Join(sshDir, "config")); !os.IsNotExist(statErr) {
		t.Fatalf("setup changed SSH config before repository access was verified: %v", statErr)
	}
}

func TestSetupPromptUsesSharedInventoryAsDefault(t *testing.T) {
	var stdout bytes.Buffer
	repo, err := setupRepoURL(nil, strings.NewReader("\n"), &stdout)
	if err != nil {
		t.Fatal(err)
	}
	if repo != "git@git.dawq.me:sergeyb/sshconfig.git" {
		t.Fatalf("default setup repo = %q", repo)
	}
	if !strings.Contains(stdout.String(), "git@git.dawq.me:sergeyb/sshconfig.git") {
		t.Fatalf("prompt does not show default repository: %q", stdout.String())
	}
}

func TestEnsureSetupPublicKeyReplacesOrphanedPublicKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	orphanedKey := "ssh-ed25519 AAAAOrphaned stale"
	publicPath := filepath.Join(sshDir, "id_ed25519.pub")
	if err := os.WriteFile(publicPath, []byte(orphanedKey+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	generatedKey := "ssh-ed25519 AAAAGenerated usable"
	keygen := "#!/bin/sh\nprivate=$9\nprintf 'private\\n' >\"$private\"\nprintf '%s\\n' \"$F_TEST_PUBLIC_KEY\" >\"$private.pub\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "ssh-keygen"), []byte(keygen), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("F_TEST_PUBLIC_KEY", generatedKey)

	publicKey, gotPath, err := ensureSetupPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	if publicKey != generatedKey {
		t.Fatalf("public key = %q, want newly generated %q", publicKey, generatedKey)
	}
	if gotPath != publicPath {
		t.Fatalf("public key path = %q, want %q", gotPath, publicPath)
	}
	if _, err := os.Stat(filepath.Join(sshDir, "id_ed25519")); err != nil {
		t.Fatalf("private key was not generated: %v", err)
	}
}

func TestSetupDoesNotChangeSSHConfigWhenDestinationNeedsForce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	hostsDir := filepath.Join(home, ".ssh", "config.d", "f_hosts")
	if err := os.MkdirAll(hostsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostsDir, "existing.conf"), []byte("Host existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	git := `#!/bin/sh
case "$*" in
  *"rev-parse --verify HEAD"*) echo test-head ;;
	*"symbolic-ref --quiet --short HEAD"*) echo main ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(git), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	var stdout, stderr bytes.Buffer
	err := runSetup([]string{"git@example.test:hosts.git"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("setup error = %v, want destination precondition failure", err)
	}
	configPath := filepath.Join(home, ".ssh", "config")
	if _, statErr := os.Stat(configPath); !os.IsNotExist(statErr) {
		t.Fatalf("setup changed SSH config before destination preconditions passed: %v", statErr)
	}
}

func TestSetupDoesNotChangeSSHConfigWhenFinalCloneFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	git := `#!/bin/sh
case "$*" in
  *"clone --quiet --no-checkout"*) exit 0 ;;
  *"rev-parse --verify HEAD"*) echo test-head; exit 0 ;;
	*"symbolic-ref --quiet --short HEAD"*) echo main; exit 0 ;;
  *"push --dry-run"*) exit 0 ;;
  *"clone git@example.test:hosts.git"*) exit 1 ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(git), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	var stdout, stderr bytes.Buffer
	err := runSetup([]string{"git@example.test:hosts.git"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "git clone hosts repo") {
		t.Fatalf("setup error = %v, want final clone failure", err)
	}
	configPath := filepath.Join(home, ".ssh", "config")
	if _, statErr := os.Stat(configPath); !os.IsNotExist(statErr) {
		t.Fatalf("setup changed SSH config before clone succeeded: %v", statErr)
	}
}

func TestCheckSetupRepoAccessPreservesSSHHostKeyVerification(t *testing.T) {
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(t.TempDir(), "ssh-command.log")
	git := `#!/bin/sh
printf '%s' "$GIT_SSH_COMMAND" >"$F_TEST_LOG"
exit 1
`
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(git), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("F_TEST_LOG", logPath)
	t.Setenv("GIT_SSH_COMMAND", "")

	if err := checkSetupRepoAccess("git@example.test:hosts.git"); err == nil {
		t.Fatal("repository access check unexpectedly succeeded")
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	command := string(raw)
	if strings.Contains(command, "StrictHostKeyChecking=accept-new") || strings.Contains(command, "StrictHostKeyChecking=no") {
		t.Fatalf("repository access weakened host-key verification: %q", command)
	}
}

func TestHostKeyFailureDoesNotProduceDeployKeyGuidance(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var stdout bytes.Buffer
	accessErr := errors.New("Host key verification failed. fatal: Could not read from remote repository.")
	if err := printSetupRepoAccessHelp("git@example.test:hosts.git", accessErr, &stdout); err != nil {
		t.Fatal(err)
	}
	text := stdout.String()
	if !strings.Contains(text, "host key verification") {
		t.Fatalf("host-key guidance missing from output: %s", text)
	}
	if strings.Contains(text, "deploy key") || strings.Contains(text, "id_ed25519") || strings.Contains(text, "ssh-ed25519") {
		t.Fatalf("host-key failure was incorrectly presented as a deploy-key problem: %s", text)
	}
	if _, err := os.Stat(filepath.Join(home, ".ssh", "id_ed25519")); !os.IsNotExist(err) {
		t.Fatalf("host-key failure created an SSH key: %v", err)
	}
}

func TestEnsureSetupPublicKeyRewritesMismatchedPublicKeyFromPrivateKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	privatePath := filepath.Join(sshDir, "id_ed25519")
	publicPath := privatePath + ".pub"
	privateMarker := "PRIVATE-MATERIAL-MUST-NOT-BE-PRINTED"
	if err := os.WriteFile(privatePath, []byte(privateMarker+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publicPath, []byte("ssh-ed25519 AAAAStale old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	derived := "ssh-ed25519 AAAADerived current"
	keygen := "#!/bin/sh\n[ \"$1\" = -y ] || exit 2\nprintf '%s\\n' \"$F_TEST_DERIVED_KEY\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "ssh-keygen"), []byte(keygen), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("F_TEST_DERIVED_KEY", derived)

	publicKey, gotPath, err := ensureSetupPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	if publicKey != derived || gotPath != publicPath {
		t.Fatalf("derived public key = %q at %q, want %q at %q", publicKey, gotPath, derived, publicPath)
	}
	raw, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != derived+"\n" {
		t.Fatalf("public key file was not replaced from private key: %q", raw)
	}
	if strings.Contains(publicKey, privateMarker) {
		t.Fatal("private key material escaped through public-key output")
	}
}

func TestHTTPSAccessFailureUsesHTTPSCredentialGuidance(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var stdout bytes.Buffer
	accessErr := errors.New("fatal: Authentication failed for 'https://github.example/team/hosts.git/'")
	if err := printSetupRepoAccessHelp("https://github.example/team/hosts.git", accessErr, &stdout); err != nil {
		t.Fatal(err)
	}
	text := stdout.String()
	for _, want := range []string{"HTTPS", "credential", "write access"} {
		if !strings.Contains(text, want) {
			t.Fatalf("HTTPS guidance is missing %q: %s", want, text)
		}
	}
	for _, unwanted := range []string{"deploy key", "id_ed25519", "ssh-ed25519"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("HTTPS failure incorrectly suggests %q: %s", unwanted, text)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".ssh", "id_ed25519")); !os.IsNotExist(err) {
		t.Fatalf("HTTPS failure created an SSH key: %v", err)
	}
}

func TestCheckSetupRepoAccessDryRunsActualDefaultBranch(t *testing.T) {
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(t.TempDir(), "git.log")
	git := `#!/bin/sh
case "$*" in
  *"rev-parse --verify HEAD"*) printf '%s\n' abc123 ;;
  *"symbolic-ref --quiet --short HEAD"*) printf '%s\n' main ;;
  *"push --dry-run"*) printf '%s\n' "$*" >"$F_TEST_LOG" ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(git), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("F_TEST_LOG", logPath)

	if err := checkSetupRepoAccess("git@example.test:hosts.git"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	push := string(raw)
	if !strings.Contains(push, "HEAD:refs/heads/main") {
		t.Fatalf("non-empty repository was not dry-run pushed to its default branch: %s", push)
	}
	if strings.Contains(push, "f-setup-access-check") {
		t.Fatalf("non-empty repository used a synthetic branch: %s", push)
	}
}

func TestCheckSetupRepoAccessUsesSyntheticBranchOnlyForEmptyRepo(t *testing.T) {
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(t.TempDir(), "git.log")
	git := `#!/bin/sh
case "$*" in
  *"rev-parse --verify HEAD"*) exit 1 ;;
  *"push --dry-run"*) printf '%s\n' "$*" >"$F_TEST_LOG" ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(git), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("F_TEST_LOG", logPath)

	if err := checkSetupRepoAccess("git@example.test:empty.git"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	push := string(raw)
	if !strings.Contains(push, "push --dry-run origin HEAD:refs/heads/f-setup-access-check") {
		t.Fatalf("empty repository did not use the temporary dry-run branch: %s", push)
	}
}

func TestSetupRestoresExistingOriginWhenPullFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	hostsDir := filepath.Join(home, ".ssh", "config.d", "f_hosts")
	if err := os.MkdirAll(hostsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(t.TempDir(), "origin")
	oldURL := "git@old.example:team/hosts.git"
	newURL := "git@new.example:team/hosts.git"
	if err := os.WriteFile(statePath, []byte(oldURL+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	git := `#!/bin/sh
case "$*" in
  "rev-parse --show-toplevel") printf '%s\n' "$PWD" ;;
  "remote get-url origin") while IFS= read -r line; do printf '%s\n' "$line"; done <"$F_TEST_ORIGIN" ;;
  "remote set-url origin "*) for arg in "$@"; do value="$arg"; done; printf '%s\n' "$value" >"$F_TEST_ORIGIN" ;;
  "pull --rebase --autostash") exit 1 ;;
  *"rev-parse --verify HEAD"*) printf '%s\n' abc123 ;;
  *"symbolic-ref --quiet --short HEAD"*) printf '%s\n' main ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(git), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("F_TEST_ORIGIN", statePath)

	var stdout, stderr bytes.Buffer
	if err := runSetup([]string{newURL}, strings.NewReader(""), &stdout, &stderr); err == nil {
		t.Fatal("setup unexpectedly succeeded when pull failed")
	}
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(raw)); got != oldURL {
		t.Fatalf("origin URL after failed pull = %q, want restored %q", got, oldURL)
	}
}

func TestSetupRestoresExistingOriginWhenIncludeInstallFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	hostsDir := filepath.Join(home, ".ssh", "config.d", "f_hosts")
	if err := os.MkdirAll(hostsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(home, ".ssh", "config"), 0o700); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(t.TempDir(), "origin")
	oldURL := "git@old.example:team/hosts.git"
	newURL := "git@new.example:team/hosts.git"
	if err := os.WriteFile(statePath, []byte(oldURL+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	git := `#!/bin/sh
case "$*" in
  "rev-parse --show-toplevel") printf '%s\n' "$PWD" ;;
  "remote get-url origin") while IFS= read -r line; do printf '%s\n' "$line"; done <"$F_TEST_ORIGIN" ;;
  "remote set-url origin "*) for arg in "$@"; do value="$arg"; done; printf '%s\n' "$value" >"$F_TEST_ORIGIN" ;;
  *"rev-parse --verify HEAD"*) printf '%s\n' abc123 ;;
  *"symbolic-ref --quiet --short HEAD"*) printf '%s\n' main ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(git), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("F_TEST_ORIGIN", statePath)

	var stdout, stderr bytes.Buffer
	if err := runSetup([]string{newURL}, strings.NewReader(""), &stdout, &stderr); err == nil {
		t.Fatal("setup unexpectedly succeeded when SSH include installation failed")
	}
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(raw)); got != oldURL {
		t.Fatalf("origin URL after failed include installation = %q, want restored %q", got, oldURL)
	}
}
