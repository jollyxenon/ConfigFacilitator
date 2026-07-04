# Workflow Example

This page shows one realistic OpenCode-style workflow: you keep multiple model configs and skill directories in one warehouse, apply a single setting for quick testing, then switch back to a full mode when you want the complete environment. For the exact command surface, see [Command Reference](commands.en.md). For field-level JSONC rules, see [JSONC Guide](jsonc-guide.en.md).

## Goal

In this example, `OpenCode` stores two Columns:

- `oh-my-openagent` for model config files such as `OMOMax.json` and `OMOLight.json`
- `Skills` for skill directories such as `Skill-A` and `Skill-B`

The workflow solves two common needs:

- quickly apply one config file for temporary testing
- restore a full working setup with one named Mode

## Warehouse layout

This example uses the default warehouse root `~/.configfacilitator/`. If you previously ran `cfgfc root <path>`, substitute that effective root instead. After scaffolding and manual edits, the representative layout looks like this:

Project directories are discovered directly under the effective warehouse root. A root-level directory named `SettingWarehouse` participates the same way as any other project directory.

```text
~/.configfacilitator/
├── ProjectIndex.jsonc
└── OpenCode/
    ├── Column/
    │   ├── ColumnIndex.jsonc
    │   ├── oh-my-openagent/
    │   │   ├── SettingIndex.jsonc
    │   │   ├── OMOMax.json
    │   │   └── OMOLight.json
    │   └── Skills/
    │       ├── SettingIndex.jsonc
    │       ├── Skill-A/
    │       └── Skill-B/
    ├── Mode/
    │   └── ModeIndex.jsonc
    └── Backup/
        ├── current_state.json
        └── history.log
```

`Backup/current_state.json` and `Backup/history.log` are state files used by `apply`, `reset`, and `revert`. Treat them as runtime data, not as files you edit by hand.

## 1. Scaffold the project, columns, and mode

Start by generating the project skeleton and the templates you will fill in later:

```bash
cfgfc new -p OpenCode
cfgfc new -p OpenCode -c oh-my-openagent
cfgfc new -p OpenCode -c Skills
cfgfc new -p OpenCode -m Max
```

`new` creates templates for Projects, Columns, and Modes. It does not create individual Settings for you. Put the real setting files or directories into each Column yourself.

## 2. Add the real files and edit JSONC manually

After scaffolding, copy the real payloads into the warehouse:

- put `OMOMax.json` and `OMOLight.json` into `OpenCode/Column/oh-my-openagent/`
- put `Skill-A/` and `Skill-B/` into `OpenCode/Column/Skills/`

Then edit the JSONC indexes. These manual edits are part of the intended workflow today.

### Project and column metadata

Use `description` for durable notes, `displayName` for presentation, and `aliases` for extra CLI references. `displayName` is not an implicit CLI alias, and the top-level key is the canonical identity.

```jsonc
// ProjectIndex.jsonc
{
  "OpenCode": {
    "displayName": "OpenCode",
    "aliases": ["oc"],
    "description": "OpenCode workspace config set"
  }
}
```

```jsonc
// OpenCode/Column/ColumnIndex.jsonc
{
  "oh-my-openagent": {
    "displayName": "Main Config",
    "aliases": ["omo"],
    "description": "Primary model config file"
  },
  "Skills": {
    "displayName": "Skills",
    "aliases": ["ski"],
    "description": "Skill directories"
  }
}
```

The top-level key is the canonical identity. `displayName` and `aliases` stay available as authored metadata around that key.

### Setting targets

Targets are split into directory and name arrays. `defaultTargetDir` / `defaultTargetName` define Column-level defaults, while `targetDir` / `targetName` can override them per Setting by matching index. Empty setting entries inherit defaults; an empty default target name falls back to the Setting warehouse name.

```jsonc
// OpenCode/Column/oh-my-openagent/SettingIndex.jsonc
{
  "description": "Main config settings",
  "defaultTargetDir": ["~/.config/opencode"],
  "defaultTargetName": ["oh-my-openagent.jsonc"],
  "settings": {
    "OMOMax.json": {
      "displayName": "OMOMax Config",
      "aliases": ["max"],
      "description": "OMOMax model config"
    },
    "OMOLight.json": {
      "displayName": "OMOLight Config",
      "aliases": ["light"],
      "description": "OMOLight model config"
    }
  }
}
```

```jsonc
// OpenCode/Column/Skills/SettingIndex.jsonc
{
  "description": "Skills column",
  "defaultTargetDir": ["~/.config/opencode/skills"],
  "defaultTargetName": [""],
  "settings": {
    "Skill-A": {
      "displayName": "Skill A",
      "aliases": ["a"],
      "description": "First skill directory",
      "targetDir": [""],
      "targetName": ["Skill-A"]
    },
    "Skill-B": {
      "displayName": "Skill B",
      "aliases": ["b"],
      "description": "Second skill directory",
      "targetDir": [""],
      "targetName": ["Skill-B"]
    }
  }
}
```

