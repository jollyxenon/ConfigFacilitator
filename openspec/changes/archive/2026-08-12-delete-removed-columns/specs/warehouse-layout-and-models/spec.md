## MODIFIED Requirements

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
