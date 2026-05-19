# ConfigFacilitator Documentation

ConfigFacilitator is a portable Go CLI that manages configuration warehouses in `~/.configfacilitator/SettingWarehouse/`.

## Start here

- [Architecture](architecture.en.md)
- [Command Reference](commands.en.md)
- [JSONC Guide](jsonc-guide.en.md)
- [Platform Notes](platform-notes.en.md)
- [Developer Setup](developer-setup.en.md)

## Quick facts

- Binary name: `cfgfc`
- Warehouse root: `~/.configfacilitator/SettingWarehouse/`
- Core entities: `Project`, `Column`, `Setting`, `Mode`
- Commands: `new`, `sync`, `switch`, `list`, `apply`, `reset`, `revert`

## What it does

It scaffolds warehouses, reconciles indexes with filesystem reality, stores PPID-scoped convenience context, applies symlink-backed configurations, and supports `reset` and single-step `revert`.
