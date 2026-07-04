# 平台说明

## 硬链接策略

ConfigFacilitator 只使用真实的文件硬链接，不会退回到 symlink、目录 junction、文件复制、`mklink`、PowerShell 辅助命令或其他替代机制。

硬链接只适用于常规文件。目录型映射不受支持，并且会直接报出清晰错误，不会转成目录 symlink、junction 或复制。

激活时会先检查 source 路径是否存在且确实是常规文件。硬链接通常不能跨文件系统或跨 Windows 卷，因此当仓库 source 与 target 位于不同设备、文件系统不支持，或 source 不是常规文件时，激活会失败。

无论编辑仓库里的 source 路径，还是编辑已激活的 target 路径，改动都会落到同一份文件内容上，因为这两个名字指向同一个底层数据。

## 便携式目录布局

仓库根目录会从当前用户的 home/profile 目录解析，而不是按当前 shell 工作目录解析。移动二进制文件，不会改变生效的仓库根目录。

- Unix-like 平台默认根目录：`~/.configfacilitator/`
- 原生 Windows 默认根目录：`%USERPROFILE%/.configfacilitator`
- 持久化 override bootstrap：Unix-like 平台是 `~/.cfgfc-root`，原生 Windows 是 `%USERPROFILE%/.cfgfc-root`

`cfgfc root` 会输出当前生效的仓库根目录。`cfgfc root <path>` 会在 bootstrap 文件中持久化新的生效根目录，并在写入前完成路径展开和规范化。切换根目录不会迁移、复制或初始化任何仓库内容。

根目录项目发现会直接在当前生效仓库根目录下进行。只要目录符合项目布局，像 `SettingWarehouse` 这样的目录名也会和其他项目目录一样参与发现。

## 原生 Windows 与 WSL 边界

原生 Windows `cfgfc.exe` 与 WSL 下运行的 Linux 构建是两个不同运行环境。原生 Windows 使用 `%USERPROFILE%` 路径和 Windows 硬链接限制；WSL 使用传入路径对应的 Linux 路径与硬链接语义。ConfigFacilitator 不会自动在 `%USERPROFILE%` 与 `/mnt/c/...` 之间转换。

## 会话上下文

`switch` 按父进程 ID 保存活动项目。这是为了多终端并行使用而提供的便利能力，不是强隔离边界。

`.cfgfc-session/` 目录始终位于当前生效仓库根目录下，所以切换根目录也会切换会话上下文存储位置。

## 回退范围

`revert` 只支持单步回退，只会恢复到上一次 `apply` 的快照状态。

## 原生 Windows 冒烟测试

如需手动验证原生 Windows 支持，请在普通 Windows shell 中运行 `cfgfc.exe`，应用一个 source 和 target 位于同一卷的文件型 setting。确认 target 变成普通文件硬链接，确认不需要 Developer Mode 或 Administrator 的 symlink 权限，并确认仓库根目录要么是默认的 `%USERPROFILE%/.configfacilitator`，要么是通过 `cfgfc root` 持久化后的新根目录。同时验证跨卷、source 缺失、source 不是常规文件等情况会给出清晰错误。
