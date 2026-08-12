# warehouse-layout-and-models Specification

## Purpose
TBD - created by archiving change warehouse-layout-and-models. Update Purpose after archive.
## Requirements
### Requirement: Warehouse domain objects are explicit
The system SHALL represent warehouse state using explicit project, column, setting, and mode domain objects rather than relying on unstructured filesystem traversal alone.

#### Scenario: Loading a project model
- **WHEN** the CLI inspects a project warehouse
- **THEN** it produces a structured project model with columns, settings, modes, and backup state references

### Requirement: Missing and orphaned entries remain visible in the model
The system SHALL keep missing or orphaned project and mode entries visible in the warehouse model so later commands can preserve their metadata and report their status accurately. Once synchronization detects that an indexed Column source directory no longer exists, the reconciled warehouse model SHALL exclude that Column rather than retain it in a missing or orphaned state. Once synchronization detects that an indexed Setting source no longer exists, the reconciled warehouse model SHALL exclude that Setting rather than retain it in a missing or orphaned state.

#### Scenario: Reconciling a missing column
- **WHEN** sync loads a warehouse whose project index contains a Column but the corresponding source directory is absent
- **THEN** the reconciled project model does not include that Column
- **AND** serialization removes the Column from the project's column index

#### Scenario: Reconciling a missing setting
- **WHEN** sync loads a warehouse whose index contains a Setting but the corresponding source file or directory is absent
- **THEN** the reconciled project model does not include that Setting
- **AND** serialization removes the Setting from the column index

#### Scenario: Loading another missing entity
- **WHEN** a parsed warehouse contains a missing or orphaned project or mode entry
- **THEN** the resulting model still includes that non-Column, non-Setting entry with its missing/orphaned state preserved

### Requirement: Warehouse fixtures map to the archived directory contract
The system SHALL interpret test fixtures using the archived warehouse directory contract, including the project root, `Column/`, `Mode/`, and `Backup/` directories and their associated index/state files.

#### Scenario: Reading a sample warehouse fixture
- **WHEN** the CLI loads a representative warehouse fixture from disk
- **THEN** it recognizes the required directories and index/state files according to the archived layout contract

### Requirement: Column, setting, and mode relationships are preserved
The system SHALL preserve the relationship between a column's settings, a mode's declared column selections, and the warehouse's project-level organization in the in-memory model.

#### Scenario: Parsing a project with multiple columns and modes
- **WHEN** a warehouse contains more than one column and one mode
- **THEN** the resulting model keeps the column-to-setting and mode-to-column relationships intact

### Requirement: Backup references remain attached to the project model
The system SHALL attach the project's backup state references to the project model so later commands can locate `current_state.json` and `history.log` without re-deriving their location.

#### Scenario: Inspecting a project model with backup state
- **WHEN** a project model is loaded
- **THEN** the model includes the project's backup file references as part of its structured state

### Requirement: Warehouse models expose normalized entity identity and alias metadata
The system SHALL load warehouse project, column, setting, and mode models with normalized identity derived from warehouse-side names and current index keys, and SHALL keep alias metadata available as a separate concern.

#### Scenario: Loading a column from a current normalized index
- **WHEN** the CLI loads a project that contains a column entry written by the current normalized contract
- **THEN** the resulting in-memory model exposes the column's canonical identity consistently from warehouse-side naming and index position
- **AND** that load does not require a persisted `warehouseName` field

#### Scenario: Loading an entry with additional identity-shaped metadata
- **WHEN** the CLI loads a project, column, setting, or mode entry that includes additional identity-shaped metadata
- **THEN** the resulting in-memory model derives canonical identity from the current warehouse name and index position

### Requirement: Warehouse models preserve alias resolution metadata
The system SHALL keep per-scope alias metadata in the in-memory warehouse model so later command flows can resolve projects, columns, settings, and modes through normalized persisted identity or alias.

#### Scenario: Loading a mode with aliases
- **WHEN** the CLI loads a mode entry that includes aliases
- **THEN** the resulting project model keeps those aliases available for later resolution without changing the mode's normalized persisted identity
