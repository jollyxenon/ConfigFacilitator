# ConfigFacilitator Master Plan

## Context

- Repository state is greenfield: no implementation code, no README, no tests, empty `AGENTS.md`.
- OpenSpec workflow is installed and usable: `openspec` CLI works, `opsx-explore / propose / apply / sync / archive` command docs exist.
- `openspec/config.yaml` is minimal and `openspec list --json` currently reports no active changes.
- Product requirements are defined in `ConfigFacilitatorDemanding.md`.

## Recommended Runtime

Use **Go** for v1.

### Why Go

- Portable single-binary CLI aligns with the requirement that `cfgfc` lives beside `SettingWarehouse/`.
- Standard library plus mature ecosystem are sufficient for symlink handling, filesystem operations, and process inspection.
- Cross-platform release and CI are straightforward for a repo that currently has no scaffolding.
- The product is dominated by I/O correctness and safety, not performance-sensitive compute.

### Suggested Stack

- Go
- Cobra for CLI command structure
- gopsutil for PPID/process inspection where needed
- A JSONC-capable parser/normalizer library plus project-owned serialization rules
- Go `testing` for unit/integration tests
- Goreleaser for packaging

## Architecture Direction

Organize the implementation into small internal packages:

1. `cmd/` for CLI wiring only
2. `internal/warehouse/` for executable-relative warehouse resolution
3. `internal/model/` for project/column/setting/mode/state/history models
4. `internal/jsonc/` for template generation, parsing, normalization, and comment stripping
5. `internal/index/` for reading/writing/validating index files
6. `internal/pathvars/` for path variable expansion and normalization
7. `internal/session/` for PPID-bound project context handling
8. `internal/linker/` for symlink conflict detection and link lifecycle
9. `internal/syncer/` for filesystem scan and index reconciliation
10. `internal/history/` for `current_state.json` and historical restore data
11. `internal/render/` for human-readable output

## Phased Plan

### Phase 0 — Planning Foundation

Goal: convert the requirement document into stable, reviewable OpenSpec artifacts and execution slices.

Deliverables:

- OpenSpec capability specs
- project context for OpenSpec
- architecture decisions for runtime and portability
- approved change slicing for downstream work

### Phase 1 — Project Skeleton And Developer Workflow

Goal: create the minimum runnable repo shape.

Deliverables:

- Go module
- `cfgfc` entrypoint
- test layout
- developer task conventions
- initial CI outline
- README/documentation skeleton

### Phase 2 — Warehouse, Models, And JSONC Foundations

Goal: lock down the filesystem and data contracts before risky behavior.

Deliverables:

- executable-relative `SettingWarehouse` discovery
- domain models
- index parsing/validation
- JSONC template generation and normalization rules

### Phase 3 — Session Context And Path Resolution

Goal: make project resolution deterministic before command breadth expands.

Deliverables:

- PPID-bound context handling
- `-p` omission rules
- cross-platform path expansion and normalization

### Phase 4 — Symlink Engine, Safety Layer, And State Tracking

Goal: implement the highest-risk core behind strong tests.

Deliverables:

- conflict detection
- owned-link detection rules
- link creation/removal engine
- `current_state.json`
- historical state tracking for revert

### Phase 5 — Lower-Risk Command Surface

Goal: deliver skeleton-building and inspection commands first.

Deliverables:

- `new -p`, `new -c`, `new -m`
- `sync`
- `list`
- `switch`

### Phase 6 — Apply/Reset/Revert Command Surface

Goal: complete the behavior-critical CLI.

Deliverables:

- mode apply with `full` and `incremental`
- single-column apply
- `reset`
- `revert`

### Phase 7 — Hardening, Docs, Packaging, And UX

Goal: make v1 reviewable and releasable.

Deliverables:

- bilingual docs and README
- packaging/release automation
- platform caveat docs
- acceptance matrix and end-to-end examples

## Proposed OpenSpec Change Slices

