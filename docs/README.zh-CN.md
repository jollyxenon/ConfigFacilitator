# ConfigFacilitator 文档

ConfigFacilitator 通过面向资源的 CLI 命令完整管理可移植配置仓库。JSONC 仍然是可查看的互操作格式，但常规工作流不需要编辑器。

## 从这里开始

- [命令参考](commands.zh-CN.md)
- [纯 CLI 工作流示例](example.zh-CN.md)
- [架构说明](architecture.zh-CN.md)
- [JSONC 与互操作指南](jsonc-guide.zh-CN.md)
- [平台说明](platform-notes.zh-CN.md)
- [开发环境](developer-setup.zh-CN.md)
- [Agent 使用 Skill](../skills/configfacilitator-usage/SKILL.md)

## 关键信息

- 二进制名称：`cfgfc`
- 安装命令：`npm install -g @jollyxenon/cfgfc`
- 默认仓库根目录：`~/.configfacilitator/`
- 根目录查看与切换：`cfgfc root` 和 `cfgfc root <Path>`；切换根目录不会迁移内容
- 资源：Project、Column、Setting、Mode、Column 目标位置、Setting 目标覆盖和 Mode 的 Column 选择
- 顶层命令：`project`、`column`、`setting`、`mode`、`current`、`use`、`status`、`apply`、`refresh`、`sync`、`root`、`reset`、`revert`、`web`、`completion`
- Project 作用域：显式 `-p/--project` 优先于 `cfgfc use` 选择的 PPID 作用域 Project
- 机器输出：`--json` 输出一个稳定的成功或错误对象；退出码见命令参考
- Shell 补全：`cfgfc completion <bash|zsh|fish|powershell>`
- 符号链接：Linux、macOS、原生 Windows 和 WSL 都只使用真实符号链接
- Web UI：`cfgfc web` 在 `127.0.0.1:38031` 提供内嵌离线 UI

## 推荐使用模型

1. 使用 `project`、`column`、`setting`、`mode` 命令创建和修改仓库资源。
2. 使用 `column target` 配置逻辑目标位置，使用 `setting target` 配置每个 Setting 的覆盖值。
3. 使用 `setting create` 和 `setting content` 创建或修改内容；stdin 会保留精确字节。
4. 使用资源 `list`/`show` 查看元数据，使用 `status` 查看活动状态。
5. 应用 Mode（Current 跟随它）或用 `apply column` 设置独立 Current；只有需要重新规划时才使用 `refresh`。
6. Git 或其他外部工具改变仓库后，使用 `sync`。已索引的 Project、Column 或 Setting 来源消失时，它会立即删除对应 Index 元数据，不重建来源，也不级联 Mode/runtime 引用；不存在 prune 流程。
7. 把 `--yes`、`--cascade`、`--force-targets` 视为三种独立授权。

Canonical 名称同时是索引 key 和文件系统标识。显示名称只用于展示，别名则是命令中的替代引用。命令会解析别名，并持久化 canonical 标识。
