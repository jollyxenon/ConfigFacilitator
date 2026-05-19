# ConfigFacilitator 文档

ConfigFacilitator 是一个便携式 Go CLI，用于管理 `~/.configfacilitator/SettingWarehouse/` 中的配置仓库。

## 从这里开始

- [架构说明](architecture.zh-CN.md)
- [命令参考](commands.zh-CN.md)
- [JSONC 指南](jsonc-guide.zh-CN.md)
- [平台说明](platform-notes.zh-CN.md)
- [开发环境](developer-setup.zh-CN.md)

## 关键信息

- 二进制名称：`cfgfc`
- 仓库根目录：`~/.configfacilitator/SettingWarehouse/`
- 核心实体：`Project`、`Column`、`Setting`、`Mode`
- 命令：`new`、`sync`、`switch`、`list`、`apply`、`reset`、`revert`

## 项目作用

它负责搭建仓库骨架、同步索引与磁盘实体、保存基于 PPID 的便利上下文、应用符号链接配置，并支持 `reset` 和单步 `revert`。
