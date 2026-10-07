# f

`f` is a personal OpenSSH picker presented as a compact Bubble Tea application.

It reads normal OpenSSH config, fuzzy-searches every SSH entry type in one list, and runs real `ssh`. Imported SSH entries are not copied into a private host database.

## Interface

Both sizes use a flat interface without an outer frame or background container. In compact mode it stays near the left edge with a small margin, remains width-capped, and keeps the configured fixed height even when filtering leaves only a few results, so the command block and footer never jump. The picker always uses the terminal's alternate screen, so quitting restores the previous shell screen instead of leaving UI rows behind. The UI is built with Bubble Tea, Bubbles (`textinput`, `spinner`), and Lip Gloss:

- small colored active-category badge and counter;
- query line without a painted background or reverse-video block cursor, so transparent terminal backgrounds stay clean;
- match counter;
- ordinary results show only the complete entry name; the selected entry's composed command appears in a fixed two-line lower block, so moving the cursor never changes row heights; long commands use an explicit middle omission while retaining the target tail, and `Tab` shows the complete source-style block; entry names are never shortened;
- colored fuzzy-match characters;
- slim colored marker for the selected row;
- `Tab` detail view for imported SSH entries shows the concrete OpenSSH block (`Host` plus every directive) without duplicate service metadata; `Ctrl+Y` prints that same block to the terminal; non-SSH command entries retain their command details;
- muted key-binding line without tabs or preview panes.

Choose the size:

```bash
f config ui compact      # compact picker; terminal content is restored on exit
f config ui full-screen  # the same picker using the full screen
```

Place compact mode at either edge:

```bash
f config layout bottom
f config layout top
```

Show the selected entry's composed command in a fixed lower block (the legacy `address` setting name remains for compatibility):

```bash
f config address on
f config address off     # hide the fixed command block
```

Legacy config values migrate automatically: `light` → `compact`, `full` → `full-screen`.

### Compact, bottom

![Compact picker at the bottom](docs/screenshots/compact-bottom.png)

### Compact, top

![Compact picker at the top](docs/screenshots/compact-top.png)

### Compact, top with fixed command block

![Compact picker with composed SSH commands](docs/screenshots/compact-top-address.png)

### Full-screen

![Full-screen picker](docs/screenshots/full-screen.png)

## Install on a new machine

```bash
curl -fsSL https://raw.githubusercontent.com/sergeybychkovvvpgroup-beep/f/main/install.sh | sh
f setup
```

The installer downloads the latest public GitHub release. On Debian/Ubuntu it installs the release `.deb` through `dpkg`; without root or `sudo`, it installs the release binary into `~/.local/bin/f`.

`f setup` asks for the SSH inventory repository, defaulting to `git@git.dawq.me:sergeyb/sshconfig.git`. It verifies both clone and dry-run push access before changing SSH config, using normal SSH host-key verification. For an SSH authentication failure, it creates or reuses `~/.ssh/id_ed25519`, derives and validates the matching public key, prints only that public key for a read/write deploy key, and tells you to rerun `f setup`. Host-key failures require verifying the server fingerprint and updating `known_hosts`; HTTPS failures require HTTPS credentials with write access.

The built-in update checker uses the public GitHub repository by default. Override it temporarily with `F_UPGRADE_REPO` when needed.

To publish the current machine's existing hosts as the initial repository contents:

```bash
f setup --adopt <ssh-config-repo-url>
```

## Quick start

```bash
f                       # open picker
f prod db               # start with a query
f list                  # print known entries
f config show           # show config paths
f config                # show current settings, paths, and available config commands
f config sync           # pull host changes
f config ui compact     # compact UI
f config ui full-screen # full-screen UI
f config layout bottom  # compact at bottom
f config layout top     # compact at top
f config height 20      # compact picker height (minimum 6)
f config address on     # fixed lower command block; result rows remain name-only
f setup                 # interactive repository setup and access check
```

The settings are stored in `~/.config/f/config.yaml` (or the path printed by `f config`). The address command changes the YAML key `show_address`; every modifying config command prints the changed key and the exact file path.

Keys:

- type to filter;
- `↑` / `↓`, `Ctrl+K` / `Ctrl+J` to move through results in visual top-to-bottom order in every layout;
- `Tab` to open details for the selected entry; `q` or `Tab` returns to the list, while `↑` / `↓` scroll long details;
- `Enter` to run the selected SSH command;
- `Ctrl+Y` / `Alt+Enter` to print the selected SSH config block without running it (or the command for non-SSH entries);
- `Ctrl+E` / `Alt+E` to open the complete source SSH config in `$EDITOR` at the selected entry;
- `Ctrl+N` to add a host;
- `Esc` / `Ctrl+C` to quit.

## Unified search

An ordinary query searches SSH logins, jump routes, port forwards, and `RemoteCommand` entries together; no prefix is required.

Category bindings filter without clearing the query: `F1` all, `F2` hosts, `F3` commands, `F4` forwards, and `F5` jumps. A muted one-line legend is shown in the status bar.

## SSH config model

The expected active file is:

```text
~/.ssh/config.d/f_hosts/f.conf
```

`~/.ssh/config` should include:

```ssh-config
Include ~/.ssh/config.d/f_hosts/*.conf
```

Imported entries execute through the exact alias:

```bash
ssh alias-name
```

This preserves OpenSSH behavior for `ProxyJump`, forwards, `RemoteCommand`, identity files, legacy algorithms, and other options.

## Editing and sync

`Ctrl+E` exits the TUI and opens the complete source SSH config in `$EDITOR` (`nano` fallback) at the selected entry's line. For the shared inventory this is `~/.ssh/config.d/f_hosts/f.conf`; the old temporary one-block editor is no longer used. After the editor exits, `f` validates the file with `ssh -G` and restores the original content if validation fails.

When `~/.ssh/config.d/f_hosts` is a Git repository, `f` pulls on startup and commits/pushes after edits or `f add`.

## Display conventions

- obsolete noisy prefixes on imported aliases are hidden only in display names;
- route suffixes become labels such as `[netbird emergency]`, `[jump]`, and `[tunnel]`;
- forwards include their remote endpoint;
- real aliases and commands remain unchanged.

## Adding entries

```bash
f add NAME HOST
f add db 10.20.30.40 -user admin -p 2222 --args "-A -J jump"
```

`Ctrl+N` starts interactive creation. New entries are stored as OpenSSH blocks in `~/.ssh/config.d/f_hosts/f.conf`.

## Build from source

```bash
go test ./...
go build -o ~/.local/bin/f ./cmd/f
```

Production builds should embed the exact commit:

```bash
commit=$(git rev-parse HEAD)
go build -buildvcs=false -ldflags "-X f/internal/app.buildCommit=$commit" -o ~/.local/bin/f ./cmd/f
```
