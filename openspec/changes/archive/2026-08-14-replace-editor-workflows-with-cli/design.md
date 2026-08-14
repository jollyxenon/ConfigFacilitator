## Context

The existing CLI routes all commands and hand-parses flags in one large package. Project and Column scaffolding, sync normalization, model loading, planning, link-state persistence, and PPID context are separate packages, but each writes its own files directly. Canonical identities are JSONC map keys coupled to Project, Column, and Setting filesystem names. Mode selections and apply intents store those identities, while current and historical mappings store absolute source paths. These constraints make rename and cascade deletion cross-cutting operations rather than simple filesystem changes.

The warehouse format must remain inspectable and accept valid external JSONC or Git changes. At the same time, CLI-owned operations must no longer require users to understand parallel target arrays or edit indexes. Linux, native Windows, and macOS builds must continue to use real symlinks only.

## Goals / Non-Goals

**Goals:**

- Provide one command model for all supported warehouse metadata and Setting-content mutations.
- Make each CLI-owned mutation atomic from the user's perspective and recover interrupted multi-file operations.
- Preserve existing valid warehouse, current-state, and history files without a second permanent storage model.
- Keep aliases as input conveniences while persisting canonical identities everywhere.
- Keep external filesystem and JSONC changes observable through explicit synchronization, including immediate index removal for disappeared filesystem-backed resources.
- Make command behavior scriptable through stable JSON envelopes and exit-code classes.

**Non-Goals:**

- Editing arbitrary files outside a Setting source root.
- Providing an interactive text editor, TUI, GUI, or shell prompt workflow.
- Following or importing symbolic links inside Setting content.
- Providing arbitrary history checkout beyond the existing one-step `revert` behavior.
- Preserving the removed command syntax or automatic compatibility aliases.
- Changing the real-symlink-only target application model.

## Decisions

### 1. Use nested resource commands backed by Cobra

The CLI will use Cobra to represent top-level resources and nested actions. `-p` / `--project`, `--json`, and output/error handling will be defined consistently rather than copied into hand-written parsers. Dynamic completion can resolve canonical names and aliases from the effective warehouse.

The root command will set `SilenceUsage` after parsing so runtime errors do not print full help. Each runnable command will validate positional arguments and mutually exclusive flags before loading mutable state. The existing `Run` and `RunWithExecutable` test entry points will construct and execute a fresh root command with injected stdin, stdout, stderr, executable path, environment, home path, and PPID dependencies.

Alternative considered: extend the existing parser. Rejected because dozens of nested verbs and shared flags would duplicate parsing, help, and validation logic and further enlarge `internal/cli/cli.go`.

### 2. Separate command parsing, domain mutation, and persistence

The implementation will introduce three boundaries:

- `internal/cli`: command construction, argument validation, output envelopes, and translation into typed requests.
- `internal/repository`: warehouse loading, atomic index/runtime serialization, transaction records, snapshots, recovery, and canonical reference rewrites.
- `internal/content`: bounded imports and Setting-root content operations.

`internal/mutate` will contain small use cases for Project, Column, Setting, Mode, target, and selection operations. It will call `repository` rather than writing files directly. Existing `scaffold` behavior will be absorbed into create mutations. `syncer` will use the same repository writer and immediate disappearance-removal rules. The planner and linker remain responsible for mapping semantics and managed targets, but repository transactions coordinate them for rename and cascade.

### 3. Keep current JSONC schemas and hide parallel-array mechanics behind typed operations

Project, Column, Setting, and Mode indexes retain their existing shapes, key-derived identity, `Extra` unknown fields, and target arrays. Column target commands will modify a logical `TargetPosition{Dir, NameMode, Name}` representation and serialize it back to `targetNumber`, `defaultTargetDir`, and `defaultTargetName`. Setting target operations will expose inherited or explicit components and serialize inheritance as empty array entries.

Deleting a Column target position removes the same zero-based index from both defaults and every Setting override before decrementing `targetNumber`. Adding one extends every array. The repository validates equal lengths before commit. This avoids a disruptive warehouse migration while removing array bookkeeping from the user interface.

