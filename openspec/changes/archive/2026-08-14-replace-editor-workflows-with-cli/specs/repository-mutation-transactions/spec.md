## Purpose

Defines recoverable, all-or-nothing repository mutations so resource renames and deletions keep indexes, references, managed links, runtime state, history, and session context coherent.

## ADDED Requirements

### Requirement: Repository mutations validate before changing durable state
Every create, set, target, Mode-selection, rename, delete, and content mutation SHALL resolve references and validate names, aliases, paths, target arrays, Mode selections, dependencies, confirmations, and target ownership before committing its first durable change. A validation failure SHALL leave repository data, runtime state, session context, and managed targets unchanged.

#### Scenario: Rename destination already exists
- **WHEN** a user requests a resource rename whose destination canonical name already exists in that scope
- **THEN** the command fails before moving a source path or writing an index

#### Scenario: Mutation would create an alias collision
- **WHEN** a metadata mutation would make any canonical name or alias ambiguous in its resolution scope
- **THEN** the command fails without changing the previous metadata

#### Scenario: Mode selection references an unknown setting
- **WHEN** a Mode-selection mutation names a Setting that cannot be resolved in the selected Column
- **THEN** the Mode index remains unchanged

### Requirement: Multi-artifact mutations are recoverable transactions
A mutation that affects more than one repository file, repository path, session record, or managed target SHALL create a durable transaction record before committing changes. On ordinary failure the CLI SHALL restore the exact pre-mutation repository files, paths, runtime state, session context, and managed-target ownership before returning an error. If a process or machine stops mid-transaction, the next mutating `cfgfc` invocation SHALL recover the incomplete transaction to its recorded pre-mutation state before processing a new request. Temporary and transaction artifacts SHALL remain inside the effective warehouse root and SHALL not be discovered as Projects or Settings.

#### Scenario: Second index write fails
- **WHEN** a rename successfully stages its source path but a later required index write fails
- **THEN** the original path and every original index are restored
- **AND** no renamed resource is visible after the command returns

#### Scenario: Managed-link replacement fails
- **WHEN** a rename moves repository data but cannot recreate one required managed link
- **THEN** the repository path, indexes, state, history, and already changed links are restored to their pre-rename state

#### Scenario: Recovering an interrupted transaction
- **WHEN** a previous process stopped after writing a transaction record but before recording commit completion
- **THEN** the next mutating command restores the recorded pre-mutation state first
- **AND** it does not begin the requested mutation until recovery succeeds

### Requirement: Canonical rename updates every live and historical reference
`project rename`, `column rename`, `setting rename`, and `mode rename` SHALL change the canonical key and corresponding repository path where one exists. The transaction SHALL rewrite all affected Mode Column and Setting references, current apply intent references, current mapping source paths, history mapping source paths, history intent references, and PPID-scoped Project contexts to the new canonical identity or path. Existing aliases, display names, descriptions, target metadata, unknown index fields, and Setting contents SHALL be preserved unless the command explicitly replaces them.

#### Scenario: Renaming a setting used by a mode
- **WHEN** a user renames `GPT.json` to `Primary.json` and one or more Modes select `GPT.json`
- **THEN** the source is present at the new canonical path only
- **AND** every affected Mode selects `Primary.json`
- **AND** the Setting metadata and contents are preserved

#### Scenario: Renaming a column used by modes
- **WHEN** a user renames a Column selected by multiple Modes
- **THEN** every affected Mode uses the new canonical Column key
- **AND** Setting references inside those selections remain valid

#### Scenario: Renaming the selected project
- **WHEN** a Project is renamed while one or more PPID contexts store its canonical identity
- **THEN** those contexts store the new canonical Project identity
- **AND** later context-scoped commands resolve the renamed Project

#### Scenario: Renaming a mode referenced by current intent
- **WHEN** the active apply intent references a Mode that is renamed
- **THEN** current and historical Mode intents use the new canonical name
- **AND** the current managed mappings remain active

### Requirement: Rename keeps managed targets valid
When a Project, Column, or Setting rename changes a source path used by current mappings, the transaction SHALL replace every owned managed link so it points to the new source path and SHALL update the corresponding current and historical mapping records. If any recorded target is absent, unmanaged, or no longer points to its recorded source, rename SHALL fail without mutation unless `--force-targets` is supplied. With `--force-targets`, the transaction SHALL reclaim the affected target and install the renamed mapping.

#### Scenario: Renaming an actively linked setting
- **WHEN** a Setting has owned managed targets and the user renames it
- **THEN** every target remains readable after commit
- **AND** each target points to the renamed source path

#### Scenario: Rename encounters drifted target ownership
- **WHEN** a recorded target no longer points to the old source and `--force-targets` is absent
- **THEN** rename fails and the source keeps its old canonical name and path

