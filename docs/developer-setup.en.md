# Developer Setup

## Tooling

- Language: Go 1.24.4
- Environment manager: `pixi` supplies the Go toolchain for development
- Entry point: `cmd/cfgfc/main.go`
- npm distribution package: `npm/`

## Baseline commands

```bash
pixi run test
pixi run compile
pixi run build
pixi run help
pixi run bash -lc 'for cmd in new sync switch root list apply update reset revert; do go run ./cmd/cfgfc "$cmd" --help; done'
```

## Validation expectations

- Use `pixi run test` for the full Go test suite.
- Use `pixi run compile` to confirm all Go packages still compile.
- Use `pixi run build` to create the local CLI binary at `dist/cfgfc`.
- Use `pixi run help` to verify the root command surface.
- Use the subcommand help sweep above to verify every registered command returns structured help through the pixi-managed Go toolchain.
- For command changes, also run a real CLI smoke test against a temp home/profile plus an alternate warehouse root persisted with `cfgfc root <path>`.

## npm package and release workflow

The npm package under `npm/` is a thin wrapper around the Go binary. Its `postinstall` script downloads a platform-specific binary from the GitHub Release whose tag matches `npm/package.json` version.

Before publishing a release:

```bash
pixi run test
pixi run compile
pixi run build
cd npm
npm pack --dry-run
CFGFC_BINARY_PATH=../dist/cfgfc npm install -g .
cfgfc --help
CFGFC_TEST_PLATFORM=freebsd CFGFC_TEST_ARCH=x64 node install.js
```

Expected release order:

1. Ensure `npm/package.json` version is `X.Y.Z`.
2. Push Git tag `vX.Y.Z`.
3. Let GoReleaser publish GitHub Release assets named like `cfgfc_X.Y.Z_linux_amd64.tar.gz` plus `checksums.txt`.
4. Publish the npm package only after the matching GitHub Release assets exist.

## Documentation workflow

When a change affects user-facing behavior, command surface, project structure, or developer workflow, update the matching English and Chinese docs under `docs/` in the same change.
