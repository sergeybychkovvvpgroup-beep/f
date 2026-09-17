# f / aoo

`f` / `aoo` is a personal OpenSSH picker and launcher.

It is intentionally small: it reads normal OpenSSH config, shows a fuzzy TUI, and then runs real `ssh`. There is no private host database for imported SSH entries.

Current UX direction is inspired by `sshelf`: top search box, compact list on the left, selected entry details on the right.

## Install on a new machine

One command installs `f` and the legacy `aoo` symlink:

```bash
curl -fsSL https://git.dawq.me/sergeyb/aoo/raw/branch/main/install.sh | sh
```

Then attach your hosts repository:

```bash
f setup <ssh-config-repo-url>
```

`f setup` clones the repo into `~/.ssh/config.d`, ensures `~/.ssh/config` has `Include ~/.ssh/config.d/*.conf`, and the machine is ready to use. To publish the current machine's existing `~/.ssh/config.d` as the initial repo contents, run `f setup --adopt <ssh-config-repo-url>` once on that machine.

## Quick start

```bash
f             # open picker
f prod db     # open picker with initial query
f list        # print known entries
f config show # show config paths
f config sync # pull latest host changes
```

In the picker:

- type to filter;
- `Enter` runs the selected SSH command;
- `Ctrl+Y` / `Alt+Enter` prints the command without running;
- `e` edits the selected SSH config block;
- `Esc` / `Ctrl+C` quits.

## Modes

Entries are grouped automatically from SSH config syntax:

| Key | Mode | Detection |
| --- | --- | --- |
| `F1` | general SSH logins | default ordinary SSH entries |
| `F2` | jumps | `ProxyJump` / `ProxyCommand` |
| `F3` | port forwards | `LocalForward` / `RemoteForward` / `DynamicForward` |
| `F4` | commands | `RemoteCommand` or custom command entries |

The current mode is shown in the top status line.

## SSH config model

`aoo` works with normal OpenSSH config files.

The expected active file is:

```text
~/.ssh/config.d/aoo.conf
```

`~/.ssh/config` should include it, commonly via:

```ssh-config
Include ~/.ssh/config.d/*.conf
```

The picker reads `~/.ssh/config` and follows `Include` directives. For imported SSH config entries, execution uses the exact alias:

```bash
ssh alias-name
```

This preserves OpenSSH behavior for `ProxyJump`, `LocalForward`, `RemoteCommand`, legacy algorithms, forwards, and other options.

The right preview shows a best-effort expanded multiline command for readability, but execution still uses the alias.

## Editing

Press `e` on an entry to edit it.

Behavior:

1. `aoo` exits the TUI and opens `$EDITOR` (`nano` fallback).
2. It writes an editable SSH config block generated from `ssh -G <alias>`.
3. On save, it upserts the marked block into:

```text
~/.ssh/config.d/aoo.conf
```

The block is wrapped with markers:

```ssh-config
# aoo-edit begin alias-name
Host alias-name
  HostName ...
  User ...
# aoo-edit end alias-name
```

Generated or imported source files are not rewritten; `~/.ssh/config.d/aoo.conf` is the source of truth for user edits. If `~/.ssh/config.d` is a git repository, aoo pulls on start and commits/pushes after `e` edits or `f add`, so host sync works in both directions similarly to nb notes.

## Display conventions

The left pane is intentionally short:

- only the readable entry name is shown;
- noisy prefixes such as `ssh-` and `aoo-` are hidden in display names;
- route suffixes are humanized, for example:

```text
ssh-pve-beria-03-netrack-netbird-emergency
```

is displayed as:

```text
pve-beria-03-netrack [netbird emergency]
```

For forwards, the remote endpoint is included in the short name:

```text
omada-chashnikovo [tunnel] [remote 10.117.100.10:443]
```

The right pane contains selected-entry details:

- name;
- description;
- short command;
- full command, formatted multiline when useful.

Tags are currently not shown in the UI.

## Adding entries

```bash
f add NAME HOST
f add db 10.20.30.40 -user admin -p 2222 --args "-A -J jump"
```

`Ctrl+N` in the picker also starts interactive add.

New entries are written as marked OpenSSH blocks into `~/.ssh/config.d/aoo.conf`. Legacy YAML hosts under `~/.config/aoo/` are still read for compatibility, but new work should live in the SSH config repository.

## Build and install from source

```bash
go test ./...
go build -o ~/.local/bin/f ./cmd/f
ln -sf f ~/.local/bin/aoo
```

Cross-build example for Linux amd64:

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/aoo-f-linux-amd64 ./cmd/f
```
