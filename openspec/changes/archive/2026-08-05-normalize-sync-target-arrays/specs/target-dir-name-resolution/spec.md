## MODIFIED Requirements

### Requirement: Directory/name target schema
The system SHALL use `targetNumber`, `defaultTargetDir`, `defaultTargetName`, `targetDir`, and `targetName` in `SettingIndex.jsonc` as the only target metadata fields. `targetNumber` SHALL be a required top-level non-negative integer that declares the number of target positions for the column. The four target fields SHALL remain string arrays; target arrays are not valid substitutes for a missing `targetNumber`.

#### Scenario: Parsing current target fields
- **WHEN** a setting index contains top-level `targetNumber`, column defaults, and setting overrides using the directory/name fields
- **THEN** the system parses `targetNumber` and the string arrays and preserves them when rewriting the index

#### Scenario: Missing target count
- **WHEN** a setting index omits `targetNumber`
- **THEN** the system rejects the index instead of deriving a target count from any target array

#### Scenario: Invalid target count
- **WHEN** a setting index supplies a negative, fractional, or non-numeric `targetNumber`
- **THEN** the system rejects the index with an error identifying `targetNumber`

#### Scenario: Rejecting old target fields
- **WHEN** a setting index contains `defaultTarget` or setting-level `target`
- **THEN** the system rejects the index instead of treating the old fields as compatible aliases

### Requirement: Strict zip expansion
The system SHALL generate target mappings by strictly zipping resolved target directory arrays with resolved target name arrays by matching index. A successful `sync` SHALL normalize each column's `defaultTargetDir` and `defaultTargetName`, and each extant Setting entry's `targetDir` and `targetName`, to exactly the column's `targetNumber` before writing the index.

#### Scenario: Matching normalized array lengths
- **WHEN** a synced setting index has `targetNumber: 2`
- **THEN** its default directory and name arrays and every persisted setting directory and name array contain exactly two entries
- **AND** the system creates one target mapping per array index by joining `dir[i]` and `name[i]`

#### Scenario: Sync truncates surplus values
- **WHEN** a synced target array has more entries than its column's `targetNumber`
- **THEN** sync retains only the entries at indexes before `targetNumber`
- **AND** sync removes all later entries from the rewritten index

#### Scenario: Sync repeats a uniform short array
- **WHEN** a non-empty target array has fewer entries than `targetNumber`
- **AND** every existing entry has the same string value
- **THEN** sync extends the array to `targetNumber` by repeating that value

#### Scenario: Sync fills a varied short array
- **WHEN** a target array has fewer entries than `targetNumber`
- **AND** it contains at least two different string values
- **THEN** sync preserves its existing entries in order
- **AND** sync appends `""` entries until the array reaches `targetNumber`

#### Scenario: Sync fills an empty short array
- **WHEN** an empty target array has fewer entries than a positive `targetNumber`
- **THEN** sync fills it with `""` entries until it reaches `targetNumber`

#### Scenario: Zero target positions
- **WHEN** a setting index has `targetNumber: 0`
- **THEN** sync rewrites all four target-array kinds as empty arrays
- **AND** target planning continues to reject a selected setting that has no resolved target

### Requirement: Scaffold and sync generation
The system SHALL generate visible `targetNumber`, directory, and name target fields in new and synced setting indexes. Sync SHALL use the declared target count only; it SHALL not derive a count from default arrays or preserve a legacy count behavior.

#### Scenario: New column template
- **WHEN** the user creates a new column
- **THEN** its `SettingIndex.jsonc` contains `targetNumber: 1`, `defaultTargetDir: [""]`, `defaultTargetName: [""]`, and an empty `settings` object

#### Scenario: Sync creates a new setting entry
- **WHEN** sync writes a newly discovered setting entry for a column with `targetNumber: 2`
- **THEN** the entry contains `targetDir` and `targetName` arrays with exactly two entries
- **AND** each generated entry follows the same default-name and warehouse-name semantics at every position

#### Scenario: Sync normalizes authored entries without a fallback
- **WHEN** sync rewrites an existing setting index whose target arrays are not the declared length
- **THEN** it applies the declared-length truncation or extension rules to every target array
- **AND** it does not infer a count from defaults or use an old schema field
