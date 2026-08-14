## Purpose

Defines the resource-oriented command contract that lets users manage every Project, Column, Setting, Mode, target selection, and active context without editing index files directly.

## ADDED Requirements

### Requirement: CLI exposes resource-oriented command families
The CLI SHALL expose `project`, `column`, `setting`, `mode`, `use`, `status`, `apply`, `refresh`, `sync`, `root`, `reset`, and `revert` as the supported top-level command families. The removed `new`, `switch`, `list`, and `update` command families and the removed flag-only `apply` forms SHALL fail as unknown commands or invalid usage and SHALL NOT be retained as aliases.

#### Scenario: Inspecting the root command surface
- **WHEN** a user runs `cfgfc --help`
- **THEN** the resource-oriented command families are listed
- **AND** the removed command families are not listed

#### Scenario: Invoking a removed command
- **WHEN** a user invokes `cfgfc new`, `cfgfc switch`, `cfgfc list`, or `cfgfc update`
- **THEN** the CLI exits with a usage error
- **AND** it does not mutate warehouse or managed-target state

### Requirement: Resource commands use consistent project scope
Commands below the `column`, `setting`, `mode`, `apply`, `refresh`, `status`, `reset`, and `revert` families SHALL accept `-p <Project>` and `--project <Project>` as equivalent explicit project selectors. When explicit project input is absent, project-scoped commands SHALL use the PPID-scoped project selected by `cfgfc use`. Explicit project input SHALL override the selected context. A command that requires one project SHALL fail without mutation when neither source resolves a project.

#### Scenario: Using selected project context
- **WHEN** a user runs `cfgfc use OpenCode` and then `cfgfc column list` without a project flag
- **THEN** the CLI lists Columns from `OpenCode`

#### Scenario: Explicit project overrides selected context
- **WHEN** the selected context is `ProjectA` and the user runs `cfgfc mode list -p ProjectB`
- **THEN** the CLI lists Modes from `ProjectB`

#### Scenario: Required project is unavailable
- **WHEN** no project context is selected and the user runs a project-scoped command without `-p` or `--project`
- **THEN** the command exits with a scope error
- **AND** no data is changed

### Requirement: Project resources support complete metadata CRUD
The CLI SHALL support `cfgfc project list`, `cfgfc project show <Project>`, `cfgfc project create <Project>`, `cfgfc project set <Project>`, `cfgfc project rename <Old> <New>`, and `cfgfc project delete <Project> --yes`. `project create` SHALL create the complete standard Project structure and index entry in one operation. `project set` SHALL support `--display-name`, `--description`, `--description-file <Path|->`, `--aliases <CommaSeparatedValues>`, and `--clear-aliases`; conflicting value sources SHALL be rejected. Canonical names and aliases SHALL be unique within project scope, and `global` SHALL remain reserved.

#### Scenario: Creating a project
- **WHEN** a user runs `cfgfc project create OpenCode`
- **THEN** the standard Project directories, index files, and runtime-state files are created
- **AND** the Project is immediately available to later CLI commands without running `sync`

#### Scenario: Replacing project metadata
- **WHEN** a user runs `cfgfc project set OpenCode --display-name "OpenCode Config" --aliases oc,code --description "Managed OpenCode configuration"`
- **THEN** all supplied fields replace their current values in one mutation
- **AND** `oc` and `code` resolve to the canonical Project

#### Scenario: Reading a description from stdin
- **WHEN** a user runs `cfgfc project set OpenCode --description-file -` with text on standard input
- **THEN** the complete standard-input text becomes the Project description

#### Scenario: Rejecting ambiguous aliases
- **WHEN** a requested Project alias collides with another Project canonical name or alias
- **THEN** the command fails before writing any index

### Requirement: Column resources support complete metadata CRUD
The CLI SHALL support `cfgfc column list`, `cfgfc column show <Column>`, `cfgfc column create <Column>`, `cfgfc column set <Column>`, `cfgfc column rename <Old> <New>`, and `cfgfc column delete <Column> --yes`. Column metadata flags SHALL have the same replacement semantics as Project metadata flags. A newly created Column SHALL start with zero target positions, an empty Setting collection, and a usable index file; it SHALL NOT require a later `sync` before Settings or targets can be added.

