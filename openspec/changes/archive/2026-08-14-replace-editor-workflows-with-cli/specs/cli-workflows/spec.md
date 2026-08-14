## MODIFIED Requirements

### Requirement: CLI exposes the required command families
The system SHALL expose `project`, `column`, `setting`, `mode`, `use`, `status`, `apply`, `refresh`, `sync`, `reset`, `revert`, and `root` through the `cfgfc` CLI. The previous `new`, `switch`, `list`, and `update` command families SHALL NOT remain supported aliases.

#### Scenario: Inspecting available commands
- **WHEN** a user inspects the CLI help surface
- **THEN** the required resource-oriented command families are present as part of the supported interface
- **AND** the removed command families are absent

### Requirement: Sync reconciles filesystem reality with index metadata
The `sync` workflow SHALL scan the effective warehouse scope and add newly discovered Projects, Columns, and Settings to index data. When an indexed Project directory, Column directory, or Setting file/directory has disappeared, `sync` SHALL immediately remove that resource metadata from its corresponding index and SHALL NOT recreate the source. Sync SHALL NOT implicitly cascade or rewrite Mode selections, current/history runtime records, or PPID context; later `apply` or `refresh` SHALL fail if such references no longer resolve. `--prune` and `--prune --yes` are unsupported and SHALL fail without index changes. `sync` SHALL use the selected PPID Project by default when available, SHALL use the whole warehouse when no Project is selected, and SHALL accept `-p` / `--project` and `--all` as explicit mutually exclusive scopes. `global` SHALL remain reserved.

#### Scenario: Sync discovers a new setting file
- **WHEN** a user adds a new filesystem Setting and runs `sync`
- **THEN** the corresponding index metadata is updated to include the discovered resource

#### Scenario: Sync removes disappeared filesystem resources
- **WHEN** an indexed Project directory, Column directory, or Setting file/directory disappears and the user runs `cfgfc sync`
- **THEN** the corresponding Project, Column, or Setting metadata is removed from its index immediately
- **AND** the disappeared source path and child indexes are not recreated
- **AND** Mode/runtime references are not implicitly cascaded

#### Scenario: Sync removes disappeared setting
- **WHEN** a previously indexed Setting source disappears and the user runs `cfgfc sync`
- **THEN** its metadata is removed from `SettingIndex.jsonc`
- **AND** `cfgfc setting show` and later apply/refresh report an unresolved reference when applicable

#### Scenario: Prune flags are unsupported
- **WHEN** a user runs `cfgfc sync --prune` or `cfgfc sync --prune --yes`
- **THEN** the command exits with a usage error
- **AND** no index is changed

#### Scenario: Sync uses selected project context
- **WHEN** `cfgfc use OpenCode` has selected a Project and the user runs `cfgfc sync`
- **THEN** only `OpenCode` is synchronized

#### Scenario: Sync falls back to all projects
- **WHEN** no effective Project is selected and the user runs `cfgfc sync`
- **THEN** every discoverable Project in the warehouse is synchronized

#### Scenario: Explicit all-project sync ignores context
- **WHEN** a Project context is selected and the user runs `cfgfc sync --all`
- **THEN** the entire warehouse is synchronized

#### Scenario: Rejecting conflicting sync scopes
- **WHEN** a user combines `--all` with `-p` or `--project`
- **THEN** the command exits with a usage error without synchronizing

#### Scenario: Sync removes a disappeared column
- **WHEN** a previously indexed Column directory disappears and the user runs `cfgfc sync`
- **THEN** the Column is removed from the Project index immediately
- **AND** Mode/runtime references to it are not implicitly cascaded

#### Scenario: Sync removes a disappeared setting
- **WHEN** a previously indexed Setting source disappears and the user runs `cfgfc sync`
- **THEN** the Setting is removed from the Column index immediately
- **AND** later apply or refresh fails if retained references cannot resolve it

#### Scenario: Sync rewrites a project's indexes
- **WHEN** the user runs `cfgfc sync -p <Project>`
- **THEN** the resolved Project indexes are normalized while preserving metadata for present resources and removing disappeared filesystem-backed resources

