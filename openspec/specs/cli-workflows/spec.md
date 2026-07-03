# cli-workflows Specification

## Purpose
TBD - created by archiving change bootstrap-main-specs. Update Purpose after archive.
## Requirements
### Requirement: CLI exposes the required command families
The system SHALL expose the command families `new`, `sync`, `switch`, `list`, `apply`, `update`, `reset`, `revert`, and `root` through the `cfgfc` CLI.

#### Scenario: Inspecting available commands
- **WHEN** a user inspects the CLI help surface
- **THEN** the required command families are present as part of the supported interface

### Requirement: New commands scaffold editable templates
The `new` workflows SHALL create editable project, column, and mode scaffolding that includes template guidance and the required metadata fields, and any `new` workflow that accepts `-p <ProjectName>` SHALL also resolve the active switched project when `-p` is omitted. The `new` workflow SHALL reject `global` as a project name. When a `new` workflow reports a resolved project in a user-facing status message, it SHALL render that project with its display-oriented label instead of echoing the raw shorthand input.

#### Scenario: Creating a new mode template
- **WHEN** the user creates a mode scaffold
- **THEN** the resulting editable artifact includes the structure needed for the user to fill in mappings and descriptions
- **AND** any generated entity example in that artifact includes `"aliases": []` as part of the editable metadata shape
- **AND** the generated ModeIndex guidance describes `cover`, `increment`, `none`, and `full` using their current semantics

#### Scenario: Creating a new project scaffold
- **WHEN** the user runs `cfgfc new -p <ProjectName>`
- **THEN** the CLI creates the project's `Column/`, `Mode/`, and `Backup/` directories, updates `ProjectIndex.jsonc`, and creates the required project-local index/state files

#### Scenario: Creating a new column scaffold
- **WHEN** the user runs `cfgfc new -p <ProjectName> -c <ColumnName>`
- **THEN** the CLI creates the column directory and a `SettingIndex.jsonc` template containing editable guidance and required metadata fields

#### Scenario: Creating a column scaffold through an aliased project reference
- **WHEN** the user runs `cfgfc new -p <ProjectAlias> -c <ColumnName>` and that alias resolves uniquely
- **THEN** the CLI creates the requested scaffold in the resolved project
- **AND** the success message identifies that project by its display-oriented label instead of the raw alias text

#### Scenario: Creating a new column scaffold from switched context
- **WHEN** the user runs `cfgfc switch <ProjectName>` and then runs `cfgfc new -c <ColumnName>` without `-p`
- **THEN** the CLI resolves `<ProjectName>` from the active convenience context and creates the requested column scaffold in that project

#### Scenario: Explicit project selection overrides switched context for new
- **WHEN** the user runs `cfgfc switch <ProjectA>` and then runs `cfgfc new -p <ProjectB> -c <ColumnName>`
- **THEN** the CLI creates the requested scaffold in `<ProjectB>` instead of the switched project context

#### Scenario: Rejecting the reserved global project name
- **WHEN** the user runs `cfgfc new -p global`
- **THEN** the CLI fails with an error that explains `global` is reserved

### Requirement: Sync reconciles filesystem reality with index metadata
The `sync` workflow SHALL scan the warehouse, add newly discovered entities into index data, and preserve missing indexed entities instead of deleting them. When `sync` accepts `-p <ProjectName>`, omitting `-p` SHALL cause the CLI to sync the active switched project when one is available, and SHALL fall back to syncing every project only when no effective project can be resolved. The `sync` workflow SHALL accept `--all` and `-a` as explicit warehouse-wide sync flags, and SHALL treat `global` as a reserved project target name. When syncing one resolved project, the success message SHALL report the resolved project with its display-oriented label instead of the raw alias text.

#### Scenario: Sync discovers a new setting file
- **WHEN** the user adds a new filesystem setting and runs sync
- **THEN** the corresponding index metadata is updated to include the discovered entity

#### Scenario: Sync rewrites a project's indexes
- **WHEN** the user runs `cfgfc sync -p <ProjectName>`
- **THEN** the project's warehouse indexes are rewritten from the reconciled model while keeping required descriptions and missing markers intact

