# Architecture

## Overview

`cfgfc` is a single-binary Go CLI. `cmd/cfgfc/main.go` delegates to `internal/cli`, which routes the command families directly.

## Packages

- `internal/warehouse`: resolves `~/.configfacilitator/SettingWarehouse/` and loads the warehouse model.
- `internal/index`: parses and writes JSONC index files.
- `internal/jsonc`: strips comments and normalizes JSONC content.
- `internal/scaffold`: creates project, column, and mode templates.
- `internal/syncer`: reconciles index files with filesystem reality.
- `internal/session`: stores PPID-scoped project context.
- `internal/pathvars`: expands portable path variables.
- `internal/planner`: turns CLI intent into link mappings.
- `internal/linker`: applies, resets, and reverts symlink state.

## Storage model

The warehouse lives under `~/.configfacilitator/SettingWarehouse/`, not beside the shell working directory. Projects contain `Column/`, `Mode/`, and `Backup/` trees, with `ProjectIndex.jsonc`, `ColumnIndex.jsonc`, `SettingIndex.jsonc`, `ModeIndex.jsonc`, `current_state.json`, and `history.log` as the main persisted files.

## Behavioral rules

- Setting-level `target` overrides column-level `defaultTarget`.
- `Mode` can apply `full` or `incremental` column strategies.
- `switch` stores a convenience project context by PPID.
- `revert` restores only the previous apply state.
