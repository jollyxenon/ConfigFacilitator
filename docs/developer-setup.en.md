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
pixi run install-global
pixi run bash -lc 'for cmd in new sync switch list apply reset revert; do go run ./cmd/cfgfc "$cmd" --help; done'
```

## Validation expectations

- Use `pixi run test` for the full Go test suite.
- Use `pixi run build` to confirm the project still compiles.
- Use `pixi run help` to verify the root command surface.
- Use `pixi run install-global` when you need `cfgfc` available directly from the shell.
- Use a subcommand help sweep to verify every registered command returns structured help from `cfgfc <command> --help`.
- For command changes, also run a real CLI smoke test against a temp `~/.configfacilitator/`.

## Documentation workflow

When a change affects user-facing behavior, command surface, project structure, or developer workflow, update the matching English and Chinese docs under `docs/` in the same change.
