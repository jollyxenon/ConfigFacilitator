# 架构说明

## 总览

`cfgfc` 是一个单可执行文件的 Go CLI。`cmd/cfgfc/main.go` 只负责转发到 `internal/cli`，由后者直接路由各个命令族。

## 包划分

- `internal/warehouse`：解析当前生效的仓库根目录（默认 `~/.configfacilitator/`，也支持持久化 override）并加载仓库模型。
- `internal/index`：解析和写入 JSONC 索引文件。
- `internal/jsonc`：移除注释并规范化 JSONC 内容。
- `internal/scaffold`：创建项目、栏目和模式模板。
- `internal/syncer`：将索引与文件系统实体进行对齐。
- `internal/session`：保存基于 PPID 的项目上下文。
- `internal/pathvars`：展开可移植路径变量。
- `internal/planner`：把命令意图转换为链接映射。
- `internal/linker`：执行符号链接的应用、重置和回退；创建链接前会检查 source 是否存在，并始终只调用真实 symlink 创建逻辑。

## 存储模型

仓库位于当前生效的仓库根目录下，而不是当前 shell 目录旁边。默认根目录是当前用户 home/profile 下的 `.configfacilitator/`（原生 Windows 为 `%USERPROFILE%/.configfacilitator`），但也可以通过 `cfgfc root <path>` 在仓库外的用户作用域 bootstrap 文件里持久化切换。项目目录下包含 `Column/`、`Mode/` 和 `Backup/`，主要持久化文件是 `ProjectIndex.jsonc`、`ColumnIndex.jsonc`、`SettingIndex.jsonc`、`ModeIndex.jsonc`、`current_state.json` 和 `history.log`。

## 行为规则

- Setting 目标路径由目录 / 名称数组按下标 zip 得出：`targetDir` / `targetName` 会按下标覆盖 `defaultTargetDir` / `defaultTargetName`。
- `Mode` 支持 `cover`、`increment`、`none`、`full` 四种栏目策略。
- `switch` 会按 PPID 保存一个便利用项目上下文。
- `revert` 只恢复上一次应用状态。
