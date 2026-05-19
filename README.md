# ConfigFacilitator

ConfigFacilitator is a portable Go CLI for managing configuration sets inside an executable-relative `SettingWarehouse/`. It uses real symlinks to activate selected settings, tracks the current managed state, and supports single-step revert.

ConfigFacilitator 是一个使用 Go 编写的便携式 CLI，用于管理位于可执行文件同级 `SettingWarehouse/` 中的配置集合。它通过真实软链接激活选定配置，记录当前受管状态，并支持单步回退。

## Features / 功能

- Manage projects, columns, settings, and modes in one portable warehouse.
- Scaffold editable JSONC templates with `cfgfc new`.
- Reconcile warehouse indexes with filesystem reality through `cfgfc sync`.
- Inspect projects, columns, modes, and settings through `cfgfc list`.
- Store PPID-scoped convenience context with `cfgfc switch`.
- Apply a mode or one column selection, then `reset` or `revert` safely.

- 在同一个便携仓库中管理 Project、Column、Setting 与 Mode。
- 使用 `cfgfc new` 生成可编辑的 JSONC 模板。
- 使用 `cfgfc sync` 让仓库索引与文件系统实体保持一致。
- 使用 `cfgfc list` 查看 Project、Column、Mode 和 Setting。
- 使用 `cfgfc switch` 保存基于 PPID 的便利上下文。
- 应用模式或单栏目选择后，可安全执行 `reset` 与 `revert`。

## Warehouse Layout / 仓库结构

```text
cfgfc
SettingWarehouse/
├── ProjectIndex.jsonc
└── OpenCode/
    ├── Backup/
    │   ├── current_state.json
    │   └── history.log
    ├── Column/
    │   ├── ColumnIndex.jsonc
    │   ├── opencode.json/
    │   │   ├── SettingIndex.jsonc
    │   │   ├── GPT.json
    │   │   └── CLAUDE.json
    │   └── Skills/
    │       ├── SettingIndex.jsonc
    │       ├── Skill-A/
    │       └── Skill-B/
    └── Mode/
        └── ModeIndex.jsonc
```

The CLI always looks for `SettingWarehouse/` beside the executable rather than beside the current shell directory.

CLI 总是从可执行文件的同级目录寻找 `SettingWarehouse/`，而不是从当前 shell 工作目录寻找。

## Commands / 命令

### Scaffold / 搭建骨架

```bash
cfgfc new -p OpenCode
cfgfc new -p OpenCode -c opencode.json
cfgfc new -p OpenCode -c Skills
cfgfc new -p OpenCode -m Max
```

### Sync / 同步

```bash
cfgfc sync
cfgfc sync -p OpenCode
```

### Inspect and context / 查看与上下文

```bash
cfgfc switch OpenCode
cfgfc list
cfgfc list -p OpenCode -c Skills
cfgfc list -p OpenCode -m Max
```

### Apply, reset, revert / 应用、重置、回退

```bash
cfgfc apply -p OpenCode -m Max
cfgfc apply -p OpenCode -c opencode.json -s GPT.json
cfgfc reset -p OpenCode
cfgfc revert -p OpenCode
```

After `cfgfc switch OpenCode`, commands that support project resolution can omit `-p`.

执行 `cfgfc switch OpenCode` 之后，支持项目解析的命令可以省略 `-p`。

## Current Semantics / 当前语义

- Real symlinks only. No junction or copy fallback.
- Revert is single-step only.
- Conflict handling is interactive-only at the product-contract level.
- A setting-specific `target` overrides a column `defaultTarget`.
- Template comments may disappear after sync; permanent notes belong in `"description"`.
- PPID context is a convenience feature, not a hard isolation guarantee.

- 只使用真实 symlink，不做 junction 或 copy fallback。
- `revert` 只支持回退到上一次。
- 冲突处理在产品契约层面是交互式的。
- Setting 自身的 `target` 优先于 Column 的 `defaultTarget`。
- 模板注释在 sync 后可以消失，永久备注应写在 `"description"` 中。
- PPID 上下文只是便利功能，不是强隔离保证。

## Representative Workflow / 代表性流程

```bash
cfgfc new -p OpenCode
cfgfc switch OpenCode
cfgfc new -c opencode.json
cfgfc new -c Skills
cfgfc new -m Max

# Fill JSONC templates and add real setting files/directories
cfgfc sync
cfgfc list -c Skills

cfgfc apply -c opencode.json -s GPT.json
cfgfc apply -m Max
cfgfc revert
cfgfc reset
```

This workflow is covered by automated tests and a real manual smoke run in the implementation history.

这条流程已经被自动化测试与真实手工 smoke run 覆盖。

## Developer Commands / 开发命令

- `pixi run test`
- `pixi run build`
- `pixi run help`

## Notes / 说明

- The environment manager for local development is `pixi`.
- The CLI entry point is `cmd/cfgfc/main.go`.
- OpenSpec changes under `openspec/changes/` were used to drive the implementation slices.

- 本地开发环境管理器使用 `pixi`。
- CLI 入口位于 `cmd/cfgfc/main.go`。
- 实现过程通过 `openspec/changes/` 下的变更切片推进。
