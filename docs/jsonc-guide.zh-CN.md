# JSONC 与互操作指南

## JSONC 的职责

`ProjectIndex.jsonc`、`ColumnIndex.jsonc`、`SettingIndex.jsonc`、`ModeIndex.jsonc` 仍然是持久、可查看的互操作格式。普通用户使用 CLI 命令管理支持的元数据、目标、Setting 来源、内容和 Mode 选择。直接编辑索引是可选能力，适合外部工具、Git merge 或高级互操作，不再是必需的创建步骤。

有效的外部 JSONC（包括注释）会在下次加载或 `sync` 时解析。无效 JSONC 或无效的必填字段类型会让命令在规范化或相关变更前失败。

## 标识与元数据

- 资源 map key 是 canonical 标识，并在适用时对应文件系统位置。
- `displayName` 只用于展示，不是隐式别名。
- `aliases` 是明确的替代命令引用。空别名会规范化为 `"aliases": []`。
- `description` 是持久元数据；除非显式替换或删除所属资源，否则会保留。
- 通过别名输入时，CLI 会在持久化前解析，因此选择、上下文和应用意图都保存 canonical 标识。
- 更新别名时，会拒绝空值、重复值，以及同一解析作用域中的 canonical 名称或别名冲突。

应使用资源 `create`、`set`、`rename` 命令，而不是直接改这些字段。

## 未知字段

只要不与 schema 定义的 key 冲突，可解析未知字段会在无关的 set、target、selection、rename、sync 和序列化操作中保留。发生冲突时，schema 定义值优先。删除资源也会删除属于该资源的未知字段。

CLI 只重写 schema 定义的引用。如果扩展在未知字段中保存标识或路径，扩展自身必须更新这些引用。

## 资源消失与同步

已索引的 Project 目录、Column 目录或 Setting 文件/目录消失时，`sync` 会立即从对应 Index 删除元数据。Sync 不会重建缺失的来源路径或子索引。

Mode 选择、当前/历史 runtime 记录和 PPID 上下文不会被隐式级联清理。它们会作为无法解析的引用保留供诊断；如果 `apply` 或 `refresh` 需要这些引用，会在目标变化前失败。若要清理依赖，应使用资源 CLI delete，并独立使用 `--yes`、`--cascade`、`--force-targets`。

`sync --prune` 和 `sync --prune --yes` 均不受支持。重新创建旧来源路径可能会发现一个新资源，但 sync 不会恢复已从 Index 删除的说明、别名、目标、未知字段或其他元数据。

## 目标持久化

CLI 对外提供逻辑位置，同时保留现有数组 schema：

- Column `targetNumber` 是从零开始位置的数量。
- `defaultTargetDir` 和 `defaultTargetName` 是 Column 默认值。
- 每个 Setting 的 `targetDir`、`targetName` 数组长度必须完全相同。
- Setting 空条目表示继承对应 Column 部分。
- Column 默认名称为空时，目标名称从 Setting canonical 名称派生。
- 有效目标目录为空时不能规划。

使用 `column target add/set/delete` 和 `setting target set/reset`。这些命令会在所有 Setting 之间一致地调整或更新数组。目标目录支持 `~`、`${VAR}` 和 Windows `%VAR%`；展开后的目标必须非空且唯一，目标名称必须是一个普通路径组件。

## Mode 持久化

Mode 的 Column 策略包括：

- `cover`：只应用明确选择的 Setting。
- `increment`：保留该 Column 已有受管映射，并增加选择的 Setting。
- `none`：该 Column 不应用映射。
- `full`：应用该 Column 中所有存在的 Setting。

`cover` 和 `increment` 需要一个或多个 Setting；`none` 和 `full` 不保存 Setting 列表。使用 `mode column set/delete`，让引用得到 canonical 化和验证。

## CLI 自有文件

不要手工编辑：

- `Backup/current_state.json`
- `Backup/history.log`
- `.cfgfc-session/`
- `.cfgfc-transactions/`

它们保存应用意图、当前/前一个映射、PPID 上下文、变更锁、快照、暂存和恢复记录。只读 `status` 可以报告未完成事务；下一个变更命令会执行恢复。
