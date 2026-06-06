# pixi-development-workflow Specification

## Purpose
Define the developer-facing pixi task workflow for testing, compilation checks, CLI binary builds, and help verification.

## Requirements
### Requirement: Minimal pixi task surface
The project SHALL expose only the developer pixi tasks `test`, `compile`, `build`, and `help`.

#### Scenario: Listing configured pixi tasks
- **WHEN** a contributor inspects the `[tasks]` table in `pixi.toml`
- **THEN** the table contains `test`, `compile`, `build`, and `help`
- **AND** it does not contain direct install tasks such as `usr-install`, `global-install`, or `install-global`

### Requirement: Separate compile check and binary build
The project SHALL distinguish whole-project compilation checks from producing a local CLI binary.

#### Scenario: Running compile check
- **WHEN** a contributor runs `pixi run compile`
- **THEN** pixi invokes `go build ./...` to verify all Go packages compile

#### Scenario: Building CLI artifact
- **WHEN** a contributor runs `pixi run build`
- **THEN** pixi invokes `go build -o dist/cfgfc ./cmd/cfgfc` to create the local `cfgfc` binary artifact

### Requirement: Pixi-managed development workflow documentation
The developer documentation SHALL describe pixi as the source of the Go toolchain and SHALL document the four supported tasks consistently in English and Chinese.

#### Scenario: Reading developer setup docs
- **WHEN** a contributor reads either developer setup document
- **THEN** the baseline commands include `pixi run test`, `pixi run compile`, `pixi run build`, and `pixi run help`
- **AND** the validation expectations do not instruct contributors to use pixi for global installation
