## 1. Column reconciliation

- [x] 1.1 Update the syncer (`internal/syncer/sync.go`, `rewriteProject`) so an indexed Column whose source directory is absent is omitted from `columnIndex.Columns` instead of being written back with its metadata.
- [x] 1.2 Ensure the `column.Missing` skip also prevents the per-column `rewriteSettingIndex` call, so the deleted column directory is not recreated with an empty `SettingIndex.jsonc` during sync.
- [x] 1.3 Confirm the warehouse and CLI listing paths consume the reconciled Column collection without additional stale-entry handling.

## 2. Regression coverage

- [x] 2.1 Add a sync unit test that indexes a Column with authored metadata, deletes its directory, syncs, and verifies the rewritten `ColumnIndex.jsonc` omits the entry while an extant sibling Column retains its metadata.
- [x] 2.2 Add an assertion in the sync unit test that the deleted column directory is not recreated after synchronization.
- [x] 2.3 Add a CLI integration test mirroring `TestRunWithExecutableSyncRemovesDeletedSettings` that verifies `cfgfc list -p <Project>` no longer displays a Column after its directory is deleted and sync completes.

## 3. Documentation and validation

- [x] 3.1 Update English and Chinese documentation (`docs/commands.en.md`, `docs/commands.zh-CN.md`, and example docs if they mention Column retention) to describe removal of disappeared Columns during sync, while retaining the existing description of Settings removal.
- [x] 3.2 Run `pixi run test`, `pixi run compile`, and a temporary-warehouse CLI smoke test that creates a column, deletes its directory, syncs, and lists the project to confirm the column no longer appears.