#### Scenario: Syncing a project through an alias
- **WHEN** the user runs `cfgfc sync -p <ProjectAlias>` and that alias resolves uniquely
- **THEN** the CLI syncs the matching project successfully
- **AND** the success message identifies the project by its display-oriented label instead of the raw alias text

#### Scenario: Sync rewrites the switched project's indexes
- **WHEN** the user runs `cfgfc switch <ProjectName>` and then runs `cfgfc sync` without `-p`
- **THEN** the CLI resolves `<ProjectName>` from the active convenience context and rewrites only that project's indexes

#### Scenario: Sync rewrites all project indexes when no project is resolved
- **WHEN** the user runs `cfgfc sync` without `-p`
- **THEN** the CLI reconciles every project under the warehouse root instead of only one project

#### Scenario: Sync includes `SettingWarehouse` at the root level
- **WHEN** the user runs `cfgfc sync --all` in a warehouse whose root contains a project directory named `SettingWarehouse`
- **THEN** the CLI treats that directory like any other root-level project during discovery and index reconciliation

#### Scenario: Explicit project selection overrides switched context for sync
- **WHEN** the user runs `cfgfc switch <ProjectA>` and then runs `cfgfc sync -p <ProjectB>`
- **THEN** the CLI syncs only `<ProjectB>` instead of the switched project context

#### Scenario: Explicit all-project sync ignores switched context
- **WHEN** the user runs `cfgfc switch <ProjectA>` and then runs `cfgfc sync --all`
- **THEN** the CLI performs a warehouse-wide sync instead of syncing only `<ProjectA>`

#### Scenario: Aliased all-project sync is supported
- **WHEN** the user runs `cfgfc sync -a`
- **THEN** the CLI performs a warehouse-wide sync

#### Scenario: Handling the reserved global project target for sync
- **WHEN** the user runs `cfgfc sync -p global`
- **THEN** the CLI fails with an error that explains `global` is reserved

### Requirement: Mode apply clears undeclared columns by default
When applying a mode, the system SHALL clear managed links for columns that are not declared in that mode.

#### Scenario: A mode omits a previously active column
- **WHEN** apply runs for a mode that does not declare one of the project's columns
- **THEN** the managed links for that omitted column are removed as part of the apply lifecycle

#### Scenario: Applying a mode after a previous column-only apply
- **WHEN** a previously active column is not declared by the selected mode
- **THEN** its managed links are excluded from the next mapping set and are removed by the apply operation

### Requirement: Declared columns honor cover, increment, none, and full strategies
When a mode declares a column, the system SHALL honor the configured strategy for that column and distinguish between explicit replacement, additive reuse, intentional no-linking, and full-column linking.

#### Scenario: Applying a column in cover mode
- **WHEN** a mode declares a column with `strategy: cover`
- **THEN** the CLI clears the existing managed links for that column before applying the newly selected settings

#### Scenario: Applying a column in increment mode
- **WHEN** a mode declares a column with `strategy: increment`
- **THEN** the CLI preserves existing managed links for that column and adds the newly selected settings on top

#### Scenario: Increment mode keeps prior mappings for that column
- **WHEN** a column already has managed mappings and the selected mode applies that same column with `strategy: increment`
- **THEN** the resulting mapping set contains both the prior mappings for that column and the newly selected settings

#### Scenario: Applying a column in none mode
- **WHEN** a mode declares a column with `strategy: none`
- **THEN** the CLI contributes no mappings for that column even if the column contains Settings or previously managed links

#### Scenario: Applying a column in full mode
- **WHEN** a mode declares a column with `strategy: full`
- **THEN** the CLI resolves every known Setting in that column and contributes them as the column's next managed mappings

#### Scenario: Full mode does not require authored settings
- **WHEN** a mode declares a column with `strategy: full` and omits `settings`
- **THEN** the CLI still applies every known Setting in that column successfully

#### Scenario: None mode does not require authored settings
- **WHEN** a mode declares a column with `strategy: none` and omits `settings`
- **THEN** the CLI still applies the mode successfully without resolving any Setting from that column