#### Scenario: Syncing a project through an alias
- **WHEN** the user runs `cfgfc sync -p <ProjectAlias>` and the alias resolves uniquely
- **THEN** the canonical Project is synchronized

#### Scenario: Sync rewrites the switched project's indexes
- **WHEN** the user selects a Project with `cfgfc use` and runs `cfgfc sync` without explicit scope
- **THEN** only the selected Project indexes are synchronized

#### Scenario: Sync rewrites all project indexes when no project is resolved
- **WHEN** no Project context exists and the user runs `cfgfc sync` without explicit scope
- **THEN** every discoverable Project is synchronized

#### Scenario: Sync includes `SettingWarehouse` at the root level
- **WHEN** warehouse-wide sync sees a root-level Project directory named `SettingWarehouse`
- **THEN** it treats that directory like any other Project

#### Scenario: Explicit project selection overrides switched context for sync
- **WHEN** one Project is selected and the user runs `cfgfc sync -p <OtherProject>`
- **THEN** only the explicitly selected Project is synchronized

#### Scenario: Explicit all-project sync ignores switched context
- **WHEN** one Project is selected and the user runs `cfgfc sync --all`
- **THEN** every Project is synchronized

#### Scenario: Aliased all-project sync is supported
- **WHEN** a user requests warehouse-wide synchronization
- **THEN** the supported explicit form is `cfgfc sync --all`
- **AND** the removed `-a` shorthand is rejected as invalid usage

#### Scenario: Handling the reserved global project target for sync
- **WHEN** the user runs `cfgfc sync -p global`
- **THEN** the command fails with a reserved-name error without synchronizing

### Requirement: Single-column apply accepts explicit settings input
The CLI SHALL accept one or more Setting arguments through `cfgfc apply column <Column> <Setting>...`, resolve the effective Project from explicit `-p` / `--project` input or selected PPID context, and persist canonical Project, Column, and Setting identities as a direct-Column apply intent. Successful file-backed applies SHALL produce managed links whose readable contents match the selected sources. `--force-targets` SHALL request destructive target reclamation when unmanaged or drifted recorded targets would otherwise block apply.

#### Scenario: Applying multiple settings to one column
- **WHEN** the user runs `cfgfc apply column Skills Skill-A Skill-B -p OpenCode`
- **THEN** the request is resolved as one direct-Column apply intent containing both canonical Settings

#### Scenario: Single-column apply resolves targets
- **WHEN** the user applies one Column with one or more named Settings
- **THEN** the CLI resolves each Setting's explicit target or inherited default target before sending the mapping set to the engine

#### Scenario: Applying a file-backed setting creates a readable managed link
- **WHEN** the user applies a Column Setting whose source is a regular file
- **THEN** the resulting managed target path reads back the same contents as the selected source file

#### Scenario: Forced single-column apply overrides an occupied target
- **WHEN** `cfgfc apply column` is invoked with `--force-targets`
- **AND** one planned target is unmanaged or drifted
- **THEN** the recorded target path is reclaimed and the requested apply commits

#### Scenario: Single-column apply uses switched project context
- **WHEN** a Project has been selected with `cfgfc use` and the user runs `cfgfc apply column <Column> <Setting>` without explicit Project scope
- **THEN** the selected canonical Project is used

#### Scenario: Explicit project selection overrides switched context for single-column apply
- **WHEN** one Project is selected and `cfgfc apply column` supplies a different explicit Project
- **THEN** the explicitly selected Project is applied

#### Scenario: Applying one column through an alias
- **WHEN** Column and Setting aliases resolve uniquely in `cfgfc apply column`
- **THEN** their canonical resources are planned and persisted

### Requirement: Mode apply preserves readable contents for file-backed links
The CLI SHALL apply a Mode through `cfgfc apply mode <Mode>`, persist its canonical identity as apply intent, and leave file-backed managed links readable with source-matching contents after success. `--force-targets` SHALL request destructive target reclamation when unmanaged or drifted recorded targets would otherwise block apply.

#### Scenario: Applying a mode with a file-backed setting
- **WHEN** the user runs `cfgfc apply mode Max -p OpenCode` and the Mode selects a regular-file Setting
- **THEN** reading the managed target returns the same file contents as the selected source

