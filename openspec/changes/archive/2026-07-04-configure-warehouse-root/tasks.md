## 1. Bootstrap Root Resolution

- [x] 1.1 Add focused tests for default warehouse-root fallback and persisted alternate-root resolution on Unix-like and native-Windows path rules.
- [x] 1.2 Implement user-scoped bootstrap persistence for the effective warehouse root, including path normalization and shared read/write helpers.
- [x] 1.3 Route the existing shared warehouse-root entry point through the new bootstrap resolver so all command paths receive the same effective root.

## 2. `root` Command Surface

- [x] 2.1 Register `root` in the CLI command list, root help overview, command dispatch, and `cfgfc root --help` output.
- [x] 2.2 Implement `cfgfc root` so zero path arguments print the current effective root and exactly one path argument persists a new effective root without migrating warehouse data.
- [x] 2.3 Add CLI tests for `cfgfc root`, including help output, no-argument inspection, one-argument persistence, and rejecting extra positional arguments.

## 3. Existing Workflow Integration

- [x] 3.1 Ensure `new`, `sync`, `switch`, `list`, `apply`, `update`, `reset`, and `revert` all operate against the configured effective warehouse root.
- [x] 3.2 Verify that session convenience state remains rooted under the effective warehouse root so switching roots also switches the active `.cfgfc-session` store.
- [x] 3.3 Add end-to-end workflow tests that set an alternate root, exercise at least one real command flow there, and confirm the previous root is left untouched.

## 4. Documentation and Verification

- [x] 4.1 Update `README.md`, `docs/README.en.md`, `docs/README.zh-CN.md`, and `docs/platform-notes.zh-CN.md` to describe default-root fallback, `cfgfc root`, and the no-auto-migration behavior.
- [x] 4.2 Update `AGENTS.md` command-surface and verification notes for the new `root` command and alternate-root smoke coverage.
- [x] 4.3 Run `pixi run test`, `pixi run compile`, `pixi run build`, `pixi run help`, and the subcommand help sweep extended to include `root`.
- [x] 4.4 Run a real CLI smoke test with a temporary home/profile plus alternate warehouse root to verify root switching and non-migration behavior.
