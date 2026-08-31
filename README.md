# f / aoo

A tiny SSH login picker. Run `f`, type fuzzy keywords, choose a login variant, press `Enter`, and it opens `ssh`.
Search is always command/SSH search: `:` and `>` prefixes still work, but are no longer needed.

## Main flow

```bash
f             # fuzzy-pick and ssh
f prod db     # start with query "prod db"
```

Add a new login variant from the picker:

```text
Ctrl+N        # add new login
```

After `Ctrl+N`, `f` asks for alias, host, user, port, tags, and description.

## Extra commands

```bash
f add NAME HOST      # non-interactive/scripted add
f list               # print hosts/ssh commands
f config show        # show file paths
```

Examples:

```bash
f add nas root@192.168.88.10 --tag home --desc "home NAS"
f add db 10.20.30.40 -user admin -p 2222 --tag prod --args "-A -J jump"
```

Hosts are stored in one file:

```text
~/.config/aoo/hosts.yaml
```

Aliases from `~/.ssh/config` are imported automatically.

## Keys

- type to filter
- `↑/↓` or `ctrl+k/ctrl+j` to move
- `Enter` to ssh
- `Ctrl+N` to add a new login variant
- `Alt+Enter` / `Ctrl+Y` to print the command only
- `Esc` / `Ctrl+C` to quit