### Requirement: Single-column apply accepts explicit settings input
The CLI SHALL allow single-column apply with one or more explicitly named settings, SHALL resolve the effective project from either explicit `-p <ProjectName>` input or the active switched project context, and successful file-backed applies SHALL produce managed links whose readable contents match the selected source files. When a single-column apply succeeds, the success message SHALL render the resolved project and column with display-oriented labels instead of echoing raw shorthand input. When single-column apply runs with `-f` / `--force`, it SHALL pass a destructive overwrite request into the symlink engine so unmanaged targets and drifted recorded targets do not block the apply.

#### Scenario: Applying multiple settings to one column
- **WHEN** the user invokes single-column apply with a comma-separated list of settings
- **THEN** the CLI resolves that request as one column apply action targeting the named settings

#### Scenario: Single-column apply resolves targets
- **WHEN** the user applies one column with one or more named settings
- **THEN** the CLI resolves each setting's explicit target or inherited default target before sending the mapping set to the engine

#### Scenario: Applying a file-backed setting creates a readable managed link
- **WHEN** the user applies a column setting whose source is a regular file
- **THEN** the resulting managed target path reads back the same contents as the selected source file

#### Scenario: Single-column apply uses switched project context
- **WHEN** the user runs `cfgfc switch <ProjectName>` and then runs `cfgfc apply -c <ColumnName> -s <SettingName>` without `-p`
- **THEN** the CLI resolves `<ProjectName>` from the active convenience context before applying the requested column setting

#### Scenario: Explicit project selection overrides switched context for single-column apply
- **WHEN** the user runs `cfgfc switch <ProjectA>` and then runs `cfgfc apply -p <ProjectB> -c <ColumnName> -s <SettingName>`
- **THEN** the CLI applies the requested setting within `<ProjectB>` instead of the switched project context

#### Scenario: Applying one column through an alias
- **WHEN** the user runs `cfgfc apply -p <ProjectName> -c <ColumnAlias> -s <SettingName>` and the column alias resolves uniquely
- **THEN** the CLI applies the matching column successfully
- **AND** the success message identifies the resolved column by its display-oriented label

#### Scenario: Forced single-column apply overrides an occupied target
- **WHEN** the user runs `cfgfc apply -f -p <ProjectName> -c <ColumnName> -s <SettingName>`
- **AND** one planned target path is currently occupied by an unmanaged file, directory, or mismatched symlink
- **THEN** the CLI still commits the requested column apply instead of failing the workflow at command level

### Requirement: Mode apply preserves readable contents for file-backed links
The CLI SHALL leave file-backed managed links readable with source-matching contents after a successful mode apply. When a mode apply succeeds, the success message SHALL render the resolved project and mode with display-oriented labels instead of echoing raw shorthand input. When mode apply runs with `-f` / `--force`, it SHALL pass a destructive overwrite request into the symlink engine so unmanaged targets and drifted recorded targets do not block the apply.

#### Scenario: Applying a mode with a file-backed setting
- **WHEN** the user applies a mode that selects a regular-file setting
- **THEN** reading through the managed target path returns the same file contents as the mode-selected source file

#### Scenario: Applying a mode through an alias
- **WHEN** the user runs `cfgfc apply -p <ProjectName> -m <ModeAlias>` and that alias resolves uniquely
- **THEN** the CLI applies the matching mode successfully
- **AND** the success message identifies the resolved mode by its display-oriented label

#### Scenario: Forced mode apply reclaims an occupied directory target
- **WHEN** the user runs `cfgfc apply --force -p <ProjectName> -m <ModeName>`
- **AND** one planned target path is currently occupied by a directory that is not owned by the recorded source-target mapping
- **THEN** the CLI still commits the requested mode apply instead of failing the workflow at command level

### Requirement: Switch stores the active project context
The `switch` workflow SHALL store the selected project as PPID-scoped convenience context for later commands, and SHALL accept `global` as an explicit request to clear the active project context for the current PPID scope. When `switch` confirms a resolved project selection, the status message SHALL render that project with its display-oriented label.