Alternative considered: replace arrays with an array of target objects. Rejected for this change because it would combine a large persisted-format migration with the command redesign and make external repository upgrades harder to diagnose.

### 4. Use durable snapshot transactions for cross-artifact operations

A reserved warehouse-root directory, `.cfgfc-transactions/`, will hold at most one active mutation per effective warehouse. Before the first durable change, the repository will:

1. acquire an exclusive warehouse mutation lock;
2. finish rollback of any incomplete transaction;
3. validate the full request and compute affected paths and target mappings;
4. create a transaction directory containing a manifest and exact pre-mutation copies of every affected repository/runtime file plus metadata needed to restore moved paths and target links;
5. fsync the manifest and mark it `prepared`;
6. perform staged repository writes and moves, then managed-link changes;
7. mark the transaction `committed` and remove it.

Individual files will be written to same-directory temporary files and renamed into place. Content creation/import will stage beside the final source and rename only during commit. Source renames will first move into transaction staging, allowing restoration even when final installation fails.

On a failure after preparation, rollback restores target ownership, moved source paths, indexes, runtime files, and session records from the manifest. A later mutating command performs the same rollback when it finds `prepared` without `committed`. Read-only commands report an incomplete transaction in status but do not mutate it.

The lock is warehouse-wide because Project rename, ProjectIndex changes, and PPID contexts cross Project boundaries. This favors correctness and a simple recovery model over parallel mutations, which are expected to be infrequent.

Alternative considered: best-effort compensating actions without a durable journal. Rejected because process termination can otherwise leave canonical keys, paths, and managed links disagreeing.

### 5. Rewrite all persisted identities and paths during rename

A rename plan will resolve all aliases first and then operate only on canonical identities. It will rewrite:

- the owning index key and filesystem path;
- Mode Column keys and Setting lists;
- current `ApplyIntent` fields;
- current mapping source paths;
- every history entry's previous/next intents and mapping source paths;
- all matching PPID Project context records.

Source paths are rewritten only when they are equal to or descendants of the renamed canonical path. Target paths do not change unless their configured target name derives from the Setting canonical name. Existing managed links are inspected before mutation, removed using normal or `--force-targets` ownership rules, and recreated against rewritten sources inside the same transaction.

History remains newline-delimited JSON with rewritten entries. No new version marker is required because field shapes do not change.

### 6. Treat delete as dependency planning plus transactional commit

Delete first builds a dependency report containing Mode references, current mappings, current intent, history references, and selected PPID contexts. `--yes` authorizes deletion; `--cascade` authorizes rewriting dependent records; `--force-targets` authorizes reclaiming only recorded affected targets.

Without cascade, any dependency blocks deletion. With cascade:

- Setting removal deletes it from Mode selections, mappings, and direct-Column intent; empty `cover` or `increment` selections are removed.
- Column removal deletes all Mode selections and all Setting mappings for that Column; a direct intent for it is cleared.
- Mode removal clears matching current/history intents but retains existing mapping records as mapping-only state.
- Project removal resets affected targets, removes matching contexts, and deletes its directory and Project index entry.

History entries are rewritten rather than discarded wholesale so one-step revert remains meaningful for unaffected resources. If an entry becomes mapping-only, its intent is omitted.

### 7. Make sync an explicit external-reconciliation boundary

CLI mutations write normalized indexes immediately and do not invoke sync. `sync` remains for Git pulls, file-manager changes, or direct JSONC interoperability. Discovery merges filesystem presence with indexed metadata:

- new filesystem resources receive default metadata;
- absent indexed Project directories, Column directories, and Setting files/directories are removed from their corresponding indexes immediately;
- sync never recreates absent source paths or child indexes merely to normalize another index;
- Mode metadata, current/history runtime records, and PPID context are not implicitly cascaded or rewritten when sync removes an index entry;
- apply and refresh fail before target changes when their persisted Mode or runtime references no longer resolve.

No `sync --prune --yes` workflow is required. Restoring a deleted source path does not restore metadata; users must recreate metadata through resource commands.

### 8. Enforce Setting-root boundaries without following symlinks

