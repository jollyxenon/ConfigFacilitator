# ConfigFacilitator Documentation

ConfigFacilitator is a portable Go CLI that manages configuration warehouses in `~/.configfacilitator/`.

## Start here

- [Architecture](architecture.en.md)
- [Command Reference](commands.en.md)
- [Workflow Example](example.en.md)
- [JSONC Guide](jsonc-guide.en.md)
- [Platform Notes](platform-notes.en.md)
- [Developer Setup](developer-setup.en.md)

## Quick facts

- Binary name: `cfgfc`
- npm install: `npm install -g @jollyxenon/cfgfc`
- Development build: `pixi run compile` checks all Go packages; `pixi run build` creates `dist/cfgfc`
- License: MIT License (see [`LICENSE`](../LICENSE))
- Warehouse root: `~/.configfacilitator/`
- Root-level project discovery: direct child project directories under `~/.configfacilitator/`, including `SettingWarehouse`, participate in discovery
- Core entities: `Project`, `Column`, `Setting`, `Mode`
- Commands: `new`, `sync`, `switch`, `list`, `apply`, `update`, `reset`, `revert`

## What it does

It scaffolds warehouses, reconciles indexes with filesystem reality, stores PPID-scoped convenience context, applies symlink-backed configurations, and supports `reset` and single-step `revert`.

## Installation

After maintainers publish a tagged GitHub Release and the matching npm package, install the CLI with:

```bash
npm install -g @jollyxenon/cfgfc
cfgfc --help
```

The npm package is only an installation wrapper. It downloads the prebuilt Go binary from the GitHub Release whose tag matches the npm package version, then exposes that binary through npm's `cfgfc` command.

## Identity model

- Every `Project`, `Column`, `Setting`, and `Mode` uses the top-level index key as its canonical persisted identity, stores a presentation-only `displayName`, and supports zero or more `aliases`.
- Commands resolve references through canonical names and aliases.
- `switch` stores the normalized project identifier in session context.