#### Scenario: Applying a mode through an alias
- **WHEN** a Mode alias resolves uniquely and the user invokes `apply mode` through that alias
- **THEN** the canonical Mode is applied and persisted in current intent

#### Scenario: Forced mode apply reclaims an occupied directory target
- **WHEN** mode apply uses `--force-targets` and a planned target is occupied by an unmanaged directory
- **THEN** the directory is reclaimed and the Mode apply commits

### Requirement: Reset removes the resolved project's current managed mappings
The `reset` workflow SHALL resolve one effective Project and clear its current managed mappings through the symlink engine. `--force-targets` SHALL allow removal of every target recorded in current Project state even when ownership has drifted. Reset SHALL preserve warehouse resources and indexes.

#### Scenario: Reset using selected context
- **WHEN** a Project has been selected with `cfgfc use` and the user runs `cfgfc reset`
- **THEN** the selected Project's mappings and current intent are cleared

#### Scenario: Forced reset ignores ownership drift
- **WHEN** the user runs `cfgfc reset --force-targets -p OpenCode`
- **AND** one recorded target has been replaced by unmanaged content
- **THEN** the target is reclaimed and reset commits

#### Scenario: Reset using convenience context
- **WHEN** the user runs `cfgfc reset` without explicit Project scope and a selected Project exists
- **THEN** that selected Project is reset

#### Scenario: Resetting after selecting a project through an alias
- **WHEN** a Project was selected through an alias and the user runs `cfgfc reset`
- **THEN** reset acts on the stored canonical Project

### Requirement: Revert restores the last snapshot for the resolved project
The `revert` workflow SHALL resolve one effective Project and restore its most recent previous mapping and intent snapshot. `--force-targets` SHALL allow occupied or drifted recorded targets to be reclaimed during restoration.

#### Scenario: Reverting after a successful apply sequence
- **WHEN** the user runs `cfgfc revert` for a Project with a previous snapshot
- **THEN** the mapping set and apply intent from that snapshot are restored

#### Scenario: Reverting using selected context
- **WHEN** a Project has been selected with `cfgfc use` and the user runs `cfgfc revert`
- **THEN** that Project's most recent snapshot is restored

#### Scenario: Forced revert overrides an occupied restore target
- **WHEN** the user runs `cfgfc revert --force-targets -p OpenCode`
- **AND** a restore target is occupied or drifted
- **THEN** the target is reclaimed and restoration commits

#### Scenario: Reverting using convenience context
- **WHEN** the user runs `cfgfc revert` without explicit Project scope and a selected Project exists
- **THEN** that selected Project is reverted

#### Scenario: Explicit project selection overrides switched context for revert
- **WHEN** one Project is selected and `cfgfc revert` supplies a different explicit Project
- **THEN** the explicitly selected Project is reverted

#### Scenario: Reverting through an aliased project reference
- **WHEN** the explicit Project reference is a unique alias
- **THEN** the canonical Project's previous snapshot is restored

### Requirement: Registered commands expose standardized help sections
Every registered command and runnable nested subcommand SHALL provide structured help containing purpose, usage, arguments, flags, destructive behavior where applicable, and at least one example. Root help SHALL explain project context, `--json`, destructive confirmations, and target reclamation.

#### Scenario: Inspecting resource mutation help
- **WHEN** a user runs `cfgfc setting create --help`
- **THEN** help explains Setting kind, content-source exclusivity, project and Column scope, and examples

#### Scenario: Inspecting destructive deletion help
- **WHEN** a user runs `cfgfc column delete --help`
- **THEN** help explains `--yes`, `--cascade`, and `--force-targets` as distinct controls

#### Scenario: Inspecting root help
- **WHEN** a user runs `cfgfc --help`
- **THEN** help describes command discovery, selected Project context, JSON output, and destructive-operation conventions

#### Scenario: Inspecting an operational command help surface
- **WHEN** a user runs `cfgfc sync --help`
- **THEN** help documents immediate index removal for disappeared resources, unresolved-reference behavior, explicit scope, unsupported prune flags, and examples

#### Scenario: Inspecting a project-scoped command help surface
- **WHEN** a user runs `cfgfc apply --help`
- **THEN** help presents the `mode` and `column` subcommands, Project resolution, and `--force-targets`

