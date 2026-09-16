package app

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"aoo/internal/config"
	"aoo/internal/hosts"
	"aoo/internal/notes"
	"aoo/internal/ui"
)

const version = "0.2.0"

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "add":
			return runAdd(args[1:], stdin, stdout, stderr)
		case "list", "ls":
			return runList(stdout)
		case "config":
			return runConfig(args[1:], stdout, stderr)
		case "themes":
			return runThemes(stdout)
		case "set-theme":
			return runSetTheme(args[1:], stdout, stderr)
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
	return runInteractive(args, stdout, stderr)
}

func runInteractive(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("f", flag.ContinueOnError)
	fs.SetOutput(stderr)
	query := fs.String("query", "", "initial search query")
	themeFlag := fs.String("theme", "", "theme name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*query) == "" && fs.NArg() > 0 {
		*query = strings.Join(fs.Args(), " ")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	themeName, _, err := config.ResolveTheme(*themeFlag)
	if err != nil {
		return err
	}
	syncStatus := syncHostsConfig(stderr)
	list, err := hosts.Load()
	if err != nil {
		return err
	}

	options := ui.Options{
		FullScreen:        cfg.FullScreen,
		Height:            cfg.PickerHeight,
		FocusMode:         cfg.FocusMode,
		ShowMatchContext:  false,
		ShowListOnStart:   cfg.ShowListOnStart,
		SingleLineResults: !cfg.TwoLineResults,
		Layout:            cfg.Layout,
		InitialSync:       syncStatus,
	}
	if options.FullScreen {
		options.Height = 0
	}

	selected, _, nextQuery, cancelled, editRequested, printOnly, createKind, err := ui.RunPicker(hosts.ToEntries(list), *query, themeName, options)
	if err != nil || cancelled {
		return err
	}
	if editRequested {
		if selected == nil {
			return nil
		}
		return editSSHConfigEntry(*selected, stdout, stderr)
	}
	if createKind != "" {
		saved, err := promptAndSaveHost(nextQuery, stdout, stderr)
		if err != nil {
			return err
		}
		entries := hosts.ToEntries([]hosts.Host{saved})
		selected = &entries[0]
	}
	if selected == nil {
		return nil
	}
	action := selected.QuickAction()
	if action == nil || !action.IsCmd() {
		return errors.New("selected entry has no ssh command")
	}
	if printOnly {
		fmt.Fprintln(stdout, strings.TrimSpace(action.Cmd))
		return nil
	}
	return runCommand(*selected, action, stdout, stderr)
}

func editSSHConfigEntry(entry notes.Entry, stdout, stderr io.Writer) error {
	action := entry.QuickAction()
	if action == nil || !action.IsCmd() {
		return errors.New("selected entry has no ssh command to edit")
	}
	alias, ok := sshAliasFromCommand(action.Cmd)
	if !ok {
		return fmt.Errorf("cannot edit non-ssh-alias command: %s", action.Cmd)
	}
	block, err := editableSSHBlock(alias)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp("", "aoo-ssh-edit-*.conf")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.WriteString(block); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "nano"
	}
	cmd := exec.Command("/bin/sh", "-lc", shellQuoteArg(editor)+" "+shellQuoteArg(tmpPath))
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(edited)) == "" {
		return errors.New("edited SSH block is empty; aborting")
	}
	path, err := userSSHConfigPath()
	if err != nil {
		return err
	}
	if err := upsertMarkedBlock(path, alias, strings.TrimRight(string(edited), "\n")+"\n"); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "saved SSH override: %s\nfile: %s\n", alias, path)
	return nil
}

