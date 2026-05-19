# Developer Setup

## Tooling

- Language: Go 1.24.4
- Environment manager: `pixi`
- Entry point: `cmd/cfgfc/main.go`

## Baseline commands

```bash
pixi run test
pixi run build
pixi run help
```

## Validation expectations

- Use `pixi run test` for the full Go test suite.
- Use `pixi run build` to confirm the project still compiles.
- Use `pixi run help` to verify the root command surface.
- For command changes, also run a real CLI smoke test against a temp executable-relative `SettingWarehouse/`.

## Documentation workflow

When a change affects user-facing behavior, command surface, project structure, or developer workflow, update the matching English and Chinese docs under `docs/` in the same change.