Target directories can use `~`, `${VAR}`, and Windows `%VAR%` forms. Target names must be normal single-component file or directory names. After expansion, every target path in the planned state must be unique.

### Mode selections

Mode membership is also edited by hand in `ModeIndex.jsonc`. Use `settings` to pick the included Settings for each Column when the strategy is `cover` or `increment`, and use `strategy` to control the column behavior.

```jsonc
// OpenCode/Mode/ModeIndex.jsonc
{
  "Max": {
    "displayName": "Max",
    "aliases": ["m"],
    "description": "Full OpenCode workspace",
    "columns": {
      "oh-my-openagent": {
        "settings": ["OMOMax.json"],
        "strategy": "cover"
      },
      "Skills": {
        "settings": ["Skill-A", "Skill-B"],
        "strategy": "increment"
      }
    }
  }
}
```

`cover` applies only the authored `settings` for that Column. `increment` keeps existing managed links and adds new ones. `none` links nothing for that Column. `full` links every known Setting in the Column. Only `none` and `full` may omit `settings`.

## 3. Sync the warehouse after manual edits

Once the files and JSONC indexes are ready, reconcile the warehouse with filesystem reality:

```bash
cfgfc sync -p OpenCode
```

Use `cfgfc sync -p <project>` when you want to target one project explicitly. Without an active project, plain `cfgfc sync` falls back to the whole warehouse. `cfgfc sync --all` and `cfgfc sync -a` always force a full-warehouse sync and ignore any active project context.

## 4. Switch context and inspect the result

Bind the current terminal session to `OpenCode` before using project-scoped commands without `-p`:

```bash
cfgfc switch OpenCode
cfgfc list
cfgfc list -c Skills
cfgfc list -m Max
```

Before switching, the same warehouse-wide view would append one usage summary per project:

```text
OpenCode (None)
```

After `cfgfc switch OpenCode`, plain `cfgfc list` becomes project-scoped and appends one usage label per Column:

```text
Project: OpenCode
Columns:
  - Main Config [oh-my-openagent] (None)
  - Skills (None)
Modes:
  - Max
```

`cfgfc list -c Skills` still shows every known Setting in the Column. Missing entries remain visible when they exist:

```text
Column: Skills
  - Skill A [Skill-A] (present)
  - Skill B [Skill-B] (present)
```

`cfgfc list -m Max` continues to show the Mode's declared strategies and Settings:

```text
Mode: Max
  - Main Config [oh-my-openagent]: strategy=cover settings=OMOMax Config [OMOMax.json]
  - Skills: strategy=increment settings=Skill A [Skill-A],Skill B [Skill-B]
```

After `cfgfc switch OpenCode`, later project-scoped `new`, `sync`, `list`, `apply`, `reset`, and `revert` commands can omit `-p`. `cfgfc switch global` clears that PPID-scoped convenience context and returns later commands to global resolution.

## 5. Apply one setting or a full mode

For quick testing, apply a single Setting from one Column:

```bash
cfgfc apply -c oh-my-openagent -s OMOLight.json
```

That form is useful when you only want one config file linked to `~/.config/opencode/oh-my-openagent.jsonc`.

When you want the full workspace again, apply the named Mode:

```bash
cfgfc apply -m Max
```

In this example, `apply -m Max` replaces the `oh-my-openagent` link with `OMOMax.json` because that Column uses `cover`, then adds the `Skill-A` and `Skill-B` directory links because `Skills` uses `increment`.

After that mode apply, `cfgfc list` shows both Columns as fully covered, the terminal highlights the active `Max` mode, and `cfgfc list -c Skills` highlights the enabled settings. If you clear the switched context, the global view shows the matched persisted mode name in parentheses:

```text
OpenCode (Max)
```

If a project still has active mappings but they no longer match any current Mode, that same global summary becomes `Unmatched`.

## 6. Recover with revert or reset

Use `revert` when you want to restore the previous apply state, and use `reset` when you want to remove the current project's managed links entirely. Add `-f` / `--force` when you intentionally want `cfgfc` to reclaim occupied files or directories recursively instead of stopping on unmanaged targets or drift.

```bash
cfgfc revert
cfgfc reset
```

`revert` is single-step only: it restores the previous apply snapshot, not an arbitrary point in history. `reset` removes the currently managed mappings and clears the current state for that project. Forced `apply`, `update`, `reset`, or `revert` only restore the last confirmed managed state—they do not reconstruct overwritten unmanaged file or directory contents.

## When to read the reference docs

Use this page as a representative workflow, not as the only supported shape. For field-level identity rules, follow the top-level key, `displayName`, and `aliases` model described in the JSONC guide.

- Read [Command Reference](commands.en.md) when you need the full help-aligned command forms.
- Read [JSONC Guide](jsonc-guide.en.md) when you need the exact identity and target-field rules.
