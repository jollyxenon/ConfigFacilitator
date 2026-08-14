# Change: Replace editor-dependent workflows with complete cfgfc resource management

## Why

ConfigFacilitator currently requires users to create source files and manually edit Project, Column, Setting, and Mode JSONC indexes before `sync`, so the CLI cannot express the complete configuration lifecycle. External filesystem disappearance is reconciled directly into index metadata, while Mode and runtime references are kept separate and may become unresolved until explicitly repaired.

## What Changes

- **BREAKING** Replace the flat `new`, `switch`, `list`, `apply`, and `update` syntax with resource-oriented `project`, `column`, `setting`, `mode`, `use`, `status`, `apply`, and `refresh` command families; retain `root`, `sync`, `reset`, and `revert` with normalized flags.
- Add complete create, inspect, update, rename, and delete operations for Project, Column, Setting, and Mode resources, including aliases, display names, descriptions, Column defaults, Setting target overrides, and Mode column selections.
- Add CLI-only management of file-backed and directory-backed Setting contents through stdin, literal text, local imports, and bounded relative-path operations.
- Make CLI-owned mutations update filesystem data, indexes, Mode references, PPID context, current apply intent, mappings, and history as one recoverable transaction.
- Change `sync` to remove indexed Project, Column, and Setting metadata immediately when their filesystem sources have disappeared, add newly discovered resources, and leave Mode/runtime references untouched rather than implicitly cascading them.
- Separate repository-deletion confirmation (`--yes`), reference cleanup (`--cascade`), and managed-target reclamation (`--force-targets`) instead of overloading `--force`.
- Add stable machine-readable `--json` output and documented exit-code classes for command automation.
- Make generated JSONC indexes an implementation-facing persistence format rather than a required editing interface while continuing to parse external JSONC edits and preserve unknown fields.
- Update English and Chinese documentation, CLI help, agent guidance, and smoke tests so a fresh warehouse can be fully created, changed, applied, renamed, deleted, and synchronized without direct filesystem or index editing.

## Capabilities

### New Capabilities

- `configuration-resource-management`: Resource-oriented Project, Column, Setting, Mode, target, context, status, and machine-readable command behavior.
- `setting-content-management`: Safe CLI creation, import, inspection, replacement, movement, and deletion of file-backed and directory-backed Setting content.
- `repository-mutation-transactions`: Atomic rename and deletion behavior across repository paths, references, runtime state, history, and session context.

### Modified Capabilities

- `cli-workflows`: Replace the existing command families and argument forms, rename update to refresh, redefine apply syntax, and make sync remove disappeared filesystem-backed index entries immediately without cascading references.
- `index-and-jsonc-contracts`: Treat index JSONC as CLI-managed persistence while preserving externally authored valid data and unknown fields.
- `warehouse-layout-and-models`: Remove disappeared filesystem-backed Projects, Columns, and Settings from their indexes during sync while keeping unresolved Mode/runtime references observable.
- `session-and-path-resolution`: Replace `switch` with `use` and keep PPID-scoped context valid across project renames and deletions.

## Impact

- CLI routing/help in `internal/cli` will be replaced and split by command family; a nested-command library such as Cobra will be added.
- New repository mutation and content-management packages will centralize validation, atomic writes, rollback, reference rewriting, and path-boundary enforcement.
- `internal/scaffold`, `internal/syncer`, `internal/warehouse`, `internal/planner`, `internal/linker`, and `internal/session` require coordinated changes.
- Existing command scripts are intentionally incompatible and must migrate to the new forms documented by this change.
- Existing index, current-state, and history formats remain loadable; rename and cascade operations extend their current records in place rather than introducing a second persistent state format.
- README, bilingual docs, root/subcommand help, the ConfigFacilitator usage Skill, Go tests, CLI smoke tests, and npm packaging verification are affected.
