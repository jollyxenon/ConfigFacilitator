## MODIFIED Requirements

### Requirement: Documentation Parity
Documentation updates MUST keep the English and Chinese user-facing docs in sync for the same behavioral, workflow, or repository metadata change.

#### Scenario: Workflow documentation changes
- **WHEN** a developer workflow changes
- **THEN** the matching English and Chinese documentation are both updated in the same change
- **AND** both versions describe the same supported commands and expectations

#### Scenario: Obsolete workflow removal
- **WHEN** a task or command is removed from the supported workflow
- **THEN** documentation no longer recommends that removed task or command as a baseline path

#### Scenario: License documentation changes
- **WHEN** repository license information changes
- **THEN** matching English and Chinese user-facing documentation are both updated in the same change
- **AND** both versions name the same license and direct users to the same authoritative license file
