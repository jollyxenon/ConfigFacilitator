## Why

`cfgfc list` 现在只能展示仓库里“有哪些项目、栏目、模式与设置”，却不能直接回答“当前到底启用了什么”。当用户切换项目、应用模式、或只启用部分 Setting 之后，仍然需要再去读 `current_state.json` 或手动比对输出，才能判断当前是否正处于某个 Mode、某个 Column 是否已全部启用。

## What Changes

- Expand `cfgfc list` global output so each project name appends a parenthesized current-usage summary that shows the active mode when one is recognized, or a red `Unmatched` / `None` state when the persisted managed state does not correspond to a current mode.
- Expand project-scoped `cfgfc list` output so each column appends a `Full` / `Partial` / `None` usage label, and the currently active mode is visually highlighted in green.
- Expand `cfgfc list -c <column>` output so settings that are currently enabled in the persisted managed state are highlighted in green while missing entries remain visible.
- Update the CLI workflow contract, examples, and tests so the new status-oriented list output is specified and verified consistently.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `cli-workflows`: Change the `list` workflow requirements so list surfaces report current project, column, mode, and setting usage state instead of only static inventory data.

## Impact

- Affected code: `internal/cli/cli.go`, plus any small CLI-local helpers needed to inspect persisted current state and apply terminal styling.
- Affected behavior: stdout rendering for `cfgfc list`, `cfgfc list -p <project>`, and `cfgfc list -p <project> -c <column>` (including the equivalent switched-project forms).
- Affected tests/docs: `internal/cli/cli_test.go`, `docs/commands.en.md`, `docs/commands.zh-CN.md`, `docs/example.en.md`, `docs/example.zh-CN.md`, and `cfgfc list --help` text if examples or descriptions need to mention the new status annotations.
