
# ConfigFacilitator Agent Notes

## Current Stack

- Language: Go
- Environment manager: pixi-managed Go toolchain
- Entry point: `cmd/cfgfc/main.go`
- npm distribution package: `npm/`, wrapping prebuilt Go release binaries for `npm install -g @jollyxenon/cfgfc`

## Implemented Command Surface

- `new`: project / column / mode scaffolding
- `sync`: global and project-local index reconciliation, plus explicit `--all` / `-a` full-warehouse refresh
- `switch`: PPID-scoped convenience context with `switch global` context clear
- `root`: inspect or persist the effective warehouse root without migrating existing warehouse contents
- `list`: project, column, and mode inspection
- `apply`: mode apply and single-column apply
- `update`: refresh the persisted apply intent from current warehouse metadata, with legacy mapping fallback plus `--all` / `-a` and `--column` / `-c` scopes
- `reset`: clear current managed mappings
- `revert`: restore previous snapshot
- `apply` / `update` / `reset` / `revert`: support destructive `-f` / `--force` recovery that reclaims occupied files, symlinks, and directories recursively while restoring only the last confirmed managed state

## Baseline Commands

- `pixi run test`
- `pixi run compile`
- `pixi run build`
- `pixi run help`
- `pixi run bash -lc 'for cmd in new sync switch root list apply update reset revert; do go run ./cmd/cfgfc "$cmd" --help; done'`
- `cd npm && npm pack --dry-run`
- `pixi run build && cd npm && CFGFC_BINARY_PATH=../dist/cfgfc npm install -g . && cfgfc --help`
- `cd npm && CFGFC_TEST_PLATFORM=freebsd CFGFC_TEST_ARCH=x64 node install.js` (expected failure path for unsupported tuple messaging)

## Verification Expectations

- Use `pixi run test` for the full Go test suite.
- Use `pixi run compile` to confirm the project still compiles.
- Use `pixi run build` to create the local CLI binary at `dist/cfgfc`.
- Use `pixi run help` to verify the root command surface.
- Use a subcommand help sweep to verify every registered command returns structured help through the pixi-managed Go toolchain.
- For npm packaging changes, use `npm pack --dry-run`, a local install with `CFGFC_BINARY_PATH=../dist/cfgfc`, and an unsupported-platform installer smoke test.
- For command changes, also run a real CLI smoke test against a temp home/profile plus an alternate warehouse root persisted with `cfgfc root <path>`; for `update`, cover mode apply with a `full` column, sync a newly added source, then update to verify the new source is hard-linked into place when source and target stay on the same filesystem.
- For hard-link activation changes, include smoke paths for regular-file success, cross-filesystem or cross-volume failure, missing or non-regular source failure, and clear rejection of directory-backed mappings.

## Documentation Expectations

- Sync documentation after every modification that changes user-facing behavior, command surface, project structure, or developer workflow.
- Keep `README.md`, `docs/`, and the `cfgfc --help` output synchronized whenever user-facing commands, flags, examples, installation steps, or workflows change.
- Keep non-root project documentation under `docs/`.
- Maintain English and Chinese document parity when updating docs.

## OpenSpec Workflow Expectations

- After every OpenSpec Archive operation, automatically create a git commit for the archive changes.

## Local Secrets

- Local npm publish token is stored outside git at `.secrets/npm-publish.env`.
- The token expires around 2026-10-02; rotate it before publishing after that date.
- Do not commit or print the token value; load it only for `npm publish` or GitHub `NPM_TOKEN` setup.
