# f / aoo

`f` / `aoo` is a personal OpenSSH picker with a modern `fzf`-style terminal UI.

It reads normal OpenSSH config, fuzzy-searches every SSH entry type in one list, and runs real `ssh`. Imported SSH entries are not copied into a private host database.

## Interface

Frames, tabs, preview panes, and the former large split UI have been removed. Both sizes use the same minimal interface:

- query line;
- match counter;
- single-line results;
- colored fuzzy-match characters;
- high-contrast selected row.

Choose the size:

```bash
f config ui compact      # compact picker in the current terminal
f config ui full-screen  # the same picker using the full screen
```

Place compact mode at either edge:

```bash
f config layout bottom
f config layout top
```

Legacy config values migrate automatically: `light` → `compact`, `full` → `full-screen`.

### Compact, bottom

![Compact picker at the bottom](docs/screenshots/compact-bottom.png)

### Compact, top

![Compact picker at the top](docs/screenshots/compact-top.png)

### Full-screen

![Full-screen picker](docs/screenshots/full-screen.png)

## Install on a new machine

```bash
curl -fsSL https://raw.githubusercontent.com/sergeybychkovvvpgroup-beep/f/main/install.sh | sh
f setup <ssh-config-repo-url>
```

The installer adds `f` and the legacy `aoo` symlink. `f setup` clones the hosts repository into `~/.ssh/config.d/aoo_hosts` and ensures `~/.ssh/config` includes `~/.ssh/config.d/aoo_hosts/*.conf`.

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
f config sync           # pull host changes
f config ui compact     # compact UI
f config ui full-screen # full-screen UI
f config layout bottom  # compact at bottom
f config layout top     # compact at top
```

Keys:

- type to filter;
- `↑` / `↓`, `Ctrl+K` / `Ctrl+J` to move through results in visual top-to-bottom order in every layout;
- `Enter` to run the selected SSH command;
- `Ctrl+Y` / `Alt+Enter` to print without running;
- `Ctrl+E` / `Alt+E` to edit the selected SSH config block;
- `Ctrl+N` to add a host;
- `Esc` / `Ctrl+C` to quit.

## Unified search

Normal SSH logins, jump routes, port forwards, and `RemoteCommand` entries share one fuzzy result list. There are no tabs and no `F1`–`F4` filters.

## SSH config model

The expected active file is:

```text
~/.ssh/config.d/aoo_hosts/aoo.conf
```

`~/.ssh/config` should include:

```ssh-config
Include ~/.ssh/config.d/aoo_hosts/*.conf
```

Imported entries execute through the exact alias:

```bash
ssh alias-name
```

This preserves OpenSSH behavior for `ProxyJump`, forwards, `RemoteCommand`, identity files, legacy algorithms, and other options.

## Editing and sync

`Ctrl+E` exits the TUI and opens `$EDITOR` (`nano` fallback). The saved block is upserted into `~/.ssh/config.d/aoo_hosts/aoo.conf` between `# aoo-edit begin/end` markers.

When `~/.ssh/config.d/aoo_hosts` is a Git repository, `aoo` pulls on startup and commits/pushes after edits or `f add`.

## Display conventions

- noisy `ssh-` and `aoo-` prefixes are hidden only in display names;
- route suffixes become labels such as `[netbird emergency]`, `[jump]`, and `[tunnel]`;
- forwards include their remote endpoint;
- real aliases and commands remain unchanged.

## Adding entries

```bash
f add NAME HOST
f add db 10.20.30.40 -user admin -p 2222 --args "-A -J jump"
```

`Ctrl+N` starts interactive creation. New entries are stored as OpenSSH blocks in `~/.ssh/config.d/aoo_hosts/aoo.conf`.

## Build from source

```bash
go test ./...
go build -o ~/.local/bin/f ./cmd/f
ln -sf f ~/.local/bin/aoo
```

Production builds should embed the exact commit:

```bash
commit=$(git rev-parse HEAD)
go build -buildvcs=false -ldflags "-X aoo/internal/app.buildCommit=$commit" -o ~/.local/bin/f ./cmd/f
```
