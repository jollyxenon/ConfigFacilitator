## 1. CLI foundation and compatibility break

- [x] 1.1 Add Cobra to the Go module and build a fresh injectable root command that preserves `Run` and `RunWithExecutable` testability for stdin, stdout, stderr, executable path, home, environment, and PPID.
- [x] 1.2 Define typed command errors, the documented exit-code classes, and shared human/JSON renderers that emit one stable JSON envelope and no ANSI or extra prose in JSON mode.
- [x] 1.3 Register `project`, `column`, `setting`, `mode`, `use`, `status`, `apply`, `refresh`, `sync`, `root`, `reset`, and `revert` with shared `-p`/`--project`, `--json`, help, argument validation, and mutually exclusive scope flags.
- [x] 1.4 Add CLI tests proving removed `new`, `switch`, `list`, `update`, flag-only apply, `-f`, and `--force` forms fail as usage errors without mutating a fixture.
- [x] 1.5 Split the former monolithic CLI implementation into command-family files and remove obsolete manual parsers, placeholder routing, and old help tables after equivalent new routes are covered.

## 2. Repository persistence and transaction core

- [x] 2.1 Create repository load/save APIs for all indexes, current state, history, and session records, preserving canonical key identity, descriptions, aliases, target arrays, and unknown JSONC fields.
- [x] 2.2 Implement same-directory temporary-file writes, permission preservation where applicable, flush/close/rename error handling, and unit tests showing readers never observe partial index or runtime files.
- [x] 2.3 Implement a warehouse-wide exclusive mutation lock and reserve `.cfgfc-transactions/` so warehouse and Setting discovery ignore transaction and staging artifacts.
- [x] 2.4 Implement prepared/committed transaction manifests and exact pre-mutation snapshots for affected files, moved paths, managed links, and PPID context records.
- [x] 2.5 Implement rollback for ordinary failures and automatic recovery of incomplete prepared transactions before every mutating command, including tests with injected failures at each commit stage.
- [x] 2.6 Expose incomplete-transaction diagnostics to read-only status without letting read-only commands silently mutate or discard recovery state.

## 3. Resource inspection and metadata mutation

- [x] 3.1 Implement shared canonical-name, display-name, description, description-file/stdin, alias-replacement, alias-clearing, reserved-name, and per-scope collision validation.
- [x] 3.2 Implement `project list/show/create/set` so creation commits the Project directory, indexes, and runtime files together and requires no subsequent sync.
- [x] 3.3 Implement `column list/show/create/set` so a new Column starts with zero target positions and an immediately usable empty Setting index.
- [x] 3.4 Implement `setting list/show/create/set` for empty file-backed and directory-backed Settings, including kind reporting, inherited target arrays, aliases, and unresolved-reference inspection.
- [x] 3.5 Implement `mode list/show/create/set` so a new Mode has no placeholder Column selections and metadata mutations preserve existing selections and extension fields.
- [x] 3.6 Add command and repository tests for explicit project scope, selected context, aliases, conflicting aliases, stdin descriptions, unresolved references after sync, JSON envelopes, and all documented exit-code classes.

## 4. Column and Setting target management

- [x] 4.1 Add a typed logical target-position model that converts between CLI-facing directory/name inheritance and the existing `targetNumber`, default arrays, and Setting override arrays.
- [x] 4.2 Implement `column target list/add/set/delete`, including zero-based index validation, `--name-from-setting`, confirmation, and lockstep resizing or removal across every Setting override.
- [x] 4.3 Implement `setting target list/set/reset`, including independent `--inherit-dir` and `--inherit-name` semantics and validation against current Column target positions.
- [x] 4.4 Reuse planner validation to reject empty effective directories, invalid target names, unresolved variables where planning is requested, and duplicate expanded targets before relevant commits.
- [x] 4.5 Add tests covering zero, one, and multiple positions; fixed and Setting-derived names; insert extension; middle deletion and reindexing; inherited components; invalid indices; and unchanged unknown fields.

## 5. Mode selection management

