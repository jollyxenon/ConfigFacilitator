# JSONC and Interoperability Guide

## Role of JSONC

`ProjectIndex.jsonc`, `ColumnIndex.jsonc`, `SettingIndex.jsonc`, and `ModeIndex.jsonc` remain the durable, inspectable interoperability format. Normal users manage supported metadata, targets, Setting sources, content, and Mode selections with CLI commands. Direct index editing is optional and intended for external tools, Git merges, or advanced interoperability—not as a required setup step.

Valid external JSONC, including comments, is parsed on the next load or `sync`. Invalid JSONC or invalid required field types cause the command to fail before normalization or related mutation.

## Identity and metadata

- A resource's map key is its canonical identity and corresponds to its filesystem position where applicable.
- `displayName` is presentation-only and is not an implicit alias.
- `aliases` are explicit alternative command references. Empty aliases normalize to `"aliases": []`.
- `description` is durable metadata and is preserved unless explicitly replaced or the owning resource is deleted.
- CLI input through an alias is resolved before persistence, so selections, contexts, and apply intent contain canonical identities.
- Alias updates reject empty/duplicate values and collisions with canonical names or aliases in the same resolution scope.

Use resource `create`, `set`, and `rename` commands rather than changing these fields directly.

## Unknown fields

Unknown parseable fields are preserved through unrelated set, target, selection, rename, sync, and serialization operations whenever they do not collide with schema-defined keys. Schema-defined values win on collision. Deleting a resource also deletes unknown fields owned by that resource.

The CLI rewrites only schema-defined references. Extensions that store identities or paths inside unknown fields must update those references themselves.

## Disappearance and synchronization

If an indexed Project directory, Column directory, or Setting file/directory disappears, `sync` immediately removes its metadata from the corresponding Index. Sync does not recreate absent source paths or child indexes.

Mode selections, current/history runtime records, and PPID context are not implicitly cascaded. They remain available for diagnostics as unresolved references, and `apply` or `refresh` fails before target mutation when it requires one. Use resource CLI deletion with independent `--yes`, `--cascade`, and `--force-targets` controls when dependent cleanup is intended.

`sync --prune` and `sync --prune --yes` are unsupported. Recreating a former source path may discover a new resource, but sync does not restore descriptions, aliases, targets, unknown fields, or other metadata deleted from the Index.

## Target persistence

The CLI exposes logical positions while retaining the existing array schema:

- Column `targetNumber` is the number of zero-based positions.
- `defaultTargetDir` and `defaultTargetName` are Column defaults.
- Each Setting's `targetDir` and `targetName` arrays have exactly the same length.
- Empty Setting entries mean inherit the matching Column component.
- An empty Column default name means derive the target name from the Setting canonical name.
- An empty effective target directory is invalid for planning.

Use `column target add/set/delete` and `setting target set/reset`. These commands resize or update arrays consistently across all Settings. Target directories support `~`, `${VAR}`, and Windows `%VAR%`; expanded targets must be non-empty and unique, and target names must be one normal path component.

## Mode persistence

Mode Column strategies are:

- `cover`: apply exactly the selected Settings.
- `increment`: retain existing managed mappings for that Column and add selected Settings.
- `none`: apply no mappings for that Column.
- `full`: apply every present Setting in that Column.

`cover` and `increment` require one or more Settings. `none` and `full` store no Setting list. Use `mode column set/delete` so references are canonicalized and validated.

## CLI-owned files

Do not manually edit:

- `Backup/current_state.json`
- `Backup/history.log`
- `.cfgfc-session/`
- `.cfgfc-transactions/`

They contain apply intent, current/previous mappings, PPID context, mutation locks, snapshots, staging, and recovery records. Read-only `status` can report incomplete transactions; the next mutating command performs recovery.
