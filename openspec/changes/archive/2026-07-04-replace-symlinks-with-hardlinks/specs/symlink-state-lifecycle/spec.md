## MODIFIED Requirements

### Requirement: Apply uses real symlinks only

Apply MUST activate managed file-backed mappings through real filesystem hard links from each warehouse source file to each configured target path. It MUST NOT create symlinks, junctions, copies, shell fallbacks, or platform-specific substitute links. If a planned source is a directory or other non-regular filesystem object, apply MUST fail before mutating that target and report that directory-backed mappings are unsupported by the hard-link backend.

#### Scenario: Regular file source is activated through a hard link

- **GIVEN** a planned mapping from a regular warehouse source file to a target path
- **WHEN** apply creates the target
- **THEN** the target MUST be a hard link to the source file
- **AND** reading or writing either path MUST observe the same file contents

#### Scenario: Platform cannot create a hard link

- **GIVEN** a planned mapping from a regular source file to a target path
- **WHEN** the platform rejects hard-link creation, including cross-device or cross-volume failures
- **THEN** cfgfc MUST fail the operation with the target and source paths in the diagnostic
- **AND** cfgfc MUST NOT fall back to a symlink, junction, copy, shell command, or other substitute mechanism

#### Scenario: Directory source cannot be activated

- **GIVEN** a planned mapping whose source path is a directory
- **WHEN** cfgfc validates the mapping for apply
- **THEN** cfgfc MUST fail before replacing or creating the target
- **AND** the diagnostic MUST state that directory-backed mappings cannot be represented by hard links

### Requirement: Conflict handling is interactive-only

Apply MUST treat unmanaged targets and recorded-state drift as hard-stop errors by default. The `--force` / `-f` flag MUST be the only non-interactive way to remove or overwrite an occupied target. Ownership validation MUST use filesystem identity between the recorded source path and target path; a target is managed only when both paths exist as regular files and refer to the same underlying file.

#### Scenario: Unmanaged real file blocks apply without force

- **GIVEN** a target path already exists as a regular file that is not the same filesystem file as the planned source
- **WHEN** the user runs `cfgfc apply` without `--force`
- **THEN** the command MUST fail before replacing the target
- **AND** the diagnostic MUST identify the occupied target as unmanaged

#### Scenario: Unmanaged directory blocks apply without force

- **GIVEN** a target path already exists as a directory
- **WHEN** the user runs `cfgfc apply` without `--force`
- **THEN** the command MUST fail before removing the directory
- **AND** the diagnostic MUST identify the occupied target as unmanaged

#### Scenario: Existing symlink target blocks hard-link apply without force

- **GIVEN** a target path already exists as a symlink, including a symlink recorded by an older cfgfc version
- **WHEN** the user runs `cfgfc apply` without `--force`
- **THEN** the command MUST fail before replacing the target
- **AND** the diagnostic MUST explain that the target is not owned by the hard-link backend

#### Scenario: Recorded hard-link target drifts away from source

- **GIVEN** current state records a source-target mapping
- **AND** the target path exists but is not the same filesystem file as the recorded source
- **WHEN** cfgfc validates ownership for apply, update, reset, or revert without `--force`
- **THEN** cfgfc MUST report state drift instead of treating the target as owned

#### Scenario: Force reclaims occupied target for a file mapping

- **GIVEN** an occupied target path for a planned regular-file mapping
- **WHEN** the user runs `cfgfc apply --force`
- **THEN** cfgfc MAY remove the occupied target according to force recovery rules
- **AND** cfgfc MUST create a hard link from the source file to the target path after reclamation succeeds

### Requirement: Directory targets refer to the final directory itself

Directory-based settings MAY still resolve a final target path through target directory and target name metadata, but activation MUST fail when the resolved mapping source is a directory because normal filesystem hard links cannot represent directories. This failure MUST happen before target mutation unless the command is only inspecting or listing metadata.

#### Scenario: Directory source mapping resolves a final target path

- **GIVEN** a setting whose source path is a directory and whose metadata resolves a final target path
- **WHEN** cfgfc applies, updates into, or reverts into that mapping
- **THEN** cfgfc MUST reject the mapping as unsupported by hard-link activation
- **AND** cfgfc MUST leave the resolved target path unchanged unless an earlier force cleanup already occurred for another valid file mapping

### Requirement: Current state tracks active owned links

Current state MUST continue to record active project-owned source-target mappings and apply intent in `Backup/current_state.json`. A persisted mapping is considered currently owned only when the source and target both exist as regular files and `os.SameFile`-equivalent filesystem identity confirms that they are hard links to the same file. Current-state readers MUST remain compatible with legacy mapping-only state files, but legacy symlink targets MUST NOT be treated as owned hard-link targets.

#### Scenario: Completing apply reflects active hard-link mappings

- **GIVEN** a successful apply of a mode with multiple regular-file mappings
- **WHEN** cfgfc writes current state
- **THEN** `Backup/current_state.json` MUST list exactly those active source-target mappings
- **AND** each listed target MUST be validated by filesystem identity against its source

#### Scenario: Current state persists source-target pairs

- **GIVEN** a successful apply operation
- **WHEN** cfgfc writes current state
- **THEN** each active mapping entry MUST persist both the source path and target path
- **AND** it MUST NOT depend on a symlink target string for future ownership checks

#### Scenario: File-backed managed hard link exposes source contents through target

