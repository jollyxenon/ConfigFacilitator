## Purpose

Defines safe CLI operations for creating, importing, inspecting, and changing the text content of file-backed and directory-backed Settings without direct filesystem editing.

## ADDED Requirements

### Requirement: Setting creation accepts exactly one content source
`cfgfc setting create <Setting> -c <Column> --kind <file|directory>` SHALL accept at most one of `--from <Path>`, `--stdin`, or `--text <Text>`. `--stdin` and `--text` SHALL be valid only for file-backed Settings. `--from` SHALL accept a regular file for `--kind file` and a directory for `--kind directory`. When no content source is supplied, file creation SHALL produce an empty regular file and directory creation SHALL produce an empty directory.

#### Scenario: Creating a file from standard input
- **WHEN** a user pipes text to `cfgfc setting create GPT.json -c Models --kind file --stdin`
- **THEN** the new Setting source contains exactly the standard-input bytes
- **AND** its index entry is committed in the same operation

#### Scenario: Creating a file from literal text
- **WHEN** a user runs file-backed Setting creation with `--text` and no other content source
- **THEN** the source contains exactly the provided argument bytes without an added newline

#### Scenario: Importing a file
- **WHEN** a user creates a file-backed Setting with `--from <RegularFile>`
- **THEN** the source content is copied into the warehouse
- **AND** later changes to `<RegularFile>` do not change the warehouse copy

#### Scenario: Importing a directory tree
- **WHEN** a user creates a directory-backed Setting with `--from <Directory>`
- **THEN** the directory's regular files and subdirectories are recursively copied under the Setting source

#### Scenario: Rejecting conflicting content sources
- **WHEN** more than one of `--from`, `--stdin`, and `--text` is supplied
- **THEN** the command exits with a usage error
- **AND** neither source content nor metadata is created

#### Scenario: Rejecting a source-kind mismatch
- **WHEN** `--kind file` receives a directory or `--kind directory` receives a regular file
- **THEN** the command fails without creating a partial Setting

### Requirement: Imported content is bounded to supported filesystem objects
Directory import SHALL accept regular files and directories only. It SHALL reject symbolic links, sockets, devices, named pipes, and any object whose resolved traversal would escape the selected import root. A failed import SHALL leave no Setting source or metadata entry.

#### Scenario: Import tree contains a symbolic link
- **WHEN** a directory supplied to `--from` contains a symbolic link at any depth
- **THEN** import fails before the Setting is committed
- **AND** the link target is not copied or followed

#### Scenario: Import tree contains only regular content
- **WHEN** every imported object is a regular file or directory below the import root
- **THEN** the complete tree is copied successfully

### Requirement: Content inspection distinguishes file and directory Settings
The CLI SHALL provide `cfgfc setting content list <Setting> -c <Column>` and `cfgfc setting content read <Setting> [RelativePath] -c <Column>`. `list` on a directory-backed Setting SHALL recursively return relative paths, entry kinds, and byte sizes in lexical path order; `list` on a file-backed Setting SHALL return the Setting source as one file entry. `read` SHALL require no relative path for a file-backed Setting and SHALL require a relative path naming an existing regular file for a directory-backed Setting. Human-readable `read` SHALL write file bytes without decoration; JSON `read` SHALL return UTF-8 text when valid and base64 content with an explicit encoding marker otherwise.

#### Scenario: Listing directory content
- **WHEN** a user lists a directory-backed Setting containing nested files
- **THEN** every regular file and directory is reported by normalized relative path in lexical order

#### Scenario: Reading a file-backed setting
- **WHEN** a user reads a file-backed Setting without a relative path
- **THEN** standard output contains exactly the source file bytes in human-readable mode

#### Scenario: Reading a nested file
- **WHEN** a user reads a directory-backed Setting with a relative path naming a regular file
- **THEN** the selected file bytes are returned

#### Scenario: Rejecting incompatible read arguments
- **WHEN** a file-backed Setting receives a relative path or a directory-backed Setting omits it
- **THEN** the command exits with a usage error

