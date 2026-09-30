# AOO development context for coding agents

## Project purpose

`aoo` / `f` is a personal OpenSSH picker, not a generic notes app anymore.

Primary goal: fast SSH/TUI workflow over normal OpenSSH config with a modern, frameless `fzf`-style Bubble Tea UI.

Do not reintroduce separate hidden storage for imported SSH config entries. The tool should read and edit ordinary SSH config.

## Current source of truth

Active user SSH config is expected to be a single ordinary file:

```text
~/.ssh/config.d/aoo_hosts/aoo.conf
```

`~/.ssh/config` should include it through:

```ssh-config
Include ~/.ssh/config.d/aoo_hosts/*.conf
```

The historical/generated files may exist in archive directories, but active config.d should ideally have one `aoo.conf` file.

User edit action must write back to `~/.ssh/config.d/aoo_hosts/aoo.conf`, not a hidden DB and not per-feature override files like `00-aoo-user.conf`.

## Important UX decisions

- UI is Bubble Tea/Bubbles/Lipgloss.
- `ui_mode: compact | full-screen` is persisted in `~/.config/aoo/config.yaml`; `f config ui compact|full-screen` changes it.
- `picker_height` controls compact-mode height; full-screen always uses the terminal height.
- Legacy `light` migrates to `compact`; legacy `full` migrates to `full-screen`.
- `focus_mode`, `show_list_on_start`, `two_line_results`, `show_match_context`, and `full_screen` are obsolete and must be removed when rewriting config. The frameless picker always shows its single-line result list.
- Both modes use the same frameless, tabless, preview-free, single-line fzf-style result list.
- `compact` stays in the normal terminal buffer and supports `layout: top | bottom`, changed by `f config layout top|bottom`.
- `full-screen` uses the alternate screen and always renders from the top.
- Do not reintroduce frames, mode tabs, split panes, preview panels, or a help footer.
- Use the fixed modern fzf-like palette in `internal/ui/theme.go`: pink prompt and match accents, muted status text, and a dark selected-row background.
- Avoid forced full background fill outside the selected row; transparent terminals made ANSI background painting fragile.
- Tags are currently intentionally hidden from UI.

## Search and hotkeys

All SSH logins, jumps, forwards, and command entries share one fuzzy-search result set. Do not add mode tabs or `F1`–`F4` filtering.

Keys:

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

Expanded commands may be used by non-picker output, but the picker itself has no preview pane.

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

Entries retain internal classification for display and preview metadata, but classification must not split the picker search. It is based on parsed directives:

- `LocalForward`, `RemoteForward`, `DynamicForward` -> forwards;
- `RemoteCommand` -> commands;
- `ProxyJump`, `ProxyCommand` -> jumps.

Expanded command generation should include important directives:

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
~/.ssh/config.d/aoo_hosts/aoo.conf
```

Legacy YAML files under `~/.config/aoo/config.d/*.yaml` and `~/.config/aoo/hosts.yaml` may still be read for compatibility, but they are not the write path for new additions.

If `~/.ssh/config.d/aoo_hosts` is a git repository, host sync should behave like nb notes: pull before reading/writing where practical, then commit and push after host edits.

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

## Self-update behavior

- The update-check cache is valid only when its commit matches the running binary's embedded `buildCommit`.
- If the cached commit differs from the running build, refresh remote `HEAD` instead of offering the cached commit.
- Before replacing the binary, verify with Git ancestry that the running commit is present in the update checkout and is an ancestor of the target commit. Refuse missing, divergent, or older targets; a different remote SHA is not by itself proof of an upgrade.
- Never deploy a commit until the public update repository and every required mirror resolve to that exact SHA.
- Production builds must embed the exact Git commit with `-ldflags "-X aoo/internal/app.buildCommit=$(git rev-parse HEAD)"`.

## Git workflow

Public update/source repo: `https://github.com/sergeybychkovvvpgroup-beep/f.git`

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

- Replace editor-based `Ctrl+E` with an inline Bubble Tea popup only if it can safely edit multiline SSH config blocks.
- Improve normalization of route/display names if more noisy aliases appear.
- Consider frecency sorting later, but do not hide exact fuzzy-search behavior.
- Keep `~/.ssh/config.d/aoo_hosts` consolidated; avoid adding new generated active `.conf` files unless the user explicitly asks.
