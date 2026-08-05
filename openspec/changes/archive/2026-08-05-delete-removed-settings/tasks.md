## 1. Setting reconciliation

- [x] 1.1 Update the syncer so an indexed Setting whose file or directory source is absent is omitted from the reconciled column model instead of being marked missing.
- [x] 1.2 Remove or narrow Setting-specific missing-marker serialization so a synchronized `SettingIndex.jsonc` deletes the disappeared entry while existing Settings retain their authored metadata.
- [x] 1.3 Confirm the warehouse and CLI listing paths consume the reconciled Setting collection without additional stale-entry handling.

## 2. Regression coverage

- [x] 2.1 Update unit tests that currently expect a disappeared Setting to remain with `"missing": true`.
- [x] 2.2 Add sync tests for removing previously indexed file-backed and directory-backed Settings, including removal of their authored metadata from the rewritten index.
- [x] 2.3 Add a CLI integration test verifying `cfgfc list -c` no longer displays a Setting after its source is deleted and sync completes.

## 3. Documentation and validation

- [x] 3.1 Update English and Chinese documentation that describes missing Setting retention after sync, while retaining any applicable behavior for other entity types.
- [x] 3.2 Run `pixi run test`, `pixi run compile`, and a temporary-warehouse CLI smoke test that creates, indexes, deletes, syncs, and lists both Setting source types.
