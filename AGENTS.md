
# ConfigFacilitator Agent Notes

## Current Stack

- Language: Go
- Environment manager: pixi
- Entry point: `cmd/cfgfc/main.go`

## Implemented Command Surface

- `new`: project / column / mode scaffolding
- `sync`: global and project-local index reconciliation, plus explicit `--all` / `-a` full-warehouse refresh
- `switch`: PPID-scoped convenience context with `switch global` context clear
- `list`: project, column, and mode inspection
- `apply`: mode apply and single-column apply
- `update`: refresh the persisted apply intent from current warehouse metadata, with legacy mapping fallback plus `--all` / `-a` and `--column` / `-c` scopes
- `reset`: clear current managed mappings
- `revert`: restore previous snapshot
- `apply` / `update` / `reset` / `revert`: support destructive `-f` / `--force` recovery that reclaims occupied files, symlinks, and directories recursively while restoring only the last confirmed managed state

## Baseline Commands

- `pixi run test`
- `pixi run build`
- `pixi run help`
- `pixi run bash -lc 'for cmd in new sync switch list apply update reset revert; do go run ./cmd/cfgfc "$cmd" --help; done'`

## Verification Expectations

- Use `pixi run test` for the full Go test suite.
- Use `pixi run build` to confirm the project still compiles.
- Use `pixi run help` to verify the root command surface.
- Use a subcommand help sweep to verify every registered command returns structured help from `cfgfc <command> --help`.
- For command changes, also run a real CLI smoke test against a temp `~/.configfacilitator/`; for `update`, cover mode apply with a `full` column, sync a newly added source, then update to verify the new source is linked.
- For destructive command changes, include a smoke path that verifies `-f` / `--force` can reclaim both file-backed and directory-backed targets.

## Documentation Expectations

- Sync documentation after every modification that changes user-facing behavior, command surface, project structure, or developer workflow.
- Keep non-root project documentation under `docs/`.
- Maintain English and Chinese document parity when updating docs.

## OpenSpec Workflow Expectations

- After every OpenSpec Archive operation, automatically create a git commit for the archive changes.
