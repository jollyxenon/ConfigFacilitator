# 命令参考

## `new`

搭建项目、栏目或模式骨架。

```bash
cfgfc new -p OpenCode
cfgfc new -p OpenCode -c Skills
cfgfc new -p OpenCode -m Max
```

## `sync`

同步仓库索引与文件系统。

```bash
cfgfc sync
cfgfc sync -p OpenCode
```

## `switch`

为当前 PPID 作用域会话保存活动项目。

```bash
cfgfc switch OpenCode
```

## `list`

查看项目、栏目、模式和子配置。

```bash
cfgfc list
cfgfc list -p OpenCode -c Skills
cfgfc list -p OpenCode -m Max
```

## `apply`

应用模式或单个栏目选择。

```bash
cfgfc apply -p OpenCode -m Max
cfgfc apply -p OpenCode -c opencode.json -s GPT.json
```

## `reset`

移除当前项目的受管映射。

```bash
cfgfc reset -p OpenCode
```

## `revert`

恢复项目的上一次应用状态。

```bash
cfgfc revert -p OpenCode
```

## 说明

- 执行 `cfgfc switch` 后，支持项目解析的命令可以省略 `-p`。
- 模式策略只有 `full` 和 `incremental` 两种。
