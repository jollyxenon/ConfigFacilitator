# Developer Setup

## Tooling

- Language: Go 1.24.4
- Environment manager: `pixi` supplies the Go toolchain for development
- Entry point: `cmd/cfgfc/main.go`

## Baseline commands

```bash
pixi run test
pixi run compile
pixi run build
pixi run help
pixi run bash -lc 'for cmd in new sync switch list apply update reset revert; do go run ./cmd/cfgfc "$cmd" --help; done'
```

## Validation expectations

- Use `pixi run test` for the full Go test suite.
- Use `pixi run compile` to confirm all Go packages still compile.
- Use `pixi run build` to create the local CLI binary at `dist/cfgfc`.
- Use `pixi run help` to verify the root command surface.
- Use the subcommand help sweep above to verify every registered command returns structured help through the pixi-managed Go toolchain.
- For command changes, also run a real CLI smoke test against a temp `~/.configfacilitator/`.

## Documentation workflow

When a change affects user-facing behavior, command surface, project structure, or developer workflow, update the matching English and Chinese docs under `docs/` in the same change.
