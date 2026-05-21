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

- `defaultTarget` is the column-level default path.
- A setting-level `target` overrides `defaultTarget`.
- Paths can use `~`, `${VAR}`, and Windows `%VAR%` forms.
- `target` stays independent from normalized persisted identity and warehouse-side source names.

## Mode semantics

- `full` clears previous links for the column before applying.
- `incremental` keeps existing links and adds new ones.
