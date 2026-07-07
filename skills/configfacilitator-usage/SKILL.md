---
name: configfacilitator-usage
description: Guide agents operating ConfigFacilitator warehouses and modifying config through cfgfc. Use for configuration inspection, warehouse edits, mode apply/update, and managed-state recovery.
license: MIT
compatibility: Requires cfgfc CLI and ConfigFacilitator repository docs.
metadata:
  author: ConfigFacilitator
  version: "1.0"
  generatedBy: "1.3.1"
---

Use this Skill when an agent needs to inspect or change ConfigFacilitator-managed configuration through `cfgfc`.

## When to Use

- Configuration inspection: checking roots, projects, columns, modes, settings, or managed mappings.
- Warehouse edits: adding or editing project metadata, column metadata, mode metadata, or source files.
- Mode apply/update: applying modes or columns and refreshing persisted apply intent after warehouse changes.
- Managed-state recovery: considering `reset`, `revert`, `-f`, or `--force` to restore or reclaim targets.

## First Read

- Read `README.md` for the project summary and top-level docs links.
- Read `AGENTS.md` for repository rules, validation expectations, and safety requirements.
- Read the relevant docs under `docs/` before changing state: start with `docs/README.en.md` or `docs/README.zh-CN.md`, then use `docs/commands.en.md` / `docs/commands.zh-CN.md`, `docs/example.en.md` / `docs/example.zh-CN.md`, `docs/jsonc-guide.en.md` / `docs/jsonc-guide.zh-CN.md`, and `docs/platform-notes.en.md` / `docs/platform-notes.zh-CN.md` as needed.
- For validation choices, check `docs/developer-setup.en.md` or `docs/developer-setup.zh-CN.md`.

## Command Map

Use `docs/commands.en.md` or `docs/commands.zh-CN.md` for exact syntax and examples.

| Command | Use |
| --- | --- |
| `root` | Inspect the effective warehouse root or persist a new root without moving data. |
| `switch` | Set or clear PPID-scoped convenience context for a project. |
| `list` | Inspect projects, columns, modes, settings, and current warehouse state. |
| `new` | Scaffold project, column, and mode data before manual edits. |
| `sync` | Reconcile indexes with on-disk warehouse metadata and source files. |
| `apply` | Apply a mode or single-column mapping to targets. |
| `update` | Refresh the persisted apply intent from current warehouse metadata. |
| `reset` | Clear current managed mappings after explicit user approval. |
| `revert` | Restore the previous managed snapshot after explicit user approval. |

## Safe Modification Workflow

1. Select and confirm the effective warehouse root with `cfgfc root`; only persist a different root with clear user intent.
2. Inspect current state with `cfgfc list` and relevant docs before editing metadata or source files.
3. Modify only the needed warehouse metadata or source files, following the JSONC and platform rules in `docs/`.
4. Run `cfgfc sync` after manual warehouse edits; use full-warehouse sync only when the requested scope needs it.
5. Apply or update mappings with `cfgfc apply` or `cfgfc update` according to the requested change.
6. Verify the result with `cfgfc list`, targeted file/link checks, and command output that matches the user's intent.

## Destructive Guardrails

- Require explicit user intent and current-state inspection before `-f`, `--force`, `reset`, or `revert`.
- Never use destructive recovery casually, to hide an unclear error, or before confirming the root, project, mode, and affected targets.
- Prefer non-destructive inspection first; document what will be reclaimed, cleared, or restored before running recovery commands.

## Validation

- For docs-only or Skill-only changes, run OpenSpec validation/status when appropriate: `openspec validate <change>` and `openspec status --change "<change>"`.
- Run the full Go test suite only when Go code is touched.
- If user-facing command behavior changes, run the relevant baseline commands from `AGENTS.md` and any targeted smoke tests for the changed workflow.

## Maintenance

- Future user-facing command, workflow, example, or safety-semantics changes must review `skills/configfacilitator-usage/SKILL.md` and update it when the agent workflow is affected.
