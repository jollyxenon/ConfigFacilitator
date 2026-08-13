# 架构说明

## 总览

`cfgfc` 是单二进制 Go CLI。`cmd/cfgfc/main.go` 会转发到 `internal/cli` 中每次新建的 Cobra 命令树。公开接口面向资源：Project、Column、Setting、Mode、目标、选择、内容、上下文、状态、apply/refresh、同步和状态恢复都通过命令表达，不再依赖编辑器工作流。

## 主要边界

- `internal/cli`：命令构造、作用域与参数验证、人类/JSON 输出和退出分类。
- `internal/warehouse`：解析生效根目录并加载索引中存在的资源；同步时会移除文件系统来源已经消失的条目。
- `internal/index` 与 `internal/jsonc`：解析和序列化持久 JSONC 索引，同时保留支持的元数据与可解析未知字段。
- `internal/mutate`：资源元数据、目标、选择、重命名和删除用例。
- `internal/content`：安全的文件/目录导入，以及限制在 Setting 根目录内的内容操作。
- `internal/repository`：原子写入、全仓变更锁、快照、事务恢复、引用重写和会话/运行时持久化。
- `internal/syncer`：协调外部文件系统/索引变化，立即移除消失的 Project/Column/Setting 元数据，不级联 Mode/runtime 引用，而是让这些引用处于无法解析状态。
- `internal/session`：`use` 使用的 PPID 作用域便利 Project 上下文。
- `internal/pathvars`：展开可移植目标路径变量。
- `internal/planner`：把 canonical apply/refresh 意图和逻辑目标转换为映射。
- `internal/linker`：检查所有权，并通过真实符号链接完成应用、替换、重置或恢复。

## 存储模型

默认仓库根目录是 `~/.configfacilitator/`。`cfgfc root <Path>` 会在用户 bootstrap 文件中持久化其他根目录，但不迁移数据。Project 包含 `Column/`、`Mode/`、`Backup/` 目录树。主要持久文件包括：

- `ProjectIndex.jsonc`、`ColumnIndex.jsonc`、`SettingIndex.jsonc`、`ModeIndex.jsonc`：可查看的互操作数据。
- `Backup/current_state.json`、`Backup/history.log`：CLI 自有的当前和前一个映射/意图状态。
- `.cfgfc-session/`：位于生效根目录下的 CLI 自有 PPID 上下文记录。
- `.cfgfc-transactions/`：保留给 CLI 的变更锁、manifest、暂存和恢复数据。

运行时、会话和事务文件都不是用户可编辑契约。

## 标识与目标模型

Canonical 标识来自索引 map key 和对应资源路径。`displayName` 只用于展示；`aliases` 是命令替代输入。命令会先解析别名再持久化，因此 Mode 选择、应用意图和上下文都保存 canonical 标识。

Column 目标位置是从零开始的逻辑记录，会序列化为现有 `targetNumber`、`defaultTargetDir`、`defaultTargetName` 数组。Setting 覆盖序列化为相同长度的 `targetDir`、`targetName` 数组；继承的部分持久化为空条目。结构化命令保证全部数组同步变化。

Setting 可以是文件型或目录型。内容路径被限制在 Setting 根目录下，拒绝路径遍历和符号链接组件，导入时也绝不跟随符号链接。

## 变更与事务模型

CLI 自有变更会在持久修改前验证名称、别名、路径、引用、目标数组、依赖、确认和所有权。单个文件使用同目录临时写入和 rename。

影响多个工件的变更使用全仓排他事务：

1. 恢复之前的 prepared 事务；
2. 验证并规划全部受影响仓库路径、运行时记录、上下文和受管目标；
3. 在 `.cfgfc-transactions/` 下记录精确的变更前快照和持久 prepared manifest；
4. 提交暂存路径/文件和受管链接变化；
5. 标记 committed，并删除事务工件。

普通失败会回滚到记录状态。如果进程中止，下一个变更命令会先恢复，再开始新操作。只读 `status` 只报告未完成事务诊断，不会修改事务。

Rename 会重写 schema 定义的活动和历史引用，并重建受影响的自有链接。删除会先报告依赖。`--yes`、`--cascade`、`--force-targets` 分别独立授权确认、引用修复和已记录目标回收。

## 应用与同步

`apply mode` 和 `apply column` 会持久化 canonical 意图与映射。`refresh` 根据当前元数据重新规划该意图，也可以刷新旧的仅映射状态。内容字节变化不需要 refresh，因为符号链接仍指向同一来源路径。

`sync` 专门处理 Git 或直接有效 JSONC 编辑等外部变化。它加入新发现资源，并在文件系统来源消失时立即移除已索引的 Project、Column、Setting 元数据；不会重建来源，也不会隐式级联 Mode/runtime 引用。若 apply 或 refresh 所需引用无法解析，会在目标变化前失败。不再提供 `sync --prune --yes` 流程，sync 也不会恢复已删除元数据。

## 输出契约

人类模式保持简洁。JSON 模式只向 stdout 输出一个成功对象，或只向 stderr 输出一个错误对象，不带 ANSI 或无关文字。退出码区分用法、资源/作用域、无效数据、拒绝和持久化/事务失败。
