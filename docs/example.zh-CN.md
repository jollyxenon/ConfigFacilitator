# 工作流示例

本页展示一个贴近 OpenCode 使用方式的真实工作流：你把多个模型配置文件和技能目录维护在同一个仓库里，先临时应用一个 Setting 做快速测试，再在需要完整环境时切回一个命名好的 Mode。完整命令面请参考 [命令参考](commands.zh-CN.md)，字段级 JSONC 规则请参考 [JSONC 指南](jsonc-guide.zh-CN.md)。

## 目标

在这个示例里，`OpenCode` 包含两个 Column：

- `oh-my-openagent`：存放 `OMOMax.json`、`OMOLight.json` 这类模型配置文件
- `Skills`：存放 `Skill-A`、`Skill-B` 这类技能目录

这个工作流解决两个常见需求：

- 临时应用一个配置文件做快速测试
- 用一个命名好的 Mode 恢复完整工作环境

## 仓库结构

仓库根目录固定在 `~/.configfacilitator/`。完成骨架创建和手工编辑后，一个有代表性的目录结构如下：

项目目录会直接从 `~/.configfacilitator/` 根目录下发现。根目录下名为 `SettingWarehouse` 的目录也会和其他项目目录一样参与发现。

```text
~/.configfacilitator/
├── ProjectIndex.jsonc
└── OpenCode/
    ├── Column/
    │   ├── ColumnIndex.jsonc
    │   ├── oh-my-openagent/
    │   │   ├── SettingIndex.jsonc
    │   │   ├── OMOMax.json
    │   │   └── OMOLight.json
    │   └── Skills/
    │       ├── SettingIndex.jsonc
    │       ├── Skill-A/
    │       └── Skill-B/
    ├── Mode/
    │   └── ModeIndex.jsonc
    └── Backup/
        ├── current_state.json
        └── history.log
```

`Backup/current_state.json` 和 `Backup/history.log` 是 `apply`、`reset`、`revert` 使用的状态文件。把它们当作运行时数据，不要手工编辑。

## 1. 先搭建项目、栏目和模式骨架

先生成项目骨架，以及后面要补全的模板文件：

```bash
cfgfc new -p OpenCode
cfgfc new -p OpenCode -c oh-my-openagent
cfgfc new -p OpenCode -c Skills
cfgfc new -p OpenCode -m Max
```

`new` 只会创建 Project、Column、Mode 的模板，不会直接替你创建具体 Setting。每个 Column 里的真实文件或目录，需要你自己放进去。

## 2. 加入真实内容并手工编辑 JSONC

完成脚手架后，把真实配置内容放进仓库：

- 把 `OMOMax.json` 和 `OMOLight.json` 放到 `OpenCode/Column/oh-my-openagent/`
- 把 `Skill-A/` 和 `Skill-B/` 放到 `OpenCode/Column/Skills/`

然后手工编辑各个 JSONC 索引。这些手工编辑本来就是当前设计的一部分。

### 项目和栏目的元数据

用 `description` 写长期说明，用 `displayName` 控制展示名称，用 `aliases` 提供额外 CLI 引用。`displayName` 只是展示字段，不会自动变成 CLI 别名，而顶层 key 才是 canonical 标识。

```jsonc
// ProjectIndex.jsonc
{
  "OpenCode": {
    "displayName": "OpenCode",
    "aliases": ["oc"],
    "description": "OpenCode 工作区配置集合"
  }
}
```

```jsonc
// OpenCode/Column/ColumnIndex.jsonc
{
  "oh-my-openagent": {
    "displayName": "主配置",
    "aliases": ["omo"],
    "description": "主要模型配置文件"
  },
  "Skills": {
    "displayName": "Skills",
    "aliases": ["ski"],
    "description": "技能目录集合"
  }
}
```

顶层 key 本身就是 canonical 标识，而 `displayName` 与 `aliases` 则作为围绕这个 key 的书写元数据保留下来。

### Setting 的目标路径

`defaultTarget` 是 Column 级默认目标路径。Setting 级的 `target` 会覆盖它。

```jsonc
// OpenCode/Column/oh-my-openagent/SettingIndex.jsonc
{
  "description": "主配置集合",
  "defaultTarget": "~/.config/opencode/oh-my-openagent.jsonc",
  "settings": {
    "OMOMax.json": {
      "displayName": "OMOMax 配置",
      "aliases": ["max"],
      "description": "OMOMax 模型配置"
    },
    "OMOLight.json": {
      "displayName": "OMOLight 配置",
      "aliases": ["light"],
      "description": "OMOLight 模型配置"
    }
  }
}
```

