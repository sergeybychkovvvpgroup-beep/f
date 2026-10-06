# f development context for coding agents

## Project purpose

`f` is a personal OpenSSH picker, not a generic notes app anymore.

Primary goal: fast SSH/TUI workflow over normal OpenSSH config with a modern Bubble Tea mini-application UI.

Do not reintroduce separate hidden storage for imported SSH config entries. The tool should read and edit ordinary SSH config.

## Current source of truth

Active user SSH config is expected to be a single ordinary file:

```text
~/.ssh/config.d/f_hosts/f.conf
```

`~/.ssh/config` should include it through:

```ssh-config
Include ~/.ssh/config.d/f_hosts/*.conf
```

The historical/generated files may exist in archive directories, but active config.d should ideally have one `f.conf` file.

User edit action must write back to `~/.ssh/config.d/f_hosts/f.conf`, not a hidden DB or per-feature override files.

## Important UX decisions

- UI is Bubble Tea/Bubbles/Lipgloss.
- `ui_mode: compact | full-screen` is persisted in `~/.config/f/config.yaml`; `f config ui compact|full-screen` changes it.
- `show_address: true | false` is persisted in the same file; `f config address on|off` toggles a plain muted `user@host:port` row beneath each name.
- `f config` and `f config show` print the current UI settings, exact config file, SSH inventory paths, and available setters. Every setter prints the persisted YAML key and config path; `f config height N` sets `picker_height` with a minimum of 6.
- `picker_height` controls compact-mode height; full-screen always uses the terminal height.
- Legacy `light` migrates to `compact`; legacy `full` migrates to `full-screen`.
- `focus_mode`, `show_list_on_start`, `two_line_results`, `show_match_context`, and `full_screen` are obsolete and must be removed when rewriting config. Do not revive legacy `two_line_results`; `show_address` is its narrow supported replacement.
- Both modes use the same flat, pink-accented, preview-free interface with an active-category badge and Bubbles `textinput`/`spinner` components. Compact mode stays near the left edge, is capped at 96 terminal cells, and shrinks vertically to the filtered result count up to `picker_height`; full-screen keeps the configured terminal height.
- `compact` stays in the normal terminal buffer and supports `layout: top | bottom`, changed by `f config layout top|bottom`.
- `full-screen` uses the alternate screen and always renders from the top.
- Keep the flat terminal-native layout; do not add an outer frame, mode tabs, split panes, preview panels, or a separate help footer.
- Use the fixed modern fzf-like palette in `internal/ui/theme.go`: pink prompt and match accents, muted status/address text, and a slim pink selection marker.
- Avoid forced full background fill; transparent terminals made ANSI background painting fragile. Keep the text-input cursor hidden to prevent a reverse-video block artifact.
- Tags are currently intentionally hidden from UI.

## Search and hotkeys

Plain text searches SSH logins, jumps, forwards, and command entries together. Category bindings filter the current result set without clearing the query: `F1` all, `F2` hosts, `F3` commands, `F4` forwards, `F5` jumps. Do not add mode tabs or visible row prefixes.

Keys:

```text
Enter        run selected command
Tab          open/close selected entry details
Ctrl+Y       print command only
e            edit selected SSH config block
Ctrl+N       add custom host
Esc/Ctrl+C   quit
```

The `Tab` view shows the selected entry name, address, kind/mode, action description, complete multiline command, and source location. Long details scroll with `Up`/`Down` or `Ctrl+K`/`Ctrl+J`; `Tab` or `Esc` returns to the unchanged result list and `Enter` still runs the entry.

`e` currently exits TUI and opens `$EDITOR`/`nano`. This was intentional: SSH config blocks are multiline and editor-based editing is safer than a hurried inline modal. A later Bubble Tea popup may reuse the same read/write logic.

## Execution model

For imported SSH config entries, execution should remain:

```bash
ssh <alias>
```

Do not flatten execution into a hand-built command. Running the alias preserves OpenSSH semantics for options not modeled by f.

Expanded commands may be used by non-picker output, but the picker itself has no preview pane.

## Display name rules

Do not mutate the actual alias used for execution. Only display names are normalized.

Current display cleanup:

- hide obsolete noisy prefixes from imported aliases;
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
~/.ssh/config.d/f_hosts/f.conf
```

Legacy YAML files may still be read for compatibility, but they are not the write path for new additions.

If `~/.ssh/config.d/f_hosts` is a git repository, host sync should behave like nb notes: pull before reading/writing where practical, then commit and push after host edits.

## Build, test, install

Always run before pushing code changes:

```bash
go test ./...
go build -o ~/.local/bin/f ./cmd/f
```

Cross-build for office Linux amd64:

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/f-linux-amd64 ./cmd/f
scp /tmp/f-linux-amd64 sergeyb@192.168.41.138:/tmp/f.new
ssh sergeyb@192.168.41.138 'sh -lc '\''install -m 0755 /tmp/f.new ~/.local/bin/f; rm -f /tmp/f.new; ~/.local/bin/f version'\'''
```

## Self-update behavior

- The update-check cache is valid only when its commit matches the running binary's embedded `buildCommit`.
- If the cached commit differs from the running build, refresh remote `HEAD` instead of offering the cached commit.
- Before replacing the binary, verify with Git ancestry that the running commit is present in the update checkout and is an ancestor of the target commit. Refuse missing, divergent, or older targets; a different remote SHA is not by itself proof of an upgrade.
- Never deploy a commit until the public update repository and every required mirror resolve to that exact SHA.
- Production builds must embed the exact Git commit with `-ldflags "-X f/internal/app.buildCommit=$(git rev-parse HEAD)"`.

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
- Keep `~/.ssh/config.d/f_hosts` consolidated; avoid adding new generated active `.conf` files unless the user explicitly asks.
