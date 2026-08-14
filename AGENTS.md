# ConfigFacilitator Agent Notes

## Current Stack

- Language: Go
- Environment manager: pixi-managed Go toolchain
- Entry point: `cmd/cfgfc/main.go`
- npm distribution package: `npm/`, wrapping prebuilt Go release binaries for `npm install -g @jollyxenon/cfgfc`
- Local development system: Linux under WSL; native Windows symlink/path verification must run with native `cfgfc.exe`

## Implemented Command Surface

- Resource metadata: `project`, `column`, `setting`, and `mode` with `list`, `show`, `create`, `set`, `rename`, and `delete`
- Target structure: `column target list/add/set/delete` and `setting target list/set/reset`
- Current state: `current show`, `current column list/set/delete` — (Current) is itself the temporary Mode; `relation.kind` is `following` (edits to that Mode auto-sync Current) or `detached` (forked, preserved `originMode`)
- Context and inspection: `use <Project|global>` and `status` (reports `Current: following (Mode) [...]`)
- Activation: `apply mode`, `apply column`, `refresh`, `reset`, and one-step `revert` (Current-only history)
- Reconciliation: per-Project atomic `sync` — removes Index metadata for disappeared Project/Column/Setting sources, rebuilds missing Index entries from the filesystem, recreates a missing `current_state.json` as empty (deleting stale `history.log`), replans following Currents; `sync --all` aggregates per-Project success/failure
- Root selection: `root [Path]`, without content migration
- Automation: stable `--json` envelopes and exit codes `0`, `2`-`6`
- Local Web UI: `cfgfc web [--port 49631]` — embedded zero-dependency frontend over `/api/snapshot`, `/api/command`, `/api/preview`; mutating commands require the whole-warehouse `revision` (409 on conflict); apply commands accept `force` to bypass duplicate-target planning conflicts (later Column wins)
- Shell completion: `completion <bash|zsh|fish|powershell>` with scoped canonical-name and alias completion
- Removed without compatibility aliases: `new`, `switch`, `list`, `update`, flag-only apply, `-a`, `-f`, and `--force`

## Baseline Commands

- `pixi run test`
- `pixi run compile`
- `pixi run build`
- `pixi run help`
- Complete runnable-command help sweep from `docs/developer-setup.en.md` / `docs/developer-setup.zh-CN.md`
- `openspec validate replace-editor-workflows-with-cli --strict`
- `openspec status --change replace-editor-workflows-with-cli --json`
- `cd npm && npm pack --dry-run`
- `pixi run build && cd npm && CFGFC_BINARY_PATH=../dist/cfgfc npm install -g . && cfgfc --help`
- `cd npm && CFGFC_TEST_PLATFORM=freebsd CFGFC_TEST_ARCH=x64 node install.js` (expected unsupported-tuple failure path)

## Help Sweep

Cover every top-level family and every runnable nested command:

```bash
pixi run bash -lc '
commands=(
  "project list" "project show" "project create" "project set" "project rename" "project delete"
  "column list" "column show" "column create" "column set" "column rename" "column delete"
  "column target list" "column target add" "column target set" "column target delete"
  "setting list" "setting show" "setting create" "setting set" "setting rename" "setting delete"
  "setting target list" "setting target set" "setting target reset"
  "setting content list" "setting content read" "setting content write"
  "setting content mkdir" "setting content move" "setting content delete"
  "mode list" "mode show" "mode create" "mode set" "mode rename" "mode delete"
  "mode column list" "mode column set" "mode column delete"
  "current show" "current column list" "current column set" "current column delete"
  "use" "status" "apply mode" "apply column" "refresh" "sync" "root" "reset" "revert" "web"
  "completion bash" "completion zsh" "completion fish" "completion powershell"
)
for cmd in "${commands[@]}"; do
  go run ./cmd/cfgfc $cmd --help >/dev/null || exit 1
done
'
```

Also verify root help and rejection of removed syntax without mutation.

## Verification Expectations

- Use `pixi run test` for the full Go test suite and `pixi run compile` for compilation.
- Use `pixi run build` to create `dist/cfgfc` and `pixi run help` for the root surface.
- For command/workflow changes, run a real CLI-only lifecycle with a temp HOME/profile and an alternate root persisted by `cfgfc root <Path>`.
- The lifecycle must use cfgfc plus stdin/redirection to cover Project/Column/target creation, file and directory Setting content, Mode selections, both apply forms, status/JSON, byte-only content visibility, refresh, active resource renames, cascade deletion, reset/revert, immediate sync removal for disappeared resources, unresolved-reference failure, transaction recovery, and recorded-target ownership recovery.
- Prove `--yes`, `--cascade`, and `--force-targets` independently. Cover file-, symlink-, and directory-backed occupied targets, and verify unrelated paths are not reclaimed.
- Verify `sync` immediately removes disappeared Project/Column/Setting metadata from the corresponding Index, never recreates absent source paths, never implicitly cascades Mode/runtime references, rejects prune flags, and cannot restore deleted metadata.
- Verify read-only status reports prepared transactions without recovery; mutating commands recover before new work.
- For npm changes, run pack dry-run, local global install with `CFGFC_BINARY_PATH`, installed CLI help/lifecycle smoke, and unsupported-platform messaging.
- For native Windows-sensitive changes, run separate native path and real file/directory symlink checks; WSL does not substitute for native Windows.

## Documentation Expectations

- Normal user and agent workflows must be CLI-only. Do not instruct direct editing of indexes, Setting sources, target arrays, Mode selections, runtime state, sessions, or transactions.
- Keep `README.md`, all English/Chinese docs, `skills/configfacilitator-usage/SKILL.md`, this file, and CLI help synchronized.
- Maintain behavioral English/Chinese parity for commands, examples, safety rules, storage responsibilities, and developer workflow.
- Keep non-root project documentation under `docs/`.
- Clearly distinguish immediate sync index removal from explicit resource deletion and cascade behavior, and content byte changes from metadata/intent refresh.
- Document JSON envelopes/exit codes and the independent meaning of `--yes`, `--cascade`, and `--force-targets` without claiming unimplemented behavior.

## OpenSpec Workflow Expectations

- Update only the task checkboxes completed by the current work.
- After every OpenSpec Archive operation, automatically create a git commit for the archive changes.

## Local Secrets

- Local npm publish token is stored outside git at `.secrets/npm-publish.env`.
- The token expires around 2026-10-02; rotate it before publishing after that date.
- Do not commit or print the token value; load it only for `npm publish` or GitHub `NPM_TOKEN` setup.