- [x] 5.1 Implement `mode column list/set/delete` with canonical Column/Setting persistence and `cover`, `increment`, `none`, and `full` validation.
- [x] 5.2 Require one or more repeated `--setting` values for `cover` and `increment`, reject Setting values for `none` and `full`, and reject missing or ambiguous references before writing.
- [x] 5.3 Add tests proving Mode selection CRUD is immediately applicable, empty new Modes contain no invalid selections, and unrelated Mode metadata and selections are preserved.

## 6. Setting content management

- [x] 6.1 Implement a Setting-root path validator that rejects absolute, empty where disallowed, dot/traversal, escaped, and symlink-component paths on Linux, Windows, and macOS path forms.
- [x] 6.2 Implement file and directory import staging for `setting create --from`, accepting only regular files/directories and rejecting symlinks and special filesystem objects without partial resources.
- [x] 6.3 Implement mutually exclusive `--stdin`, `--text`, and `--from` content sources, exact-byte semantics, source-kind checks, and empty source creation.
- [x] 6.4 Implement `setting content list/read` with lexical relative paths, kinds and sizes, exact human byte output, UTF-8 JSON text, and explicit base64 JSON fallback.
- [x] 6.5 Implement atomic `setting content write` for file-backed sources and nested directory-backed files, including automatic parent creation and refusal to replace directories.
- [x] 6.6 Implement directory-only `content mkdir/move/delete`, destination-exists refusal, `--yes` confirmation, and prohibition on deleting or moving the Setting root through content commands.
- [x] 6.7 Add tests for nested trees, binary and UTF-8 reads, exact no-newline text, recursive imports, symlink attacks, traversal attempts, move conflicts, rollback, and immediate visibility through active file and directory symlinks.

## 7. Transactional rename

- [x] 7.1 Build canonical rename plans for Project, Column, Setting, and Mode that enumerate filesystem moves, index-key rewrites, Mode references, current intent/mappings, history entries, PPID contexts, and managed links before commit.
- [x] 7.2 Implement schema-defined reference and descendant source-path rewriting while preserving display names, aliases, descriptions, targets, Setting contents, and unknown fields.
- [x] 7.3 Integrate linker ownership inspection so active targets are recreated against renamed sources and drift blocks rename unless `--force-targets` is supplied.
- [x] 7.4 Implement `project/column/setting/mode rename` on the transaction engine with exact rollback of paths, indexes, runtime records, contexts, and links.
- [x] 7.5 Add rename tests for inactive and active resources, fixed versus Setting-derived target names, multiple Mode references, current and historical intents, mapping-only state, selected Project context, drifted targets, forced reclamation, and injected failures.

## 8. Transactional deletion and cascade

- [x] 8.1 Build dependency reports for resource deletion covering Mode selections, current mappings, current intent, history references, and PPID contexts, and render them in human and JSON errors.
- [x] 8.2 Enforce non-interactive `--yes` confirmation on every specified resource, Column-target, and Setting-content delete path, independently from `--cascade` and `--force-targets`.
- [x] 8.3 Implement non-cascade deletion for resources without dependencies and prove source data and index entries commit or rollback together.
- [x] 8.4 Implement Setting cascade rewriting, including Mode Setting removal, removal of empty `cover`/`increment` selections, current/history mapping removal, and direct-intent repair.
- [x] 8.5 Implement Column cascade rewriting, including all contained Setting mappings, Mode Column selections, and direct-Column intent repair while preserving unrelated Columns.
- [x] 8.6 Implement Mode cascade rewriting so matching current/history intents are removed while valid mappings become mapping-only state.
- [x] 8.7 Implement Project cascade deletion so recorded targets are reset under ownership rules, matching PPID contexts are removed, and the Project path and Project index entry commit together.
- [x] 8.8 Add deletion tests for missing confirmation, each dependency class, cascade isolation, history rewrite, owned/drifted targets, `--force-targets`, recursive target reclamation, and rollback at every destructive stage.

## 9. Use, status, apply, refresh, reset, and revert