#### Scenario: Inspecting update help surface
- **WHEN** a user runs `cfgfc refresh --help`
- **THEN** help documents intent-aware, mapping-only, Column-scoped, and all-Project refresh

#### Scenario: Inspecting revert help surface
- **WHEN** a user runs `cfgfc revert --help`
- **THEN** help explains one-step restoration and `--force-targets` limitations

#### Scenario: Inspecting root help surface
- **WHEN** a user runs `cfgfc root --help`
- **THEN** help documents root inspection, root persistence, and the absence of automatic migration

### Requirement: Update refreshes current managed configuration state
The `refresh` workflow SHALL replace the former `update` workflow. It SHALL re-plan current managed configuration from persisted apply intent and current warehouse metadata. When state lacks intent, it SHALL refresh legacy mapping-only state by matching current mappings to current metadata. It SHALL accept `--column <Column>` for one-Column refresh and `--all` for every Project with current state, with `--all` incompatible with Project and Column scope. `--force-targets` SHALL allow unmanaged or drifted recorded targets to be reclaimed.

#### Scenario: Refreshing a full Mode after adding a source
- **WHEN** a current Mode intent uses `full`, a new Setting is added, and the user runs `cfgfc refresh`
- **THEN** the Mode is re-planned from current metadata
- **AND** the new Setting participates in managed mappings

#### Scenario: Refreshing direct-column intent
- **WHEN** current state was produced by `apply column`
- **THEN** `cfgfc refresh` refreshes that direct-Column intent without requiring a Mode

#### Scenario: Refreshing legacy mapping-only state
- **WHEN** current state has mappings but no apply intent
- **THEN** `refresh` matches and refreshes those mappings from current metadata

#### Scenario: Refreshing one column
- **WHEN** the user runs `cfgfc refresh --column Skills -p OpenCode`
- **THEN** only `Skills` is re-planned
- **AND** other current mappings are preserved

#### Scenario: Refreshing all projects
- **WHEN** the user runs `cfgfc refresh --all`
- **THEN** every Project with active mappings or intent is refreshed

#### Scenario: Rejecting conflicting refresh scopes
- **WHEN** `--all` is combined with a Project or Column selector
- **THEN** the command exits with a usage error without refreshing

#### Scenario: Updating a full mode column after adding a source artifact
- **WHEN** a full-Mode intent is active, a new Setting is added, and the user runs `cfgfc refresh`
- **THEN** the new Setting is included in refreshed mappings

#### Scenario: Updating state created by column apply
- **WHEN** current state was created by `cfgfc apply column`
- **THEN** `cfgfc refresh` re-plans the direct-Column intent

#### Scenario: Updating legacy mapping-only state
- **WHEN** current state has mappings without intent
- **THEN** `cfgfc refresh` uses mapping-based matching

#### Scenario: Forced update overrides a drifted current target
- **WHEN** `cfgfc refresh --force-targets` encounters a drifted recorded target
- **THEN** the target is reclaimed and refresh commits

### Requirement: Mutating commands accept a destructive target override
The `apply`, `refresh`, `revert`, `reset`, and cascade-deletion workflows SHALL accept `--force-targets` as their only target-reclamation flag. The removed `-f` and `--force` flags SHALL fail as invalid usage.

#### Scenario: Applying with target reclamation
- **WHEN** a user invokes apply with `--force-targets`
- **THEN** the CLI requests destructive reclamation only for planned or recorded managed-target paths

#### Scenario: Rejecting the removed short force flag
- **WHEN** a user invokes a mutating command with `-f`
- **THEN** the command exits with a usage error and performs no mutation

### Requirement: Commands resolve entity references through normalized identifiers and aliases
The CLI SHALL resolve Project, Column, Mode, and Setting references through canonical persisted keys or declared aliases before executing any command. Persisted indexes, selections, contexts, and apply intents SHALL store canonical identities rather than the alias used at invocation time. Ambiguous references SHALL fail without mutation.

#### Scenario: Selecting a Project through an alias
- **WHEN** a user runs `cfgfc use <ProjectAlias>` and it resolves uniquely
- **THEN** the canonical Project identity is stored in context

