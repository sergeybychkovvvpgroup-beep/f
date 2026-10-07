# f development context for coding agents

## Scope

`f` is a personal terminal-native OpenSSH picker built with Bubble Tea. Keep this file limited to durable project rules. Release versions, commit IDs, deployment state, temporary tasks, incident history, and future ideas do not belong here.

## Source of truth

- Read imported hosts from ordinary OpenSSH configuration, including `Include` directives.
- The default local writable inventory is `~/.ssh/config.d/f_hosts/f.conf`; honor explicitly configured inventory paths on other installations.
- `~/.ssh/config` should include `~/.ssh/config.d/f_hosts/*.conf` globally, before any `Host` or `Match` block.
- Do not create a hidden host database or per-feature override files.
- New hosts and edits must be written back as ordinary OpenSSH blocks in the canonical inventory.
- Legacy YAML may be read for compatibility, but is not the write path.

## UI contract

- Preserve the flat terminal-native interface: no outer frame, mode tabs, split panes, floating cards, heavy chrome, or additional preview panes.
- Supported modes are `compact` and `full-screen`; compact supports `top` and `bottom` layouts.
- Compact mode always occupies the configured `picker_height`. Pad short or filtered result sets so selection changes and filtering never move the command block or footer.
- Full-screen mode uses the available terminal height.
- Both modes use the alternate screen and must restore the previous terminal contents on every exit or command/editor handoff.
- Ordinary result rows show only entry names. Never restore the old inline `user@host` preview.
- Wrap complete names using display-cell-aware logic whenever they fit within the fixed result-region budget. If an extreme viewport makes the complete name physically impossible to display, use an explicit bounded `…` rather than terminal autowrap, silent clipping, or removing the name entirely.
- When `show_address` is enabled, render the selected composed command in the fixed two-line lower block: a muted `command` label and one bounded command line.

- Shorten long commands with display-cell-aware middle omission while preserving the final SSH target.
- Keep the transparent terminal background, hidden text-input cursor, slim selection marker, restrained pink accents, and muted metadata/footer.
- Tags and kind prefixes remain hidden from result rows.

## Search and controls

- Search hosts, commands, forwards, and jumps together; do not split them into separate modes.
- `F1`–`F5` filter the current query by all, hosts, commands, forwards, and jumps without clearing it.
- Preserve these controls:

```text
Enter        run selected entry
Tab          open/close details
Ctrl+Y       print the SSH config block, or command for non-SSH entries
Ctrl+E       edit the complete source SSH config at the selected line
Ctrl+N       add a custom host
Esc/Ctrl+C   quit
```

- In details, `q` or `Tab` returns without changing list state; `Esc` is ignored; arrows and `Ctrl+J`/`Ctrl+K` scroll long content.
- For imported SSH entries, details and `Ctrl+Y` use the concrete source-style `Host <alias>` block with directives in source order and no duplicate metadata wrapper.

## SSH semantics

- Execute imported entries as `ssh <alias>` so OpenSSH remains authoritative for unmodeled options.
- Never replace execution with the composed display command.
- The composed display command is display-only. It uses `HostName` and `User` for the target and must represent every other directive from the selected concrete `Host` block as shell-safe `-o Key=Value` arguments in source order.
- Classification is metadata only:
  - `LocalForward`, `RemoteForward`, `DynamicForward` → forward;
  - `RemoteCommand` → command;
  - `ProxyJump`, `ProxyCommand` → jump.
- Preserve aliases exactly for execution. Display-name normalization must not mutate the actual alias.
- Different users, ports, identities, proxy routes, forwards, remote commands, or SSH options are not duplicates merely because they share a hostname or IP.

## Editing and synchronization

- `Ctrl+E` opens the complete source file at the selected `Host` line; do not use a generated one-block temporary editor.
- Validate edited SSH configuration before accepting it and restore the original bytes on failure.
- If the inventory directory is a Git repository, pull/sync must be clean and fast-forward-only. Never reset, merge, rebase, commit, or overwrite local changes as part of a read-only sync.
- Do not push during ordinary picker startup or read-only sync. Commit and push only after an explicit inventory edit successfully changes the source of truth.
- Never weaken SSH host-key verification.
- Never print, copy, or store private keys, passwords, tokens, or complete protected allowlists. Access-failure guidance may show only the required public key.

## Configuration compatibility

- Persist the supported settings `ui_mode`, `layout`, `picker_height`, and `show_address` in `~/.config/f/config.yaml`; `show_address` defaults to false.
- `picker_height` has a minimum of 6.
- Preserve the supported setters: `f config ui compact|full-screen`, `f config layout top|bottom`, `f config height N`, and `f config address on|off`.
- Migrate legacy `light` to `compact` and `full` to `full-screen`.
- Remove obsolete `focus_mode`, `show_list_on_start`, `two_line_results`, `show_match_context`, and `full_screen` keys when rewriting configuration.
- `f config` and `f config show` must report current values and exact config/inventory paths; setters must report the changed key and file.

## Build and verification

- Develop in the local repository, not on a production workstation.
- Use test-driven development for behavior changes and regression fixes.
- Before completion or publication, format changed Go files and run:

```bash
gofmt -w <changed-go-files>
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

- Exercise UI changes in a real TTY. Cover compact top, compact bottom, full-screen, filtering to many/one/zero matches, narrow viewports, Unicode, selection movement, details, cancellation, and terminal restoration as applicable.
- Regenerate screenshots and update both READMEs when user-visible UI behavior changes.
- Production builds must embed the exact source commit in `buildCommit`.
- Before publication or deployment, verify every required source remote resolves to the reviewed commit; read back branch and tag SHAs after publishing.
- Back up the active binary before replacement, then verify the installed path, version, SHA-256, inventory count, and affected behavior in a real TTY.

## Git and release safety

- Public source/update repository: `https://github.com/sergeybychkovvvpgroup-beep/f.git`.
- Preserve the configured Gitea mirror; never force-push or discard divergence.
- Self-update must reject missing, divergent, identical, or older targets and must never offer a downgrade from stale cache data.
- `F_UPGRADE_REPO` is the primary updater override; retain `AOO_UPGRADE_REPO` only as a compatibility fallback.
- Keep legacy names and environment variables only as compatibility fallbacks; do not expose them in normal UI or documentation.

## Setup safety

- Before validating edited or newly installed SSH configuration, recursively inspect every supported OpenSSH `Include` form, reject `Match exec`, and fail closed when inspection is incomplete or errors.
- Setup and inventory reconfiguration must be atomic: failed clone, pull, validation, include modification, or repository replacement must leave the previous SSH configuration and repository state intact.
