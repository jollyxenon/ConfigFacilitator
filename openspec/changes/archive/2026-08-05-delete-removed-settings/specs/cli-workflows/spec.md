## MODIFIED Requirements

### Requirement: Sync reconciles filesystem reality with index metadata
The `sync` workflow SHALL scan the warehouse, add newly discovered entities into index data, and remove previously indexed Settings whose source file or directory is no longer present. When `sync` accepts `-p <ProjectName>`, omitting `-p` SHALL cause the CLI to sync the active switched project when one is available, and SHALL fall back to syncing every project only when no effective project can be resolved. The `sync` workflow SHALL accept `--all` and `-a` as explicit warehouse-wide sync flags, and SHALL treat `global` as a reserved project target name. When syncing one resolved project, the success message SHALL report the resolved project with its display-oriented label instead of the raw alias text.

#### Scenario: Sync discovers a new setting file
- **WHEN** the user adds a new filesystem setting and runs sync
- **THEN** the corresponding index metadata is updated to include the discovered entity

#### Scenario: Sync removes a disappeared setting
- **WHEN** a user removes a previously indexed Setting source and runs `cfgfc sync`
- **THEN** the Setting is removed from the column's `SettingIndex.jsonc`
- **AND** `cfgfc list -c <ColumnName>` no longer reports that Setting after synchronization

#### Scenario: Sync rewrites a project's indexes
- **WHEN** the user runs `cfgfc sync -p <ProjectName>`
- **THEN** the project's warehouse indexes are rewritten from the reconciled model while retaining required descriptions for entities that remain present
