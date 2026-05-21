# 命令参考

## `new`

搭建项目、栏目和模式模板。

支持的调用形式包括 `cfgfc new -p <project>`、`cfgfc new -p <project> -c <column>` 与 `cfgfc new -p <project> -m <mode>`。执行 `cfgfc switch <project>` 后，栏目和模式这两种形式可以省略 `-p`。已有项目既可以用仓库侧标识符引用，也可以用别名引用。在 `new` 命令里，`global` 是保留字，不能作为项目名使用。

```bash
cfgfc new -p OpenCode
cfgfc new -p OpenCode -c Skills
cfgfc new -p OpenCode -m Max
cfgfc switch OpenCode
cfgfc new -c Skills
cfgfc new -m Max
```

## `sync`

同步仓库索引与文件系统状态。当存在通过 `switch` 选中的活动项目时，`cfgfc sync` 只同步该项目；如果没有活动项目，`cfgfc sync` 会同步仓库中的全部项目。`cfgfc sync --all` 与 `cfgfc sync -a` 会强制同步整个仓库，并忽略当前活动项目上下文。项目引用支持仓库侧标识符和别名。在 `sync -p` 中，`global` 是保留字，不能作为项目名使用。

```bash
cfgfc sync
cfgfc sync -p OpenCode
cfgfc sync --all
cfgfc sync -a
```

## `switch`

为当前会话选择活动项目上下文。`cfgfc switch <project>` 会先通过仓库侧标识符或别名解析项目，再把规范化后的项目标识符保存到当前 PPID 作用域会话中；`cfgfc switch global` 会清除活动项目上下文，并让后续命令回到全局解析模式。

```bash
cfgfc switch OpenCode
cfgfc switch global
```

## `list`

查看项目、栏目、模式和子配置。如果当前没有有效项目，`list` 会输出仓库中的项目列表。执行 `cfgfc switch <project>` 后，项目作用域的 `list` 形式可以省略 `-p`。项目、栏目和模式引用都支持仓库侧标识符和别名。`list` 一次只接受一个详细目标：`-c` 或 `-m`。

```bash
cfgfc list
cfgfc list -p OpenCode -c Skills
cfgfc list -p OpenCode -m Max
```

## `apply`

应用模式，或显式应用一组栏目设置。`apply` 只接受两种形式：模式应用（`-m`）或单栏目应用（`-c` 配合 `-s`）。执行 `cfgfc switch <project>` 后，项目作用域的 `apply` 形式可以省略 `-p`。项目、栏目、模式和设置引用都支持仓库侧标识符和别名。`-s` 支持一个或多个以逗号分隔的设置名。

```bash
cfgfc apply -p OpenCode -m Max
cfgfc apply -p OpenCode -c opencode.json -s GPT.json
```

## `reset`

移除当前项目的受管链接。执行 `cfgfc switch <project>` 后，`reset` 可以省略 `-p` 并使用活动项目上下文。

```bash
cfgfc reset -p OpenCode
```

## `revert`

恢复项目的上一次应用状态。执行 `cfgfc switch <project>` 后，`revert` 可以省略 `-p` 并使用活动项目上下文。

```bash
cfgfc revert -p OpenCode
```

## 说明

- 执行 `cfgfc switch` 后，带项目作用域的 `new`、`sync`、`list`、`apply`、`reset` 与 `revert` 命令都可以省略 `-p`。
- 即使用户输入的是别名，`switch` 保存到会话里的也始终是规范化后的项目标识符。
- 执行 `cfgfc switch global` 后，会清除当前 PPID 的活动项目上下文；之后 `list` 会回到全局项目列表，`sync` 会回到默认的全仓同步回退行为。
- 模式策略只有 `full` 和 `incremental` 两种。