#### Scenario: Switching to an existing project
- **WHEN** the user runs `cfgfc switch <ProjectName>` for a project that exists in the current warehouse
- **THEN** the selected project is stored as the convenience context for the current PPID scope

#### Scenario: Switching back to global
- **WHEN** the user runs `cfgfc switch global`
- **THEN** the CLI clears the current PPID-scoped convenience project so later commands resolve in global mode

#### Scenario: Switching through an alias reports the resolved project label
- **WHEN** the user runs `cfgfc switch <ProjectAlias>` and that alias resolves uniquely within the warehouse
- **THEN** the selected project is stored as the convenience context for the current PPID scope
- **AND** the success message identifies the project by its display-oriented label

### Requirement: List shows available projects and project contents
The `list` workflow SHALL display available projects when no effective project is selected, and SHALL display the selected project's columns and modes when a project is resolved. In global mode, each listed project SHALL append one parenthesized usage summary derived from that project's persisted managed state: the resolved active mode label when the current state records a resolvable mode intent and that mode's current metadata-derived managed mapping set still matches the persisted mappings, `Unmatched` when the project still has active mappings or an apply intent that no longer resolves to a current mode or current metadata-derived mappings, and `None` when the project has no active mappings and no apply intent. When color output is enabled, matched mode summaries SHALL use green emphasis, and `Unmatched` / `None` summaries SHALL use red emphasis. When a project is resolved, each listed column SHALL append one parenthesized usage label of `Full`, `Partial`, or `None`, derived from how completely each non-missing setting's current metadata-derived managed mapping set is represented in the persisted mappings. When color output is enabled, `Full`, `Partial`, and `None` SHALL use green, yellow, and red emphasis respectively. The mode list SHALL positively emphasize the mode whose current metadata-derived managed mapping set still matches the persisted active mode intent when one is resolvable.

#### Scenario: Listing without an active project and showing a matched mode
- **WHEN** the user runs `cfgfc list` without an explicit project and no convenience context is set
- **AND** one project has a persisted mode apply intent that still resolves in current warehouse metadata and still matches the persisted managed mappings
- **THEN** the CLI lists that project in the warehouse view
- **AND** the project line appends the resolved mode label in parentheses with positive emphasis when color output is enabled

#### Scenario: Listing without an active project and showing unmatched usage
- **WHEN** the user runs `cfgfc list` without an explicit project and no convenience context is set
- **AND** one project still has active mappings or an apply intent that does not resolve to a current mode or current metadata-derived mappings
- **THEN** the CLI lists that project in the warehouse view
- **AND** the project line appends `(Unmatched)` with negative emphasis when color output is enabled

#### Scenario: Listing without an active project and showing no active usage
- **WHEN** the user runs `cfgfc list` without an explicit project and no convenience context is set
- **AND** one project has no active mappings and no persisted apply intent
- **THEN** the CLI lists that project in the warehouse view
- **AND** the project line appends `(None)` with negative emphasis when color output is enabled

#### Scenario: Listing with an active project
- **WHEN** the user runs `cfgfc list` and an effective project is resolved
- **THEN** the CLI lists that project's columns and modes
- **AND** each column line appends a `Full`, `Partial`, or `None` usage label
- **AND** the active mode line uses positive emphasis when the persisted mode intent still matches current metadata-derived mappings and color output is enabled

#### Scenario: Listing includes `SettingWarehouse` at the root level
- **WHEN** the user runs `cfgfc list` in global mode and the warehouse root contains a discoverable project directory named `SettingWarehouse`
- **THEN** that project appears in the rendered project list like any other current project directory

### Requirement: List can inspect one column or mode in detail
The `list` workflow SHALL support inspecting one column's settings and one mode's contents in more detail. When listing one column, the CLI SHALL keep showing missing entries and SHALL positively emphasize every setting whose full current metadata-derived managed mapping set is present in the project's persisted managed mappings.

