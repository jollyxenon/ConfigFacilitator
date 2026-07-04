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
- `internal/planner`：把命令意图转换为受管路径映射。
- `internal/linker`：执行基于硬链接的文件应用、重置和回退。

## 存储模型

仓库位于当前生效的仓库根目录下，而不是当前 shell 目录旁边。默认根目录是当前用户 home/profile 下的 `.configfacilitator/`（原生 Windows 为 `%USERPROFILE%/.configfacilitator`），但也可以通过 `cfgfc root <path>` 在仓库外的用户作用域 bootstrap 文件里持久化切换。项目目录下包含 `Column/`、`Mode/` 和 `Backup/`，主要持久化文件是 `ProjectIndex.jsonc`、`ColumnIndex.jsonc`、`SettingIndex.jsonc`、`ModeIndex.jsonc`、`current_state.json` 和 `history.log`。

## 行为规则

- Setting 目标路径由目录 / 名称数组按下标 zip 得出：`targetDir` / `targetName` 会按下标覆盖 `defaultTargetDir` / `defaultTargetName`。
- 激活时只会创建基于硬链接的常规文件；目录型映射不受支持，并且失败时不会退回到 symlink、junction、复制或其他平台替代方案。
- 硬链接通常必须位于同一文件系统或同一 Windows 卷内，而且无论编辑 source 还是 target，修改的都是同一份底层文件内容。
- `Mode` 支持 `cover`、`increment`、`none`、`full` 四种栏目策略。
- `switch` 会按 PPID 保存一个便利用项目上下文。
- `revert` 只恢复上一次应用状态。
