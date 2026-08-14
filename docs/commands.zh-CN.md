# 命令参考

请用 `cfgfc <command> --help` 查看已安装二进制生成的精确帮助。资源引用可以使用 canonical 名称或唯一别名；持久化引用始终使用 canonical 名称。

## 共享行为

### Project 作用域

`column`、`setting`、`mode`、`current`、`apply`、`refresh`、`status`、`reset`、`revert` 接受 `-p/--project`。没有该参数时，需要单个 Project 的命令会使用 `cfgfc use <Project>` 选择的 PPID 作用域 Project。显式作用域优先。`cfgfc use global` 清除上下文。`sync --all` 和 `refresh --all` 忽略上下文。

### 元数据参数

资源创建命令和资源 `set` 命令共用：

- `--display-name <Text>`
- `--description <Text>` 或 `--description-file <Path|->`
- `--aliases <CommaSeparatedValues>` 或 `--clear-aliases`

两个 description 来源互斥，两个别名控制也互斥。`--description-file -` 会把 stdin 的全部字节作为说明。`set` 命令至少需要一个元数据参数。

### JSON 与退出码

每个非 help 命令都接受 `--json`。

- 成功：stdout 只有一个对象：`{"ok":true,"data":{...}}`
- 失败：stderr 只有一个对象：`{"ok":false,"error":{"code":"...","message":"..."}}`
- JSON 模式不输出 ANSI 或额外说明文字。

| 退出码 | 含义 |
| --- | --- |
| `0` | 成功 |
| `2` | 用法或参数错误 |
| `3` | 资源/作用域缺失、歧义或冲突 |
| `4` | 仓库或资源数据无效 |
| `5` | 缺少确认，或目标所有权不安全而拒绝 |
| `6` | 文件系统、持久化或事务失败 |

### 相互独立的破坏性控制

- `--yes`：确认资源删除、Column 目标删除或内容删除。命令不会交互询问。
- `--cascade`：资源删除时，授权删除或修复依赖引用。
- `--force-targets`：目标已漂移或被占用时，只授权回收受影响且已有记录的目标路径。

任何一个参数都不会隐含其他参数。已移除的 `-f` 和 `--force` 无效。

## 资源命令

### Project

```text
cfgfc project list
cfgfc project show <Project>
cfgfc project create <Project> [元数据参数]
cfgfc project set <Project> <元数据参数>
cfgfc project rename <Old> <New> [--force-targets]
cfgfc project delete <Project> --yes [--cascade] [--force-targets]
```

`project create` 会一起提交完整 Project 结构、索引和运行时文件。`global` 是保留字。

### Column

所有形式都接受 `-p/--project` 或已选择上下文。

```text
cfgfc column list
cfgfc column show <Column>
cfgfc column create <Column> [元数据参数]
cfgfc column set <Column> <元数据参数>
cfgfc column rename <Old> <New> [--force-targets]
cfgfc column delete <Column> --yes [--cascade] [--force-targets]
```

新 Column 有零个目标位置，并带有空的可用 Setting 索引。

### Column 目标位置

下标从零开始。

```text
cfgfc column target list <Column>
cfgfc column target add <Column> --dir <Directory> (--name <Name> | --name-from-setting)
cfgfc column target set <Column> <Index> [--dir <Directory> | --clear-dir] [--name <Name> | --name-from-setting]
cfgfc column target delete <Column> <Index> --yes
```

增加或删除位置时，每个 Setting 的覆盖数组会同步调整。`--name-from-setting` 从每个 Setting 的 canonical 名称派生目标名称。

### Setting

每个形式都需要 `-c/--column`，并接受 `-p/--project` 或已选择上下文。

```text
cfgfc setting list -c <Column>
cfgfc setting show <Setting> -c <Column>
cfgfc setting create <Setting> -c <Column> --kind <file|directory> [元数据参数] [内容来源]
cfgfc setting set <Setting> -c <Column> <元数据参数>
cfgfc setting rename <Old> <New> -c <Column> [--force-targets]
cfgfc setting delete <Setting> -c <Column> --yes [--cascade] [--force-targets]
```

创建时最多接受一个内容来源：

- `--from <Path>`：`--kind file` 复制普通文件，`--kind directory` 复制普通目录树。
- `--stdin`：读取 stdin 的精确字节，仅支持 file。
- `--text <Text>`：保存参数的精确字节，不增加换行，仅支持 file。
- 不提供来源时创建空文件或空目录。

导入会拒绝符号链接和特殊文件系统对象。

### Setting 目标覆盖

```text
cfgfc setting target list <Setting> -c <Column>
cfgfc setting target set <Setting> <Index> -c <Column> [--dir <Directory> | --inherit-dir] [--name <Name> | --inherit-name]
cfgfc setting target reset <Setting> <Index> -c <Column>
```

目录继承和名称继承彼此独立。`reset` 会把两个部分都恢复为 Column 默认值。

### Setting 内容

每个形式都需要 `-c/--column`，并接受 `-p/--project`。

