# ConfigFacilitator Documentation

ConfigFacilitator manages a portable configuration warehouse through resource-oriented CLI commands and a local embedded Web UI. JSONC remains an inspectable interoperability format, but normal workflows do not require an editor.

## Start here

- [Command Reference](commands.en.md)
- [CLI-only Workflow Example](example.en.md)
- [Architecture](architecture.en.md)
- [JSONC and Interoperability Guide](jsonc-guide.en.md)
- [Platform Notes](platform-notes.en.md)
- [Developer Setup](developer-setup.en.md)
- [Agent Usage Skill](../skills/configfacilitator-usage/SKILL.md)

## Quick facts

- Binary: `cfgfc`
- Install: `npm install -g @jollyxenon/cfgfc`
- Default warehouse root: `~/.configfacilitator/`
- Root inspection/change: `cfgfc root` and `cfgfc root <Path>`; changing roots never migrates contents
- Resources: Project, Column, Setting, Mode, Column target positions, Setting target overrides, and Mode Column selections
- Top-level commands: `project`, `column`, `setting`, `mode`, `current`, `use`, `status`, `apply`, `refresh`, `sync`, `root`, `reset`, `revert`, `web`, and `completion`
- Project scope: explicit `-p/--project` takes precedence over the PPID-scoped Project selected by `cfgfc use`
- Machine output: `--json` emits one stable success or error object; see the command reference for exit codes
- Shell completion: `cfgfc completion <bash|zsh|fish|powershell>`
- Symlinks: real symlinks only on Linux, macOS, native Windows, and WSL
- Web UI: `cfgfc web` serves an embedded offline UI on `127.0.0.1:38031`; it can create Column, Setting, and Mode resources and edit existing Column/Setting Index records in the selected Project

## Recommended model

1. Create and mutate warehouse resources with `project`, `column`, `setting`, and `mode` commands, or use the Web UI creation and Index-edit forms for Column, Setting, and Mode.
2. Configure logical target positions with `column target` and per-Setting overrides with `setting target`; the Web UI exposes the same Column defaults and Setting inheritance controls.
3. Create or change payloads with `setting create` and `setting content`; stdin preserves exact bytes.
4. Inspect metadata with resource `list`/`show`, and inspect active state with `status`.
5. Apply a Mode (the Current follows it) or set an independent Current with `apply column`, then use `refresh` only when replanning is needed.
6. Use `sync` after Git or other external changes. It immediately removes Index metadata for disappeared Project, Column, or Setting sources, does not recreate sources or cascade Mode/runtime references, and provides no prune workflow.
7. Treat `--yes`, `--cascade`, and `--force-targets` as separate authorizations.

Canonical names are index keys and filesystem identities. Display names are presentation-only, while aliases are alternative command references. Commands resolve aliases and persist canonical identities.
