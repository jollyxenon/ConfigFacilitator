## MODIFIED Requirements

### Requirement: Sync reconciles filesystem reality with index metadata
The `sync` workflow SHALL scan the warehouse, add newly discovered entities into index data, and remove previously indexed Columns whose source directory is no longer present, as well as previously indexed Settings whose source file or directory is no longer present. When `sync` accepts `-p <ProjectName>`, omitting `-p` SHALL cause the CLI to sync the active switched project when one is available, and SHALL fall back to syncing every project only when no effective project can be resolved. The `sync` workflow SHALL accept `--all` and `-a` as explicit warehouse-wide sync flags, and SHALL treat `global` as a reserved project target name. When syncing one resolved project, the success message SHALL report the resolved project with its display-oriented label instead of the raw alias text.

#### Scenario: Sync discovers a new setting file
- **WHEN** the user adds a new filesystem setting and runs sync
- **THEN** the corresponding index metadata is updated to include the discovered entity

#### Scenario: Sync removes a disappeared column
- **WHEN** a user removes a previously indexed Column directory and runs `cfgfc sync`
- **THEN** the Column is removed from the project's `ColumnIndex.jsonc`
- **AND** `cfgfc list -p <ProjectName>` no longer reports that Column after synchronization

#### Scenario: Sync removes a disappeared setting
- **WHEN** a user removes a previously indexed Setting source and runs `cfgfc sync`
- **THEN** the Setting is removed from the column's `SettingIndex.jsonc`
- **AND** `cfgfc list -c <ColumnName>` no longer reports that Setting after synchronization

#### Scenario: Sync rewrites a project's indexes
- **WHEN** the user runs `cfgfc sync -p <ProjectName>`
- **THEN** the project's warehouse indexes are rewritten from the reconciled model while retaining required descriptions for entities that remain present

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
