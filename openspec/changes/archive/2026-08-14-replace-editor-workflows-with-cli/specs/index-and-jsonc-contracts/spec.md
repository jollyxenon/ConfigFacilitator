## MODIFIED Requirements

### Requirement: JSONC index files are the durable interoperability contract
The system SHALL continue to store Project, Column, Setting, and Mode metadata in parseable `.jsonc` files with canonical identity derived from parent map keys. Dedicated CLI commands SHALL be sufficient to manage every supported field, so normal workflows SHALL NOT require direct JSONC editing. If external tools produce valid JSONC changes, later `sync` and resource commands SHALL load those changes after stripping comments.

#### Scenario: Managing metadata through the CLI
- **WHEN** a user creates or changes supported metadata through a resource command
- **THEN** the corresponding JSONC index is updated with the same durable schema used for external interoperability
- **AND** no direct index edit is required

#### Scenario: Parsing externally edited JSONC
- **WHEN** the CLI reads a valid index containing JSONC comments or externally changed supported fields
- **THEN** the comments are stripped for parsing
- **AND** the supported field values participate in the resulting model and subsequent synchronization

#### Scenario: Rejecting invalid external JSONC
- **WHEN** an index cannot be parsed or violates required field types
- **THEN** the requested command fails before rewriting that index or related resources

### Requirement: Description fields persist through all repository mutations
The system SHALL preserve descriptions during parsing, synchronization, serialization, target operations, content operations, rename, and unrelated metadata mutations. A description SHALL change only when a command explicitly sets or clears that description or deletes its owning resource.

#### Scenario: Renaming an annotated resource
- **WHEN** a Project, Column, Setting, or Mode with a description is renamed
- **THEN** the description is preserved under the new canonical key

#### Scenario: Updating unrelated metadata
- **WHEN** a command changes aliases or target data without supplying a description option
- **THEN** the previous description remains unchanged

#### Scenario: Explicitly replacing a description
- **WHEN** a resource command supplies `--description` or `--description-file`
- **THEN** the prior description is replaced with the supplied value

### Requirement: Unknown user-authored fields are preserved when possible
The system SHALL preserve unknown parseable JSONC fields through CLI-owned create-adjacent, set, target, selection, rename, synchronization, and serialization operations whenever those fields do not collide with schema-defined keys. Deleting a resource SHALL delete unknown fields owned by that resource. A schema-defined field SHALL take precedence over an unknown field with the same key.

#### Scenario: Renaming an entry with extension fields
- **WHEN** a resource containing unknown parseable fields is renamed
- **THEN** those fields are preserved with that resource under the new canonical key

#### Scenario: Updating a known field
- **WHEN** a CLI command changes a known field on an entry that also contains unknown fields
- **THEN** unrelated unknown fields remain present

#### Scenario: Deleting the owning resource
- **WHEN** a resource is deleted successfully
- **THEN** its complete index entry, including unknown fields, is removed

#### Scenario: Encountering an extra metadata field
- **WHEN** an index contains an unknown but parseable field
- **THEN** the CLI preserves it through unrelated normalization and mutations

#### Scenario: A known field collides with an unknown duplicate
- **WHEN** a schema-defined field collides with an unknown duplicate key
- **THEN** the schema-defined value takes precedence and normalized output remains valid

### Requirement: Sync removes disappeared filesystem-backed nodes
The system SHALL remove indexed Project, Column, and Setting metadata when the corresponding filesystem source is absent during `sync`, while preserving metadata for present resources. Synchronization SHALL NOT recreate missing source directories, files, or Setting indexes, and SHALL NOT implicitly cascade or rewrite Mode selections, current/history runtime records, or PPID context. Later `apply` or `refresh` SHALL fail when such references no longer resolve. Resource CLI deletion continues to require its independent `--yes`, `--cascade`, and `--force-targets` controls.

#### Scenario: A project directory disappears from disk
- **WHEN** sync detects that a previously indexed Project directory is absent
- **THEN** the Project entry is removed from `ProjectIndex.jsonc`
- **AND** the Project structure is not recreated
- **AND** Mode/runtime references are left unchanged