#### Scenario: Mutating a Setting through aliases
- **WHEN** Project, Column, or Setting arguments use unique aliases
- **THEN** the canonical Setting is mutated
- **AND** canonical keys remain the persisted references

#### Scenario: Rejecting an ambiguous alias
- **WHEN** a reference matches more than one entity in its scope
- **THEN** the command exits with a conflict error instead of choosing one

#### Scenario: Switching projects through an alias
- **WHEN** a user selects a Project through a unique alias with `cfgfc use`
- **THEN** the canonical Project is selected

#### Scenario: Applying a setting through normalized references
- **WHEN** `cfgfc apply column` receives canonical names or unique aliases
- **THEN** all references are canonicalized before planning

#### Scenario: Listing a mode through an alias
- **WHEN** a user runs `cfgfc mode show <ModeAlias>` and the alias resolves uniquely
- **THEN** the canonical Mode is rendered

### Requirement: User-facing workflow documentation composes the CLI-only lifecycle
The published English and Chinese documentation SHALL present at least one realistic workflow that creates Project, Column, targets, file-backed and directory-backed Settings, Setting content, and Mode selections; applies configuration; changes content and metadata; refreshes links when needed; renames resources; deletes resources; and synchronizes external changes using only `cfgfc` plus shell input redirection. It SHALL not require direct index or warehouse source editing.

#### Scenario: Following the documented setup flow
- **WHEN** a user follows the primary workflow from an empty warehouse
- **THEN** every warehouse mutation is performed by a documented `cfgfc` command
- **AND** the resulting Mode can be applied successfully

#### Scenario: Learning when refresh is needed
- **WHEN** the workflow modifies Setting bytes without changing source or target paths
- **THEN** it explains that managed links expose the content immediately
- **AND** it reserves `refresh` for metadata or intent replanning

#### Scenario: Learning safe external synchronization
- **WHEN** the workflow describes changes made by Git or another external tool
- **THEN** it explains immediate Index removal, the absence of implicit Mode/runtime cascade, unresolved apply/refresh failure, and unsupported prune flags

### Requirement: Root help explains command discovery and project-context behavior
The root `cfgfc --help` surface SHALL summarize the resource-oriented command families and explain selected Project context, `sync --all`, `refresh --all`, persistent root selection, JSON output, confirmations, cascade, and target reclamation.

#### Scenario: Inspecting the root help overview
- **WHEN** a user runs `cfgfc --help`
- **THEN** help explains `use`, context-aware Project scope, `sync --all`, `refresh --all`, `root <Path>`, and the three independent destructive controls

## REMOVED Requirements

### Requirement: New commands scaffold editable templates
**Reason**: Resource `create`, `set`, target, selection, and content commands replace editor-oriented scaffolds.
**Migration**: Use `project create`, `column create`, `setting create`, and `mode create`, then configure them with dedicated subcommands.

### Requirement: Switch stores the active project context
**Reason**: The public context command is now `use`.
**Migration**: Replace `cfgfc switch <Project|global>` with `cfgfc use <Project|global>`.

### Requirement: List shows available projects and project contents
**Reason**: Resource inspection and runtime state are separated.
**Migration**: Use resource `list`/`show` commands for metadata and `cfgfc status` for usage state.

### Requirement: List can inspect one column or mode in detail
**Reason**: Detail inspection belongs to resource-specific commands.
**Migration**: Use `cfgfc column show`, `cfgfc setting list`, or `cfgfc mode show`.

### Requirement: Update resolves project scope like other project commands
**Reason**: The `update` command is removed and `refresh` uses the shared Project-scope contract.
**Migration**: Use `cfgfc refresh -p <Project>` or selected context.

### Requirement: Update supports all-project refresh
**Reason**: The `update` command is removed.
**Migration**: Use `cfgfc refresh --all`.

### Requirement: Update supports column-scoped refresh
**Reason**: The `update` command is removed.
**Migration**: Use `cfgfc refresh --column <Column>`.

### Requirement: Update rejects conflicting scope flags
**Reason**: The `update` command is removed; equivalent validation is part of `refresh`.
**Migration**: Do not combine `refresh --all` with Project or Column scope.
