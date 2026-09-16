# f / aoo

`f` / `aoo` is a personal OpenSSH picker and launcher.

It is intentionally small: it reads normal OpenSSH config, shows a fuzzy TUI, and then runs real `ssh`. There is no private host database for imported SSH entries.

Current UX direction is inspired by `sshelf`: top search box, compact list on the left, selected entry details on the right.

## Quick start

```bash
f             # open picker
f prod db     # open picker with initial query
f list        # print known entries
f config show # show config paths
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

Generated or imported source files are not rewritten; the single active config file is the source of truth for user edits.

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

Custom hosts are stored under:

```text
~/.config/aoo/config.d/*.yaml
```

Legacy `~/.config/aoo/hosts.yaml` is still read for compatibility.

## Themes

Default theme for new configs is `sshelf`.

```bash
f themes
f set-theme sshelf
```

## Build and install

```bash
go test ./...
go build -o ~/.local/bin/f ./cmd/f
ln -sf f ~/.local/bin/aoo
```

Cross-build example for Linux amd64:

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/aoo-f-linux-amd64 ./cmd/f
```
