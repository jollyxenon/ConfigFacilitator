## MODIFIED Requirements

### Requirement: Documentation Parity
Documentation updates MUST keep the English and Chinese user-facing docs in sync for the same behavioral or workflow change.

#### Scenario: Workflow documentation changes
- **WHEN** a developer workflow changes
- **THEN** the matching English and Chinese documentation are both updated in the same change
- **AND** both versions describe the same supported commands and expectations

#### Scenario: Obsolete workflow removal
- **WHEN** a task or command is removed from the supported workflow
- **THEN** documentation no longer recommends that removed task or command as a baseline path
