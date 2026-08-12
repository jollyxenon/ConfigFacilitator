# index-and-jsonc-contracts Specification

## Purpose
TBD - created by archiving change bootstrap-main-specs. Update Purpose after archive.
## Requirements
### Requirement: ModeIndex strategies define selection semantics explicitly
The editable `ModeIndex.jsonc` contract SHALL support `cover`, `increment`, `none`, and `full` as the valid mode column strategy values. `cover` and `increment` SHALL use authored `settings` selections, while `none` and `full` SHALL be allowed to omit `settings` because their behavior is implied by the strategy.

#### Scenario: Authoring a replacement strategy
- **WHEN** a user writes `strategy: cover` for a mode column
- **THEN** the authored contract represents explicit replacement semantics for that column

#### Scenario: Authoring an additive strategy
- **WHEN** a user writes `strategy: increment` for a mode column
- **THEN** the authored contract represents additive semantics for that column

#### Scenario: Authoring a no-link strategy without settings
- **WHEN** a user writes `strategy: none` for a mode column and leaves `settings` absent
- **THEN** the authored contract remains valid for that mode column

#### Scenario: Authoring a full-column strategy without settings
- **WHEN** a user writes `strategy: full` for a mode column and leaves `settings` absent
- **THEN** the authored contract remains valid for that mode column

### Requirement: JSONC index files are the editable contract
The system SHALL use `.jsonc` files as the editable index format for project, column, setting, and mode metadata, and SHALL support parsing those files after stripping disposable template comments.

#### Scenario: Authoring index metadata
- **WHEN** a user edits project structure metadata
- **THEN** the metadata is stored in the designated `.jsonc` index files rather than in opaque binary or proprietary formats

#### Scenario: Parsing a JSONC index file
- **WHEN** the CLI reads an index file that contains template comments
- **THEN** the file is parsed successfully after the disposable comments are removed

### Requirement: Description fields persist through synchronization
The system SHALL preserve user-authored `"description"` fields during parsing, synchronization, and serialization.

#### Scenario: Syncing a previously annotated index
- **WHEN** sync updates an index file that already contains `"description"` values
- **THEN** those `"description"` values remain present after serialization

#### Scenario: Normalizing a template index
- **WHEN** the CLI rewrites a JSONC index that already contains permanent `"description"` notes
- **THEN** the `"description"` values remain intact even if generated template comments are removed

### Requirement: Template comments are disposable
The system SHALL generate Index template guidance as a single trailing comment block at the end of each generated `.jsonc` file, and SHALL be allowed to remove that generated guidance during later synchronization or serialization.

#### Scenario: Generating a new index template
- **WHEN** the CLI creates a new `ProjectIndex.jsonc`, `ColumnIndex.jsonc`, `SettingIndex.jsonc`, or `ModeIndex.jsonc`
- **THEN** the JSON object body is emitted without inline template comments
- **AND** the file ends with one generated long-form comment block that shows an example structure for how to fill in the file
- **AND** each generated entity example shows `"aliases": []` as part of the editable metadata contract
- **AND** the generated `ModeIndex.jsonc` example uses the current strategy names and explains when `settings` may be omitted

#### Scenario: Rewriting a generated template
- **WHEN** the CLI rewrites an index file that was originally created with a trailing example comment block
- **THEN** the generated guidance comment may be removed without being treated as data loss
- **AND** durable JSON fields such as `"description"` and `"aliases"` remain the supported place for permanent metadata notes

### Requirement: Unknown user-authored fields are preserved when possible
The system SHALL make a best-effort attempt to preserve unknown user-authored fields during index normalization, provided those fields do not violate the defined schema or prevent successful parsing.

#### Scenario: Encountering an extra metadata field
- **WHEN** an index contains an unknown but parseable field
- **THEN** the CLI keeps that field through normalization when technically possible instead of discarding it by default

#### Scenario: A known field collides with an unknown duplicate
- **WHEN** a JSONC object contains both a schema-defined field and an unknown duplicate of the same key
- **THEN** the schema-defined field takes precedence and the normalized result remains valid

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

