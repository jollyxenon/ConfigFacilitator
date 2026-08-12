## Context

The warehouse loader already flags Columns whose directory is absent: `loadColumn` sets `Missing: !dirPresent` (internal/warehouse/warehouse.go), and the reconciled `project.Columns` map therefore contains both extant and disappeared Columns. The sync writer, however, writes every Column from that map back into `ColumnIndex.jsonc` (internal/syncer/sync.go, `rewriteProject`), so a deleted column directory leaves a permanent stale index entry. Worse, the per-column `rewriteSettingIndex` call writes `Column/<name>/SettingIndex.jsonc` through `writeJSON`, whose `os.MkdirAll` silently recreates the deleted column directory with an empty Setting index.

Settings already follow the opposite rule: `rewriteSettingIndex` skips `setting.Missing`, which is how disappeared Settings get dropped from `SettingIndex.jsonc` (change `delete-removed-settings`). This design extends that same rule to Columns.

See proposal.md - Why for motivation; the delta specs define the required behavior.

## Goals / Non-Goals

**Goals:**

- Remove an indexed Column from `ColumnIndex.jsonc` when its source directory is absent during `cfgfc sync`.
- Stop recreating the deleted column's directory (empty `SettingIndex.jsonc`) during that same synchronization.
- Preserve discovery and metadata retention for Columns that still exist.
- Keep the bilingual documentation and specifications aligned with the new synchronization result.

**Non-Goals:**

- Change the treatment of missing projects or modes, which remain preserved as today.
- Add a deletion command, retention period, tombstone, or recovery mechanism for removed Column metadata.
- Remove a Column merely because it is not selected by a mode or absent from active mappings.
- Change `apply`, `update`, `reset`, or `revert` behavior; `update` does not share the index-rewrite path.

## Decisions

### Make filesystem absence authoritative for Columns during sync

Mirror the existing Setting rule in `rewriteProject`: skip any `column.Missing` entry before writing it into `columnIndex.Columns` and before delegating to `rewriteSettingIndex`. `column.Missing` is already computed by `loadColumn` from directory presence, so no new model plumbing is needed.

- **Alternative considered**: filtering in the warehouse loader. Rejected — loading a manually edited index remains a parsing concern, and the Setting precedent deliberately confines removal to the sync boundary.

### Skip the Setting index rewrite for removed Columns

Because `rewriteSettingIndex` is invoked inside the per-column loop, the `column.Missing` skip naturally prevents the empty `SettingIndex.jsonc` write — and therefore prevents `writeJSON`'s `MkdirAll` from recreating the deleted directory. This behavior is asserted explicitly in tests so the accidental directory recreation cannot regress silently.

- **Alternative considered**: removing the directory after writing. Rejected — deletion should never be the writer's job; not writing is strictly safer.

### Limit deletion to the synchronization boundary

Removal happens only when `sync` has inspected the column filesystem. A direct warehouse load (e.g. by `list` before any sync) still surfaces a stale index entry; the next `sync` cleans it. This matches the Setting precedent and keeps the change tied to the explicit reconciliation command.

### Reuse existing serialization and list behavior

No separate list filtering or cleanup pass is needed. Once sync writes the reconciled `ColumnIndex.jsonc`, the normal warehouse load and `list -p` paths naturally contain only extant Columns.

### Test source removal and directory non-recreation

Add a sync unit test that indexes a Column with authored metadata, deletes its directory, syncs, and verifies the rewritten `ColumnIndex.jsonc` omits the entry while an existing sibling Column retains its metadata and the deleted directory is not recreated. Add a CLI integration test mirroring `TestRunWithExecutableSyncRemovesDeletedSettings` that asserts the removed Column disappears from `list -p` output after sync.

## Risks / Trade-offs

- **User-authored Column metadata is lost with the entry.** Intentional: a directory removed from the warehouse is no longer a Column, and Settings already follow this rule.
- **Incorrect filesystem matching could remove a valid Column.** Mitigation: reuse the current `listSubdirectories`/index-union discovery (`unionStringKeys`) unchanged, and keep a test asserting an extant sibling Column keeps its metadata.
- **Existing documentation promises missing Column retention.** Mitigation: update the sync/list guidance in `docs/commands.{en,zh-CN}.md` and the affected spec requirements in this change's delta files.
- **A vanished column directory could be recreated on a future unrelated write.** Low risk today: `rewriteSettingIndex` is only reachable from the column loop, which the skip guards; the non-recreation test locks this in.

## Migration Plan

1. Skip `column.Missing` entries in `rewriteProject` (index entry and Setting-index rewrite).
2. Add sync unit regression tests and the CLI `list -p` integration test.
3. Update bilingual documentation describing sync retention of removed Columns.
4. Run `pixi run test`, `pixi run compile`, and a temporary-warehouse CLI smoke test that creates, deletes, syncs, and lists a column.

No data migration is required. The next `cfgfc sync` rewrites affected `ColumnIndex.jsonc` files into the new state.

## Open Questions

None.
