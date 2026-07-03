## MODIFIED Requirements

### Requirement: List shows available projects and project contents
The `list` workflow SHALL display available projects when no effective project is selected, and SHALL display the selected project's columns and modes when a project is resolved. In global mode, each listed project SHALL append one parenthesized usage summary derived from that project's persisted managed state: the resolved active mode label when the current state records a resolvable mode intent and that mode's current metadata-derived managed mapping set still matches the persisted mappings, `Unmatched` when the project still has active mappings or an apply intent that no longer resolves to a current mode or current metadata-derived mappings, and `None` when the project has no active mappings and no apply intent. When color output is enabled, matched mode summaries SHALL use green emphasis, and `Unmatched` / `None` summaries SHALL use red emphasis. When a project is resolved, each listed column SHALL append one parenthesized usage label of `Full`, `Partial`, or `None`, derived from how completely each non-missing setting's current metadata-derived managed mapping set is represented in the persisted mappings. When color output is enabled, `Full`, `Partial`, and `None` SHALL use green, yellow, and red emphasis respectively. The mode list SHALL positively emphasize the mode whose current metadata-derived managed mapping set still matches the persisted active mode intent when one is resolvable.

#### Scenario: Listing without an active project and showing a matched mode
- **WHEN** the user runs `cfgfc list` without an explicit project and no convenience context is set
- **AND** one project has a persisted mode apply intent that still resolves in current warehouse metadata and still matches the persisted managed mappings
- **THEN** the CLI lists that project in the warehouse view
- **AND** the project line appends the resolved mode label in parentheses with positive emphasis when color output is enabled

#### Scenario: Listing without an active project and showing unmatched usage
- **WHEN** the user runs `cfgfc list` without an explicit project and no convenience context is set
- **AND** one project still has active mappings or an apply intent that does not resolve to a current mode or current metadata-derived mappings
- **THEN** the CLI lists that project in the warehouse view
- **AND** the project line appends `(Unmatched)` with negative emphasis when color output is enabled

#### Scenario: Listing without an active project and showing no active usage
- **WHEN** the user runs `cfgfc list` without an explicit project and no convenience context is set
- **AND** one project has no active mappings and no persisted apply intent
- **THEN** the CLI lists that project in the warehouse view
- **AND** the project line appends `(None)` with negative emphasis when color output is enabled

#### Scenario: Listing with an active project
- **WHEN** the user runs `cfgfc list` and an effective project is resolved
- **THEN** the CLI lists that project's columns and modes
- **AND** each column line appends a `Full`, `Partial`, or `None` usage label
- **AND** the active mode line uses positive emphasis when the persisted mode intent still matches current metadata-derived mappings and color output is enabled

#### Scenario: Listing includes `SettingWarehouse` at the root level
- **WHEN** the user runs `cfgfc list` in global mode and the warehouse root contains a discoverable project directory named `SettingWarehouse`
- **THEN** that project appears in the rendered project list like any other current project directory

### Requirement: List can inspect one column or mode in detail
The `list` workflow SHALL support inspecting one column's settings and one mode's contents in more detail. When listing one column, the CLI SHALL keep showing missing entries and SHALL positively emphasize every setting whose full current metadata-derived managed mapping set is present in the project's persisted managed mappings.

#### Scenario: Listing one column
- **WHEN** the user runs `cfgfc list -p <ProjectName> -c <ColumnName>`
- **THEN** the CLI shows the settings known for that column, including missing entries
- **AND** every currently enabled setting line uses positive emphasis when color output is enabled only when that setting's full current metadata-derived mapping set is present

#### Scenario: Listing one mode
- **WHEN** the user runs `cfgfc list -p <ProjectName> -m <ModeName>`
- **THEN** the CLI shows the mode's declared column selections and strategies

#### Scenario: Listing one column after a partial apply
- **WHEN** the user runs `cfgfc list -p <ProjectName> -c <ColumnName>`
- **AND** only some settings in that column, or only part of one setting's mapping set, currently participate in the persisted managed mappings
- **THEN** the CLI still lists every known setting in the column
- **AND** only the enabled subset uses positive emphasis when color output is enabled
