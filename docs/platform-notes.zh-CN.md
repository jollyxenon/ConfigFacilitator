# 平台说明

## 真实符号链接策略

ConfigFacilitator 只创建真实的文件和目录符号链接，不会退回到 junction、硬链接、复制、`mklink`、PowerShell 辅助命令或其他替代机制。规划或创建链接时会检查来源是否存在以及来源类型；运行时状态不会持久化来源类型。

原生 Windows 可能要求开启 Developer Mode 或使用 Administrator 权限。如果系统拒绝创建符号链接，cfgfc 会返回持久化失败，不会悄悄换用其他机制。

## 根目录与运行时边界

- Unix-like 默认根目录：`~/.configfacilitator/`
- 原生 Windows 默认根目录：`%USERPROFILE%/.configfacilitator`
- 持久化 override bootstrap：`~/.cfgfc-root` 或 `%USERPROFILE%/.cfgfc-root`

`cfgfc root` 输出当前生效根目录。`cfgfc root <Path>` 展开并规范化新根目录，然后为后续命令持久化。它不会迁移、复制或初始化内容。移动可执行文件也不会改变根目录。

生效根目录下的直接 Project 子目录都会参与发现，其中包括 `SettingWarehouse`。`.cfgfc-session/` 和 `.cfgfc-transactions/` 是保留目录，不会被发现为资源。

## 原生 Windows 与 WSL

原生 `cfgfc.exe` 和 WSL 中的 Linux 二进制是两个独立运行环境：

- 原生 Windows 使用 `%USERPROFILE%`、Windows 路径语法和 Windows 符号链接权限；
- WSL 使用 Linux home、路径、权限和符号链接语义；
- cfgfc 不会把 `%USERPROFILE%` 转换成 `/mnt/c/...`，也不会在两个运行环境间转换仓库路径。

请选择文件系统语义与待管理目标一致的二进制。不要让原生 Windows 和 WSL 进程共享活动运行时、会话或事务状态。

## 可移植目标路径

目标目录可以使用 `~`、`${VAR}` 和 Windows `%VAR%`。展开使用当前运行环境的 home 与环境变量。目标名称必须是一个普通文件名或目录名。同一规划中的展开目标必须非空且唯一。

Setting 内容路径相对于 Setting 根目录。绝对路径、要求子路径时的空路径、`.`/`..` 遍历、逃逸路径和符号链接组件都会被拒绝。目录导入只接受普通文件和目录，不跟随符号链接。

## 上下文与事务

`cfgfc use` 会在生效根目录下按父进程 ID 保存 canonical Project 上下文。它只是多 shell 使用时的便利状态，不是安全或隔离边界。显式 `-p/--project` 优先。切换根目录也会切换上下文存储。

变更命令通过全仓锁串行执行，并在开始新操作前恢复未完成的 prepared 事务。只读 `status` 只报告事务诊断，不执行恢复。不要手工编辑或删除 `.cfgfc-transactions/`。

## 破坏性行为

`--force-targets` 只能递归回收当前 apply、rename、delete、refresh、reset 或 revert 操作涉及且已有记录的目标路径。它不会确认仓库删除（`--yes`）、授权依赖引用修复（`--cascade`），也不会备份或重建被覆盖的外部内容。

## 原生 Windows 冒烟测试

在开启 Developer Mode 或具有 Administrator 权限的 Windows shell 中：

1. 使用 `cfgfc root <Path>` 持久化临时根目录；
2. 使用资源/内容命令创建一个文件型和一个目录型 Setting；
3. 配置不同目标并应用；
4. 确认两个目标都是真实且可读的符号链接；
5. 故意替换目标所有权，确认普通操作会拒绝，而 `--force-targets` 只回收已记录路径；
6. 确认内容相对路径和符号链接组件会被拒绝。

需要 WSL 支持时，应在 WSL 中独立运行同等冒烟测试；一个运行环境的结果不能证明另一个运行环境。
