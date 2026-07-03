## Context

`cfgfc list` currently renders inventory only. In `internal/cli/cli.go`, `renderProjectList`, `renderProject`, and `renderColumn` format names directly from warehouse metadata, while `renderColumn` only distinguishes `present` vs `missing`. The CLI already persists the currently managed state per project in `Backup/current_state.json` through `linker.CurrentState`, including both the active mappings and an optional semantic `ApplyIntent`.

The requested behavior is not just cosmetic. Project-level mode status, column-level full/partial/none coverage, and setting-level enabled markers all need to reflect the persisted managed state rather than only index metadata. The repository also does not currently use a color library, so the design should add visual emphasis without introducing a dependency or breaking buffered test output.

## Goals / Non-Goals

**Goals:**
- Teach `cfgfc list` to surface current usage state from persisted managed data at the global, project, and column-detail levels.
- Keep the existing display-label contract for project, column, mode, and setting names while appending compact status suffixes.
- Add green/yellow/red emphasis for matched/partial/error-style list states when color output is enabled, without forcing ANSI escapes into non-terminal or test output.
- Preserve existing visibility for missing settings and unchanged `list -m` mode detail output.

**Non-Goals:**
- Changing how `apply`, `update`, `reset`, or `revert` persist current state.
- Adding new list flags, JSON output, or a new generalized status/report command.
- Inferring live filesystem drift beyond what the persisted `current_state.json` already records.
- Reworking planner semantics for `cover`, `increment`, `none`, or `full` beyond what is needed to summarize current usage.

## Decisions

### Decision: Derive list status from persisted `CurrentState`, not from warehouse inventory alone
The list views should load each project's `CurrentState` and classify status from the persisted managed mappings plus optional apply intent. This keeps list aligned with what `cfgfc` believes is currently active, which is the same source of truth used by `update`, `reset`, and `revert`.

**Alternatives considered:**
- **Use only warehouse/index metadata:** rejected because metadata says what is available, not what is active.
- **Inspect the live filesystem on every `list`:** rejected because it would be slower, more invasive, and could disagree with the CLI's persisted state in ways this change is not meant to solve.

### Decision: Treat a project as mode-matched only when its persisted intent is a resolvable mode intent
The global project list and project-scoped mode highlight should use `CurrentState.Intent` as the primary signal for the active mode. If the intent is `kind=mode` and still resolves, show that mode; if there are mappings but the intent is absent, column-scoped, or no longer resolvable, show `Unmatched`; if there are no mappings and no intent, show `None`.

This deliberately avoids trying to reverse-engineer a mode purely from the current mapping set. `increment` columns are history-sensitive: the same persisted mappings could arise from multiple prior states, so exact mode recovery from mappings alone is not reliable.

**Alternatives considered:**
- **Reconstruct a mode by comparing mappings against every mode definition:** rejected because `increment` semantics make the result ambiguous.
- **Show the last requested mode name even if it no longer resolves:** rejected because the user explicitly asked for `Unmatched` when the current state no longer matches any existing mode.

### Decision: Compute column `Full` / `Partial` / `None` from active settings coverage
For project-scoped list output, each column status should summarize how many non-missing settings from that column currently participate in persisted mappings. A column is `None` when no setting source from that column is active, `Full` when every non-missing setting contributes at least one active mapping, and `Partial` otherwise.

This setting-coverage view matches the user's “全部使用 / 部分使用 / 未使用” request while staying robust for settings that materialize into multiple targets.

**Alternatives considered:**
- **Count mappings instead of settings:** rejected because one setting can expand into multiple targets, which would overstate coverage.
- **Derive the label from the current mode strategy only:** rejected because single-column applies and unmatched states still need meaningful column summaries.

### Decision: Add a small CLI-local styling helper with terminal-aware fallback
The CLI should add a tiny styling helper in `internal/cli/cli.go` (or a nearby file in the same package) that wraps strings in green/yellow/red ANSI sequences only when the output target should be colorized. In tests and non-color environments, the same helper should return plain text so assertions can stay readable and scripted output remains stable.

**Alternatives considered:**
- **Always emit ANSI codes:** rejected because it would pollute tests and redirected output.
- **Add a third-party color dependency:** rejected because the needed styling is small and localized.

### Decision: Preserve missing-setting visibility in column detail output
`list -c` should continue listing missing entries from the warehouse index, but enabled settings should gain positive emphasis. Missing metadata is already a deliberate part of the warehouse model, so the new enabled-state highlighting must layer on top of that rather than replacing it.

**Alternatives considered:**
- **Hide inactive or missing settings in column detail:** rejected because it would regress one of `list`'s current inspection benefits.

## Risks / Trade-offs

- **[Risk] Persisted state may lag behind the live filesystem** → **Mitigation:** document and implement the feature explicitly against `current_state.json`, which is already the CLI's operational source of truth.
- **[Risk] Color behavior can make snapshots and tests noisy** → **Mitigation:** keep styling behind one helper that can disable ANSI output for buffers, non-terminals, or color-suppressed environments.
- **[Risk] Columns containing only missing settings could blur `Full` vs `None` semantics** → **Mitigation:** classify coverage only across non-missing settings and continue showing missing entries separately in column detail output.
- **[Risk] Users may over-interpret `Unmatched` as a hard error** → **Mitigation:** keep the label focused on state mismatch rather than command failure, and preserve normal exit codes for successful `list` output.

## Migration Plan

1. Extend the `list` path to load each relevant project's `CurrentState` and compute project/column/setting usage summaries before rendering.
2. Introduce terminal-aware styling helpers and thread them through the list renderers.
3. Update CLI tests and English/Chinese command docs/examples to reflect the new status annotations.
4. Verify with the repository baseline (`pixi run test`, `pixi run compile`, `pixi run help`, and the registered subcommand help sweep) once implementation lands.

Rollback is low-risk because the feature is isolated to list rendering and list-specific tests/docs. Reverting the renderer/helper changes restores the previous inventory-only output without touching persisted state.

## Open Questions

- None currently. The persisted-state model already gives enough data to implement the requested summaries without changing storage formats.