1. `bootstrap-main-specs`
2. `init-go-cli-skeleton`
3. `jsonc-and-index-contracts`
4. `warehouse-layout-and-models`
5. `session-context-and-pathvars`
6. `symlink-state-engine`
7. `scaffold-commands`
8. `inspect-and-switch-commands`
9. `apply-reset-revert-commands`
10. `docs-packaging-hardening`

## Spec Traceability Notes

The requirement document contains several behaviors that cut across multiple implementation slices and must be traced explicitly in OpenSpec artifacts:

- `"description"` persistence is not just a model concern; it also constrains `sync`, template generation, and JSONC normalization.
- `sync` orphan preservation is both a data-model concern and a command-behavior concern.
- `Mode` semantics for undeclared columns clearing by default must be reflected in both the mode model and the apply engine acceptance criteria.
- Conflict handling is not only an engine concern; it also defines CLI interaction behavior when unmanaged targets already exist.
- Template comment generation and template comment stripping are separate contracts and should not be left implicit.

## Semantics That Must Be Nailed Down In Specs Before Implementation

1. What exactly qualifies as a “system-managed” target versus an unmanaged real file or directory.
2. How orphaned or missing nodes are represented in index data and how commands should display them.
3. Exact `full` versus `incremental` behavior when targets overlap or conflict.
4. What “clear/unlink” means for undeclared columns in a mode apply.
5. Directory-setting target semantics: whether targets represent a final directory, a parent directory, or another mounting rule.
6. The supported path variable set on Windows and Unix-like systems.
7. PPID binding lifetime, scope, and cleanup behavior.
8. Which JSONC comments may be stripped, and whether unknown user-authored fields must be preserved.
9. The structure of historical records needed to make `revert` reliable.

## Per-Slice QA And Archive Gates

### 1. `bootstrap-main-specs`

Verification:

- Confirm `openspec list --json` shows the active change after proposal.
- Confirm the change contains proposal, design, tasks, and any required spec artifacts.
- Read the produced artifacts and verify each high-risk topic is explicitly covered:
  - Windows symlink policy
  - history/revert model
  - PPID context guarantees
  - JSONC write-back contract
  - directory-setting target semantics

Archive gate:

- No unresolved TBDs in the artifacts.
- Every identified high-risk semantic has an explicit policy decision or a clearly scoped follow-up task.

### 2. `init-go-cli-skeleton`

Verification:

- Run `go test ./...` successfully.
- Run `go build ./...` successfully.
- Run the built `cfgfc --help` successfully.
- Confirm basic repository files exist for the chosen workflow: module file, command entrypoint, test entrypoint, and developer instructions.

Archive gate:

- Build and test are green.
- CLI binary can start and print help text.

### 3. `jsonc-and-index-contracts`

Verification:

- Run unit tests for JSONC parsing, index parsing, and serialization contracts.
- Run golden tests that generate and normalize JSONC templates.
- Confirm normalization preserves `"description"`, preserves required unknown fields if the spec says so, and removes only the intended temporary comments according to the design contract.

Archive gate:

- All template and parsing tests pass.
- Sample index fixtures round-trip without losing required metadata.

### 4. `warehouse-layout-and-models`

Verification:

- Run unit tests for executable-relative warehouse discovery.
- In a temp workspace, create a sample warehouse layout and confirm all index types and directory conventions are recognized correctly.
- Validate project/column/setting/mode data-model rules, including target inheritance and missing-node representation.

Archive gate:

- Warehouse discovery and domain-model tests pass.
- Sample warehouse fixtures match the documented structure exactly.

### 5. `session-context-and-pathvars`

Verification:

- Run unit tests for `-p` omission logic and path expansion.
- Run integration tests simulating at least two independent session contexts.
- Validate `~`, `${HOME}`, and the chosen Windows variable forms against expected normalized outputs.

Archive gate:

- Session context tests prove no cross-session leakage in the supported environments.
- Path expansion tests pass on all supported OS targets.

### 6. `symlink-state-engine`

Verification:

- Run integration tests in temp directories for:
  - unmanaged target conflict detection
  - owned-link replacement
  - partial-failure handling
  - current state persistence
  - historical snapshot creation
- Validate file and directory target behavior separately.
- On Windows-supported environments, validate the chosen symlink/junction policy explicitly.

