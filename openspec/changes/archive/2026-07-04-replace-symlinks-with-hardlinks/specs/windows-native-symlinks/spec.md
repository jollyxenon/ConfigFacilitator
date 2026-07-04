## MODIFIED Requirements

### Requirement: Native Windows uses real file and directory symlinks

When running as a native Windows binary, cfgfc MUST create managed regular-file targets using the platform hard-link API exposed through Go `os.Link`. Native Windows MUST NOT create symlinks, junctions, copies, `mklink` links, PowerShell-created links, or other substitute mechanisms for managed activation. Directory-backed mappings MUST fail clearly because normal filesystem hard links cannot represent directories.

#### Scenario: Native Windows creates regular-file hard link

- **GIVEN** cfgfc is running as a native Windows process
- **AND** a planned mapping source is a regular file
- **WHEN** cfgfc applies the mapping
- **THEN** cfgfc MUST create the target as a hard link to the source file
- **AND** cfgfc MUST NOT require Windows Developer Mode or administrator symlink privileges for that creation path

#### Scenario: Native Windows rejects directory mapping

- **GIVEN** cfgfc is running as a native Windows process
- **AND** a planned mapping source is a directory
- **WHEN** cfgfc validates the mapping for apply, update, or revert
- **THEN** cfgfc MUST fail before creating the target
- **AND** the diagnostic MUST state that directory-backed mappings are unsupported by hard-link activation

### Requirement: Source kind is inferred from the source path

cfgfc MUST inspect the existing source path before creating a managed target. If the source is a regular file, cfgfc MAY create a hard link. If the source is missing, a directory, or another unsupported filesystem object, cfgfc MUST fail before target creation. cfgfc MUST NOT persist source-kind metadata in project indexes or current state.

#### Scenario: Source is a regular file

- **GIVEN** a source path exists as a regular file
- **WHEN** cfgfc prepares to create a managed target
- **THEN** cfgfc MUST classify the mapping as eligible for hard-link activation

#### Scenario: Source is missing

- **GIVEN** a planned source path does not exist
- **WHEN** cfgfc prepares to create a managed target
- **THEN** cfgfc MUST fail before creating or replacing the target
- **AND** the diagnostic MUST identify the missing source path

#### Scenario: Source is a directory

- **GIVEN** a planned source path exists as a directory
- **WHEN** cfgfc prepares to create a managed target
- **THEN** cfgfc MUST fail before creating or replacing the target
- **AND** the diagnostic MUST explain that hard-link activation supports regular files only

### Requirement: Windows symlink failure diagnostics are actionable

When native Windows rejects hard-link creation, cfgfc MUST report the attempted source and target, preserve the underlying platform error, and explain that cfgfc did not try symlinks, junctions, copies, `mklink`, PowerShell helpers, or other substitute mechanisms. Diagnostics SHOULD mention common hard-link limitations such as cross-volume mappings, unsupported filesystems, missing sources, and non-regular sources instead of recommending Developer Mode or administrator symlink privileges.

#### Scenario: Native Windows hard-link creation fails

- **GIVEN** cfgfc is running as a native Windows process
- **AND** a regular-file mapping cannot be hard-linked by the platform
- **WHEN** cfgfc reports the failure
- **THEN** the diagnostic MUST include the source path, target path, and original platform error
- **AND** the diagnostic MUST state that cfgfc did not fall back to symlinks, junctions, copies, or shell-created links

#### Scenario: Failure is caused by cross-volume mapping

- **GIVEN** cfgfc is running as a native Windows process
- **AND** the source and target are on volumes that cannot share hard links
- **WHEN** cfgfc reports the hard-link failure
- **THEN** the diagnostic SHOULD point users toward placing the warehouse and target on the same supported filesystem volume

### Requirement: Native Windows default warehouse root is user-profile relative

If the user has not overridden the root, native Windows MUST keep resolving the default warehouse root relative to `%USERPROFILE%`.

#### Scenario: No root override on native Windows

- **GIVEN** cfgfc is running as a native Windows process
- **AND** no root override has been persisted or supplied
- **WHEN** cfgfc resolves the effective warehouse root
- **THEN** it MUST use the existing `%USERPROFILE%`-relative default root behavior

### Requirement: WSL and native Windows path semantics are separate

cfgfc MUST continue to treat WSL and native Windows as separate runtimes with their own path syntax and filesystem semantics. The hard-link backend MUST NOT translate `%USERPROFILE%` paths into `/mnt/c` paths or translate `/mnt/c` paths back into native Windows paths.

#### Scenario: WSL path is not translated for native Windows

- **GIVEN** a warehouse or target path is written using WSL path syntax
- **WHEN** cfgfc runs as a native Windows binary
- **THEN** cfgfc MUST NOT silently translate that path into native Windows syntax

#### Scenario: Native Windows path is not translated for WSL

- **GIVEN** a warehouse or target path is written using native Windows path syntax
- **WHEN** cfgfc runs inside WSL
- **THEN** cfgfc MUST NOT silently translate that path into WSL syntax
