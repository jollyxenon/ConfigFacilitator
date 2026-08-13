# 纯 CLI 工作流示例

这个生命周期从空的备用仓库开始，只使用 `cfgfc` 和 shell 输入重定向。它会创建元数据、目标、文件型和目录型 Setting、内容和 Mode；完成应用、修改、重命名、删除，并安全同步外部变化。

## 1. 选择空仓库并创建资源

```bash
cfgfc root ~/.configfacilitator-demo
cfgfc project create OpenCode \
  --display-name "OpenCode Demo" \
  --aliases oc \
  --description "CLI-managed OpenCode configuration"
cfgfc use oc

cfgfc column create Models --description "Main model file"
cfgfc column target add Models \
  --dir ~/.config/opencode \
  --name opencode.json

cfgfc column create Skills --description "Installed skill directories"
cfgfc column target add Skills \
  --dir ~/.config/opencode/skills \
  --name-from-setting
```

这些 CLI 自有的创建操作完成后，资源立即可用，不需要再运行 `sync`。目标下标从零开始。Models 目标使用固定名称；每个 Skills 目标从 Setting 的 canonical 名称派生名称。

## 2. 创建文件型和目录型 Setting

从 stdin 创建精确文件内容。这里使用 `printf`，因为它不会增加换行：

```bash
printf '%s' '{"model":"provider/example"}' | \
  cfgfc setting create Main.json \
    -c Models \
    --kind file \
    --stdin \
    --aliases main \
    --description "Primary model selection"
```

创建目录型 Setting，再通过 CLI 创建嵌套内容：

```bash
cfgfc setting create Review -c Skills --kind directory
printf '%s' '# Review skill' | \
  cfgfc setting content write Review SKILL.md -c Skills --stdin
cfgfc setting content mkdir Review references -c Skills
cfgfc setting content write Review references/checklist.md \
  -c Skills \
  --text 'Check scope, evidence, and rollback.'
```

还可以用 `--text <Text>` 提供文字文件字节，或用 `--from <Path>` 复制普通文件或目录树。`--from`、`--stdin`、`--text` 互斥。目录导入会拒绝符号链接和特殊对象。

查看结果：

```bash
cfgfc setting show Main.json -c Models
cfgfc setting content read Main.json -c Models
cfgfc setting content list Review -c Skills
cfgfc setting content read Review SKILL.md -c Skills
```

文件型 Setting 必须省略相对路径；目录型 Setting 的 read/write 要指定 Setting 根目录下的文件。

## 3. 创建并应用 Mode

```bash
cfgfc mode create Default --description "Complete OpenCode setup"
cfgfc mode column set Default Models \
  --strategy cover \
  --setting Main.json
cfgfc mode column set Default Skills --strategy full

cfgfc mode show Default
cfgfc apply mode Default
cfgfc status
```

`cover` 应用重复列出的 Setting。`full` 应用该 Column 中全部存在的 Setting。`increment` 也要求重复提供一个或多个 `--setting`，而 `none` 和 `full` 会拒绝 Setting 参数。

如果只想应用一个 Column：

```bash
cfgfc apply column Models Main.json
```

这会让 Current 不跟随任何 Mode：它变成只包含该 Column/Setting 选择的独立 Current。

查看 Current 状态及其与 Mode 的关系：

```bash
cfgfc current show
cfgfc current column list
cfgfc status
```

执行 `apply mode Default` 后，`status` 显示 `Current: following (Default) [...]`；执行上面的 `apply column` 后，Current 是独立的。直接编辑 (Current)——例如 `cfgfc current column set Models --strategy none`——会把 `following` 改为 `detached`，并保留 origin Mode 记录。

## 4. 修改内容和元数据

原子替换文件字节：

```bash
printf '%s' '{"model":"provider/new-example"}' | \
  cfgfc setting content write Main.json -c Models --stdin
```

活动符号链接仍然指向同一来源，所以新字节立即可见。仅修改内容字节时不要运行 `refresh`。

现在增加另一个目录型 Setting，并更新元数据：