Archive gate:

- The engine never silently overwrites unmanaged targets in tests.
- State files and filesystem outcomes match after success and after injected failure cases.

### 7. `scaffold-commands`

Verification:

- Run end-to-end command tests for:
  - `cfgfc new -p <ProjectName>`
  - `cfgfc new -c <ColumnName>`
  - `cfgfc new -m <ModeName>`
  - `cfgfc sync`
- Confirm generated directory structure matches the spec.
- Confirm `sync` preserves orphaned/missing entries instead of deleting them.
- Confirm generated templates include the intended JSONC guidance comments.

Archive gate:

- The documented example scaffold can be created and inspected successfully.
- `sync` behavior matches the orphan-preservation rule.

### 8. `inspect-and-switch-commands`

Verification:

- Run end-to-end command tests for:
  - `cfgfc switch <ProjectName>`
  - `cfgfc list`
  - `cfgfc list -p <ProjectName> -c <ColumnName>`
  - `cfgfc list -m <ModeName>`
- Confirm no-context and in-context list behavior matches the requirement.
- Confirm project context resolution works when `-p` is omitted after `switch`.

Archive gate:

- Context-sensitive listing behavior is stable and documented.
- Switch-driven project resolution passes its integration scenarios.

### 9. `apply-reset-revert-commands`

Verification:

- Build a temp fixture modeled on the requirement example.
- Run:
  - `cfgfc apply -c <ColumnName> -s <SettingName>`
  - `cfgfc apply -m <ModeName>`
  - `cfgfc reset -p <ProjectName>`
  - `cfgfc revert -p <ProjectName>`
- After each step, assert exact filesystem and state-file outcomes.
- Validate full vs incremental mode behavior, plus undeclared-column clearing.
- Validate revert restores the previous snapshot rather than merely clearing links.

Archive gate:

- The requirement’s OpenCode-style scenario passes end to end.
- Reset and revert both restore the expected postcondition exactly.

### 10. `docs-packaging-hardening`

Verification:

- Run the full automated test suite.
- Run a release-dry-run build for the supported targets.
- Verify `README.md`, `README.en.md`, and `README.zh.md` exist and match the implemented command set.
- Validate that at least one end-to-end example in the docs has been exercised against the built CLI.

Archive gate:

- Test suite and packaging dry run are green.
- Docs are bilingual and consistent with actual CLI behavior.

## Required OpenSpec Workflow Per Slice

For each slice, follow:

1. `/opsx-explore <change>`
2. `/opsx-propose <change>`
3. review proposal/design/tasks
4. `/opsx-apply <change>`
5. `/opsx-sync <change>`
6. `/opsx-archive <change>`

## Testing Strategy

### Unit Tests

- path expansion
- index validation
- mode resolution logic
- session resolution rules
- conflict classification

### Golden Tests

- generated JSONC templates
- normalized index serialization
- stable human-readable list output where appropriate

### Integration Tests

- project/column/mode scaffold creation
- sync preserving missing/orphaned nodes
- apply creating links correctly
- reset removing owned links only
- revert restoring prior state

### Cross-Platform Test Matrix

- Linux
- Windows

Focus areas:

- symlink or junction behavior
- path variable expansion
- file vs directory targets
- PPID context behavior

## Documentation Outputs

- `README.md`
- `README.en.md`
- `README.zh.md`
- architecture overview
- command reference
- JSONC authoring guide
- troubleshooting and platform caveats
- developer setup and test guide

## High-Risk Items Requiring Early Design Resolution

1. Windows symlink policy and whether junction fallback is acceptable
2. Revert history model and where historical snapshots live
3. PPID-bound context reliability across terminals and shells
4. JSONC write-back contract: comments, unknown fields, formatting, `description` preservation
5. Directory-setting target semantics
6. Ownership rules for deciding which existing targets are safe to remove or replace

## Immediate Next Step

Do not begin implementation yet.

First:

1. review this plan critically
2. convert the requirement doc into OpenSpec capability specs
3. lock down the unresolved high-risk semantics
4. only then start the first implementation-oriented OpenSpec change