- [x] 9.1 Replace switch behavior with `use <Project|global>`, canonical alias persistence, and transactional PPID context rewrites/clears during Project rename/delete.
- [x] 9.2 Replace list behavior with `status`, retaining warehouse summaries, active Mode matching, per-Column coverage, current intent/mappings, ANSI rules, and adding unresolved-reference and incomplete-transaction reporting.
- [x] 9.3 Implement `apply mode <Mode>` and `apply column <Column> <Setting>...`, persist canonical intents, remove old forms, and expose only `--force-targets` for target reclamation.
- [x] 9.4 Rename update behavior to `refresh`, preserving intent-aware, mapping-only, Column-scoped, and all-Project replanning while enforcing new scope syntax and `--force-targets`.
- [x] 9.5 Update reset and revert to the new shared scope, error, JSON, and `--force-targets` contracts without changing one-step revert semantics.
- [x] 9.6 Add end-to-end tests for context precedence, status human/JSON output, Mode and direct-Column apply, full-Mode refresh after adding a Setting, mapping-only refresh, Column refresh isolation, all-Project refresh, reset, revert, and forced target recovery.

## 10. Synchronization and dangling references

- [x] 10.1 Change warehouse loading and reconciliation so absent indexed Project directories, Column directories, and Setting files/directories are removed from their corresponding indexes during sync; Mode and runtime references remain inspectable as unresolved.
- [x] 10.2 Change sync serialization to add discovered resources, remove disappeared filesystem-backed index entries, preserve metadata for present resources, and never recreate absent source paths or child indexes.
- [x] 10.3 Remove the obsolete `sync --prune --yes` workflow and reject prune flags without mutating indexes.
- [x] 10.4 Reject apply and refresh plans requiring unresolved resources before managed-target changes; do not implicitly cascade Mode or runtime references during sync.
- [x] 10.5 Replace existing missing-retention/restoration/prune expectations with immediate index removal, no metadata restoration through sync, dangling-reference, context/default/all scope, and `SettingWarehouse` discovery tests.

## 11. Help, completion, and documentation

- [x] 11.1 Add standardized root and nested help for every runnable command, including argument forms, project context, JSON behavior, content-source exclusivity, confirmation, cascade, target reclamation, and examples.
- [x] 11.2 Add shell completion generation and dynamic completion for Project, Column, Setting, and Mode canonical names and aliases where Cobra supports it.
- [x] 11.3 Rewrite `README.md`, bilingual command, example, architecture, JSONC, platform, and developer documents around the CLI-only lifecycle and add an explicit old-to-new command migration table.
- [x] 11.4 Update `skills/configfacilitator-usage/SKILL.md` so agents use resource commands, stdin/content commands, status, immediate sync removal semantics, and separated destructive controls instead of direct warehouse editing.
- [x] 11.5 Update repository rules or developer workflow documentation where command sweeps, smoke paths, or validation commands changed, maintaining English/Chinese parity.

## 12. Verification and release readiness

- [x] 12.1 Run `pixi run test`, `pixi run compile`, `pixi run build`, `pixi run help`, and a help sweep covering every top-level command and runnable nested command.
- [x] 12.2 Run a temp-HOME, alternate-root smoke test that uses only `cfgfc` plus stdin to create metadata, targets, file/directory content, Modes, apply, content edit, refresh, rename active resources, cascade delete, reset, revert, external disappearance, immediate index removal, dangling-reference failure, and explicit resource recreation.
- [x] 12.3 Run destructive smoke coverage proving `--yes`, `--cascade`, and `--force-targets` remain independent and reclaim only recorded file-, symlink-, and directory-backed targets.
- [x] 12.4 Run transaction fault-injection and restart-recovery tests, race-safe concurrent mutation tests, and native Windows path/symlink guidance tests.
- [x] 12.5 Run `cd npm && npm pack --dry-run`, local global install with `CFGFC_BINARY_PATH=../dist/cfgfc`, installed CLI help/lifecycle smoke, and unsupported-platform installer failure coverage.
- [x] 12.6 Run `openspec validate replace-editor-workflows-with-cli --strict` and resolve every requirement, scenario, and artifact validation error before implementation is considered complete.
