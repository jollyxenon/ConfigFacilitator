## 1. Current-state summarization and styling helpers

- [x] 1.1 Extend the `list` code path to load each relevant project's persisted `CurrentState` and classify project mode status plus per-column setting coverage.
- [x] 1.2 Add a small terminal-aware styling helper for green/yellow/red list emphasis that falls back to plain text for tests and non-color output.

## 2. Render usage-oriented list output

- [x] 2.1 Update global `cfgfc list` rendering so each project appends a matched-mode, `Unmatched`, or `None` suffix based on persisted state.
- [x] 2.2 Update project-scoped `cfgfc list` rendering so columns append `Full` / `Partial` / `None`, the active mode is highlighted, and `list -c` highlights enabled settings while preserving missing entries.

## 3. Verify and document the new behavior

- [x] 3.1 Extend `internal/cli/cli_test.go` with coverage for matched mode, unmatched state, none state, column coverage labels, and enabled-setting highlighting.
- [x] 3.2 Update `docs/commands.en.md`, `docs/commands.zh-CN.md`, `docs/example.en.md`, `docs/example.zh-CN.md`, and any affected `cfgfc list --help` examples or descriptions to match the new output contract.
- [x] 3.3 Run `pixi run test`, `pixi run compile`, `pixi run help`, and `pixi run bash -lc 'for cmd in new sync switch list apply update reset revert; do go run ./cmd/cfgfc "$cmd" --help; done'` after implementation.
