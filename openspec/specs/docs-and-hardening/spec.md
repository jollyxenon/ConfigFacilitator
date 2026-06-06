# docs-and-hardening Specification

## Purpose
TBD - created by archiving change docs-packaging-hardening. Update Purpose after archive.
## Requirements
### Requirement: Repository docs describe the implemented CLI in both Chinese and English
The repository SHALL provide top-level user-facing documentation that describes the implemented CLI behavior in both Chinese and English, the command-reference documentation SHALL stay aligned with the runtime help surface for root and subcommand usage, caveats, and representative examples, the published docs SHALL include one scenario-driven example guide in both languages that links from the docs index, and the published docs SHALL describe the current contract directly and consistently.

#### Scenario: Reading the root README
- **WHEN** a user opens the repository README
- **THEN** they can understand the warehouse model, command families, and representative workflow in both Chinese and English
- **AND** the README describes the current Mode strategy names and meanings instead of retired terminology

#### Scenario: Comparing docs with command help
- **WHEN** a user compares `cfgfc --help` or `cfgfc <command> --help` with the command-reference docs
- **THEN** the documented usage forms, behavioral notes, and representative examples do not materially conflict

#### Scenario: Browsing the docs index for a practical walkthrough
- **WHEN** a user opens `docs/README.en.md` or `docs/README.zh-CN.md`
- **THEN** they can discover a linked scenario-driven example page in the same language as the rest of the documentation

#### Scenario: Reading the example guide
- **WHEN** a user opens the published example page in either language
- **THEN** the guide explains one realistic end-to-end workflow that combines warehouse structure, manual JSONC editing, and the implemented CLI commands
- **AND** the workflow guidance uses the current Mode strategy names and semantics consistently

#### Scenario: Reading current-contract guidance
- **WHEN** a user reads the published JSONC or workflow guidance
- **THEN** the docs describe the current canonical identity contract directly and consistently
- **AND** the docs explain the breaking rename from the retired Mode strategy terms only as a transition note when needed, without documenting runtime compatibility

### Requirement: Final validation evidence is rerun against the completed CLI
The project SHALL rerun the final validation workflow once the command surface is complete, including verification that root help and command-specific help both render successfully.

#### Scenario: Running the final verification commands
- **WHEN** the final hardening slice completes
- **THEN** the repository has passing evidence for the standard test, build, help, command-help, and representative end-to-end workflow checks

### Requirement: Documentation Parity
Documentation updates MUST keep the English and Chinese user-facing docs in sync for the same behavioral or workflow change.

#### Scenario: Workflow documentation changes
- **WHEN** a developer workflow changes
- **THEN** the matching English and Chinese documentation are both updated in the same change
- **AND** both versions describe the same supported commands and expectations

#### Scenario: Obsolete workflow removal
- **WHEN** a task or command is removed from the supported workflow
- **THEN** documentation no longer recommends that removed task or command as a baseline path