- **GIVEN** a regular source file is active at a target path through cfgfc
- **WHEN** a process reads the target path
- **THEN** it MUST observe the source file contents
- **AND** subsequent writes through either path MUST be reflected through the other path because both names refer to the same file

#### Scenario: Reading legacy mapping-only state

- **GIVEN** a current-state file written by an older version with only source-target mappings
- **WHEN** cfgfc loads current state
- **THEN** cfgfc MUST load the mapping data without schema failure
- **AND** ownership validation MUST still require current hard-link filesystem identity before mutating or removing targets without force

### Requirement: Revert restores exactly one previous apply state

Revert MUST restore exactly one previous apply state by removing the current owned hard-link targets and recreating the previous regular-file hard-link mappings and apply intent. Revert MUST fail clearly for previous mappings whose sources are directories or otherwise unsupported by hard-link activation.

#### Scenario: Revert restores previous hard-link mappings

- **GIVEN** current state contains owned hard-link mappings
- **AND** backup history contains a previous state with regular-file mappings
- **WHEN** the user runs `cfgfc revert`
- **THEN** cfgfc MUST remove only targets currently owned by hard-link identity
- **AND** cfgfc MUST recreate the previous mappings as hard links
- **AND** cfgfc MUST make that previous state the new current state

#### Scenario: Revert refuses previous directory mapping

- **GIVEN** backup history contains a previous mapping whose source is a directory
- **WHEN** the user runs `cfgfc revert`
- **THEN** cfgfc MUST fail before creating that target
- **AND** the diagnostic MUST explain that the previous mapping cannot be restored by the hard-link backend

### Requirement: Reset clears the current managed links

Reset MUST remove current managed hard-link target directory entries and clear the current apply intent. Without force, reset MUST remove only targets that are still owned by hard-link filesystem identity. With force, reset MAY reclaim every recorded target path, including files, symlinks, and directories that drifted away from ownership.

#### Scenario: Reset removes owned hard-link target

- **GIVEN** current state records a target that is the same filesystem file as its source
- **WHEN** the user runs `cfgfc reset`
- **THEN** cfgfc MUST remove the target directory entry
- **AND** cfgfc MUST leave the warehouse source file present
- **AND** cfgfc MUST clear current managed state after all owned targets are removed

#### Scenario: Reset refuses drift without force

- **GIVEN** current state records a target path
- **AND** the target path exists but is not the same filesystem file as the recorded source
- **WHEN** the user runs `cfgfc reset` without `--force`
- **THEN** cfgfc MUST fail before removing the target
- **AND** the diagnostic MUST identify the recorded target as drifted or unmanaged

#### Scenario: Forced reset reclaims recorded directory target

- **GIVEN** current state records a target path
- **AND** that target path now exists as a directory
- **WHEN** the user runs `cfgfc reset --force`
- **THEN** cfgfc MAY remove the directory recursively according to force recovery rules
- **AND** cfgfc MUST clear the recorded mapping after successful reclamation

### Requirement: Update refreshes owned current mappings in place

Update MUST refresh the persisted apply intent from current warehouse metadata and apply the refreshed mapping set through the hard-link backend. Without force, update MUST only mutate targets that are currently owned by hard-link filesystem identity or absent. With force, update MAY reclaim unmanaged files, symlinks, and directories before creating replacement hard links for regular-file sources.

#### Scenario: Update adds new regular-file mapping

- **GIVEN** current state was created from a mode apply
- **AND** warehouse metadata gains a new regular-file source in that mode
- **WHEN** the user runs `cfgfc update`
- **THEN** cfgfc MUST create a hard link for the new mapping
- **AND** cfgfc MUST persist the refreshed source-target mapping set

#### Scenario: Update rejects new directory mapping

- **GIVEN** current state was created from a mode apply
- **AND** warehouse metadata gains a directory source in that mode
- **WHEN** the user runs `cfgfc update`
- **THEN** cfgfc MUST fail before creating or replacing the target for that directory mapping
- **AND** the diagnostic MUST explain that the new mapping is unsupported by hard-link activation

#### Scenario: Forced update reclaims occupied target for regular-file mapping

- **GIVEN** a refreshed mapping points a regular source file at an occupied target path
- **WHEN** the user runs `cfgfc update --force`
- **THEN** cfgfc MAY reclaim the occupied target according to force recovery rules
- **AND** cfgfc MUST create a hard link from the source to the target after reclamation succeeds

### Requirement: Force-enabled target reclamation supports recursive directory removal

Force-enabled apply, update, reset, and revert MUST be able to reclaim occupied target paths that are files, symlinks, or directories when the command semantics allow force recovery. Recursive directory removal is target cleanup only; it MUST NOT make directory-backed source mappings supported by the hard-link backend.

#### Scenario: Forced apply reclaims directory target then creates file hard link

- **GIVEN** a planned regular-file mapping whose target path currently exists as a directory
- **WHEN** the user runs `cfgfc apply --force`
- **THEN** cfgfc MUST remove the directory recursively
- **AND** cfgfc MUST create a hard link from the source file to the target path

#### Scenario: Forced apply refuses directory source after target cleanup decision

- **GIVEN** a planned mapping whose source path is a directory
- **WHEN** the user runs `cfgfc apply --force`
- **THEN** cfgfc MUST reject the mapping as unsupported by hard-link activation
- **AND** cfgfc MUST NOT create a symlink, junction, copy, or other substitute target
