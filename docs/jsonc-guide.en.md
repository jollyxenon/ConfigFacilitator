# JSONC Guide

## Purpose

Indexes are stored as JSONC so generated templates can end with one disposable example comment block while still round-trip through the project serializer.

## Rules

- The trailing example comment block may disappear after `sync`.
- Permanent notes belong in the `"description"` field.
- Unknown fields are preserved by the index layer.
- Normalized entries persist `displayName` and `aliases`, and the top-level key remains the canonical authored identity.
- For `ProjectIndex.jsonc`, `ColumnIndex.jsonc`, and `ModeIndex.jsonc`, each top-level key is the canonical warehouse-side name.

## Main files

- `ProjectIndex.jsonc`
- `ColumnIndex.jsonc`
- `SettingIndex.jsonc`
- `ModeIndex.jsonc`

## Identity fields

- The top-level index key is the normalized persisted identifier and keeps filesystem layout unchanged.
- Additional authored fields such as `warehouseName` or `folderName` remain outside the canonical identity model; the top-level key continues to define identity.
- `displayName` is presentation-only and is not used as an implicit CLI alias.
- `aliases` provide additional callable references for projects, columns, settings, and modes, and normalized output emits `"aliases": []` when no aliases are declared.

## Target resolution

- `defaultTargetDir` and `defaultTargetName` are column-level default target directory/name arrays.
- `targetDir` and `targetName` are setting-level directory/name arrays that override defaults by matching index.
- Directory and name arrays are strictly zipped; lengths must match after inheritance.
- In setting entries, `""` means inherit the matching default. In `defaultTargetName`, `""` falls back to the setting warehouse name. In `defaultTargetDir`, `""` means unconfigured and cannot be applied.
- Target directories can use `~`, `${VAR}`, and Windows `%VAR%` forms. Target names must resolve to one normal file name or single-level directory name.
- Expanded target paths must be non-empty and unique in the planned state.

## Mode semantics

- `cover` applies only the authored `settings` for that column.
- `increment` keeps existing managed links for that column and then adds the authored `settings`.
- `none` applies no mappings for that column.
- `full` links every known Setting in that column.
- `settings` may be omitted only when `strategy` is `none` or `full`.
