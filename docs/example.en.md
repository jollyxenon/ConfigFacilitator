# CLI-only Workflow Example

This lifecycle starts with an empty alternate warehouse and uses only `cfgfc` plus shell input redirection. It creates metadata, targets, file- and directory-backed Settings, content, and a Mode; applies and changes them; renames and deletes resources; and reconciles external changes safely.

## 1. Select an empty warehouse and create resources

```bash
cfgfc root ~/.configfacilitator-demo
cfgfc project create OpenCode \
  --display-name "OpenCode Demo" \
  --aliases oc \
  --description "CLI-managed OpenCode configuration"
cfgfc use oc

cfgfc column create Models --description "Main model file"
cfgfc column target add Models \
  --dir ~/.config/opencode \
  --name opencode.json

cfgfc column create Skills --description "Installed skill directories"
cfgfc column target add Skills \
  --dir ~/.config/opencode/skills \
  --name-from-setting
```

Creation is immediately usable; no `sync` is required after these CLI-owned mutations. Target indexes are zero-based. The Models target has a fixed name; each Skills target derives its name from the Setting canonical name.

## 2. Create file and directory Settings

Create exact file content from stdin. `printf` is used here because it does not add a newline:

```bash
printf '%s' '{"model":"provider/example"}' | \
  cfgfc setting create Main.json \
    -c Models \
    --kind file \
    --stdin \
    --aliases main \
    --description "Primary model selection"
```

Create a directory Setting, then create nested content through the CLI:

```bash
cfgfc setting create Review -c Skills --kind directory
printf '%s' '# Review skill' | \
  cfgfc setting content write Review SKILL.md -c Skills --stdin
cfgfc setting content mkdir Review references -c Skills
cfgfc setting content write Review references/checklist.md \
  -c Skills \
  --text 'Check scope, evidence, and rollback.'
```

Alternatives are `--text <Text>` for literal file bytes and `--from <Path>` for copying a regular file or directory tree. `--from`, `--stdin`, and `--text` are mutually exclusive. Directory imports reject symlinks and special objects.

Inspect the results:

```bash
cfgfc setting show Main.json -c Models
cfgfc setting content read Main.json -c Models
cfgfc setting content list Review -c Skills
cfgfc setting content read Review SKILL.md -c Skills
```

For a file-backed Setting, omit a relative path. For a directory-backed Setting, reads and writes name a file below the Setting root.

## 3. Create and apply a Mode

```bash
cfgfc mode create Default --description "Complete OpenCode setup"
cfgfc mode column set Default Models \
  --strategy cover \
  --setting Main.json
cfgfc mode column set Default Skills --strategy full

cfgfc mode show Default
cfgfc apply mode Default
cfgfc status
```

`cover` applies the explicitly repeated Settings. `full` applies every present Setting in that Column. `increment` also requires one or more repeated `--setting` flags, while `none` and `full` reject Setting flags.

To apply only one Column instead:

```bash
cfgfc apply column Models Main.json
```

This persists a direct-Column intent rather than a Mode intent.

## 4. Change content and metadata

Replace file bytes atomically:

```bash
printf '%s' '{"model":"provider/new-example"}' | \
  cfgfc setting content write Main.json -c Models --stdin
```

The active symlink still points to the same source, so the new bytes are visible immediately. Do not run `refresh` for byte-only content changes.

Now add another directory Setting and update metadata:

```bash
cfgfc setting create Explain -c Skills --kind directory
cfgfc setting content write Explain SKILL.md \
  -c Skills \
  --text '# Explain skill'
printf '%s' 'Skills installed for this OpenCode profile.' | \
  cfgfc column set Skills --description-file -
```

If `Default` is still the persisted Mode intent, its `full` Skills selection must be replanned to include `Explain`:

```bash
cfgfc refresh
cfgfc status
```

Use `cfgfc refresh --column Skills` to replan only one Column while preserving other mappings, or `cfgfc refresh --all` to refresh every Project with active state.

## 5. Rename active resources

```bash
cfgfc setting rename Main.json Primary.json -c Models
cfgfc column rename Skills Extensions
cfgfc mode rename Default Work
cfgfc project rename OpenCode OpenCodeWork
cfgfc status
```

Rename rewrites schema-defined references, current and historical intents, source paths, PPID Project context, and owned managed links in one recoverable operation. Fixed target names stay fixed. A target name derived from a Setting canonical name changes with that Setting. If a recorded target has drifted, rename stops unless you deliberately add `--force-targets`.

## 6. Delete content and resources safely

Content deletion removes only a path below a directory Setting and requires confirmation:

```bash
cfgfc setting content move Review references/checklist.md notes.md -c Extensions
cfgfc setting content delete Review notes.md -c Extensions --yes
```

Resource deletion separates three decisions. For example:

```bash
cfgfc setting delete Explain -c Extensions --yes --cascade
```

- `--yes` confirms deletion.
- `--cascade` permits dependent Mode/current/history reference repair.
- `--force-targets` is needed only if an affected recorded target is occupied or ownership has drifted.

No flag implies another. `--force-targets` does not confirm deletion and does not authorize cascade. It can reclaim only affected recorded target paths and cannot reconstruct overwritten unmanaged content.

## 7. Revert or reset managed state

```bash
cfgfc revert
cfgfc reset
```

`revert` restores the previous snapshot only. `reset` removes current managed mappings while preserving warehouse resources. Add `--force-targets` only when you explicitly accept reclaiming affected recorded drifted or occupied paths.

## 8. Reconcile external changes

`sync` is for changes made outside resource/content commands, such as a Git checkout. Suppose an external operation removes the `Review` source directory. Run:

```bash
cfgfc sync
cfgfc setting show Review -c Extensions
cfgfc status
```

Sync immediately removes `Review` metadata from `SettingIndex.jsonc`; `setting show` no longer resolves it. Sync does not recreate the source or implicitly cascade Mode selections, current/history runtime records, or PPID context. If apply or refresh requires the removed Setting, it fails before managed targets change.

Recreating the former source path and running sync may discover a new Setting, but it does not restore the deleted description, aliases, target overrides, unknown fields, or other metadata. Recreate the intended metadata explicitly with resource commands.

`sync --prune` and `sync --prune --yes` are unsupported. Use `cfgfc sync --all` for the full warehouse or `cfgfc sync -p OpenCodeWork` for an explicit Project; those scopes are mutually exclusive. Resource CLI deletion remains separate and uses `--yes`, `--cascade`, and `--force-targets` independently.

## 9. Automation

```bash
cfgfc status --json
cfgfc project show OpenCodeWork --json
```

JSON success goes to stdout as one object. JSON failure goes to stderr as one object with stable `error.code` and `error.message`. Exit codes classify usage (`2`), resource/scope (`3`), invalid data (`4`), refusal (`5`), and persistence/transaction (`6`) failures.