func shellQuoteArg(s string) string {
	if s == "" {
		return "''"
	}
	if strings.IndexFunc(s, func(r rune) bool {
		return !(r == '-' || r == '_' || r == '.' || r == '/' || r == ':' || r == '@' || r == '+' || r == '=' || r == ',' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z')
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func sshAliasFromCommand(command string) (string, bool) {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) != 2 || fields[0] != "ssh" {
		return "", false
	}
	alias := strings.Trim(fields[1], "'\"")
	if alias == "" || strings.HasPrefix(alias, "-") || strings.ContainsAny(alias, " \t\n") {
		return "", false
	}
	return alias, true
}

func editableSSHBlock(alias string) (string, error) {
	attrs := sshG(alias)
	lines := []string{
		"# Edit this block. It will be saved to ~/.ssh/config.d/aoo.conf",
		"# aoo uses the normal OpenSSH config.d file directly; no hidden store.",
		"Host " + alias,
	}
	add := func(key, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			lines = append(lines, "  "+key+" "+value)
		}
	}
	add("HostName", attrs["hostname"])
	add("User", attrs["user"])
	if port := strings.TrimSpace(attrs["port"]); port != "" && port != "22" {
		add("Port", port)
	}
	add("ProxyJump", attrs["proxyjump"])
	add("ProxyCommand", attrs["proxycommand"])
	for _, value := range attrsList(attrs, "localforward") {
		add("LocalForward", value)
	}
	for _, value := range attrsList(attrs, "remoteforward") {
		add("RemoteForward", value)
	}
	for _, value := range attrsList(attrs, "dynamicforward") {
		add("DynamicForward", value)
	}
	add("RemoteCommand", attrs["remotecommand"])
	if value := strings.TrimSpace(attrs["requesttty"]); value != "" && value != "auto" {
		add("RequestTTY", value)
	}
	if len(lines) == 3 {
		lines = append(lines, "  HostName ")
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func sshG(alias string) map[string]string {
	cmd := exec.Command("ssh", "-G", alias)
	out, err := cmd.Output()
	if err != nil {
		return map[string]string{}
	}
	attrs := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		if prev := attrs[key]; prev != "" {
			attrs[key] = prev + "\n" + value
		} else {
			attrs[key] = value
		}
	}
	return attrs
}

func attrsList(attrs map[string]string, key string) []string {
	value := strings.TrimSpace(attrs[key])
	if value == "" {
		return nil
	}
	return strings.Split(value, "\n")
}

func userSSHConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "config.d", "aoo.conf"), nil
}

func upsertMarkedBlock(path, alias, block string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	begin := "# aoo-edit begin " + alias
	end := "# aoo-edit end " + alias
	marked := begin + "\n" + block + end + "\n"
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	text := string(raw)
	start := strings.Index(text, begin)
	finish := strings.Index(text, end)
	if start >= 0 && finish >= start {
		finish += len(end)
		if finish < len(text) && text[finish] == '\n' {
			finish++
		}
		text = text[:start] + marked + text[finish:]
	} else {
		if strings.TrimSpace(text) != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		if strings.TrimSpace(text) != "" {
			text += "\n"
		}
		text += marked
	}
	return os.WriteFile(path, []byte(text), 0o600)
}

func runCommand(entry notes.Entry, action *notes.Action, stdout, stderr io.Writer) error {
	command := strings.TrimSpace(action.Cmd)
	if err := promptCommandRun(runHeader(entry, action), command, stdout); err != nil {
		return err
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
	if suffix == "" || strings.EqualFold(base, suffix) || suffix == "ssh" {
		return base
	}
	return fmt.Sprintf("%s :: %s", base, suffix)
}

func promptCommandRun(desc, command string, stdout io.Writer) error {
	fmt.Fprintf(stdout, "[run] %s\n", desc)
	fmt.Fprintf(stdout, "\n[command]\n%s\n", command)
	return nil
}

func runList(stdout io.Writer) error {
	list, err := hosts.Load()
	if err != nil {
		return err
	}
	for _, h := range list {
		fmt.Fprintf(stdout, "%-24s %s\n", hosts.DisplayName(h), h.Command())
	}
	return nil
}

func runAdd(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	user := fs.String("user", "", "ssh user")
	port := fs.Int("p", 0, "ssh port")
	portLong := fs.Int("port", 0, "ssh port")
	tags := multiFlag{}
	fs.Var(&tags, "tag", "search tag (repeatable)")
	desc := fs.String("desc", "", "description")
	sshArgs := fs.String("args", "", "extra ssh args")
	if err := fs.Parse(args); err != nil {
		return err
	}

	h := hosts.Host{User: *user, Port: *port, Tags: tags, Desc: *desc, Args: *sshArgs}
	if *portLong > 0 {
		h.Port = *portLong
	}
	rest := fs.Args()
	if len(rest) >= 2 {
		h.Name = rest[0]
		h.Host = rest[1]
	} else {
		var err error
		h, err = promptHost(stdin, stdout, h)
		if err != nil {
			return err
		}
	}

	saved, err := hosts.Add(h)
	if err != nil {
		return err
	}
	pushHostsConfig(stderr)
	path, _ := hosts.Path()
	fmt.Fprintf(stdout, "saved: %s -> %s\nfile: %s\n", saved.Name, saved.Command(), path)
	return nil
}

func promptAndSaveHost(initialName string, stdout, stderr io.Writer) (hosts.Host, error) {
	h, err := promptHost(os.Stdin, stdout, hosts.Host{Name: strings.TrimSpace(initialName)})
	if err != nil {
		return hosts.Host{}, err
	}
	saved, err := hosts.Add(h)
	if err != nil {
		return hosts.Host{}, err
	}
	pushHostsConfig(stderr)
	path, _ := hosts.Path()
	fmt.Fprintf(stdout, "saved: %s -> %s\nfile: %s\n", saved.Name, saved.Command(), path)
	return saved, nil
}

func promptHost(stdin io.Reader, stdout io.Writer, h hosts.Host) (hosts.Host, error) {
	r := bufio.NewReader(stdin)
	ask := func(label, current string) (string, error) {
		if current != "" {
			fmt.Fprintf(stdout, "%s [%s]: ", label, current)
		} else {
			fmt.Fprintf(stdout, "%s: ", label)
		}
		value, err := r.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return current, nil
		}
		return value, nil
	}
	var err error
	if h.Name, err = ask("name/alias", h.Name); err != nil {
		return h, err
	}
	if h.Host, err = ask("host ([user@]ip-or-domain)", h.Host); err != nil {
		return h, err
	}
	if h.User, err = ask("user", h.User); err != nil {
		return h, err
	}
	currentPort := ""
	if h.Port > 0 {
		currentPort = strconv.Itoa(h.Port)
	}
	port, err := ask("port", currentPort)
	if err != nil {
		return h, err
	}
	if port == "0" || port == "" {
		h.Port = 0
	} else if n, err := strconv.Atoi(port); err == nil {
		h.Port = n
	}
	if tagLine, err := ask("tags (space separated)", strings.Join(h.Tags, " ")); err == nil {
		h.Tags = strings.Fields(tagLine)
	} else {
		return h, err
	}
	h.Desc, err = ask("description", h.Desc)
	return h, err
}

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' })...)
	return nil
}

