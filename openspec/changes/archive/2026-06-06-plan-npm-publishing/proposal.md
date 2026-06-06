## Why

ConfigFacilitator currently builds as a Go CLI, but users must clone the repository or manually obtain a binary to run `cfgfc`. Adding an npm-based installation path makes the CLI easier to install in JavaScript-heavy toolchains while preserving the existing Go implementation and pixi-managed development workflow.

## What Changes

- Add an npm distribution package that exposes `cfgfc` through npm's global `bin` mechanism.
- Add a small Node.js wrapper that forwards CLI arguments to the downloaded Go binary.
- Add an install-time downloader that selects the correct prebuilt release asset for the user's platform and architecture.
- Add release automation guidance/configuration so GitHub Releases provide the binary assets consumed by npm installation.
- Document npm installation and release verification in English and Chinese docs.
- No breaking changes to the existing Go command surface, warehouse layout, pixi tasks, or local binary build flow.

## Capabilities

### New Capabilities

- `npm-distribution`: Install and run the ConfigFacilitator CLI through npm using prebuilt platform binaries.

### Modified Capabilities

None.

## Impact

- Adds npm package metadata and Node.js helper scripts under a dedicated package directory.
- Adds release/build configuration for cross-platform `cfgfc` binaries.
- Adds documentation for `npm install -g` usage, local npm package validation, and release prerequisites.
- Introduces Node.js as a packaging-time/runtime wrapper dependency for npm users only; normal Go development and pixi workflows remain unchanged.
