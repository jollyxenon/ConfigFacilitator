# Architecture

## Overview

`cfgfc` is a single-binary Go CLI. `cmd/cfgfc/main.go` delegates to a fresh Cobra command tree in `internal/cli`. The public interface is resource-oriented: Project, Column, Setting, Mode, targets, selections, content, context, status, apply/refresh, reconciliation, and state recovery are all expressed through commands rather than editor workflows.

## Main boundaries

- `internal/cli`: command construction, scope and argument validation, human/JSON rendering, and exit classification.
- `internal/warehouse`: resolves the effective root and loads resources present in indexes; synchronization removes entries whose filesystem-backed sources disappeared.
- `internal/index` and `internal/jsonc`: parse and serialize durable JSONC indexes while preserving supported metadata and unknown parseable fields.
- `internal/mutate`: resource metadata, target, selection, rename, and deletion use cases.
- `internal/content`: safe file/directory import and bounded Setting content operations.
- `internal/repository`: atomic writes, warehouse-wide mutation lock, snapshots, transaction recovery, reference rewrites, and session/runtime persistence.
- `internal/syncer`: reconciles external filesystem/index changes, immediately removes disappeared Project/Column/Setting metadata, and leaves Mode/runtime references unresolved rather than cascading them.
- `internal/session`: PPID-scoped convenience Project context used by `use`.
- `internal/pathvars`: expands portable target path variables.
- `internal/planner`: converts the Current columns/relation and logical targets into mappings.
- `internal/linker`: inspects ownership and applies, replaces, resets, or restores real symlinks.

## Storage model

The default warehouse root is `~/.configfacilitator/`. `cfgfc root <Path>` persists another root in a user bootstrap file without migrating data. A Project contains `Column/`, `Mode/`, and `Backup/` trees. The main durable files are:

- `ProjectIndex.jsonc`, `ColumnIndex.jsonc`, `SettingIndex.jsonc`, `ModeIndex.jsonc`: inspectable interoperability data.
- `Backup/current_state.json`, `Backup/history.log`: CLI-owned Current state (`columns`, `relation`, `mappings`) and its previous snapshots.
- `.cfgfc-session/`: CLI-owned PPID context records under the effective root.
- `.cfgfc-transactions/`: reserved CLI-owned mutation lock, manifest, staging, and recovery data.

Runtime, session, and transaction files are not user-editable contracts.

## Identity and target model

Canonical identity comes from index map keys and corresponding resource paths. `displayName` is presentation-only; `aliases` are alternative command inputs. Commands resolve aliases before persistence, so Mode selections, Current state, and contexts store canonical identities.

Column target positions are zero-based logical records. They serialize to the existing `targetNumber`, `defaultTargetDir`, and `defaultTargetName` arrays. Setting overrides serialize to same-length `targetDir` and `targetName` arrays; an inherited component is persisted as an empty entry. Structural commands keep all arrays in lockstep.

A Setting can be file-backed or directory-backed. Content paths are bounded below its root, reject traversal and symlink components, and never follow imported symlinks.

## Mutation and transaction model

CLI-owned mutations validate names, aliases, paths, references, target arrays, dependencies, confirmation, and ownership before durable change. Individual files use same-directory temporary writes and rename.

Multi-artifact mutations use a warehouse-wide exclusive transaction:

1. recover any earlier prepared transaction;
2. validate and plan all affected repository paths, runtime records, contexts, and managed targets;
3. record exact pre-mutation snapshots and a durable prepared manifest under `.cfgfc-transactions/`;
4. commit staged paths/files and managed-link changes;
5. mark committed and remove transaction artifacts.

Ordinary failure rolls back to the recorded state. If the process stops, the next mutating command recovers before starting new work. Read-only `status` reports incomplete transaction diagnostics without mutating them.

Rename rewrites schema-defined live and historical references and recreates affected owned links. Deletion first reports dependencies. `--yes`, `--cascade`, and `--force-targets` authorize confirmation, reference repair, and recorded-target reclamation independently.

## Current state and automatic synchronization

(Current) is a temporary Mode: its `columns` are the authoritative selections, its `mappings` are the planned links (under `increment` they also keep the cumulative baseline), and its `relation` describes only the relationship to a named Mode. `relation.kind` is `following` or `detached`; no relation means independent.

`apply mode` makes the Current follow a Mode; `apply column` sets an independent Current. Editing the Current directly (`current column set/delete` or a Web UI write) turns `following` into `detached` while keeping `originMode`. Deleting a followed Mode clears only `relation` and keeps columns and mappings; renaming a followed Mode updates `originMode`. `revert` rewinds only Current state, never resource metadata or content.

Mutations that change what the Current would plan re-plan it in the same transaction: changing the selections of a Mode the Current follows, or changing resources that affect planning (Column/Setting targets, renames, deletions, a new Setting entering a `full` selection). A failed mutation rolls the whole transaction back. A `detached` Current also re-plans on resource changes, but is not affected by Mode changes. Description/display-name/alias changes and Setting content byte changes do not rebuild links.

## Apply and reconciliation

`apply mode` and `apply column` persist the Current relation and mappings. `refresh` replans the Current: a `detached` or independent Current from its own columns, a `following` Current from the origin Mode's latest selections. Content byte changes do not require refresh because symlinks keep the same source path.

`sync` is reserved for external changes such as Git or direct valid JSONC edits. It adds discovered resources and immediately removes indexed Project, Column, and Setting metadata whose filesystem sources disappeared. It does not recreate absent sources or implicitly cascade Mode/runtime references; `apply` and `refresh` fail before target changes when required references cannot resolve. There is no `sync --prune --yes` workflow, and sync does not restore deleted metadata.

Each Project commits in its own transaction, so `sync --all` isolates failures per Project. Missing Project/Column/Setting index entries are rebuilt from the filesystem with canonical names and minimal default metadata; a missing `ModeIndex.jsonc` is rebuilt empty. A missing `current_state.json` is rebuilt as an empty Current and the old `history.log` is deleted; a legacy-format or corrupted `current_state.json` keeps only that Project unusable until the user deletes it and syncs again. `sync` also replans a `following` Current from the origin Mode's latest selections.

## Output contract

Human mode is concise. JSON mode emits exactly one success object to stdout or one error object to stderr, without ANSI or unrelated prose. `status` reports `Current: following (Mode) [...]`, `Current: detached (Mode) [...]`, or `Current: independent [...]`; its JSON `current` field carries `columns`, `relation`, and `mappings`. Exit codes classify usage, resource/scope, invalid data, refusal, and persistence/transaction failures.