#### Scenario: Listing one column
- **WHEN** the user runs `cfgfc list -p <ProjectName> -c <ColumnName>`
- **THEN** the CLI shows the settings known for that column, including missing entries
- **AND** every currently enabled setting line uses positive emphasis when color output is enabled only when that setting's full current metadata-derived mapping set is present

#### Scenario: Listing one mode
- **WHEN** the user runs `cfgfc list -p <ProjectName> -m <ModeName>`
- **THEN** the CLI shows the mode's declared column selections and strategies

#### Scenario: Listing one column after a partial apply
- **WHEN** the user runs `cfgfc list -p <ProjectName> -c <ColumnName>`
- **AND** only some settings in that column, or only part of one setting's mapping set, currently participate in the persisted managed mappings
- **THEN** the CLI still lists every known setting in the column
- **AND** only the enabled subset uses positive emphasis when color output is enabled

### Requirement: Reset removes the resolved project's current managed mappings
The `reset` workflow SHALL resolve the effective project and clear its current managed mappings through the symlink engine. When `reset` reports success, it SHALL render the resolved project with its display-oriented label. When reset runs with `-f` / `--force`, it SHALL request destructive removal of every target recorded in the current project state even if that target is no longer owned by the recorded source.

#### Scenario: Reset using convenience context
- **WHEN** the user runs `cfgfc reset` without `-p` and a convenience context exists
- **THEN** the CLI resets the managed mappings for the context-selected project

#### Scenario: Resetting after selecting a project through an alias
- **WHEN** the user previously selected a project through an alias and then runs `cfgfc reset`
- **THEN** the CLI resets the resolved project's managed mappings
- **AND** the success message identifies that project by its display-oriented label

#### Scenario: Forced reset ignores ownership drift
- **WHEN** the user runs `cfgfc reset -f -p <ProjectName>`
- **AND** one recorded target path has been replaced on disk by a different file, directory, or symlink
- **THEN** the CLI still commits the reset instead of failing the workflow at command level

### Requirement: Revert restores the last snapshot for the resolved project
The `revert` workflow SHALL resolve the effective project from either explicit `-p <ProjectName>` input or the active switched project context, and SHALL restore the most recent previous mapping snapshot recorded for that project. When `revert` reports success, it SHALL render the resolved project with its display-oriented label. When revert runs with `-f` / `--force`, it SHALL request destructive overwrite semantics so occupied targets and drifted recorded targets do not block restoration of the previous managed snapshot.

#### Scenario: Reverting after a successful apply sequence
- **WHEN** the user runs `cfgfc revert` for a project with a previous snapshot
- **THEN** the CLI restores the mapping set from that snapshot through the symlink engine

#### Scenario: Reverting using convenience context
- **WHEN** the user runs `cfgfc switch <ProjectName>` and then runs `cfgfc revert` without `-p`
- **THEN** the CLI resolves `<ProjectName>` from the active convenience context and restores that project's most recent snapshot

#### Scenario: Explicit project selection overrides switched context for revert
- **WHEN** the user runs `cfgfc switch <ProjectA>` and then runs `cfgfc revert -p <ProjectB>`
- **THEN** the CLI restores the previous snapshot for `<ProjectB>` instead of the switched project context

#### Scenario: Reverting through an aliased project reference
- **WHEN** the user runs `cfgfc revert -p <ProjectAlias>` and that alias resolves uniquely
- **THEN** the CLI restores the previous snapshot for the matching project
- **AND** the success message identifies the project by its display-oriented label instead of the raw alias text

#### Scenario: Forced revert overrides an occupied restore target
- **WHEN** the user runs `cfgfc revert --force -p <ProjectName>`
- **AND** one target in the previous snapshot is currently occupied by an unmanaged file, directory, or mismatched symlink
- **THEN** the CLI still commits the revert instead of failing the workflow at command level

### Requirement: Registered commands expose standardized help sections
Each registered `cfgfc` command SHALL expose a standardized help surface that documents the command purpose, accepted usage forms, supported flags or argument forms, and at least one command-specific example.

