# JSONC 指南

## 用途

索引文件使用 JSONC，是为了让生成的模板可以在文件末尾带一个可丢弃的示例注释块，同时仍然能通过项目自己的序列化逻辑回写。

## 规则

- 文件末尾的示例注释块在执行 `sync` 后可能会消失。
- 永久说明应写入 `"description"` 字段。
- 索引层会保留未知字段。
- 规范化后的条目会持久化 `displayName` 与 `aliases`，而顶层 key 会持续作为书写时的 canonical 标识。
- 对于 `ProjectIndex.jsonc`、`ColumnIndex.jsonc` 和 `ModeIndex.jsonc`，每个顶层 key 本身就是 canonical 的仓库侧名称。

## 主要文件

- `ProjectIndex.jsonc`
- `ColumnIndex.jsonc`
- `SettingIndex.jsonc`
- `ModeIndex.jsonc`

## 标识字段

- 顶层索引 key 就是规范化后的持久化标识符，也用于保持现有文件系统布局不变。
- 像 `warehouseName`、`folderName` 这样的额外书写字段不参与 canonical 标识判定；系统始终以顶层 key 作为标识来源。
- `displayName` 仅用于展示，不会被当作隐式 CLI 别名。
- `aliases` 为项目、栏目、设置和模式提供额外可调用引用；当没有别名时，规范化输出也会显式写出 `"aliases": []`。

## 目标路径解析

- `defaultTargetDir` 与 `defaultTargetName` 是 Column 级别的默认目标目录 / 名称数组。
- `targetDir` 与 `targetName` 是 Setting 级别的目标目录 / 名称数组，会按相同下标覆盖默认值。
- 目录数组和名称数组严格按下标 zip；继承展开后长度必须一致。
- 在 Setting 条目中，`""` 表示继承对应下标的默认值；在 `defaultTargetName` 中，`""` 会回退为 Setting 的仓库侧名称；在 `defaultTargetDir` 中，`""` 表示未配置，不能执行 apply。
- 目标目录支持 `~`、`${VAR}` 和 Windows `%VAR%` 写法。目标名称必须展开为普通文件名或单层目录名。
- 展开后的目标路径必须非空，并且在同一个规划状态中唯一。

## 模式语义

- `cover` 只应用该 Column 中显式写出的 `settings`。
- `increment` 会保留该 Column 现有的受管链接，再追加显式写出的 `settings`。
- `none` 表示该 Column 本次不建立任何链接。
- `full` 会自动链接该 Column 下全部已知 Setting。
- 只有当 `strategy` 为 `none` 或 `full` 时，才可以省略 `settings`。
