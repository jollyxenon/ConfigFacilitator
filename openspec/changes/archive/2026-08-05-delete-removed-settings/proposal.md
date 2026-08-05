## Why

`cfgfc sync` currently retains an indexed Setting whose warehouse source has disappeared and serializes it with `"missing": true`. A source that was once present but has now been removed is stale warehouse metadata, so leaving it indexed makes the column and `list -c` report a Setting that can no longer be applied.

## What Changes

- Change Setting reconciliation so that `sync` removes an existing Setting index entry when its corresponding source file or directory is no longer present.
- Stop serializing the removed Setting as a `missing` marker during that reconciliation.
- Keep discovery of newly present Settings and preservation of metadata for Settings that still exist unchanged.
- Update the affected specifications and user documentation to describe removal rather than missing-state retention for disappeared Settings.
- Add regression coverage for file- and directory-backed Settings that existed in a prior index and are later deleted from the warehouse.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `index-and-jsonc-contracts`: Disappeared Settings are removed during synchronization instead of being preserved with a missing marker.
- `warehouse-layout-and-models`: The synchronized warehouse model no longer retains Settings whose source has disappeared.
- `cli-workflows`: `sync` rewrites Setting indexes without previously indexed sources that no longer exist.

## Impact

- Affected code: Setting reconciliation and index serialization in `internal/syncer`, warehouse loading/model handling in `internal/warehouse`, and associated unit and CLI tests.
- User-facing behavior: after `cfgfc sync`, a removed Setting no longer appears in `SettingIndex.jsonc` or `cfgfc list -c`.
- Compatibility: this intentionally replaces the existing missing-marker behavior for Settings; user-authored metadata attached to a deleted Setting is removed with that entry.
- No new dependencies or command-line flags.
