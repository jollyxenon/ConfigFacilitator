# 架构说明

## 总览

`cfgfc` 是一个单可执行文件的 Go CLI。`cmd/cfgfc/main.go` 只负责转发到 `internal/cli`，由后者直接路由各个命令族。

## 包划分

- `internal/warehouse`：解析可执行文件同级的 `SettingWarehouse/` 并加载仓库模型。
- `internal/index`：解析和写入 JSONC 索引文件。
- `internal/jsonc`：移除注释并规范化 JSONC 内容。
- `internal/scaffold`：创建项目、栏目和模式模板。
- `internal/syncer`：将索引与文件系统实体进行对齐。
- `internal/session`：保存基于 PPID 的项目上下文。
- `internal/pathvars`：展开可移植路径变量。
- `internal/planner`：把命令意图转换为链接映射。
- `internal/linker`：执行符号链接的应用、重置和回退。

## 存储模型

仓库固定放在可执行文件旁边，而不是当前 shell 目录旁边。项目目录下包含 `Column/`、`Mode/` 和 `Backup/`，主要持久化文件是 `ProjectIndex.jsonc`、`ColumnIndex.jsonc`、`SettingIndex.jsonc`、`ModeIndex.jsonc`、`current_state.json` 和 `history.log`。

## 行为规则

- Setting 级别的 `target` 会覆盖 Column 级别的 `defaultTarget`。
- `Mode` 支持 `full` 和 `incremental` 两种栏目策略。
- `switch` 会按 PPID 保存一个便利用项目上下文。
- `revert` 只恢复上一次应用状态。