```text
cfgfc setting content list <Setting> -c <Column>
cfgfc setting content read <Setting> [RelativePath] -c <Column>
cfgfc setting content write <Setting> [RelativePath] -c <Column> (--from <RegularFile> | --stdin | --text <Text>)
cfgfc setting content mkdir <Setting> <RelativePath> -c <Column>
cfgfc setting content move <Setting> <OldPath> <NewPath> -c <Column>
cfgfc setting content delete <Setting> <RelativePath> -c <Column> --yes
```

文件型 Setting 的 read/write 必须省略 `RelativePath`。目录型 Setting 的 read/write 需要相对文件路径；`list`、`mkdir`、`move`、`delete` 只能在 Setting 根目录内操作。路径必须整洁、非绝对、不包含遍历片段，也不能经过符号链接组件。人类可读模式的 `read` 输出精确字节；JSON `read` 输出 UTF-8 文本，或明确标记的 base64 回退。

### Mode 与选择

```text
cfgfc mode list
cfgfc mode show <Mode>
cfgfc mode create <Mode> [元数据参数]
cfgfc mode set <Mode> <元数据参数>
cfgfc mode rename <Old> <New> [--force-targets]
cfgfc mode delete <Mode> --yes [--cascade] [--force-targets]
cfgfc mode column list <Mode>
cfgfc mode column set <Mode> <Column> --strategy <cover|increment|none|full> [--setting <Setting> ...] [--force-targets]
cfgfc mode column delete <Mode> <Column> [--force-targets]
```

`cover` 和 `increment` 必须重复提供一个或多个 `--setting`；`none` 和 `full` 不能提供该参数。删除 Mode 的 Column 选择不是删除资源，不使用 `--yes`。