```bash
cfgfc setting create Explain -c Skills --kind directory
cfgfc setting content write Explain SKILL.md \
  -c Skills \
  --text '# Explain skill'
printf '%s' 'Skills installed for this OpenCode profile.' | \
  cfgfc column set Skills --description-file -
```

如果 Current 正在跟随 `Default`，那么它的 `full` Skills 选择需要重新规划，才能包含 `Explain`：

```bash
cfgfc refresh
cfgfc status
```

使用 `cfgfc refresh` 根据当前元数据重新规划 Current，或使用 `cfgfc refresh --all` 刷新所有存在活动 Current 状态的 Project。已移除的 `--column` 参数无效；refresh 总是重新规划整个 Current。

## 5. 重命名活动资源

```bash
cfgfc setting rename Main.json Primary.json -c Models
cfgfc column rename Skills Extensions
cfgfc mode rename Default Work
cfgfc project rename OpenCode OpenCodeWork
cfgfc status
```

Rename 会在一个可恢复操作中重写 schema 定义的引用、Current 状态、来源路径、PPID Project 上下文和自有受管链接。固定目标名称保持不变；从 Setting canonical 名称派生的目标名称会随 Setting 改名。如果已记录目标发生漂移，rename 会停止，除非你明确添加 `--force-targets`。

重命名 Current 正在跟随的 Mode 时，会自动更新 Current 的 `originMode`。

## 6. 安全删除内容和资源

内容删除只删除目录型 Setting 下的路径，并要求确认：

```bash
cfgfc setting content move Review references/checklist.md notes.md -c Extensions
cfgfc setting content delete Review notes.md -c Extensions --yes
```

资源删除把三个决定分开。例如：

```bash
cfgfc setting delete Explain -c Extensions --yes --cascade
```

- `--yes` 确认删除。
- `--cascade` 允许修复依赖的 Mode、Current 状态和历史引用。
- 只有受影响的已记录目标被占用或所有权发生漂移时，才需要 `--force-targets`。

任何参数都不隐含另一个参数。`--force-targets` 不确认删除，也不授权 cascade。它只能回收受影响且已有记录的目标路径，无法重建被覆盖的外部内容。

## 7. 回退或重置受管状态

```bash
cfgfc revert
cfgfc reset
```

`revert` 只恢复前一个快照：它只回退 Current 状态（columns/relation/mappings），不回退资源元数据或内容。`reset` 删除当前受管映射，但保留仓库资源。只有明确接受回收受影响且已记录的漂移或占用路径时，才添加 `--force-targets`。

## 8. 同步外部变化

`sync` 用于同步资源/内容命令之外造成的变化，例如 Git checkout。假设外部操作删除了 `Review` 来源目录，运行：

```bash
cfgfc sync
cfgfc setting show Review -c Extensions
cfgfc status
```

Sync 会立即从 `SettingIndex.jsonc` 删除 `Review` 元数据；`setting show` 不再能解析它。Sync 不会重建来源，也不会隐式级联 Mode 选择、Current/历史 runtime 记录或 PPID 上下文。如果 apply 或 refresh 需要这个已移除的 Setting，会在受管目标变化前失败。每个 Project 在各自的事务中提交，`sync --all` 按 Project 隔离失败。

重新创建旧来源路径并运行 sync，可能会发现一个新的 Setting，但不会恢复已删除的说明、别名、目标覆盖、未知字段或其他元数据。请通过资源命令显式重建需要的元数据。

`sync --prune` 和 `sync --prune --yes` 均不受支持。全仓使用 `cfgfc sync --all`，显式 Project 使用 `cfgfc sync -p OpenCodeWork`；两个作用域互斥。资源 CLI delete 仍是独立流程，分别使用 `--yes`、`--cascade`、`--force-targets`。

## 9. 自动化

```bash
cfgfc status --json
cfgfc current show --json
cfgfc project show OpenCodeWork --json
```

JSON 成功结果以一个对象写入 stdout。JSON 失败结果以一个对象写入 stderr，并带稳定的 `error.code` 和 `error.message`。退出码分别表示用法错误（`2`）、资源/作用域错误（`3`）、无效数据（`4`）、拒绝（`5`）、持久化/事务错误（`6`）。
