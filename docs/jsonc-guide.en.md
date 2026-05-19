# JSONC Guide

## Purpose

Indexes are stored as JSONC so templates can carry temporary guidance comments while still round-trip through the project serializer.

## Rules

- Temporary `//` comments may disappear after `sync`.
- Permanent notes belong in the `"description"` field.
- Unknown fields are preserved by the index layer.

## Main files

- `ProjectIndex.jsonc`
- `ColumnIndex.jsonc`
- `SettingIndex.jsonc`
- `ModeIndex.jsonc`

## Target resolution

- `defaultTarget` is the column-level default path.
- A setting-level `target` overrides `defaultTarget`.
- Paths can use `~`, `${VAR}`, and Windows `%VAR%` forms.

## Mode semantics

- `full` clears previous links for the column before applying.
- `incremental` keeps existing links and adds new ones.
