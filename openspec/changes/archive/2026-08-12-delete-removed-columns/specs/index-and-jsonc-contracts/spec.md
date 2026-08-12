## MODIFIED Requirements

### Requirement: Sync preserves missing or orphaned nodes
The system SHALL preserve index entries for missing or orphaned projects and modes discovered during sync instead of silently deleting them. When sync determines that a previously indexed Column source directory no longer exists in its project, it SHALL remove that Column entry from the reconciled index instead of retaining it with a missing or orphaned marker. When sync determines that a previously indexed Setting source no longer exists in its column directory, it SHALL remove that Setting entry from the reconciled index instead of retaining it with a missing or orphaned marker.

#### Scenario: A column directory disappears from disk
- **WHEN** sync detects that a previously indexed Column no longer exists in the filesystem
- **THEN** the corresponding Column entry is removed from `ColumnIndex.jsonc`
- **AND** the deleted column's directory is not recreated with a fresh Setting index during that synchronization

#### Scenario: A setting file disappears from disk
- **WHEN** sync detects that a previously indexed file-backed Setting no longer exists in the filesystem
- **THEN** the corresponding Setting entry is removed from `SettingIndex.jsonc`
- **AND** sync does not serialize `"missing": true` for that Setting

#### Scenario: A setting directory disappears from disk
- **WHEN** sync detects that a previously indexed directory-backed Setting no longer exists in the filesystem
- **THEN** the corresponding Setting entry is removed from `SettingIndex.jsonc`
- **AND** its prior metadata is removed with the entry

#### Scenario: An unrelated missing index node is rewritten
- **WHEN** the CLI normalizes an index that contains a missing or orphaned project or mode marker
- **THEN** that non-Column, non-Setting entry remains represented in the output instead of being removed as cleanup
