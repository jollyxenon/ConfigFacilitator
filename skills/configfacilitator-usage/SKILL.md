---
name: configfacilitator-usage
description: Guide agents using cfgfc resource, content, apply, reconciliation, and recovery commands without direct warehouse editing.
license: MIT
compatibility: Requires cfgfc CLI and ConfigFacilitator repository docs.
metadata:
  author: ConfigFacilitator
  version: "2.0"
  generatedBy: "1.3.1"
---

Use this Skill whenever an agent inspects or changes a ConfigFacilitator warehouse.

## Required reading

1. Read repository `README.md` and `AGENTS.md`.
2. Read `docs/commands.en.md` or `docs/commands.zh-CN.md` for exact syntax.
3. For a complete lifecycle, read `docs/example.en.md` or `docs/example.zh-CN.md`.
4. For persistence, paths, and validation, read the matching JSONC, platform, architecture, and developer pages.
5. Run `cfgfc <command> --help` before using unfamiliar or destructive forms.

## Core rule: use cfgfc, not an editor

Normal warehouse work must use resource and content commands. Do not directly create or edit Project/Column/Setting/Mode indexes, Setting source files, target arrays, Mode selections, runtime state, session records, or transaction records.

External JSONC or filesystem changes are supported only as an interoperability boundary. After Git or another explicit external tool changes a warehouse, inspect and reconcile with `cfgfc sync`.

Never manually edit:

- `Backup/current_state.json`
- `Backup/history.log`
- `.cfgfc-session/`
- `.cfgfc-transactions/`

## Command map

| Task | Commands |
| --- | --- |
| Root | `cfgfc root`, `cfgfc root <Path>` |
| Project context | `cfgfc use <Project>`, `cfgfc use global` |
| Project CRUD | `cfgfc project list/show/create/set/rename/delete` |
| Column CRUD | `cfgfc column list/show/create/set/rename/delete` |
| Column targets | `cfgfc column target list/add/set/delete` |
| Setting CRUD | `cfgfc setting list/show/create/set/rename/delete` |
| Setting target overrides | `cfgfc setting target list/set/reset` |
| Setting content | `cfgfc setting content list/read/write/mkdir/move/delete` |
| Mode CRUD | `cfgfc mode list/show/create/set/rename/delete` |
| Mode selections | `cfgfc mode column list/set/delete` |
| Runtime inspection | `cfgfc status` |
| Activate the Current | `cfgfc apply mode ...` (Current follows the Mode), `cfgfc apply column ...` (independent Current) |
| Replan active state | `cfgfc refresh`, `cfgfc refresh --all` (single-Column `--column` was removed) |
| Current state | `cfgfc current show`, `cfgfc current column list/set/delete` |
| Web UI | `cfgfc web [--port ...]` on `127.0.0.1` (default `38031`) |
| External reconciliation | `cfgfc sync`, `cfgfc sync -p ...`, `cfgfc sync --all` |
| Disappeared-resource reconciliation | `cfgfc sync`, which removes corresponding Index metadata immediately without cascading references |
| Managed-state recovery | `cfgfc reset`, `cfgfc revert` |
| Shell completion | `cfgfc completion <bash|zsh|fish|powershell>` |

The removed `new`, `switch`, `list`, and `update` commands, flag-only apply forms, `-a`, `-f`, and `--force` are not compatibility aliases.

## Scope and identity

- Prefer explicit `-p/--project` when automation should not depend on shell context.
- Otherwise, select a Project with `cfgfc use <Project>`; `cfgfc use global` clears the PPID-scoped context.
- Setting commands require `-c/--column`.
- Canonical names and unique aliases are accepted as input; cfgfc persists canonical identities.
- `displayName` is presentation-only, not an alias.
- Before mutation, inspect relevant resources with `list`/`show` and active state with `status`.
- Use `--json` for automation and parse the single stable envelope. Respect exit codes `2` through `6` documented in the command reference.

## Safe creation and content workflow

1. Confirm the effective root with `cfgfc root`. Change it only with explicit user intent; changing roots does not migrate data.
2. Create the Project and select it or pass `-p` explicitly.
3. Create Columns, then configure zero-based positions with `column target`.
4. Create Settings with `--kind file|directory`:
   - use `--stdin` for exact file bytes from stdin;
   - use `--text` for exact literal bytes without an added newline;
   - use `--from` to copy a regular file or directory tree;
   - omit all three for an empty file/directory.
