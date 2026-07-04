# Command Reference

Development commands use the pixi-managed Go toolchain: `pixi run compile` checks all Go packages, and `pixi run build` creates the local CLI binary at `dist/cfgfc`.

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

## `root`

Inspect or change the persistent warehouse root. `cfgfc root` prints the current effective root. `cfgfc root <path>` expands `~` and supported environment variables, persists the resulting absolute path in a user-scoped bootstrap file, and makes later warehouse-scoped commands use that root. Changing roots does not migrate, copy, or initialize warehouse contents.

```bash
cfgfc root
cfgfc root ~/.configfacilitator-alt
```

## `list`

Inspect projects, columns, modes, and settings. Without an effective project, `list` shows the available projects in the warehouse and appends one parenthesized usage summary for each project: the resolved persisted mode name when a mode intent still matches, otherwise `Unmatched` or `None`. With an effective project, plain `list` shows that project's columns and modes, and appends one parenthesized `Full`, `Partial`, or `None` label to each column according to the persisted managed mappings. After `cfgfc switch <project>`, project-scoped list forms can omit `-p`. Project, column, and mode references accept warehouse-side identifiers and aliases. `list` accepts only one detailed target at a time: `-c` or `-m`.

When color output is available, terminal rendering highlights the active mode in project-scoped `list` views and the enabled settings in `list -c`. `list -c` still shows missing entries.

```bash
cfgfc list
cfgfc list -p OpenCode
cfgfc list -p OpenCode -c Skills
cfgfc list -p OpenCode -m Max
```

## `apply`

Activate a mode or explicit settings selection. `apply` accepts either mode apply (`-m`) or single-column apply (`-c` with `-s`). After `cfgfc switch <project>`, project-scoped apply forms can omit `-p`. Project, column, mode, and setting references accept warehouse-side identifiers and aliases. `-s` accepts one or more comma-separated setting names. Activation creates hard-link-backed regular files only: directory-backed mappings are unsupported, there is no fallback to symlinks, junctions, copies, `mklink`, PowerShell, or other substitutes, and cross-filesystem or cross-volume targets fail clearly. `-f` / `--force` deletes occupied target files, symlinks, or directories recursively so the requested managed state can be re-applied even when targets are unmanaged or drifted.

```bash
cfgfc apply -p OpenCode -m Max
cfgfc apply -p OpenCode -c opencode.json -s GPT.json
```

## `update`

Refresh the last applied intent from the current warehouse metadata. `update` reads the project's `Backup/current_state.json`; when the state records a mode or direct-column apply intent, `update` replans that intent against current indexes and commits the refreshed mapping set. This lets a mode column using `full` include newly synced settings after the original `apply`.

Use `update` after changing metadata for already active configuration, for example after adding a new skill directory or retargeting a setting following an earlier `apply`. Run `cfgfc sync` first when newly added files or directories need to be reflected in indexes, then run `cfgfc update` to refresh the active state. When no apply intent is recorded, `update` matches active sources back to current metadata and refreshes those mappings only.

After `cfgfc switch <project>`, project-scoped update forms can omit `-p`. Project and column references accept warehouse-side identifiers and aliases. `cfgfc update --all` and `cfgfc update -a` ignore any switched context, enumerate all projects, and skip projects with no active mappings or intent. `cfgfc update -c <column>` or `cfgfc update --column <column>` refreshes only the selected column while preserving other current mappings; with mode intent, the selected column is replanned from the mode strategy, so `full` can include newly synced settings. `-f` / `--force` reclaims occupied target paths recursively and continues even when current targets are unmanaged or no longer match recorded ownership.

```bash
cfgfc update -p OpenCode
cfgfc switch OpenCode
cfgfc update
cfgfc update -c Skills
cfgfc update --all
cfgfc update -a
```

## `reset`

Remove the current project's managed paths. After `cfgfc switch <project>`, `reset` can omit `-p` and use the active project context. `cfgfc reset -f` / `cfgfc reset --force` deletes every target path recorded in the current project state, even when the path has drifted away from the recorded source.

```bash
cfgfc reset -p OpenCode
```

## `revert`

Restore the previous apply state for a project. After `cfgfc switch <project>`, `revert` can omit `-p` and use the active project context. `cfgfc revert -f` / `cfgfc revert --force` reclaims occupied target paths recursively so the previous managed snapshot can be restored despite unmanaged conflicts or drift.

```bash
cfgfc revert -p OpenCode
```

## Notes

- After `cfgfc switch`, project-scoped `new`, `sync`, `list`, `apply`, `update`, `reset`, and `revert` commands can omit `-p`.
- `cfgfc root` prints the effective warehouse root, and `cfgfc root <path>` persists a different root for later commands without moving existing warehouse contents.
- `switch` persists the normalized project identifier even when the user typed an alias.
- After `cfgfc switch global`, the current PPID-scoped project context is cleared; `list` returns to the global project list and `sync` returns to its default full-warehouse fallback.
- `.cfgfc-session/` stays under the effective warehouse root, so switching roots also switches the session-local active project store.
- Use `sync` to reconcile indexes, `apply` to choose what should be active, and `update` to replan the persisted apply intent after source metadata changes. When no apply intent is recorded, current mappings are refreshed mapping-by-mapping.
- `apply` manages regular-file hard links only. Directory-backed mappings are unsupported, hard links usually cannot cross filesystems or Windows volumes, and editing the activated target also edits the warehouse source because both names share the same file content.
- Mode strategies are `cover`, `increment`, `none`, and `full`; only `none` and `full` may omit `settings` in `ModeIndex.jsonc`.
- Forced operations restore only the last confirmed managed state. They do not back up or reconstruct overwritten unmanaged file or directory contents.
