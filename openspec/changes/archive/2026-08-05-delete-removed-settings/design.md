## Context

The current Setting reconciliation path retains entries whose source no longer exists and writes a `missing` marker. This makes a completed `sync` leave stale Setting metadata available to listing and later planning. The requested behavior changes only Settings that were previously indexed and are detected as absent: they must be removed from the reconciled state and their index entry must not be serialized.

## Goals / Non-Goals

**Goals:**

- Remove an indexed Setting when its source file or directory is absent during `cfgfc sync`.
- Preserve ordinary discovery and metadata retention for Settings that still exist.
- Cover both file-backed and directory-backed Settings.
- Keep the documentation and specifications aligned with the new synchronization result.

**Non-Goals:**

- Change the treatment of missing projects, columns, or modes.
- Add a deletion command, retention period, tombstone, or recovery mechanism for removed Setting metadata.
- Remove a Setting merely because it is not selected by a mode or absent from active mappings.
- Change `apply`, `update`, `reset`, or `revert` behavior beyond their use of the newly synchronized index.

## Decisions

### Make filesystem absence authoritative for Settings during sync

The Setting reconciliation pass will compare indexed Setting names with entries physically present in the column directory. Any indexed Setting without a matching source will be omitted from the reconciled `column.Settings` collection. The index writer will therefore remove the entry rather than add or preserve `missing: true`.

### Limit deletion to the synchronization boundary

Loading a manually edited index remains a parsing concern. Removal happens only when `sync` has inspected the column filesystem and can determine that the Setting source is absent. This keeps the change tied to the explicit reconciliation command and avoids unrelated model-loading side effects.

### Reuse existing serialization and list behavior

No separate list filtering or cleanup pass is needed. Once sync writes the reconciled `SettingIndex.jsonc`, the normal warehouse load and `list -c` paths naturally contain only extant Settings.

### Test source types and metadata removal

Add sync tests that first create indexed Settings with authored metadata, remove one file-backed source and one directory-backed source, then verify the rewritten index omits each entry and never serializes a missing marker. Add a CLI-level assertion that a post-sync column listing does not show the removed Setting.

## Risks / Trade-offs

- **User-authored Setting metadata is lost with the source.** This is intentional: a source removed from the warehouse is no longer a Setting and must not be retained as a missing entry.
- **Incorrect filesystem matching could remove a valid Setting.** Reuse the current discovery identity and file/directory traversal logic, and retain tests covering unchanged extant entries.
- **Existing documentation promises missing Setting retention.** Update the corresponding specifications and user-facing sync/list guidance in both languages.

## Migration Plan

1. Adjust Setting reconciliation to omit indexed sources that are absent.
2. Remove obsolete Setting-specific missing-marker expectations and add deletion regression tests.
3. Update bilingual documentation and help text if it describes missing Settings after sync.
4. Run the project test suite plus a sync smoke test covering a deleted source.

No data migration is required. The next `cfgfc sync` rewrites affected `SettingIndex.jsonc` files into the new state.

## Open Questions

None.
