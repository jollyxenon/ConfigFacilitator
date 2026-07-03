# 命令参考

开发命令使用 pixi 管理的 Go 工具链：`pixi run compile` 检查所有 Go package，`pixi run build` 在 `dist/cfgfc` 生成本地 CLI 二进制文件。

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

查看项目、栏目、模式和子配置。如果当前没有有效项目，`list` 会输出仓库中的项目列表，并在每个项目后面追加一个括号包裹的使用状态摘要：当已持久化的 mode 意图仍然能匹配当前 mode 时，显示该 mode 名；否则显示 `Unmatched` 或 `None`。当存在有效项目时，直接执行 `list` 会显示该项目的栏目和模式，并在每个栏目后面追加一个括号包裹的 `Full`、`Partial` 或 `None` 标签，表示基于已持久化受管映射计算出的当前覆盖状态。执行 `cfgfc switch <project>` 后，项目作用域的 `list` 形式可以省略 `-p`。项目、栏目和模式引用都支持仓库侧标识符和别名。`list` 一次只接受一个详细目标：`-c` 或 `-m`。

当终端支持颜色输出时，项目作用域 `list` 里的活动 mode，以及 `list -c` 里当前启用的 settings 会被高亮显示。`list -c` 仍然会显示 missing entries。

```bash
cfgfc list
cfgfc list -p OpenCode
cfgfc list -p OpenCode -c Skills
cfgfc list -p OpenCode -m Max
```

## `apply`

应用模式，或显式应用一组栏目设置。`apply` 只接受两种形式：模式应用（`-m`）或单栏目应用（`-c` 配合 `-s`）。执行 `cfgfc switch <project>` 后，项目作用域的 `apply` 形式可以省略 `-p`。项目、栏目、模式和设置引用都支持仓库侧标识符和别名。`-s` 支持一个或多个以逗号分隔的设置名。`-f` / `--force` 会递归删除已占用的目标文件、符号链接或目录，让请求的受管状态即使在目标已失管或状态漂移时也能重新生效。

```bash
cfgfc apply -p OpenCode -m Max
cfgfc apply -p OpenCode -c opencode.json -s GPT.json
```

## `update`

根据当前仓库元数据刷新上一次应用意图。`update` 会读取项目的 `Backup/current_state.json`；如果当前状态记录了模式应用或单栏目应用意图，`update` 会用当前索引重新规划该意图并提交刷新后的映射集合。因此，当某个模式栏目使用 `full` 策略时，原始 `apply` 之后新同步进索引的设置也能被纳入刷新结果。

当已经应用过配置后又修改了仍处于活动状态的配置元数据时，可以使用 `update`。例如：先执行过 `apply`，之后新增技能目录或调整设置目标；这时如果新文件或目录需要写入索引，应先运行 `cfgfc sync`，再运行 `cfgfc update` 刷新活动状态。如果当前状态没有记录应用意图，`update` 会把活动 source 匹配回当前元数据，并只刷新这些已有映射。

执行 `cfgfc switch <project>` 后，项目作用域的 `update` 形式可以省略 `-p`。项目和栏目引用支持仓库侧标识符和别名。`cfgfc update --all` 与 `cfgfc update -a` 会忽略当前活动上下文，枚举所有项目，并跳过既没有活动映射也没有应用意图的项目。`cfgfc update -c <column>` 或 `cfgfc update --column <column>` 只刷新所选栏目，同时保留其它栏目的当前映射；当状态中存在模式意图时，所选栏目会按该模式策略重新规划，因此 `full` 可以纳入新同步的设置。`-f` / `--force` 会递归回收已占用的目标路径，即使当前目标已失管或不再匹配记录所有权，也继续刷新。

```bash
cfgfc update -p OpenCode
cfgfc switch OpenCode
cfgfc update
cfgfc update -c Skills
cfgfc update --all
cfgfc update -a
```

## `reset`

移除当前项目的受管链接。执行 `cfgfc switch <project>` 后，`reset` 可以省略 `-p` 并使用活动项目上下文。`cfgfc reset -f` / `cfgfc reset --force` 会删除当前项目状态里记录的每一个目标路径，即使该路径已经偏离了记录 source 的所有权。

```bash
cfgfc reset -p OpenCode
```

## `revert`

恢复项目的上一次应用状态。执行 `cfgfc switch <project>` 后，`revert` 可以省略 `-p` 并使用活动项目上下文。`cfgfc revert -f` / `cfgfc revert --force` 会递归回收已占用的目标路径，从而在失管冲突或状态漂移时仍可恢复上一个受管快照。

```bash
cfgfc revert -p OpenCode
```

## 说明

- 执行 `cfgfc switch` 后，带项目作用域的 `new`、`sync`、`list`、`apply`、`update`、`reset` 与 `revert` 命令都可以省略 `-p`。
- 即使用户输入的是别名，`switch` 保存到会话里的也始终是规范化后的项目标识符。
- 执行 `cfgfc switch global` 后，会清除当前 PPID 的活动项目上下文；之后 `list` 会回到全局项目列表，`sync` 会回到默认的全仓同步回退行为。
- 使用 `sync` 同步索引，使用 `apply` 选择应当激活的配置，使用 `update` 在 source 元数据变化后重新规划已持久化的应用意图。如果当前状态没有记录应用意图，则按映射逐项刷新当前映射。
- 模式策略共有 `cover`、`increment`、`none`、`full` 四种；在 `ModeIndex.jsonc` 中，只有 `none` 和 `full` 可以省略 `settings`。
- 强制操作只保证恢复到“上一次确认的受管状态”，不会备份或重建被覆盖的外部文件或目录内容。
