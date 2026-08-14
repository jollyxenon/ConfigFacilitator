## MODIFIED Requirements

### Requirement: PPID context is a convenience feature
The system SHALL allow project-scoped commands to omit `-p` / `--project` after `cfgfc use <Project>` selects a Project for the current PPID scope. `cfgfc use global` SHALL clear that context. The context SHALL remain a convenience state rather than an isolation or authorization boundary, and commands that accept explicit Project input SHALL allow it to override the selected context.

#### Scenario: Using the selected project context
- **WHEN** a user selects a Project with `cfgfc use` and invokes a project-scoped command without an explicit Project
- **THEN** the command resolves the selected canonical Project through PPID context

#### Scenario: Using the active project context
- **WHEN** a user has selected a Project for the current PPID scope
- **THEN** later project-scoped commands may omit explicit Project input

#### Scenario: Reading PPID-scoped convenience state
- **WHEN** a later command resolves a Project without explicit Project input
- **THEN** it reads the convenience context associated with the current PPID

#### Scenario: Returning to global context
- **WHEN** a user runs `cfgfc use global`
- **THEN** the current PPID context is cleared
- **AND** later commands use their documented no-context behavior

### Requirement: Commands still support explicit project selection
Project-scoped commands SHALL accept both `-p <Project>` and `--project <Project>` and SHALL give explicit input precedence over PPID context. `--all` scope SHALL ignore PPID context and SHALL be rejected when combined with explicit Project input. Commands SHALL treat `global` as reserved except as the exact context-clear argument to `cfgfc use global`.

#### Scenario: Overriding implicit context
- **WHEN** a command receives explicit `-p` or `--project` input
- **THEN** that Project is used instead of PPID context

#### Scenario: Explicit input and convenience context disagree
- **WHEN** explicit Project input differs from PPID context
- **THEN** the explicit Project is the effective Project

#### Scenario: All-project operation ignores context
- **WHEN** a Project is selected and the user invokes a supported command with `--all`
- **THEN** the command operates on its documented all-Project scope

#### Scenario: Rejecting global as a Project resource
- **WHEN** a resource command receives `global` as a Project canonical name or alias
- **THEN** the command fails with a reserved-name error

### Requirement: Selected project context stores normalized persisted identity
When `use` resolves a Project from a canonical identity or alias, the system SHALL persist the canonical Project identity as PPID context. A committed Project rename SHALL rewrite every PPID context that stores the old identity. A confirmed Project deletion SHALL clear every PPID context that stores the deleted identity. Failed or rolled-back mutations SHALL leave context unchanged.

#### Scenario: Selecting through an alias and reusing context
- **WHEN** a user runs `cfgfc use <ProjectAlias>` and later invokes a project-scoped command without explicit Project input
- **THEN** the later command resolves from the stored canonical identity rather than the alias text

#### Scenario: Renaming a selected project
- **WHEN** a selected Project is renamed successfully
- **THEN** matching PPID contexts store the new canonical identity

#### Scenario: Deleting a selected project
- **WHEN** a selected Project is deleted successfully
- **THEN** matching PPID contexts are cleared

#### Scenario: Project rename rolls back
- **WHEN** Project rename fails and rolls back
- **THEN** every PPID context still stores the old canonical identity

### Requirement: Target-path resolution remains independent from normalized identity
The system SHALL continue to resolve Setting target directory and name metadata independently from Project, Column, or Setting canonical identities and aliases. Canonical rename SHALL not change fixed target components. A target name explicitly configured to derive from the Setting canonical name SHALL resolve from the new name after Setting rename.

#### Scenario: Applying a setting selected through identity or alias
- **WHEN** a user applies a Setting selected through canonical identity or alias
- **THEN** target resolution uses the Setting's explicit and inherited target metadata

#### Scenario: Applying a setting selected through normalized identity or alias
- **WHEN** a Setting is applied through canonical identity or a unique alias
- **THEN** target metadata is resolved independently from the input reference form

#### Scenario: Renaming a setting with a fixed target name
- **WHEN** a Setting is renamed and its target name is explicitly fixed
- **THEN** the fixed target name remains unchanged

#### Scenario: Renaming a setting with a derived target name
- **WHEN** a Setting is renamed and its target name inherits the setting-derived default
- **THEN** subsequent planning resolves that target name from the new canonical Setting identity

## RENAMED Requirements

- **FROM**: `### Requirement: Switched project context stores normalized persisted identity`
- **TO**: `### Requirement: Selected project context stores normalized persisted identity`