#### Scenario: Forced rename reclaims a drifted target
- **WHEN** the same rename is invoked with `--force-targets`
- **THEN** the occupied target is removed recursively as needed
- **AND** the committed target points to the renamed source

### Requirement: Resource deletion requires explicit confirmation
`project delete`, `column delete`, `setting delete`, `mode delete`, Column target deletion, and Setting-content deletion SHALL require `--yes`. An omitted confirmation SHALL produce the confirmation error class without deleting repository content, metadata, references, runtime state, or targets. The CLI SHALL not prompt interactively.

#### Scenario: Deletion lacks confirmation
- **WHEN** a user invokes a destructive delete command without `--yes`
- **THEN** the command fails without mutation

#### Scenario: Confirmed independent deletion
- **WHEN** a user supplies `--yes` for a resource with no dependent references or active mappings
- **THEN** the resource data and its direct index entry are removed atomically

### Requirement: Dependent deletion requires explicit cascade
A confirmed Column or Setting deletion SHALL fail when a Mode, current apply intent, current mapping, or history entry depends on that resource unless `--cascade` is also supplied. A confirmed Mode deletion SHALL fail while current or historical apply intent refers to that Mode unless `--cascade` is supplied. A confirmed Project deletion SHALL fail while it has current mappings unless `--cascade` is supplied. The failure SHALL identify each dependency category without mutation.

#### Scenario: Setting is selected by a mode
- **WHEN** a user confirms deletion of a Setting selected by at least one Mode without `--cascade`
- **THEN** deletion fails and reports the Mode dependency

#### Scenario: Column has active mappings
- **WHEN** a user confirms deletion of a Column represented in current mappings without `--cascade`
- **THEN** deletion fails and reports the active-state dependency

#### Scenario: Active mode is deleted without cascade
- **WHEN** the current apply intent names the Mode being deleted and `--cascade` is absent
- **THEN** the Mode and current intent remain unchanged

### Requirement: Cascade deletion removes or repairs dependent references
With `--cascade`, Setting deletion SHALL remove that Setting from every Mode selection and shall remove its mappings from current and historical states; any resulting `cover` or `increment` selection with no Settings SHALL be removed from the Mode. Column deletion SHALL remove the Column selection from every Mode and remove all of its Setting mappings and direct-Column intents from current and historical states. Mode deletion SHALL remove Mode intents from current and historical states while preserving mapping records whose source Settings still exist. Project deletion SHALL remove its current mappings, Project directory, Project index entry, and PPID contexts. Cascade SHALL not remove unrelated resources, mappings, or Mode selections.

#### Scenario: Cascading setting deletion
- **WHEN** a user confirms Setting deletion with `--cascade`
- **THEN** the Setting source and metadata are removed
- **AND** Mode selections and runtime records no longer reference it
- **AND** unrelated Settings remain unchanged

#### Scenario: Cascading mode deletion
- **WHEN** a user confirms deletion of the active Mode with `--cascade`
- **THEN** the Mode metadata and current and historical Mode intents are removed
- **AND** current mappings whose Settings still exist remain active as mapping-only state

#### Scenario: Cascading project deletion
- **WHEN** a user confirms Project deletion with `--cascade`
- **THEN** all owned current targets are removed safely
- **AND** the Project directory, index entry, and matching PPID contexts are removed

### Requirement: Cascade respects managed-target ownership
Cascade deletion SHALL remove a recorded managed target only when it is absent or still owned by the recorded source. If a target has drifted or become unmanaged, deletion SHALL fail and rollback unless `--force-targets` is supplied. `--force-targets` SHALL reclaim only target paths recorded by the resources included in the requested cascade and SHALL never remove an unrelated unrecorded path.

#### Scenario: Cascade removes owned targets
- **WHEN** affected target links still point to their recorded sources
- **THEN** cascade removes those links and commits the resource deletion

#### Scenario: Cascade finds a drifted target
- **WHEN** an affected target has been replaced by unmanaged content and `--force-targets` is absent
- **THEN** the complete deletion rolls back
- **AND** the unmanaged content remains present

#### Scenario: Forced cascade removes only recorded targets
- **WHEN** cascade is invoked with `--force-targets`
- **THEN** drifted paths recorded by the affected mappings may be removed recursively
- **AND** no path outside that recorded target set is reclaimed

### Requirement: Creation commits source and metadata together
Project, Column, Mode, and Setting creation SHALL fail without partial artifacts when any required directory, source content, index entry, or runtime-state file cannot be created. A failed operation SHALL not make the resource discoverable by `status`, resource `list`, or `sync`.

#### Scenario: Setting metadata write fails after content staging
- **WHEN** Setting source content is staged but its index update cannot be committed
- **THEN** staged content is removed
- **AND** the Setting is absent from the Column

#### Scenario: Project creation completes
- **WHEN** all required Project artifacts can be written
- **THEN** the Project structure and index entry become visible together