修改 Current 正在 `following` 的 Mode 的选择，会在同一事务内自动重新规划并更新 Current 与链接；参见 [Current 状态](#current-状态临时-mode)。删除被跟随的 Mode 只清除 Current 的 `relation`，保留 columns 与 mappings；重命名被跟随的 Mode 会自动更新 `originMode`。`--force-targets` 只授权回收受影响且已有记录的漂移或占用目标。

## Current 状态（临时 Mode）

(Current) 是活动配置状态，本身就是一个「临时 Mode」：`columns` 是权威选择，`mappings` 是规划出的链接（`increment` 策略下还保存累积基线），`relation` 只描述与具名 Mode 的关系。`relation.kind` 只有两种：`following`（跟随具名 Mode，修改该 Mode 的选择会自动同步 Current，`sync` 也会按最新 Mode 选择重规划）与 `detached`（已分叉，修改 Mode 不影响 Current，`refresh` 只按自身 columns 重规划）。无 `relation` 表示独立。

```text
cfgfc current show [-p <Project>]
cfgfc current column list [-p <Project>]
cfgfc current column set <Column> --strategy <cover|increment|none|full> [--setting <Setting> ...] [-p <Project>] [--force-targets]
cfgfc current column delete <Column> [-p <Project>] [--force-targets]
```

`current show` 输出 relation、每个 Column 的选择和规划的映射。直接编辑 (Current)（`current column set/delete` 或 Web 提交）会把 `following` 改为 `detached` 并保留 `originMode`。删除被跟随的 Mode 只清除 `relation`，保留 columns 与 mappings；重命名被跟随的 Mode 会更新 `originMode`。

## 上下文、状态、应用与维护

### `use`

```bash
cfgfc use OpenCode
cfgfc use global
```

即使输入别名，当前 PPID 保存的也是 canonical Project。它只是便利状态，不是隔离或授权机制。

### `status`

```bash
cfgfc status
cfgfc status -p OpenCode
cfgfc status --json
```

没有有效 Project 作用域时，status 汇总所有 Project；有作用域时，报告 Current 状态——relation 类型、following 时的 origin Mode、每个 Column 的覆盖情况、规划的映射、无法解析的引用和未完成事务诊断。人类输出显示 `Current: following (Mode) [...]`、`Current: detached (Mode) [...]` 或 `Current: independent [...]`；JSON 通过 `current` 字段暴露同样的数据，包含 `columns`、`relation` 和 `mappings`。它是只读命令，不会恢复事务。

### `apply`

```bash
cfgfc apply mode <Mode> [-p <Project>] [--force-targets]
cfgfc apply column <Column> <Setting>... [-p <Project>] [--force-targets]
```

`apply mode <Mode>` 让 Current 跟随该 Mode，并按该 Mode 的最新选择重新规划映射。`apply column <Column> <Setting>...` 把 Current 设为只包含给定 Setting 的独立状态。无法解析的资源引用会在目标变化前阻止规划。

### `refresh`

```bash
cfgfc refresh [-p <Project>] [--force-targets]
cfgfc refresh --all [--force-targets]
```

`refresh` 根据当前元数据重新规划 Current。`detached` 或独立的 Current 按自身 `columns` 重新规划；`following` 的 Current 按 origin Mode 的最新选择重新规划。`--all` 刷新所有存在活动 Current 状态的 Project，不能与 Project 作用域组合。仅修改内容字节不需要 refresh，因为受管链接仍指向同一来源。已移除的 `--column` 参数无效。

### `sync`

```bash
cfgfc sync
cfgfc sync -p <Project>
cfgfc sync --all
```

Sync 是 Git、文件管理器或直接有效 JSONC 修改的同步边界。它会发现新资源，并立即删除来源已消失的 Project 目录、Column 目录、Setting 文件/目录对应的 Index 元数据。它不会重建来源，也不会隐式级联 Mode 选择、当前/历史 runtime 记录或 PPID 上下文；后续 `apply` 或 `refresh` 若无法解析这些引用，会在目标变化前失败。`sync --prune` 和 `sync --prune --yes` 均不受支持。重新创建旧来源路径不会恢复已删除元数据。`--all` 与 `-p/--project` 互斥。已移除的 `-a` 简写无效。

每个 Project 在各自的事务中提交。`sync --all` 独立处理每个 Project：失败的 Project 回滚自身变更，成功的 Project 保留结果，命令汇总输出并列明失败项。缺失的 Project/Column/Setting 索引条目会从文件系统重建（canonical 名称、最小默认元数据）；`ModeIndex.jsonc` 没有目录来源，缺失时只重建为空。`current_state.json` 缺失时重建为空 Current，并删除旧 `history.log`。旧格式或损坏的 `current_state.json` 只会让该 Project 保持不可用（其他 Project 不受影响），需要用户删除后重新 sync。`sync` 也会按 origin Mode 的最新选择重新规划 `following` 的 Current。

### `completion`

```bash
cfgfc completion bash
cfgfc completion zsh
cfgfc completion fish
cfgfc completion powershell
```

该命令把补全脚本写入 stdout。在可以解析当前作用域时，生成的补全会包含 Project、Column、Setting、Mode 的 canonical 名称和别名。

### `reset`、`revert` 与 `root`

```bash
cfgfc reset [-p <Project>] [--force-targets]
cfgfc revert [-p <Project>] [--force-targets]
cfgfc root [Path]
```

`reset` 删除当前受管映射，但保留仓库资源。`revert` 只恢复前一个快照，不支持任意历史检出。
`revert` 只回退 Current 状态（columns/relation/mappings），不回退资源元数据或内容。`root <Path>` 持久化后续根目录解析，不迁移、复制或初始化仓库内容。

### `web`

```bash
cfgfc web [--port <Port>]
```

`web` 在本机 `127.0.0.1` 上提供 Web UI（默认端口 `38031`）。前端嵌入单二进制，无需单独安装或联网；浏览器通过 `/api/snapshot`、`/api/command`、`/api/preview` 与后端交互。写命令携带全仓 `revision`，过期时返回 HTTP `409`。端口被占用时命令直接报错，不会悄悄换端口。按 Ctrl-C 退出。

## 旧命令到新命令迁移

| 已移除形式 | 当前形式 / 说明 |
| --- | --- |
| `cfgfc new -p P` | `cfgfc project create P` |
| `cfgfc new -p P -c C` | `cfgfc column create C -p P` |
| `cfgfc new -p P -m M` | `cfgfc mode create M -p P` |
| 手工创建 Setting 文件/目录 | `cfgfc setting create S -c C --kind file|directory`，配合 `--stdin`、`--text` 或 `--from` |
| 手工编辑 Setting 内容 | `cfgfc setting content read/write/mkdir/move/delete` |
| 手工编辑目标数组 | `cfgfc column target ...` 和 `cfgfc setting target ...` |
| 手工编辑 Mode 选择 | `cfgfc mode column set/delete ...` |
| `cfgfc switch P` | `cfgfc use P` |
| `cfgfc switch global` | `cfgfc use global` |
| `cfgfc list` | 运行时状态用 `cfgfc status`；资源用 `project/column/setting/mode list|show` |
| `cfgfc list -p P -c C` | `cfgfc setting list -p P -c C` 或 `cfgfc column show C -p P` |
| `cfgfc list -p P -m M` | `cfgfc mode show M -p P` 或 `cfgfc mode column list M -p P` |
| `cfgfc apply -p P -m M` | `cfgfc apply mode M -p P`（Current 将跟随该 Mode） |
| `cfgfc apply -p P -c C -s A,B` | `cfgfc apply column C A B -p P` |
| `cfgfc update -p P` | `cfgfc refresh -p P` |
| `cfgfc update -c C` | `cfgfc refresh`（单 Column 刷新已移除；refresh 重新规划整个 Current） |
| `cfgfc update --all` / `-a` | `cfgfc refresh --all` |
| `cfgfc sync -a` | `cfgfc sync --all` |
| `-f` / `--force` | 只有需要回收目标时使用 `--force-targets`；还要按需独立添加 `--yes` 和/或 `--cascade` |
| 消失资源在 sync 时保留元数据 | 当前 sync 会立即删除文件系统资源对应的 Index 元数据；不会级联引用，也不会恢复已删除元数据 |
