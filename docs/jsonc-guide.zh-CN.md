# JSONC 指南

## 用途

索引文件使用 JSONC，是为了让模板可以带临时指导注释，同时仍然能通过项目自己的序列化逻辑回写。

## 规则

- 临时 `//` 注释在执行 `sync` 后可能会消失。
- 永久说明应写入 `"description"` 字段。
- 索引层会保留未知字段。

## 主要文件

- `ProjectIndex.jsonc`
- `ColumnIndex.jsonc`
- `SettingIndex.jsonc`
- `ModeIndex.jsonc`

## 目标路径解析

- `defaultTarget` 是 Column 级别的默认路径。
- Setting 级别的 `target` 会覆盖 `defaultTarget`。
- 路径支持 `~`、`${VAR}` 和 Windows `%VAR%` 写法。

## 模式语义

- `full` 会先清理该 Column 之前的链接，再重新应用。
- `incremental` 会保留现有链接，并追加新链接。