#### Scenario: Creating an empty column
- **WHEN** a user runs `cfgfc column create Skills -p OpenCode`
- **THEN** the Column directory and index metadata are created
- **AND** its target count is zero
- **AND** `cfgfc column show Skills -p OpenCode` succeeds immediately

#### Scenario: Updating column metadata through an alias
- **WHEN** a Column alias resolves uniquely and the user invokes `column set` through that alias
- **THEN** the canonical Column entry is updated
- **AND** the alias text is not substituted for the canonical key

### Requirement: Column target positions are managed structurally
The CLI SHALL expose `cfgfc column target list <Column>`, `add`, `set`, and `delete` operations. `add` SHALL append one zero-based target position using `--dir <Directory>` and exactly one of `--name <Name>` or `--name-from-setting`. `set` SHALL replace only supplied components at an existing index and SHALL support `--clear-dir` and `--name-from-setting` for the persisted empty-value semantics. `delete <Column> <Index> --yes` SHALL remove that position and the corresponding position from every Setting override in the Column. Every successful operation SHALL keep target count and all default and override arrays at identical lengths.

#### Scenario: Adding a fixed-name target
- **WHEN** a user runs `cfgfc column target add Models --dir ~/.config/opencode --name opencode.json`
- **THEN** a new target position is appended with those default components
- **AND** every Setting override array in `Models` is extended with inherited values

#### Scenario: Adding a setting-derived target name
- **WHEN** a user runs `cfgfc column target add Skills --dir ~/.config/opencode/skills --name-from-setting`
- **THEN** the target name at that position resolves from each Setting canonical name unless overridden

#### Scenario: Deleting a target position
- **WHEN** a user confirms deletion of target position 1 from a Column with three positions
- **THEN** the former position 2 becomes position 1 in every default and override array
- **AND** target count becomes two

#### Scenario: Rejecting an invalid target index
- **WHEN** the user addresses a negative or out-of-range target position
- **THEN** the command fails without changing any target array

### Requirement: Setting resources support metadata and target override CRUD
The CLI SHALL support `cfgfc setting list -c <Column>`, `cfgfc setting show <Setting> -c <Column>`, `cfgfc setting create <Setting> -c <Column> --kind <file|directory>`, `cfgfc setting set <Setting> -c <Column>`, `cfgfc setting rename <Old> <New> -c <Column>`, and `cfgfc setting delete <Setting> -c <Column> --yes`. The Column selector SHALL also have a `--column` long form. Setting metadata flags SHALL follow the common replacement semantics. `cfgfc setting target list`, `set`, and `reset` SHALL manage an existing target position; `set` SHALL accept `--dir`, `--name`, `--inherit-dir`, and `--inherit-name`, while `reset` SHALL restore both components to inheritance.

#### Scenario: Creating a file-backed setting
- **WHEN** a user creates a file-backed Setting without an initial content source
- **THEN** an empty regular file and its metadata entry are created atomically
- **AND** its target arrays inherit every existing Column target position

#### Scenario: Creating a directory-backed setting
- **WHEN** a user creates a directory-backed Setting without an import source
- **THEN** an empty directory and its metadata entry are created atomically

#### Scenario: Overriding one target component
- **WHEN** a user runs `cfgfc setting target set GPT.json 0 -c Models --inherit-dir --name model.json`
- **THEN** the target directory at position 0 inherits the Column default
- **AND** the target name at position 0 is `model.json`

#### Scenario: Resetting one setting target
- **WHEN** a user runs `cfgfc setting target reset GPT.json 0 -c Models`
- **THEN** both target components at position 0 inherit their Column defaults

### Requirement: Mode resources and selections support complete CRUD
The CLI SHALL support `cfgfc mode list`, `show`, `create`, `set`, `rename`, and `delete`. A new Mode SHALL start with no Column selections. `cfgfc mode column list <Mode>`, `set <Mode> <Column>`, and `delete <Mode> <Column>` SHALL manage selections. `mode column set` SHALL require `--strategy <cover|increment|none|full>`; repeated `--setting <Setting>` flags SHALL be required for `cover` and `increment` and SHALL be rejected for `none` and `full`.

#### Scenario: Creating a mode
- **WHEN** a user runs `cfgfc mode create Max -p OpenCode`
- **THEN** an empty Mode is created without invalid placeholder selections

