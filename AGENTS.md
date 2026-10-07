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
- `show_address: true | false` is the compatibility name persisted in the same file; `f config address on|off` toggles a fixed two-line command block at the bottom of the picker. Ordinary rows show only complete entry names and never show the old `user@host` target preview. Entry names are never truncated: wrap long names on explicit display-cell-safe lines and count those physical lines in the viewport budget. Moving the selection must never change row heights or the command block position. Show the selected command on one line with display-width-safe middle omission when required, preserving the target tail. `Tab` provides the complete source-style block.
- `f config` and `f config show` print the current UI settings, exact config file, SSH inventory paths, and available setters. Every setter prints the persisted YAML key and config path; `f config height N` sets `picker_height` with a minimum of 6.
- `picker_height` controls compact-mode height; full-screen always uses the terminal height.
- Legacy `light` migrates to `compact`; legacy `full` migrates to `full-screen`.
- `focus_mode`, `show_list_on_start`, `two_line_results`, `show_match_context`, and `full_screen` are obsolete and must be removed when rewriting config. Do not revive legacy `two_line_results`; `show_address` is its narrow supported replacement.
- Both modes use the same flat, pink-accented, preview-free interface with an active-category badge and Bubbles `textinput`/`spinner` components. Compact mode stays near the left edge, is capped at 96 terminal cells, and always keeps the configured `picker_height`; short or filtered result sets leave reserved blank rows so the command block and footer never move. Full-screen keeps the terminal height.
- Both `compact` and `full-screen` use the alternate screen so every exit path restores the previous terminal contents without leaving picker rows behind. Bubble Tea must restore the screen before command/editor handoff.
- `compact` supports `layout: top | bottom`, changed by `f config layout top|bottom`; `full-screen` always renders from the top.
- Keep the flat terminal-native layout; the fixed lower command block is the only preview area. Do not add an outer frame, mode tabs, split panes, floating cards, or a separate help footer.
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
Ctrl+E       edit the complete source SSH config at the selected line
Ctrl+N       add custom host
Esc/Ctrl+C   quit
```

The `Tab` view for an imported OpenSSH entry shows only its concrete config block: `Host <alias>` followed by every directive in source order. Do not duplicate name/address/kind/mode/about/source metadata around that block. `Ctrl+Y` prints the same block verbatim to the restored terminal; non-SSH command entries retain the generic details and command-print fallback. Long details scroll with `Up`/`Down` or `Ctrl+K`/`Ctrl+J`; `q` or `Tab` returns to the unchanged result list, `Esc` has no details action, and `Enter` still runs `ssh <alias>`.

`Ctrl+E` exits TUI and opens the complete `SourcePath` in `$EDITOR`/`nano` at `SourceLine`, so the operator sees the full inventory and surrounding entries. Validate the edited source with `ssh -G -F`; restore the original bytes on failure. Do not reintroduce a generated temporary one-block editor.

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

Expanded command generation must preserve every directive in the selected concrete `Host` block. Render `HostName`/`User` as the target and every other directive in source order as shell-safe `-o Key=Value`. Execution still uses the alias. Ordinary rows show only the full entry name, using explicit display-cell-aware wrapping when needed. The selected command is rendered only in the fixed two-line lower block; if it does not fit on one line, use a display-width-safe middle `…` and retain the target tail. `Tab` and `Ctrl+Y` use the complete source-style `Host` block.

## Host storage and sync

New custom hosts are written back to ordinary OpenSSH config blocks in:

```text
~/.ssh/config.d/f_hosts/f.conf
```

Legacy YAML files may still be read for compatibility, but they are not the write path for new additions.

If `~/.ssh/config.d/f_hosts` is a git repository, host sync should behave like nb notes: pull before reading/writing where practical, then commit and push after host edits.

Fresh-machine setup is interactive when no URL is supplied: `f setup` prompts with `git@git.dawq.me:sergeyb/sshconfig.git` as the default, verifies clone plus a dry-run push before modifying SSH config, and prints `~/.ssh/id_ed25519.pub` when repository access is missing. Never print or copy the private key. The public install command is `curl -fsSL https://raw.githubusercontent.com/sergeybychkovvvpgroup-beep/f/main/install.sh | sh`; Debian/Ubuntu uses the latest release `.deb`, with a user-local release archive fallback when root or `sudo` is unavailable.

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
