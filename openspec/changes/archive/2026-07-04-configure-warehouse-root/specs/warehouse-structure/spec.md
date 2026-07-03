## MODIFIED Requirements

### Requirement: Project hierarchy shape
Each project SHALL live under `<warehouse-root>/<ProjectName>/` and SHALL contain `Column/`, `Mode/`, and `Backup/` subdirectories, with `ProjectIndex.jsonc` at the effective warehouse root and the project-specific index/state files in their prescribed locations.

#### Scenario: Creating a project skeleton
- **WHEN** a new project is created
- **THEN** the resulting warehouse structure contains the required subdirectories and index/state file locations directly beneath the effective warehouse root

#### Scenario: Discovering root-level project directories
- **WHEN** the CLI scans direct child directories beneath the effective warehouse root
- **THEN** each discovered project directory, including `SettingWarehouse`, is eligible for discovery as a project

### Requirement: Internal session storage remains reserved at the warehouse root
The system SHALL continue reserving internal operational directories such as `.cfgfc-session` under the effective warehouse root so warehouse-root project discovery does not treat them as user projects.

#### Scenario: Scanning the warehouse root with session state present
- **WHEN** the CLI scans the effective warehouse root and finds `.cfgfc-session`
- **THEN** that directory is excluded from project discovery because it is internal operational state rather than a user project