#### Scenario: Defining an explicit selection
- **WHEN** a user runs `cfgfc mode column set Max Models --strategy cover --setting GPT.json --setting Tools.json`
- **THEN** the Mode stores a `cover` selection for the canonical Column and canonical Settings

#### Scenario: Defining a full selection
- **WHEN** a user runs `cfgfc mode column set Max Skills --strategy full`
- **THEN** the Mode stores a `full` selection without an explicit Setting list

#### Scenario: Rejecting an incomplete selection
- **WHEN** `cover` or `increment` is requested without any Setting
- **THEN** the command fails without changing the Mode

### Requirement: Use and status expose context and runtime state
`cfgfc use <Project>` SHALL select a canonical Project for the current PPID scope, and `cfgfc use global` SHALL clear that scope. `cfgfc status` SHALL show warehouse-wide Project usage when no effective Project is selected and SHALL show the effective Project's current intent, mappings, Column coverage, unresolved references, and matched Mode when one is selected. `cfgfc status -p <Project>` SHALL inspect that Project regardless of context.

#### Scenario: Selecting a project through an alias
- **WHEN** a user runs `cfgfc use oc` and `oc` uniquely resolves to `OpenCode`
- **THEN** the current PPID scope stores `OpenCode` rather than `oc`

#### Scenario: Inspecting project status
- **WHEN** a user runs `cfgfc status -p OpenCode`
- **THEN** the output identifies the active apply intent and managed mappings
- **AND** it reports unresolved references and per-Column coverage

### Requirement: Apply and refresh use nested resource syntax
The CLI SHALL accept exactly `cfgfc apply mode <Mode>` and `cfgfc apply column <Column> <Setting>...` for new apply requests. `cfgfc refresh` SHALL re-plan the persisted apply intent from current metadata, SHALL accept `--column <Column>` to refresh only one Column while retaining other mappings, and SHALL accept `--all` to refresh every Project with an active intent. `refresh --all` SHALL be incompatible with project and Column selectors. `apply`, `refresh`, `reset`, and `revert` SHALL accept `--force-targets` to reclaim unmanaged or drifted target paths and SHALL NOT expose `-f` or `--force`.

#### Scenario: Applying a mode
- **WHEN** a user runs `cfgfc apply mode Max -p OpenCode`
- **THEN** the Mode is planned and persisted as the active intent

#### Scenario: Applying multiple settings from one column
- **WHEN** a user runs `cfgfc apply column Skills Skill-A Skill-B -p OpenCode`
- **THEN** both Settings are planned as one direct-Column apply intent

#### Scenario: Refreshing one column
- **WHEN** a Mode intent is active and the user runs `cfgfc refresh --column Skills -p OpenCode`
- **THEN** the current Mode strategy is re-evaluated for `Skills`
- **AND** managed mappings from other Columns remain unchanged

#### Scenario: Rejecting removed apply syntax
- **WHEN** a user invokes `cfgfc apply -m Max` or `cfgfc apply -c Skills -s Skill-A`
- **THEN** the command exits with a usage error and performs no apply

### Requirement: Commands provide stable machine-readable results
Every non-help command SHALL accept `--json`. On success the CLI SHALL write exactly one JSON object to standard output with `ok: true` and a command-specific `data` object. On failure it SHALL write exactly one JSON object to standard error with `ok: false` and an `error` object containing stable `code` and human-readable `message` fields. JSON mode SHALL suppress ANSI color and unrelated prose. Exit codes SHALL be `0` for success, `2` for usage errors, `3` for missing or conflicting resources, `4` for invalid resource data, `5` for missing destructive confirmation or unsafe target ownership, and `6` for filesystem, persistence, or transaction failures.

#### Scenario: Successful JSON inspection
- **WHEN** a user runs `cfgfc project show OpenCode --json`
- **THEN** standard output contains one valid success object
- **AND** the process exits with code 0

#### Scenario: JSON resource conflict
- **WHEN** a JSON-mode mutation encounters a canonical-name or alias conflict
- **THEN** standard error contains one valid error object with a stable conflict code
- **AND** the process exits with code 3

#### Scenario: Human-readable output remains available
- **WHEN** `--json` is absent
- **THEN** commands emit concise human-readable output and errors