#### Scenario: Inspecting an operational command help surface
- **WHEN** a user runs `cfgfc sync --help`
- **THEN** the CLI shows a structured help response that includes the command summary, usage, supported sync forms, and at least one sync example

#### Scenario: Inspecting a project-scoped command help surface
- **WHEN** a user runs `cfgfc apply --help`
- **THEN** the CLI explains the accepted apply forms, the role of `-p` versus switched-project context, representative apply examples, and the destructive semantics of `-f` / `--force`

#### Scenario: Inspecting update help surface
- **WHEN** a user runs `cfgfc update --help`
- **THEN** the CLI explains that update refreshes the last applied intent when intent metadata is available, falls back to mapping refresh for legacy state, supports `-p` / `--project`, switched-project context, `-a` / `--all`, `-c` / `--column`, `-f` / `--force`, and representative update examples

#### Scenario: Inspecting revert help surface
- **WHEN** a user runs `cfgfc revert --help`
- **THEN** the CLI documents that `-f` / `--force` restores only the last confirmed managed snapshot and does not recover overwritten unmanaged contents

#### Scenario: Inspecting root help surface
- **WHEN** a user runs `cfgfc root --help`
- **THEN** the CLI explains the read-current-root form, the single-path set form, and that changing the root does not migrate existing warehouse contents

### Requirement: Root help explains command discovery and project-context behavior
The root `cfgfc --help` surface SHALL summarize the registered command families and explain the project-context rules and warehouse-root selection behavior that materially affect later command behavior.

#### Scenario: Inspecting the root help overview
- **WHEN** a user runs `cfgfc --help`
- **THEN** the CLI lists the registered command families and explains the switched-project, `sync --all`, `update --all`, and `root <path>` behaviors that affect command resolution

### Requirement: Root command manages persistent warehouse root selection
The `root` workflow SHALL report the current effective warehouse root when invoked without a path argument and SHALL persist a new effective warehouse root when invoked with exactly one path argument. Changing the configured root SHALL affect later command resolution for the same user and SHALL NOT migrate or copy any warehouse content from the previously effective root.

#### Scenario: Inspecting the current effective warehouse root
- **WHEN** a user runs `cfgfc root`
- **THEN** the CLI prints the effective warehouse root that later commands will use

#### Scenario: Persisting a new effective warehouse root
- **WHEN** a user runs `cfgfc root <WarehousePath>`
- **THEN** the CLI stores the normalized path as the effective warehouse root for later invocations
- **AND** a later warehouse command resolves projects from `<WarehousePath>` instead of the previous root

#### Scenario: Changing roots does not migrate warehouse contents
- **WHEN** a user runs `cfgfc root <WarehousePath>` and the previous effective root already contains warehouse data
- **THEN** the CLI leaves the previous root untouched
- **AND** it does not copy or move those files into `<WarehousePath>`

### Requirement: Update refreshes current managed configuration state
The `update` workflow SHALL refresh the currently active managed configuration for the resolved project from the latest warehouse source files and indexes. When current state records the previous apply intent, update SHALL re-plan from that intent so mode strategies such as `full` are evaluated against current metadata; when current state lacks intent metadata, update SHALL fall back to refreshing the recorded current mappings without requiring the current mappings to have originated from a mode apply. When update runs with `-f` / `--force`, it SHALL pass destructive overwrite semantics into the symlink engine so unmanaged targets and drifted recorded targets do not block the refresh.

#### Scenario: Updating a full mode column after adding a source artifact
- **WHEN** a project has current state from `cfgfc apply -p <ProjectName> -m <ModeName>` and that mode declares a column with `strategy: full`
- **AND** the user adds a new setting under that column and reconciles warehouse metadata
- **THEN** `cfgfc update -p <ProjectName>` re-plans the mode from current metadata
- **AND** the newly added setting is included in the refreshed managed targets

#### Scenario: Updating state created by column apply
- **WHEN** the current managed state was created by applying one or more column settings directly rather than by applying a mode
- **THEN** `cfgfc update -p <ProjectName>` refreshes that direct-column intent successfully instead of requiring a mode selection