### Requirement: Content write creates or replaces one regular file
`cfgfc setting content write <Setting> [RelativePath] -c <Column>` SHALL require exactly one of `--from <RegularFile>`, `--stdin`, or `--text <Text>`. For a file-backed Setting, relative path SHALL be omitted and the Setting source SHALL be atomically replaced. For a directory-backed Setting, relative path SHALL identify a file below the Setting root; missing parent directories SHALL be created, and an existing regular file SHALL be atomically replaced. The operation SHALL reject an existing directory at the destination.

#### Scenario: Replacing file content from stdin
- **WHEN** a user writes a file-backed Setting with `--stdin`
- **THEN** readers observe either the complete previous content or complete new content
- **AND** never a partially written file

#### Scenario: Creating a nested directory-setting file
- **WHEN** a user writes `prompts/system.md` below a directory-backed Setting and `prompts` does not exist
- **THEN** the parent directory is created
- **AND** `system.md` contains exactly the supplied content

#### Scenario: Rejecting a directory destination
- **WHEN** the destination relative path already names a directory
- **THEN** write fails without deleting or replacing that directory

### Requirement: Directory content supports explicit mkdir, move, and delete
For directory-backed Settings the CLI SHALL provide `content mkdir <Setting> <RelativePath>`, `content move <Setting> <OldPath> <NewPath>`, and `content delete <Setting> <RelativePath> --yes`. `mkdir` SHALL create the requested directory and missing parents. `move` SHALL move one regular file or directory within the same Setting and SHALL fail if the destination exists. `delete` SHALL remove a regular file or directory tree only after confirmation. These operations SHALL be rejected for file-backed Settings.

#### Scenario: Creating a nested directory
- **WHEN** a user invokes `content mkdir` with `prompts/system`
- **THEN** the complete nested directory path is created below the Setting root

#### Scenario: Moving a subtree
- **WHEN** a user moves `old/prompts` to `new/prompts` and the destination is absent
- **THEN** the complete subtree is present only at the destination after success

#### Scenario: Refusing an overwrite during move
- **WHEN** the requested move destination already exists
- **THEN** the command fails without changing either path

#### Scenario: Requiring content deletion confirmation
- **WHEN** a user invokes `content delete` without `--yes`
- **THEN** the command exits with a confirmation error
- **AND** the content remains unchanged

#### Scenario: Deleting a directory tree
- **WHEN** a user invokes `content delete` with `--yes` for an existing directory below the Setting
- **THEN** that directory and all descendants are removed
- **AND** the enclosing Setting remains present

### Requirement: Relative content paths cannot escape the Setting root
Every directory-content relative path SHALL be non-empty, non-absolute, clean, and composed without `.` or `..` traversal segments. Before reading or mutating content, the CLI SHALL inspect every existing path component without following symbolic links and SHALL reject an operation when any component is a symbolic link or the resolved destination is not below the canonical Setting root.

#### Scenario: Rejecting parent traversal
- **WHEN** a content command receives `../outside`, `a/../../outside`, or an absolute path
- **THEN** the command fails before reading or changing any filesystem object

#### Scenario: Rejecting a symlinked parent component
- **WHEN** an existing component of a requested content path is a symbolic link
- **THEN** the command fails without following the link

#### Scenario: Accepting a clean nested path
- **WHEN** the requested path consists only of normal relative segments under the Setting root
- **THEN** the content operation proceeds subject to its other validations

### Requirement: Content mutations preserve managed-link behavior
Changing bytes or descendants inside an existing Setting source SHALL not rewrite apply intent, current mappings, or history because managed links continue to reference the same source path. Creating or deleting the Setting resource itself SHALL use the repository mutation requirements instead.

#### Scenario: Editing an actively linked file setting
- **WHEN** a file-backed Setting is currently linked to a managed target and its content is replaced
- **THEN** reading the managed target returns the new content
- **AND** no `refresh` is required

#### Scenario: Editing a file within an actively linked directory setting
- **WHEN** a file below a currently linked directory-backed Setting is changed
- **THEN** the change is visible through the managed directory target
- **AND** the persisted mapping set is unchanged