5. Use `setting content` for later inspection and mutation. Never substitute direct source editing.
6. Create Modes and configure selections with `mode column set`; repeat `--setting` for `cover`/`increment`, and omit it for `none`/`full`.
7. Apply with nested syntax and verify with `status` plus targeted content/link checks.

`--from`, `--stdin`, and `--text` are mutually exclusive. Directory imports and content paths reject symlinks, special objects, absolute paths, and traversal. For human `setting content read`, preserve exact output bytes; use JSON when text/base64 encoding metadata is needed.

Content writes under an existing source path are immediately visible through active symlinks. Do not run `refresh` for byte-only changes. Use `refresh` after target/selection metadata changes, or when a `full` selection must include a newly created/discovered Setting.

## Current state and automatic synchronization

(Current) is a temporary Mode: `current show` reports its `relation` (kind `following` or `detached`, or none for independent), per-Column selections, and planned mappings. `apply mode` makes the Current follow a Mode; `apply column` sets an independent Current. Editing the Current directly (`current column set/delete` or a Web UI write) turns `following` into `detached` while keeping `originMode`. Deleting a followed Mode clears only `relation`; renaming a followed Mode updates `originMode`.

Mutations that change what the Current would plan re-plan it in the same transaction: changing the selections of a followed Mode, or changing resources that affect planning (targets, renames, deletions, a new Setting entering a `full` selection). Description/display-name/alias changes and content byte changes do not rebuild links. `revert` rewinds only Current state, never resource metadata or content.

## Synchronization and disappeared resources

`sync` is not needed after successful CLI-owned mutations. Use it after Git, file-manager, or valid direct JSONC interoperability changes.

- Sync discovers new resources.
- If an indexed Project directory, Column directory, or Setting file/directory disappears, sync immediately removes its metadata from the corresponding Index.
- Sync does not recreate absent sources or child indexes.
- Sync does not implicitly cascade Mode selections, current/history runtime records, or PPID context.
- Inspect unresolved references with `status` and relevant `show` commands; apply/refresh fails before target changes when required references cannot resolve.
- `sync --prune` and `sync --prune --yes` are unsupported.
- Recreating a former source path does not restore deleted metadata; recreate intended metadata through resource commands.
- `sync --all` and `-p/--project` are mutually exclusive.
- Each Project commits in its own transaction, so `sync --all` isolates failures per Project.
- Missing Project/Column/Setting index entries are rebuilt from the filesystem with canonical names and minimal default metadata; a missing `ModeIndex.jsonc` is rebuilt empty. A missing `current_state.json` is rebuilt as an empty Current and the old `history.log` is deleted; a legacy-format or corrupted one keeps only that Project unusable until the user deletes it and syncs again.
- `sync` also replans a `following` Current from the origin Mode's latest selections.

## Independent destructive controls

Treat these as separate user authorizations:

- `--yes`: confirm resource, Column-target, or Setting-content deletion; no interactive prompt exists.
- `--cascade`: permit dependent-reference cleanup during resource deletion.
- `--force-targets`: reclaim only affected recorded target paths whose ownership is unsafe or drifted.

Never infer one authorization from another. Before resource deletion or forced target reclamation:

1. confirm root, Project, Column, resource, and active mappings;
2. run `status` and relevant `show` commands;
3. explain what repository data, references, and recorded targets are affected;
4. require explicit user intent for each needed control;
5. verify unrelated resources and targets remain intact afterward.

`--force-targets` can recursively remove recorded occupied paths. It does not back up or reconstruct unmanaged content.

## Transactions and recovery

Mutating commands recover an incomplete prepared transaction before new work. Read-only `status` only reports transaction diagnostics. If status reports an incomplete transaction, do not delete or edit `.cfgfc-transactions/`; run a suitable mutating cfgfc command only after confirming the effective root and user intent, then verify recovery.

Rename and cascade delete may update indexes, source paths, Mode references, current/history state, PPID contexts, and managed links together. Do not reproduce these operations manually.

## Validation

For docs/Skill-only work, run the applicable help/doc checks plus:

```bash
openspec validate replace-editor-workflows-with-cli --strict
openspec status --change replace-editor-workflows-with-cli --json
```

For command or behavior changes, use the complete help sweep and temp-HOME alternate-root lifecycle smoke in `AGENTS.md` and `docs/developer-setup.*.md`. Run the full Go suite when Go code changes.

## Maintenance

When commands, flags, workflow, output, safety behavior, storage responsibility, or validation changes, update this Skill together with `README.md`, English/Chinese docs, `AGENTS.md`, and CLI help. English and Chinese docs must remain behaviorally equivalent.