#### Scenario: Updating legacy mapping-only state
- **WHEN** a project has active current mappings from a state file that does not contain apply-intent metadata
- **THEN** `cfgfc update -p <ProjectName>` refreshes the recorded mappings using the backward-compatible mapping-based behavior

#### Scenario: Forced update overrides a drifted current target
- **WHEN** the user runs `cfgfc update --force -p <ProjectName>`
- **AND** one current recorded target path no longer points to the recorded source
- **THEN** the CLI still commits the refreshed managed state instead of failing the workflow at command level

### Requirement: Mutating commands accept a destructive force override
The `apply`, `update`, `revert`, and `reset` workflows SHALL accept `-f` and `--force` as aliases for the same destructive override flag.

#### Scenario: Applying with the short force flag
- **WHEN** the user runs `cfgfc apply ... -f`
- **THEN** the CLI parses the command as a forced apply request

#### Scenario: Updating with the long force flag
- **WHEN** the user runs `cfgfc update ... --force`
- **THEN** the CLI parses the command as a forced update request

#### Scenario: Resetting with the long force flag
- **WHEN** the user runs `cfgfc reset --force`
- **THEN** the CLI parses the command as a forced reset request

#### Scenario: Reverting with the short force flag
- **WHEN** the user runs `cfgfc revert -f`
- **THEN** the CLI parses the command as a forced revert request

### Requirement: Update resolves project scope like other project commands
The `update` workflow SHALL resolve the effective project from explicit `--project <ProjectName>` / `-p <ProjectName>` input or the active switched project context, SHALL let explicit project input override switched context, and SHALL treat `global` as a reserved project target name.

#### Scenario: Updating through switched context
- **WHEN** the user runs `cfgfc switch <ProjectName>` and then runs `cfgfc update` without `-p`
- **THEN** the CLI resolves `<ProjectName>` from the active convenience context and refreshes that project's current managed mappings

#### Scenario: Explicit project selection overrides switched context for update
- **WHEN** the user runs `cfgfc switch <ProjectA>` and then runs `cfgfc update -p <ProjectB>`
- **THEN** the CLI refreshes `<ProjectB>` instead of the switched project context

#### Scenario: Updating through an aliased project reference
- **WHEN** the user runs `cfgfc update -p <ProjectAlias>` and that alias resolves uniquely
- **THEN** the CLI refreshes the matching project successfully
- **AND** the success message identifies the project by its display-oriented label instead of the raw alias text

#### Scenario: Handling the reserved global project target for update
- **WHEN** the user runs `cfgfc update -p global`
- **THEN** the CLI fails with an error that explains `global` is reserved

### Requirement: Update supports all-project refresh
The `update` workflow SHALL accept `--all` and `-a` as explicit warehouse-wide refresh flags and SHALL ignore switched-project context when either all-project flag is present.

#### Scenario: Updating all projects explicitly
- **WHEN** the user runs `cfgfc update --all`
- **THEN** the CLI attempts to refresh every project with active current managed state in the warehouse

#### Scenario: Aliased all-project update is supported
- **WHEN** the user runs `cfgfc update -a`
- **THEN** the CLI performs a warehouse-wide refresh

#### Scenario: Explicit all-project update ignores switched context
- **WHEN** the user runs `cfgfc switch <ProjectName>` and then runs `cfgfc update --all`
- **THEN** the CLI performs a warehouse-wide refresh instead of refreshing only the switched project

### Requirement: Update supports column-scoped refresh
The `update` workflow SHALL accept `--column <ColumnName>` / `-c <ColumnName>` to refresh only the selected column within the resolved project. When current state records mode apply intent, column-scoped update SHALL re-plan that column from the mode's column strategy; when current state records direct-column intent or legacy mapping-only state, column-scoped update SHALL refresh the selected column's active mappings and preserve active mappings from other columns.

#### Scenario: Updating one active full-mode column
- **WHEN** current state was created by a mode apply whose selected column uses `strategy: full`
- **AND** the user adds a new setting under that column and reconciles warehouse metadata
- **THEN** `cfgfc update -p <ProjectName> -c <ColumnName>` refreshes only that column from the mode strategy
- **AND** the newly added setting in that column is linked
- **AND** active managed mappings from other columns remain unchanged

