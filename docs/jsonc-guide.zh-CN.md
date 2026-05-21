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

- `defaultTarget` 是 Column 级别的默认路径。
- Setting 级别的 `target` 会覆盖 `defaultTarget`。
- 路径支持 `~`、`${VAR}` 和 Windows `%VAR%` 写法。
- `target` 与规范化后的持久化标识和仓库侧源名称保持解耦。

## 模式语义

- `full` 会先清理该 Column 之前的链接，再重新应用。
- `incremental` 会保留现有链接，并追加新链接。
