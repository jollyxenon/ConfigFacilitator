# 平台说明

## 符号链接策略

ConfigFacilitator 只使用真实 symlink，不会退回到目录 junction 或文件复制。

## 便携式目录布局

仓库根目录固定解析为 `~/.configfacilitator/`，而不是按当前 shell 工作目录解析。移动二进制文件，不会改变生效的仓库根目录。

根目录项目发现会直接在 `~/.configfacilitator/` 下进行。只要目录符合项目布局，像 `SettingWarehouse` 这样的目录名也会和其他项目目录一样参与发现。

## 会话上下文

`switch` 按父进程 ID 保存活动项目。这是为了多终端并行使用而提供的便利能力，不是强隔离边界。

## 回退范围

`revert` 只支持单步回退，只会恢复到上一次 `apply` 的快照状态。