#### Scenario: A column directory disappears from disk
- **WHEN** sync detects that a previously indexed Column directory is absent
- **THEN** the Column entry is removed from `ColumnIndex.jsonc`
- **AND** the Column directory and Setting index are not recreated

#### Scenario: A setting source disappears from disk
- **WHEN** sync detects that a previously indexed file-backed or directory-backed Setting is absent
- **THEN** the Setting entry is removed from `SettingIndex.jsonc`
- **AND** prior metadata is not available for restoration through a later sync

#### Scenario: Sync does not provide prune
- **WHEN** a user supplies `--prune` to `sync`
- **THEN** the command exits with a usage error without changing indexes

#### Scenario: A setting file disappears from disk
- **WHEN** sync detects that a previously indexed file-backed Setting is absent
- **THEN** its metadata is removed and its source file is not recreated

#### Scenario: A setting directory disappears from disk
- **WHEN** sync detects that a previously indexed directory-backed Setting is absent
- **THEN** its metadata is removed and its source directory is not recreated

#### Scenario: An unrelated removed index entry is normalized
- **WHEN** synchronization normalizes an index after removing a filesystem-backed resource
- **THEN** the removed entry remains absent
- **AND** unrelated present resources remain unchanged

### Requirement: Alias metadata is preserved and made explicit during normalization
The system SHALL preserve aliases for Project, Column, Setting, and Mode entries through CLI mutation and synchronization and SHALL serialize an explicit empty alias array when no aliases are declared. Alias updates SHALL reject empty values, duplicate values, canonical-name collisions, and collisions with any other alias in the same resolution scope.

#### Scenario: Clearing aliases through the CLI
- **WHEN** a user invokes a resource set command with `--clear-aliases`
- **THEN** the normalized entry contains `"aliases": []`

#### Scenario: Replacing aliases through the CLI
- **WHEN** a user supplies `--aliases alpha,beta`
- **THEN** the previous alias set is replaced by `alpha` and `beta`
- **AND** both values resolve uniquely to the resource

#### Scenario: Rejecting an alias collision
- **WHEN** a requested alias already resolves to another resource in the same scope
- **THEN** the command fails without changing the previous aliases

#### Scenario: Sync rewrites an annotated index with aliases
- **WHEN** synchronization rewrites an entry that contains valid aliases
- **THEN** those aliases remain present

#### Scenario: Sync rewrites an entry without aliases
- **WHEN** synchronization rewrites an entry with no aliases
- **THEN** normalized output includes `"aliases": []`

### Requirement: User-facing docs explain CLI-managed index responsibilities
Published user-facing documentation SHALL explain that JSONC indexes remain the durable, inspectable persistence and external-interoperability format, while supported metadata, targets, and Mode selections are normally managed through dedicated CLI commands. The documentation SHALL describe canonical keys, aliases, immediate sync removal for disappeared filesystem-backed resources, unresolved Mode/runtime references, external edit synchronization, unknown-field preservation, and the fact that runtime files remain non-user-editable.

#### Scenario: Learning the supported workflow
- **WHEN** a user reads the primary workflow guide
- **THEN** it uses resource commands instead of instructing the user to edit JSONC

#### Scenario: Learning external interoperability
- **WHEN** a user reads the JSONC guide
- **THEN** it explains that valid external edits are accepted on the next load or sync
- **AND** invalid JSONC causes the command to fail before normalization

#### Scenario: Learning runtime-file boundaries
- **WHEN** a user reads about `current_state.json`, `history.log`, session state, or transaction records
- **THEN** the documentation identifies them as CLI-owned data that must not be edited manually

## REMOVED Requirements

### Requirement: Template comments are disposable
**Reason**: New resource creation no longer scaffolds editor-guidance templates as part of the normal workflow.
**Migration**: Use nested command help and user documentation for field guidance; existing JSONC comments remain parseable.

## RENAMED Requirements

- **FROM**: `### Requirement: JSONC index files are the editable contract`
- **TO**: `### Requirement: JSONC index files are the durable interoperability contract`

- **FROM**: `### Requirement: Description fields persist through synchronization`
- **TO**: `### Requirement: Description fields persist through all repository mutations`

- **FROM**: `### Requirement: User-facing docs explain editable index responsibilities`
- **TO**: `### Requirement: User-facing docs explain CLI-managed index responsibilities`
