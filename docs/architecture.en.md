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
- `internal/planner`: converts canonical apply/refresh intent and logical targets into mappings.
- `internal/linker`: inspects ownership and applies, replaces, resets, or restores real symlinks.

## Storage model

The default warehouse root is `~/.configfacilitator/`. `cfgfc root <Path>` persists another root in a user bootstrap file without migrating data. A Project contains `Column/`, `Mode/`, and `Backup/` trees. The main durable files are:

- `ProjectIndex.jsonc`, `ColumnIndex.jsonc`, `SettingIndex.jsonc`, `ModeIndex.jsonc`: inspectable interoperability data.
- `Backup/current_state.json`, `Backup/history.log`: CLI-owned current and previous mapping/intent state.
- `.cfgfc-session/`: CLI-owned PPID context records under the effective root.
- `.cfgfc-transactions/`: reserved CLI-owned mutation lock, manifest, staging, and recovery data.

Runtime, session, and transaction files are not user-editable contracts.

## Identity and target model

Canonical identity comes from index map keys and corresponding resource paths. `displayName` is presentation-only; `aliases` are alternative command inputs. Commands resolve aliases before persistence, so Mode selections, apply intents, and contexts store canonical identities.

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

## Apply and reconciliation

`apply mode` and `apply column` persist canonical intent and mappings. `refresh` replans that intent from current metadata; it can also refresh legacy mapping-only state. Content byte changes do not require refresh because symlinks keep the same source path.

`sync` is reserved for external changes such as Git or direct valid JSONC edits. It adds discovered resources and immediately removes indexed Project, Column, and Setting metadata whose filesystem sources disappeared. It does not recreate absent sources or implicitly cascade Mode/runtime references; `apply` and `refresh` fail before target changes when required references cannot resolve. There is no `sync --prune --yes` workflow, and sync does not restore deleted metadata.

## Output contract

Human mode is concise. JSON mode emits exactly one success object to stdout or one error object to stderr, without ANSI or unrelated prose. Exit codes classify usage, resource/scope, invalid data, refusal, and persistence/transaction failures.
