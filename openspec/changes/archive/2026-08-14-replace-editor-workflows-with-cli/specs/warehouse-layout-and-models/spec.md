## MODIFIED Requirements

### Requirement: Disappeared filesystem-backed entries are removed during sync
The warehouse model SHALL remove indexed Projects, Columns, and Settings whose filesystem-backed source is absent when `sync` runs. Mode and runtime references to removed identities SHALL remain inspectable as unresolved references; sync SHALL not implicitly cascade them. `apply` and `refresh` SHALL be rejected before managed-target changes when required references cannot be resolved. Sync cannot restore metadata after the source path is recreated; users must recreate the resource through its CLI command.

#### Scenario: Loading an unresolved project reference
- **WHEN** `sync` removes a Project whose directory is absent while a Mode or runtime record still names it
- **THEN** the warehouse model exposes that reference as unresolved
- **AND** the absent Project source is not recreated

#### Scenario: Loading an unresolved column reference
- **WHEN** `sync` removes a Column whose directory is absent while a Mode or runtime record still names it
- **THEN** the Project model exposes that reference as unresolved
- **AND** the Column remains absent from its index

#### Scenario: Loading an unresolved setting reference
- **WHEN** `sync` removes a Setting whose source is absent while a Mode or runtime record still names it
- **THEN** the Column model exposes that reference as unresolved
- **AND** its removed metadata is not restored by sync

#### Scenario: Applying an unresolved setting reference
- **WHEN** apply or refresh requires a Setting whose index entry was removed by sync
- **THEN** planning fails with an unresolved-resource error
- **AND** current mappings and targets remain unchanged

#### Scenario: Recreating a removed source externally does not restore metadata
- **WHEN** an external tool recreates a removed resource at its former canonical path and sync runs
- **THEN** the resource is discovered as new metadata rather than restoring the deleted entry

#### Scenario: Reconciling a removed column reference
- **WHEN** synchronization removes an indexed Column whose source directory is absent
- **THEN** retained Mode/runtime relationships expose that Column as unresolved

#### Scenario: Reconciling a removed setting reference
- **WHEN** synchronization removes an indexed Setting whose source is absent
- **THEN** retained Mode/runtime relationships expose that Setting as unresolved

#### Scenario: Loading another unresolved entity reference
- **WHEN** a parsed warehouse contains a Mode or runtime record naming a removed Project, Column, or Setting
- **THEN** the model retains the reference and its metadata in the record for diagnostics
- **AND** apply cannot use the unresolved reference

### Requirement: Column, setting, and mode relationships are preserved
The system SHALL preserve relationships between Columns, Settings, and Mode selections in the in-memory model. Relationships SHALL use canonical identities after resolution. Resource rename SHALL rewrite affected relationships, confirmed cascade deletion SHALL remove affected relationships, and sync removal SHALL keep dangling Mode/runtime references visible for inspection while preventing apply.

#### Scenario: Parsing a project with multiple columns and modes
- **WHEN** a warehouse contains multiple Columns, Settings, and Modes
- **THEN** the resulting model keeps every valid Column-to-Setting and Mode-to-Column relationship intact

#### Scenario: Loading a mode that references a removed setting
- **WHEN** a retained Mode selection refers to a Setting removed from its Index by sync
- **THEN** the model preserves that relationship as unresolved for status and repair
- **AND** applying the Mode fails before changing managed targets

#### Scenario: Renaming a referenced column
- **WHEN** a Column rename commits
- **THEN** every Mode relationship refers to the new canonical Column identity

### Requirement: Backup references remain attached to the project model
The system SHALL attach current-state, history, and repository-transaction references to each present Project model so commands can inspect, migrate, rewrite, recover, and commit runtime state without deriving paths independently. If sync removes a Project entry because its directory disappeared, persisted runtime records remain CLI-owned diagnostic data and are not recreated or rewritten implicitly.

#### Scenario: Inspecting a present project model with runtime state
- **WHEN** a Project model is loaded
- **THEN** it exposes current-state, history, and transaction locations

#### Scenario: Inspecting a project model with backup state
- **WHEN** a Project model is loaded
- **THEN** its current-state and history references remain attached alongside its transaction reference

#### Scenario: Loading a removed project reference
- **WHEN** a Project directory and index entry are absent but a runtime record still refers to it
- **THEN** the reference is exposed as unresolved for diagnostics
- **AND** loading does not recreate any Project or runtime path

### Requirement: Warehouse models expose normalized entity identity and alias metadata
The system SHALL derive canonical Project, Column, Setting, and Mode identity from current index keys and corresponding filesystem names, keep aliases separate, and expose enough identity information to rewrite references during rename. Persisted `warehouseName` fields SHALL not be required.

#### Scenario: Loading a current normalized entry
- **WHEN** the CLI loads a current index entry
- **THEN** the model derives canonical identity from its key and filesystem position
- **AND** aliases remain available as alternative input references

#### Scenario: Loading a column from a current normalized index
- **WHEN** a Column is loaded from a current key-derived index entry
- **THEN** its canonical identity is exposed without requiring `warehouseName`

#### Scenario: Loading an entry with additional identity-shaped metadata
- **WHEN** an index contains unknown identity-shaped metadata
- **THEN** canonical identity still comes from the current key and filesystem position
- **AND** the additional field is preserved as unknown metadata where valid

#### Scenario: Resolving before persistence
- **WHEN** a command receives a unique alias
- **THEN** the model resolves it to canonical identity before any index, Mode, context, or intent value is persisted
