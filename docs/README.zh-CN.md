# ConfigFacilitator 文档

ConfigFacilitator 是一个便携式 Go CLI，用于管理 `~/.configfacilitator/` 中的配置仓库。

## 从这里开始

- [架构说明](architecture.zh-CN.md)
- [命令参考](commands.zh-CN.md)
- [工作流示例](example.zh-CN.md)
- [JSONC 指南](jsonc-guide.zh-CN.md)
- [平台说明](platform-notes.zh-CN.md)
- [开发环境](developer-setup.zh-CN.md)

## 关键信息

- 二进制名称：`cfgfc`
- 开发构建：`pixi run compile` 检查所有 Go package；`pixi run build` 生成 `dist/cfgfc`
- 开源协议：MIT License（见 [`LICENSE`](../LICENSE)）
- 仓库根目录：`~/.configfacilitator/`
- 根目录项目发现：`~/.configfacilitator/` 下的直接项目目录都会参与发现，其中也包括 `SettingWarehouse`
- 核心实体：`Project`、`Column`、`Setting`、`Mode`
- 命令：`new`、`sync`、`switch`、`list`、`apply`、`update`、`reset`、`revert`

## 项目作用

它负责搭建仓库骨架、同步索引与磁盘实体、保存基于 PPID 的便利上下文、应用符号链接配置，并支持 `reset` 和单步 `revert`。

## 标识模型

- `Project`、`Column`、`Setting`、`Mode` 都以顶层索引 key 作为 canonical 持久化标识，额外保存仅用于展示的 `displayName` 和零个或多个 `aliases`。
- 命令解析同时支持 canonical 名称和别名。
- `switch` 会在会话上下文中保存规范化后的项目标识符。
