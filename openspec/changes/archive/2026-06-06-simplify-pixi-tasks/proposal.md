## Why

The current pixi task surface mixes validation, build, and direct installation commands, including tasks that assume system-level Go or Unix-specific install paths. The project should reflect its intended workflow: contributors use pixi-managed Go, while end users will install released binaries through package managers such as Scoop or npm.

## What Changes

- Keep only four pixi tasks: `test`, `compile`, `build`, and `help`.
- Change `build` to produce the distributable `cfgfc` CLI binary instead of only checking compilation.
- Add `compile` as the explicit whole-project compilation check.
- Remove direct install tasks such as `usr-install` and `global-install` from the public pixi task surface.
- Update English and Chinese developer documentation to describe the new task semantics and avoid recommending pixi-based global installation.

## Capabilities

### New Capabilities
- `pixi-development-workflow`: Defines the developer-facing pixi task workflow for testing, compilation checks, CLI binary builds, and help verification.

### Modified Capabilities
- `docs-and-hardening`: Update developer documentation expectations for the revised pixi task surface.

## Impact

- Affected files: `pixi.toml`, `docs/developer-setup.en.md`, `docs/developer-setup.zh-CN.md`, and project agent/developer notes if they reference the old baseline task names.
- User-facing CLI behavior is unchanged.
- Developer workflow changes by removing pixi tasks that installed binaries directly into user or system paths.