```jsonc
// OpenCode/Column/Skills/SettingIndex.jsonc
{
  "description": "Skills 栏目",
  "settings": {
    "Skill-A": {
      "displayName": "Skill A",
      "aliases": ["a"],
      "description": "第一个技能目录",
      "target": "~/.config/opencode/skills/Skill-A"
    },
    "Skill-B": {
      "displayName": "Skill B",
      "aliases": ["b"],
      "description": "第二个技能目录",
      "target": "~/.config/opencode/skills/Skill-B"
    }
  }
}
```

路径支持 `~`、`${VAR}` 和 Windows `%VAR%` 形式。

### Mode 选择与策略

`ModeIndex.jsonc` 也需要手工编辑。`settings` 用来声明每个 Column 要包含哪些 Setting，`strategy` 用来控制应用时的替换策略。

```jsonc
// OpenCode/Mode/ModeIndex.jsonc
{
  "Max": {
    "displayName": "Max",
    "aliases": ["m"],
    "description": "完整 OpenCode 工作区",
    "columns": {
      "oh-my-openagent": {
        "settings": ["OMOMax.json"],
        "strategy": "full"
      },
      "Skills": {
        "settings": ["Skill-A", "Skill-B"],
        "strategy": "incremental"
      }
    }
  }
}
```

`full` 会先清理该 Column 之前受管的链接，再重新应用；`incremental` 会保留现有链接，并继续追加新的链接。

## 3. 手工编辑后执行 sync

当文件内容和 JSONC 索引都准备好以后，用 `sync` 让仓库与磁盘现实重新对齐：

```bash
cfgfc sync -p OpenCode
```

当你只想同步一个项目时，使用 `cfgfc sync -p <project>`。如果当前没有活动项目，直接执行 `cfgfc sync` 会回退到同步整个仓库。`cfgfc sync --all` 和 `cfgfc sync -a` 则总是强制全仓同步，并忽略当前活动项目上下文。

## 4. 切换上下文并检查结果

在省略 `-p` 之前，先把当前终端会话绑定到 `OpenCode`：

```bash
cfgfc switch OpenCode
cfgfc list
cfgfc list -c Skills
cfgfc list -m Max
```

执行 `cfgfc switch OpenCode` 之后，后续带项目作用域的 `new`、`sync`、`list`、`apply`、`reset`、`revert` 才可以省略 `-p`。`cfgfc switch global` 会清除这个基于 PPID 的便利上下文，让后续命令重新回到全局解析模式。

## 5. 应用单个 Setting，或应用完整 Mode

如果只是想临时测试一个配置文件，可以先应用单个 Column 里的一个 Setting：

```bash
cfgfc apply -c oh-my-openagent -s OMOLight.json
```

这种形式适合把单个配置文件临时链接到 `~/.config/opencode/oh-my-openagent.jsonc`。

当你想恢复完整工作环境时，再应用命名好的 Mode：

```bash
cfgfc apply -m Max
```

在这个示例里，`apply -m Max` 会先把 `oh-my-openagent` 的目标链接替换成 `OMOMax.json`，因为这个 Column 使用了 `full`；随后再把 `Skill-A` 和 `Skill-B` 的目录链接补上，因为 `Skills` 使用的是 `incremental`。

## 6. 用 revert 或 reset 恢复环境

如果你想恢复到上一次 `apply` 之前的状态，用 `revert`；如果你想把当前项目的受管链接全部移除，用 `reset`。

```bash
cfgfc revert
cfgfc reset
```

`revert` 只支持单步回退：它恢复的是上一次 `apply` 的快照，而不是任意历史点。`reset` 会移除当前项目受管的映射，并清空该项目的当前状态。

## 什么时候再去看参考文档

把这页当作一个代表性工作流，而不是唯一支持的使用形态。涉及标识字段时，请始终以顶层 key、`displayName` 与 `aliases` 这一套模型为准。

- 需要完整、和帮助输出对齐的命令形式时，请看 [命令参考](commands.zh-CN.md)。
- 需要精确确认标识字段和目标路径规则时，请看 [JSONC 指南](jsonc-guide.zh-CN.md)。
