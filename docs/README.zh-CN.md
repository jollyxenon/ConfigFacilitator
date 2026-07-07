# ConfigFacilitator 文档

ConfigFacilitator 是一个便携式 Go CLI，用于管理配置仓库；默认仓库根目录是 `~/.configfacilitator/`。

## 从这里开始

- [架构说明](architecture.zh-CN.md)
- [命令参考](commands.zh-CN.md)
- [工作流示例](example.zh-CN.md)
- [JSONC 指南](jsonc-guide.zh-CN.md)
- [平台说明](platform-notes.zh-CN.md)
- [开发环境](developer-setup.zh-CN.md)
- [Agent 使用 Skill](../skills/configfacilitator-usage/SKILL.md)

## 关键信息

- 二进制名称：`cfgfc`
- npm 安装：`npm install -g @jollyxenon/cfgfc`
- 开发构建：`pixi run compile` 检查所有 Go package；`pixi run build` 生成 `dist/cfgfc`
- 开源协议：MIT License（见 [`LICENSE`](../LICENSE)）
- 仓库根目录：默认是 `~/.configfacilitator/`；使用 `cfgfc root` 查看当前生效根目录，使用 `cfgfc root <path>` 持久化切换
- 根目录项目发现：当前生效仓库根目录下的直接项目目录都会参与发现，其中也包括 `SettingWarehouse`
- 核心实体：`Project`、`Column`、`Setting`、`Mode`
- 命令：`new`、`sync`、`switch`、`root`、`list`、`apply`、`update`、`reset`、`revert`
- Agent Skill 维护：当面向用户的命令、工作流、示例或安全规则变化时，需要随文档一起检查并更新 [`configfacilitator-usage`](../skills/configfacilitator-usage/SKILL.md)

## 项目作用

它负责搭建仓库骨架、同步索引与磁盘实体、保存基于 PPID 的便利上下文、通过 `cfgfc root` 持久化切换仓库根目录、应用符号链接配置，并支持 `reset` 和单步 `revert`。

## 安装

维护者发布带 tag 的 GitHub Release 和匹配版本的 npm 包后，可以这样安装 CLI：

```bash
npm install -g @jollyxenon/cfgfc
cfgfc --help
```

npm 包只是安装包装层。它会从与 npm 包版本匹配的 GitHub Release 下载预编译 Go 二进制文件，然后通过 npm 的 `cfgfc` 命令暴露该二进制文件。

## 标识模型

- `Project`、`Column`、`Setting`、`Mode` 都以顶层索引 key 作为 canonical 持久化标识，额外保存仅用于展示的 `displayName` 和零个或多个 `aliases`。
- 命令解析同时支持 canonical 名称和别名。
- `switch` 会在会话上下文中保存规范化后的项目标识符。