### Requirement: Index entries use key-derived identity metadata
The system SHALL treat the parent map key as the canonical persisted identity for project, column, setting, and mode entries, and SHALL preserve display metadata and zero or more aliases as the authored identity surface.

#### Scenario: Reading a normalized project entry
- **WHEN** the CLI parses a project index entry written by a current version
- **THEN** the entry derives its canonical persisted identity from the parent map key
- **AND** the entry retains display and alias metadata without requiring a separate persisted identity field

#### Scenario: Reading an entry with additional identity-shaped fields
- **WHEN** the CLI parses an index entry that includes additional authored identity-shaped fields
- **THEN** the CLI continues to derive canonical identity from the parent map key
- **AND** the authored contract remains centered on the key, display metadata, and aliases

### Requirement: Setting target paths remain distinct from warehouse-side source metadata
The system SHALL persist each setting's target-path metadata independently from the warehouse-side name or path of the source file or directory that will be linked.

#### Scenario: Serializing a setting entry with explicit target metadata
- **WHEN** a setting entry is normalized and written back to `SettingIndex.jsonc`
- **THEN** the stored target-path field still represents the symlink destination input and is not rewritten to match the warehouse-side source path

### Requirement: Alias metadata is preserved and made explicit during normalization
The system SHALL preserve alias metadata for project, column, setting, and mode entries when parsing, synchronizing, and serializing editable JSONC indexes, and SHALL serialize an empty alias array when no aliases are declared.

#### Scenario: Sync rewrites an annotated index with aliases
- **WHEN** the user runs sync on an index that already includes aliases
- **THEN** those aliases remain present after normalization unless the input is invalid because of a collision the system must reject

#### Scenario: Sync rewrites an entry without aliases
- **WHEN** the user runs sync on an index entry that has no declared aliases
- **THEN** the normalized output still includes `"aliases": []` instead of omitting the field

### Requirement: User-facing docs explain editable index responsibilities
The published user-facing documentation SHALL explain which JSONC index fields are authored manually by users, how those fields relate to warehouse structure and apply behavior, which of them are not directly managed through dedicated CLI flags, and that canonical identity comes from the top-level key.

#### Scenario: Reading about warehouse metadata fields
- **WHEN** a user reads the example workflow's JSONC editing guidance
- **THEN** the guide explains the purpose of `description`, `displayName`, and `aliases` within the relevant index files
- **AND** the guide explains that the top-level entry key is the canonical name used by the system

#### Scenario: Reading about target-path fields
- **WHEN** a user configures setting destinations from the example workflow
- **THEN** the guide explains how `defaultTarget` and setting-level `target` interact, including that setting-level `target` overrides the column default

#### Scenario: Reading about mode authoring
- **WHEN** a user reaches the example workflow's mode setup step
- **THEN** the guide explains that mode selections and `strategy` values are edited in `ModeIndex.jsonc` rather than configured through a dedicated command
- **AND** the guide lists `cover`, `increment`, `none`, and `full` with the current meanings of each strategy
- **AND** the guide explains that `settings` may be omitted for `none` and `full`

### Requirement: Normalized index output omits redundant warehouseName fields
The system SHALL omit `warehouseName` from normalized `ProjectIndex.jsonc`, `ColumnIndex.jsonc`, `SettingIndex.jsonc`, and `ModeIndex.jsonc` output when writing current-format index entries.

#### Scenario: Sync rewrites a current-format project entry
- **WHEN** the user runs sync on a project index entry that can be represented by the current contract
- **THEN** the rewritten JSONC output uses the parent map key as the canonical identity
- **AND** the serialized entry omits `warehouseName`

#### Scenario: Scaffold generates a fresh index example
- **WHEN** the CLI generates a new project, column, setting, or mode index example
- **THEN** the example shows editable metadata fields such as `displayName` and `aliases`
- **AND** the example does not include `warehouseName`
