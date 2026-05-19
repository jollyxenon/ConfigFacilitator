# Command Reference

## `new`

Scaffold a project, column, or mode.

```bash
cfgfc new -p OpenCode
cfgfc new -p OpenCode -c Skills
cfgfc new -p OpenCode -m Max
```

## `sync`

Reconcile warehouse indexes with the filesystem.

```bash
cfgfc sync
cfgfc sync -p OpenCode
```

## `switch`

Store the active project for the current PPID-scoped session.

```bash
cfgfc switch OpenCode
```

## `list`

Inspect projects, columns, modes, and settings.

```bash
cfgfc list
cfgfc list -p OpenCode -c Skills
cfgfc list -p OpenCode -m Max
```

## `apply`

Apply a mode or a single column selection.

```bash
cfgfc apply -p OpenCode -m Max
cfgfc apply -p OpenCode -c opencode.json -s GPT.json
```

## `reset`

Remove the current project's managed mappings.

```bash
cfgfc reset -p OpenCode
```

## `revert`

Restore the previous apply state for a project.

```bash
cfgfc revert -p OpenCode
```

## Notes

- After `cfgfc switch`, commands that support project resolution can omit `-p`.
- Mode strategies are `full` or `incremental`.