func runConfig(args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 && args[0] == "show" {
		cfgPath, _ := config.ConfigPath()
		hostsPath, _ := hosts.Path()
		hostsDir, _ := hosts.ConfigDir()
		fmt.Fprintf(stdout, "config file: %s\nhosts dir: %s\ndefault hosts file: %s\n", cfgPath, hostsDir, hostsPath)
		return nil
	}
	if len(args) > 0 && args[0] == "sync" {
		status := syncHostsConfig(stderr)
		if status.Message != "" {
			fmt.Fprintf(stdout, "%s: %s\n", status.State, status.Message)
		} else {
			fmt.Fprintf(stdout, "%s\n", status.State)
		}
		return nil
	}
	return errors.New("usage: f config show|sync")
}

func syncHostsConfig(stderr io.Writer) ui.SyncStatus {
	dir, err := hosts.ConfigDir()
	if err != nil {
		return ui.SyncStatus{State: ui.SyncStateWarn, Message: "no config.d"}
	}
	root, ok := gitRoot(dir)
	if !ok {
		return ui.SyncStatus{State: ui.SyncStateHidden}
	}
	if err := runQuietGit(root, "pull", "--rebase", "--autostash"); err != nil {
		fmt.Fprintf(stderr, "[sync] git pull failed in %s: %v\n", root, err)
		return ui.SyncStatus{State: ui.SyncStateWarn, Message: "pull failed"}
	}
	return ui.SyncStatus{State: ui.SyncStateOK}
}

