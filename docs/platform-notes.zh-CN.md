# 平台说明

## 符号链接策略

ConfigFacilitator 只使用真实 symlink，不会退回到目录 junction、硬链接、文件复制、`mklink`、PowerShell 辅助命令或其他替代机制。

在原生 Windows 下，`cfgfc.exe` 管理 Windows 用户目录中的配置，并使用真实的文件 symlink 和目录 symlink。Windows 可能要求开启 Developer Mode，或用 Administrator 权限运行，才允许创建 symlink。如果 Windows 拒绝创建 symlink，ConfigFacilitator 会报告错误并停止，不会改用 junction 或复制。

创建链接时会先检查 source 路径是否存在，并让平台根据该路径推断文件/目录类型。ConfigFacilitator 不会在状态文件中持久化 source 类型。

## 便携式目录布局

仓库根目录会从当前用户的 home/profile 目录解析，而不是按当前 shell 工作目录解析。移动二进制文件，不会改变生效的仓库根目录。

- Unix-like 平台：`~/.configfacilitator/`
- 原生 Windows：`%USERPROFILE%/.configfacilitator`

根目录项目发现会直接在 `~/.configfacilitator/` 下进行。只要目录符合项目布局，像 `SettingWarehouse` 这样的目录名也会和其他项目目录一样参与发现。

## 原生 Windows 与 WSL 边界

原生 Windows `cfgfc.exe` 与 WSL 下运行的 Linux 构建是两个不同运行环境。原生 Windows 使用 `%USERPROFILE%` 路径和 Windows symlink 权限规则；WSL 使用传入路径对应的 Linux 路径与 symlink 语义。ConfigFacilitator 不会自动在 `%USERPROFILE%` 与 `/mnt/c/...` 之间转换。

## 会话上下文

`switch` 按父进程 ID 保存活动项目。这是为了多终端并行使用而提供的便利能力，不是强隔离边界。

## 回退范围

`revert` 只支持单步回退，只会恢复到上一次 `apply` 的快照状态。

## 原生 Windows 冒烟测试

如需手动验证原生 Windows 支持，请在已开启 Developer Mode 或具有 Administrator 权限的 Windows shell 中运行 `cfgfc.exe`，分别应用一个文件型 setting 和一个目录型 setting。确认两个 target 都是真实 symlink，并确认仓库根目录位于 `%USERPROFILE%/.configfacilitator`。