All content paths are parsed as platform-native relative paths but rejected if absolute, empty where not allowed, or containing `.`/`..` traversal. The content layer walks existing path components with `Lstat`, rejects any symlink, and verifies the cleaned destination remains a descendant of the Setting root.

Directory import recursively accepts only regular files and directories and rejects symlinks and special files. File replacement uses a temporary sibling plus rename. Directory mutations never operate on the Setting root itself; deleting the whole Setting belongs to resource deletion.

### 9. Centralize result envelopes and exit classification

Commands return typed errors with one of five non-success classes: usage, resource/scope conflict, invalid data, destructive/ownership refusal, or persistence/transaction failure. The human renderer prints concise text. The JSON renderer emits exactly one success object to stdout or one error object to stderr, without color or extra logs. Read-content JSON encodes invalid UTF-8 as base64 with an encoding field.

The same command handler produces both outputs to prevent behavior divergence. Help remains human-readable and is outside command JSON envelopes.

### 10. Preserve the link engine's ownership model while renaming force semantics

The linker will retain its current absent/owned/unmanaged inspection and real-symlink implementation. Public CLI flags change from `-f` / `--force` to `--force-targets`; internal options will be renamed accordingly. Repository confirmation and cascade are independent and never imply target reclamation.

## Risks / Trade-offs

- **[Large breaking command change]** Existing scripts stop working immediately. → Publish a complete old-to-new command migration table, remove old help entries, and test that old syntax fails without mutation.
- **[Transaction restoration fails because of external concurrent writes]** A process outside cfgfc can change snapshotted paths while a transaction runs. → Hold the cfgfc mutation lock, keep transactions short, verify expected ownership before each destructive step, and retain the recovery directory with a precise diagnostic if automatic rollback cannot finish.
- **[History rewriting is expensive]** Large newline-delimited history files must be parsed and rewritten for rename/cascade. → Stream parsing into a staged replacement file and commit by rename; no full in-memory history requirement.
- **[Windows rename and symlink behavior differs]** Open handles and privilege requirements may block staging or link recreation. → Use same-volume sibling staging, keep existing Windows symlink guidance, and add native-path unit tests plus documented Windows smoke coverage.
- **[Sync can leave dangling references]** External deletion removes filesystem-backed index entries without implicit cascade. → Keep Mode/runtime references inspectable, report unresolved references, and fail apply/refresh before managed-target changes; use explicit resource deletion when dependent cleanup is intended.
- **[Unknown JSONC fields may contain hidden references]** The CLI cannot safely rewrite arbitrary extension-field semantics. → Preserve unknown fields byte values but rewrite only schema-defined references; document that extensions owning references must manage those fields themselves.
- **[Warehouse-wide lock limits concurrency]** Independent Project mutations serialize. → Accept the trade-off for a single-user local configuration manager; reads remain unlocked except when they detect an incomplete transaction.

## Migration Plan

1. Add Cobra and the new command tree behind the existing executable entry point while tests still exercise existing packages directly.
2. Add repository writers, typed errors, JSON rendering, transaction lock/recovery, and snapshot tests before exposing destructive commands.
3. Implement resource inspection/create/set and target/selection commands, then Setting content operations.
4. Implement transactional rename and cascade deletion, including state, history, context, and managed-link rewrites.
5. Change sync to remove disappeared filesystem-backed index entries immediately; update model behavior and regression tests to cover dangling Mode/runtime references and no metadata restoration.
6. Replace apply/update/switch/list public routes with apply/refresh/use/status and remove old parsers in the same release.
7. Update bilingual docs, Skill guidance, help, completion, and old-to-new migration examples.
8. Run the full baseline, temp-HOME and alternate-root lifecycle smoke tests, native path-focused tests, and npm packaging checks.

Rollback before release is a normal code revert because no released persistent schema has changed. After users perform new rename/cascade operations, rollback to an older binary remains possible for ordinary load/apply behavior because index, state, and history shapes remain compatible, but older binaries will not understand active transaction recovery. Release documentation will require completing or recovering any transaction with the new binary before downgrading.
