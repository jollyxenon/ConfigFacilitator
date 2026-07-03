# config-root-warehouse-layout Specification

## Purpose
Define the canonical warehouse root and root-level metadata contract for ConfigFacilitator.

## Requirements
### Requirement: User-config-root warehouse path
The system SHALL resolve an effective warehouse root before any warehouse-scoped command reads project data. It SHALL first consult a user-scoped bootstrap location outside the warehouse for a persisted root override and SHALL use that override when present. When no override is present, the system SHALL fall back to the current user's `.configfacilitator/` directory. On native Windows the default fallback root SHALL be `%USERPROFILE%/.configfacilitator`; on Unix-like platforms the default fallback root SHALL be `~/.configfacilitator/`.

#### Scenario: Resolving a configured warehouse root
- **WHEN** a user has already configured an alternate warehouse root
- **THEN** the CLI uses that configured path before scanning project directories or root metadata

#### Scenario: Resolving the default warehouse root
- **WHEN** the CLI derives its warehouse root and no persisted override exists
- **THEN** it resolves the root as that home directory joined with `.configfacilitator/`

#### Scenario: Resolving the default warehouse root on native Windows
- **WHEN** native Windows `cfgfc` derives its warehouse root and no persisted override exists
- **THEN** it resolves the root as `%USERPROFILE%/.configfacilitator`

### Requirement: Root-level metadata placement
The system SHALL store warehouse metadata files directly under the effective warehouse root, including `ProjectIndex.jsonc` at the warehouse root and `.cfgfc-session/` as the reserved internal session directory. The persisted root-override bootstrap state SHALL remain outside the effective warehouse root so the CLI can resolve the warehouse before scanning its contents.

#### Scenario: Reading warehouse metadata from a configured root
- **WHEN** a command loads warehouse index or session context after the user configured an alternate root
- **THEN** it reads `ProjectIndex.jsonc` and `.cfgfc-session/` from that configured root
- **AND** it does not require bootstrap state to live inside the warehouse directory being selected

### Requirement: No legacy warehouse fallback
The system MUST NOT read from, write to, or auto-migrate `~/.configfacilitator/SettingWarehouse/` when resolving the effective warehouse root, and changing the configured warehouse root MUST NOT copy or move warehouse contents between the previous and next root paths.

#### Scenario: Legacy directory still exists
- **WHEN** `~/.configfacilitator/SettingWarehouse/` exists on disk during command execution
- **THEN** the CLI still treats the effective warehouse root as the only canonical warehouse root and performs no compatibility fallback or automatic migration

#### Scenario: Switching the configured root leaves previous data untouched
- **WHEN** a user changes the configured warehouse root from one path to another
- **THEN** the CLI updates only the persisted root selection
- **AND** it leaves the previously selected warehouse contents in place