func pushHostsConfig(stderr io.Writer) ui.SyncStatus {
	dir, err := hosts.ConfigDir()
	if err != nil {
		return ui.SyncStatus{State: ui.SyncStateWarn, Message: "no config.d"}
	}
	root, ok := gitRoot(dir)
	if !ok {
		return ui.SyncStatus{State: ui.SyncStateHidden}
	}
	_ = runQuietGit(root, "add", ".")
	status := strings.TrimSpace(gitOutput(root, "status", "--porcelain"))
	if status != "" {
		if err := runQuietGit(root, "commit", "-m", "Update aoo hosts"); err != nil {
			fmt.Fprintf(stderr, "[sync] git commit failed in %s: %v\n", root, err)
			return ui.SyncStatus{State: ui.SyncStateWarn, Message: "commit failed"}
		}
	}
	if err := runQuietGit(root, "push"); err != nil {
		fmt.Fprintf(stderr, "[sync] git push failed in %s: %v\n", root, err)
		return ui.SyncStatus{State: ui.SyncStateWarn, Message: "push failed"}
	}
	return ui.SyncStatus{State: ui.SyncStateOK}
}

func gitRoot(dir string) (string, bool) {
	if _, err := os.Stat(dir); err != nil {
		return "", false
	}
	root := strings.TrimSpace(gitOutput(dir, "rev-parse", "--show-toplevel"))
	if root == "" {
		return "", false
	}
	return root, true
}

func runQuietGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func gitOutput(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out strings.Builder
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return out.String()
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
	if _, err := ui.ResolveTheme(fs.Arg(0)); err != nil {
		return err
	}
	saved, err := config.SetTheme(fs.Arg(0))
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "configured theme: %s\n", saved)
	return nil
}

func runThemes(stdout io.Writer) error {
	for _, name := range ui.ThemeNames() {
		fmt.Fprintln(stdout, name)
	}
	return nil
}

func printUsage(w io.Writer) {
	name := cliName()
	fmt.Fprintf(w, `%s — quick SSH host picker

Usage:
  %s                    fuzzy-pick host and ssh into it
  %s prod               start with a query
  Ctrl+N in picker      add a new login variant interactively
  %s add NAME HOST      also works for scripted adding
  %s list               print saved/imported hosts
  %s config show        show config/hosts paths

Add options:
  -user USER  -p PORT  --tag TAG  --desc TEXT  --args "-A -J jump"

Hosts are kept in one file: ~/.config/aoo/hosts.yaml
Aliases from ~/.ssh/config are also shown automatically.
`, name, name, name, name, name, name)
}

func cliName() string {
	if name := strings.TrimSpace(filepath.Base(os.Args[0])); name != "" {
		return name
	}
	return "f"
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
	dir := strings.TrimSpace(*workDir)
	if dir == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			return err
		}
		dir = filepath.Join(cacheDir, "aoo", "source")
	}
	fmt.Fprintf(stdout, "[upgrade] repo: %s\n[upgrade] source: %s\n[upgrade] target: %s\n", safeRepoURL(*repoURL), dir, target)
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
	if value := strings.TrimSpace(os.Getenv("AOO_UPGRADE_REPO")); value != "" {
		return value
	}
	return "https://git.dawq.me/sergeyb/aoo.git"
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
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
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
		return runUpgradeGit(dir, stdout, stderr, "pull", "--ff-only", "origin", branch)
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
