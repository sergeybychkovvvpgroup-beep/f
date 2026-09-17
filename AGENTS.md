# AOO development context for coding agents

## Project purpose

`aoo` / `f` is a personal OpenSSH picker, not a generic notes app anymore.

Primary goal: fast SSH/TUI workflow over normal OpenSSH config with a compact `sshelf`-like Bubble Tea UI.

Do not reintroduce separate hidden storage for imported SSH config entries. The tool should read and edit ordinary SSH config.

## Current source of truth

Active user SSH config is expected to be a single ordinary file:

```text
~/.ssh/config.d/aoo.conf
```

`~/.ssh/config` should include it through:

```ssh-config
Include ~/.ssh/config.d/*.conf
```

The historical/generated files may exist in archive directories, but active config.d should ideally have one `aoo.conf` file.

User edit action must write back to `~/.ssh/config.d/aoo.conf`, not a hidden DB and not per-feature override files like `00-aoo-user.conf`.

## Important UX decisions

- UI is Bubble Tea/Bubbles/Lipgloss.
- Wide layout is split-pane:
  - top framed search/status box;
  - left compact result list;
  - right preview/details pane;
  - bottom help line.
- Theme should stay close to `sshelf`.
- Avoid forced full background fill; transparent terminals made ANSI background painting fragile.
- Left pane should be short and readable.
- Right pane should contain all details:
  - name;
  - description;
  - short command;
  - full command, multiline when useful.
- Tags are currently intentionally hidden from UI.

## Modes and hotkeys

Picker modes:

```text
F1 general   ordinary SSH logins
F2 jumps     ProxyJump / ProxyCommand
F3 forwards  LocalForward / RemoteForward / DynamicForward
F4 commands  RemoteCommand / custom command entries
```

Other keys:

```text
Enter        run selected command
Ctrl+Y       print command only
e            edit selected SSH config block
Ctrl+N       add custom host
Esc/Ctrl+C   quit
```

`e` currently exits TUI and opens `$EDITOR`/`nano`. This was intentional: SSH config blocks are multiline and editor-based editing is safer than a hurried inline modal. A later Bubble Tea popup may reuse the same read/write logic.

## Execution model

For imported SSH config entries, execution should remain:

```bash
ssh <alias>
```

Do not flatten execution into a hand-built command. Running the alias preserves OpenSSH semantics for options not modeled by aoo.

The right preview may show a best-effort expanded command for readability only.

## Display name rules

Do not mutate the actual alias used for execution. Only display names are normalized.

Current display cleanup:

- hide `ssh-` and `aoo-` prefixes;
- humanize route suffixes:
  - `-netbird-emergency` -> `[netbird emergency]`;
  - `-netbird` -> `[netbird]`;
  - `-router-wh-jump` -> `[router wh jump]`;
  - `-jump` -> `[jump]`;
  - `-tunnel` -> `[tunnel]`.
- forward entries append remote endpoint, e.g.:

```text
omada-chashnikovo [tunnel] [remote 10.117.100.10:443]
```

## SSH parsing and preview

The loader reads `~/.ssh/config` and follows `Include` directives.

Classification is based on parsed directives:

- `LocalForward`, `RemoteForward`, `DynamicForward` -> forwards;
- `RemoteCommand` -> commands;
- `ProxyJump`, `ProxyCommand` -> jumps.

Expanded previews should include important directives:

- `-p` for non-default port;
- `-J` for `ProxyJump`;
- `-L` / `-R` / `-D` for forwards;
- `-i` for identity file when explicitly present;
- selected `-o` options such as legacy algorithms and known_hosts behavior;
- remote command as trailing command.

Long commands should render multiline with backslashes. Group flag/value pairs together, for example:

```bash
ssh \
  -J sergeyb@100.126.127.82 \
  -L 8443 10.117.100.10:443 \
  root@192.168.11.10
```

## Host storage and sync

New custom hosts are written back to ordinary OpenSSH config blocks in:

```text
~/.ssh/config.d/aoo.conf
```

Legacy YAML files under `~/.config/aoo/config.d/*.yaml` and `~/.config/aoo/hosts.yaml` may still be read for compatibility, but they are not the write path for new additions.

If `~/.ssh/config.d` is a git repository, host sync should behave like nb notes: pull before reading/writing where practical, then commit and push after host edits.

## Build, test, install

Always run before pushing code changes:

```bash
go test ./...
go build -o ~/.local/bin/f ./cmd/f
cp ~/.local/bin/f ~/.local/bin/aoo
```

`~/.local/bin/aoo` may be a symlink to `f` on some hosts.

Cross-build for office Linux amd64:

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/aoo-f-linux-amd64 ./cmd/f
scp /tmp/aoo-f-linux-amd64 sergeyb@192.168.41.138:/tmp/f.new
ssh sergeyb@192.168.41.138 'sh -lc '\''install -m 0755 /tmp/f.new ~/.local/bin/f; rm -f /tmp/f.new; ~/.local/bin/f version'\'''
```

## Git workflow

Repo: `https://git.dawq.me/sergeyb/aoo.git`

The user may provide a Gitea token. Do not print tokens in final answers. Use `http.extraHeader` when needed and avoid persisting secrets in git config.

Commit and push meaningful changes to `main` after tests pass.

## Recent important commits

- `652ff4d Save SSH edits into common config file`
- `745ee67 Add SSH config edit action`
- `ca9bc06 Include forward flags in expanded previews`
- `013f77f Show remote endpoint in forward names`
- `3efd88f Add picker modes for logins jumps forwards commands`
- `9b94a7a Simplify SSH rows and multiline command previews`
- `36509eb Add sshelf-inspired default theme`

## Known follow-ups

- Replace editor-based `e` with an inline Bubble Tea popup only if it can safely edit multiline SSH config blocks.
- Improve normalization of route/display names if more noisy aliases appear.
- Consider frecency sorting later, but do not hide exact fuzzy-search behavior.
- Keep `~/.ssh/config.d` consolidated; avoid adding new generated active `.conf` files unless the user explicitly asks.
