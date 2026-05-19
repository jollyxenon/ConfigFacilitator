
# ConfigFacilitator Agent Notes

## Current Stack

- Language: Go
- Environment manager: pixi
- Entry point: `cmd/cfgfc/main.go`

## Implemented Command Surface

- `new`: project / column / mode scaffolding
- `sync`: global and project-local index reconciliation
- `switch`: PPID-scoped convenience context
- `list`: project, column, and mode inspection
- `apply`: mode apply and single-column apply
- `reset`: clear current managed mappings
- `revert`: restore previous snapshot

## Baseline Commands

- `pixi run test`
- `pixi run build`
- `pixi run help`

## Verification Expectations

- Use `pixi run test` for the full Go test suite.
- Use `pixi run build` to confirm the project still compiles.
- Use `pixi run help` to verify the root command surface.
- For command changes, also run a real CLI smoke test against a temp executable-relative `SettingWarehouse/`.

## Documentation Expectations

- Sync documentation after every modification that changes user-facing behavior, command surface, project structure, or developer workflow.
- Keep non-root project documentation under `docs/`.
- Maintain English and Chinese document parity when updating docs.