#### Scenario: Updating one column through an alias
- **WHEN** the user runs `cfgfc update -p <ProjectName> -c <ColumnAlias>` and that column alias resolves uniquely
- **THEN** the CLI refreshes the matching column successfully
- **AND** the success message identifies the resolved column by its display-oriented label

#### Scenario: Column update requires project scope
- **WHEN** the user runs `cfgfc update -c <ColumnName>` without `-p` and without an active switched project context
- **THEN** the CLI fails with an error explaining that a project is required for column-scoped update

### Requirement: Update rejects conflicting scope flags
The `update` workflow SHALL reject invocations that combine warehouse-wide update with project-only or column-only scope in the same command.

#### Scenario: Rejecting all with project selection
- **WHEN** the user runs `cfgfc update --all -p <ProjectName>`
- **THEN** the CLI fails with an error explaining that `--all` cannot be combined with `--project`

#### Scenario: Rejecting all with column selection
- **WHEN** the user runs `cfgfc update --all -c <ColumnName>`
- **THEN** the CLI fails with an error explaining that `--all` cannot be combined with `--column`

### Requirement: Commands resolve entity references through normalized identifiers and aliases
The CLI SHALL allow project-, column-, mode-, and setting-scoped command references to resolve through the canonical persisted identifier defined by the index key and declared aliases, and SHALL NOT require users to provide a separate persisted `warehouseName` field value when invoking commands.

#### Scenario: Switching projects through an alias
- **WHEN** the user runs `cfgfc switch <ProjectAlias>` and that alias resolves uniquely within the warehouse
- **THEN** the CLI switches into the matching project successfully

#### Scenario: Applying a setting through normalized references
- **WHEN** the user runs `cfgfc apply -p <ProjectName> -c <ColumnAlias> -s <SettingName>`
- **THEN** the CLI resolves those references to the matching project, column, and setting before planning mappings

#### Scenario: Listing a mode through an alias
- **WHEN** the user runs `cfgfc list -p <ProjectName> -m <ModeAlias>`
- **THEN** the CLI renders the matching mode instead of requiring a legacy storage field value

#### Scenario: Rejecting an ambiguous alias
- **WHEN** a command reference matches more than one entity within the same scope by alias or identifier
- **THEN** the CLI fails with an explicit ambiguity error instead of picking one implicitly

### Requirement: User-facing workflow documentation composes the existing command families into one realistic scenario
The published user-facing documentation SHALL explain at least one realistic workflow that composes the implemented `cfgfc` command families in execution order, including which steps rely on CLI scaffolding and which steps require manual JSONC editing before later commands are run.

#### Scenario: Following the documented setup flow
- **WHEN** a user follows the example workflow from project creation through first apply
- **THEN** the guide presents `new`, manual index editing, `sync`, `switch`, `list`, and `apply` in a sequence that matches the implemented CLI behavior

#### Scenario: Understanding command context within the workflow
- **WHEN** a user reads the documented workflow steps that omit `-p`
- **THEN** the guide explains that those steps depend on a prior `cfgfc switch <ProjectName>` convenience context instead of implying that `-p` is never needed

#### Scenario: Learning recovery commands from the workflow
- **WHEN** a user reaches the end of the example workflow
- **THEN** the guide explains when `reset` and `revert` are useful for undoing or restoring managed mappings in the same scenario

### Requirement: Help and workflow guidance use canonical names instead of warehouseName terminology
The CLI help surface and user-facing workflow guidance SHALL describe entity references in terms of canonical names and aliases.

#### Scenario: Inspecting apply help after the contract change
- **WHEN** a user runs `cfgfc apply --help`
- **THEN** the help text explains that project, column, mode, and setting references use canonical names or aliases

#### Scenario: Following an example workflow after the contract change
- **WHEN** a user reads the documented command sequence from scaffold through apply
- **THEN** the workflow describes entity references using top-level index keys and aliases
