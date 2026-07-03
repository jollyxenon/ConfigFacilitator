## Why

ConfigFacilitator currently resolves every warehouse operation from a fixed home-relative root, so users who keep their warehouse on another disk, sync folder, or custom workspace cannot point `cfgfc` at that location without moving data back into `~/.configfacilitator`. A built-in persistent root command is needed so the CLI can follow a user-selected warehouse path while keeping the current default unchanged.

## What Changes

- Add a standalone `cfgfc root` command family that prints the current effective warehouse root with no path argument and persists a new warehouse root when given one path argument.
- Resolve the effective warehouse root from user bootstrap state before falling back to the current defaults of `~/.configfacilitator` on Unix-like platforms and `%USERPROFILE%/.configfacilitator` on native Windows.
- Make `new`, `sync`, `switch`, `list`, `apply`, `update`, `reset`, and `revert` all operate against the effective warehouse root instead of assuming only the default location.
- Keep warehouse switching non-migratory: changing the configured root does not copy, move, or auto-create the previous warehouse contents in the new location.
- Update command help, user documentation, and CLI/runtime tests to cover alternate-root behavior.

## Capabilities

### New Capabilities

### Modified Capabilities

- `cli-workflows`: add the `root` command family, its help surface, and the workflow-level behavior for inspecting and changing the effective warehouse root.
- `config-root-warehouse-layout`: allow a persisted effective warehouse root override while preserving the current default fallback and the no-auto-migration boundary.
- `warehouse-structure`: define project discovery, backup files, and reserved internal directories relative to the effective warehouse root instead of only the default path.

## Impact

- Affected CLI surface: command registration, root help, `cfgfc root --help`, and command dispatch.
- Affected runtime behavior: warehouse-root bootstrap resolution, persisted root override storage, project discovery, and session-state placement.
- Affected tests: command registration/help tests, root-resolution tests, alternate-root workflow tests, and smoke tests that exercise commands after switching the configured root.
- Affected documentation: `README.md`, Chinese and English user docs under `docs/`, and `AGENTS.md` command-surface notes.
