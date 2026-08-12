## Why

`cfgfc sync` retains an indexed Column whose source directory has disappeared from the warehouse, writing the stale entry back into `ColumnIndex.jsonc`. A deleted column is stale warehouse metadata: it can no longer be applied, yet `list` and the index keep reporting it. The same removal behavior already exists for Settings (implemented in `delete-removed-settings`), but Columns are still preserved.

## What Changes

- Change Column reconciliation so that `sync` removes an existing Column index entry when its corresponding source directory is no longer present.
- Stop rewriting the Setting index of a removed Column during that reconciliation, so the deleted column directory is not silently recreated with a fresh `SettingIndex.jsonc`.
- Keep discovery of newly present Columns and preservation of metadata for Columns that still exist unchanged.
- Keep the existing removal behavior for disappeared Settings unchanged; Projects and Modes remain preserved as today.
- Update the affected specifications and user documentation to describe removal rather than retention for disappeared Columns.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `index-and-jsonc-contracts`: The `Sync preserves missing or orphaned nodes` requirement is extended so disappeared Columns are removed from `ColumnIndex.jsonc` during synchronization, not preserved like missing Projects and Modes.
- `warehouse-layout-and-models`: The synchronized warehouse model no longer retains Columns whose source directory has disappeared.
- `cli-workflows`: `sync` rewrites `ColumnIndex.jsonc` without previously indexed Columns that no longer exist on disk.

## Impact

- Affected code: Column reconciliation and index serialization in `internal/syncer/sync.go` (the `rewriteProject` path that writes `ColumnIndex.jsonc` and delegates to the Setting index), plus associated unit and CLI tests.
- User-facing behavior: after `cfgfc sync`, a deleted column no longer appears in `ColumnIndex.jsonc` or `cfgfc list -p <Project>`, and its empty directory is not recreated.
- Compatibility: this intentionally replaces the current preservation behavior for Columns; user-authored metadata attached to a deleted Column is removed with that entry. Projects and Modes keep their current missing/orphaned preservation.
- No new dependencies or command-line flags.
