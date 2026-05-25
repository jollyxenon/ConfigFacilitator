# Command Reference

## `new`

Scaffold project, column, and mode templates.

Accepted forms: `cfgfc new -p <project>`, `cfgfc new -p <project> -c <column>`, and `cfgfc new -p <project> -m <mode>`. After `cfgfc switch <project>`, the column and mode forms can omit `-p`. Existing projects can be referenced by warehouse-side identifier or alias. In the `new` command, `global` is reserved and cannot be used as a project name.

```bash
cfgfc new -p OpenCode
cfgfc new -p OpenCode -c Skills
cfgfc new -p OpenCode -m Max
cfgfc switch OpenCode
cfgfc new -c Skills
cfgfc new -m Max
```

## `sync`

Reconcile warehouse indexes with filesystem state. When a switched project is active, `cfgfc sync` targets that project. Without an active project, `cfgfc sync` reconciles every project under the warehouse root. `cfgfc sync --all` and `cfgfc sync -a` force a full-warehouse sync and ignore any active project context. Project references accept warehouse-side identifiers and aliases. Within `sync -p`, `global` is reserved and cannot be used as a project target.

```bash
cfgfc sync
cfgfc sync -p OpenCode
cfgfc sync --all
cfgfc sync -a
```

## `switch`

Select the active project context for this session. `cfgfc switch <project>` resolves the project by warehouse-side identifier or alias, then stores the normalized project identifier for the current PPID-scoped session. `cfgfc switch global` clears the active project context and returns later commands to global resolution.

```bash
cfgfc switch OpenCode
cfgfc switch global
```

## `list`

Inspect projects, columns, modes, and settings. Without an effective project, `list` shows the available projects in the warehouse. After `cfgfc switch <project>`, project-scoped list forms can omit `-p`. Project, column, and mode references accept warehouse-side identifiers and aliases. `list` accepts only one detailed target at a time: `-c` or `-m`.

```bash
cfgfc list
cfgfc list -p OpenCode -c Skills
cfgfc list -p OpenCode -m Max
```

## `apply`

Activate a mode or explicit settings selection. `apply` accepts either mode apply (`-m`) or single-column apply (`-c` with `-s`). After `cfgfc switch <project>`, project-scoped apply forms can omit `-p`. Project, column, mode, and setting references accept warehouse-side identifiers and aliases. `-s` accepts one or more comma-separated setting names.

```bash
cfgfc apply -p OpenCode -m Max
cfgfc apply -p OpenCode -c opencode.json -s GPT.json
```

## `reset`

Remove the current project's managed links. After `cfgfc switch <project>`, `reset` can omit `-p` and use the active project context.

```bash
cfgfc reset -p OpenCode
```

## `revert`

Restore the previous apply state for a project. After `cfgfc switch <project>`, `revert` can omit `-p` and use the active project context.

```bash
cfgfc revert -p OpenCode
```

## Notes

- After `cfgfc switch`, project-scoped `new`, `sync`, `list`, `apply`, `reset`, and `revert` commands can omit `-p`.
- `switch` persists the normalized project identifier even when the user typed an alias.
- After `cfgfc switch global`, the current PPID-scoped project context is cleared; `list` returns to the global project list and `sync` returns to its default full-warehouse fallback.
- Mode strategies are `cover`, `increment`, `none`, and `full`; only `none` and `full` may omit `settings` in `ModeIndex.jsonc`.
