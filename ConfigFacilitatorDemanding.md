# ConfigFacilitator 制作要求

## 基本需求

我需要做一个配置管理器。这个配置管理器应该是这样的：

- 首先，管理器可以管理不同的项目 (**P**roject)。
- 对于每个项目，有不同的栏目 (**C**olumn) 可以管理。
- 每个栏目下有几个子配置 (**S**etting)。
- 对于每个项目，还可以创建模式 (**M**ode，即预设)。模式会为每个栏目选择指定的子配置。

这个软件应该是 portable（便携式）的，所有需要管理的配置放在统一的 `SettingWarehouse` 文件夹中，该文件夹固定存放在 `~/.configfacilitator/SettingWarehouse/`。系统的底层核心机制采用**软链接 (Symlink)** 驱动，并在每次应用配置时记录状态，以便于撤销和回溯。

## 简单示例

- 在 `SettingWarehouse` 中可以有不同的项目，如 `Claude Code`、`OpenCode` 等。
- 对于项目 `OpenCode` 而言，它可能有三个栏目。一个是主配置 `opencode.json`，一个是提示词配置 `oh-my-openagent.jsonc`，另一个是技能配置 `Skills`。
- 对于其中一个栏目，如 `oh-my-openagent.jsonc` 而言，下面可能有两种子配置：一种是更省钱的 OMO Light 配置，一种是能力更强的 OMO Max 配置。
- 另外，用户可以创建模式。比如，可以选择 `GPT.json` + `OMOLight.jsonc` + 部分 `Skills` 作为 Light 模式，或者选择 `CLAUDE.json` + `OMOMax.jsonc` + 所有 `Skills` 作为 Max 模式。

## 配置管理结构

所有的索引文件均采用 `.jsonc` 格式。系统在生成空模板时，会使用 `//` 注释来提供书写指引（告诉用户哪些项可以填什么）。**当用户完成书写并在随后执行同步时，这些 `//` 模板注释被程序抹除是正常且被允许的行为**。用户必须统一使用 JSON 中的 `"description"` 字段来承载永久性的说明与备注，该字段在数据序列化时绝对不会丢失。

`SettingWarehouse` 目录下除了各个项目文件夹外，还包含一个 `ProjectIndex.jsonc` 文件，记录每个项目的文件夹名、默认显示名和别名。

**项目层级划分**：每个项目文件夹下分为 `Column`、`Mode` 和 `Backup` 三个专门的子目录。

- **Backup 文件夹规则**：存放状态与应用历史。包含 `current_state.json`（记录当前项目在系统中建立了哪些软链接映射）以及 `history.log`（历史操作的时间戳记录）。
- **Column 文件夹规则**：
  - `Column` 目录下包含 `ColumnIndex.jsonc` 文件，纯粹用于记录每个栏目的文件夹名、默认显示名及说明（description）。
  - 每个具体的栏目文件夹内除了用户手动放置的配置文件/文件夹外，还包含 `SettingIndex.jsonc` 文件。
  - **`SettingIndex.jsonc` 的目标路径设计**：
    - `"defaultTarget"`：这是一个主要为了可读性和“单文件替换”场景存在的选项（例如 `~/.config/opencode/opencode.json`）。底下的各个 Setting 若未特别指定 `"target"`，则默认继承此路径。
    - `"target"`：如果某个栏目包含多个需要分别链接到不同位置的子配置（例如独立的技能 A 和技能 B），则完全可以不配置 `defaultTarget`，而是直接为每个 Setting 单独定义 `"target"`。**如果 Setting 中配置了 `"target"`，则无论是否存在 `defaultTarget`，都会以 Setting 的 `"target"` 为准。**
- **Mode 文件夹规则**：
  - `Mode` 目录下存放 `ModeIndex.jsonc` 文件，由用户通过文本编辑器手动维护（或通过 CLI 生成带注释的模板后填写）。
  - 记录模式的显示名称、默认名称、别名，及其对应的栏目映射关系。
  - **数据结构约束与覆盖策略**：针对 Mode 内的每一个 Column，用户可以指定应用的子配置，并选择覆盖策略。如果在模式中未写明某个栏目，则在应用该模式时，该栏目对应的现有链接默认会被**清空（撤销）**。如果声明了栏目，可以指定为“全量覆盖”（清理该栏目旧链接后再应用）或“增量覆盖”（保留该栏目已有链接，仅附加新链接）。

### 目录结构样例

```text
cfgfc
~/.configfacilitator/SettingWarehouse/
├── ProjectIndex.jsonc
└── OpenCode/
    ├── Backup/
    │   ├── current_state.json
    │   └── history.log
    ├── Column/
    │   ├── ColumnIndex.jsonc
    │   ├── opencode.json/
    │   │   ├── SettingIndex.jsonc
    │   │   ├── GPT.json
    │   │   └── CLAUDE.json
    │   └── Skills/
    │       ├── SettingIndex.jsonc
    │       ├── Skill-A/
    │       └── Skill-B/
    └── Mode/
        └── ModeIndex.jsonc
```

### `SettingIndex.jsonc` 样例

```jsonc
{
  "description": "这是 OpenCode 的主配置文件栏目，存放不同的模型选项。",
  "defaultTarget": "~/.config/opencode/opencode.json", 
  "settings": {
    "GPT.json": {
      "displayName": "GPT 模型",
      "description": "调用 OpenAI 接口，默认应用到 defaultTarget 路径"
    },
    "Special-Skill.json": {
      "displayName": "特殊技能",
      "description": "单独放到别处，覆盖 defaultTarget",
      "target": "~/.config/opencode/special/special.json" 
    }
  }
}
```

### `ModeIndex.jsonc` 样例

```jsonc
{
  "Max": {
    "displayName": "火力全开模式",
    "description": "应用 Claude 模型并开启全部技能",
    "columns": {
      "opencode.json": {
        "settings": "CLAUDE.json",
        "strategy": "full" // 全量覆盖，默认策略
      },
      "Skills": {
        "settings": ["Skill-A", "Skill-B"],
        "strategy": "incremental" // 增量覆盖
      }
    }
  }
}
```

## 核心机制说明

1. **软链接驱动 (Symlink-based) 与防冲突**：所有的 `apply` 操作本质上是在**目标绝对路径**建立指向 `SettingWarehouse` 内部真实文件的软链接。如果程序发现在准备创建软链接的 `target` 路径上，**已经存在非本系统生成的真实物理文件/文件夹**，程序必须挂起并**提示用户**（确认是否覆盖、备份或跳过），绝不能悄无声息地覆盖用户原有数据。
2. **状态记录与回溯**：在每次 `apply` 前，工具会读取当前项目的 `Backup/current_state.json`，撤销（Unlink）需要清理的旧软链接，然后再生成新的软链接。新生成的链接表会被重新写入该状态文件。
3. **会话级上下文 (Session Context)**：`switch` 提供的免除输入项目名（省略 `-p`）的上下文状态，通过**绑定终端父进程 ID (PPID)** 来实现。这样即使在多个终端窗口同时维护不同项目时，状态也绝对不会发生污染。
4. **路径变量解析**：为了保证真正的 Portable，配置中的路径（如 `target`）支持内置的基础路径变量解析，自动转换如 `~`、`${HOME}` 或 Windows 环境下的对应变量，以确保跨平台和跨设备的可用性。

## 使用方法

对于第一版 ConfigFacilitator，只配置 CLI 可用性，不配置 GUI。秉持“CLI 搭建骨架与同步，文本编辑器填充血肉”的设计哲学。

CLI 使用 `cfgfc` 唤起 ConfigFacilitator。
**核心缩写规范**：项目 `-p` (Project)、模式 `-m` (Mode)、栏目 `-c` (Column)、子配置 `-s` (Setting)。
**注意**：`switch` 只影响 `list`、`apply`、`reset` 和 `revert` 的项目上下文；`new` 与 `sync` 仍然需要显式的 `-p <ProjectName>`。

### 1. 搭建骨架 (`new`)

*说明：每次使用 `new` 指令时，自动生成带有 `//` 临时注释与 `"description"` 字段的空模板。*

- **新建项目**：`cfgfc new -p <ProjectName>`。生成标准化的 `Column`、`Mode` 与 `Backup` 目录，并更新 `ProjectIndex.jsonc`。
- **新建栏目**：`cfgfc new -p <ProjectName> -c <ColumnName>`。在指定的项目下新建一个栏目文件夹，并生成基础的 `SettingIndex.jsonc`。
- **新建模式**：`cfgfc new -p <ProjectName> -m <ModeName>`。在 `ModeIndex.jsonc` 中新增一个模板，自动列出当前已有的栏目供用户填空。

### 2. 同步数据 (`sync`)

- **全局同步**：`cfgfc sync`。自动扫描 `~/.configfacilitator/SettingWarehouse/` 下所有项目及栏目的实体文件，补全和修复所有的 Index 记录。
- **项目同步**：`cfgfc sync -p <ProjectName>`。仅扫描并同步指定项目。
- *同步逻辑说明*：如果发现新实体文件，则自动加入 JSON。**如果发现实体文件丢失（如被用户重命名或误删），系统会在 Index 中保留这些“游离节点”**（可标记为 missing 状态），绝不擅自删除该节点的配置条目，以保证用户之前手写的 `"description"` 备注与路径配置不会丢失。

### 3. 环境与信息查看 (`switch` / `list`)

- **切换上下文 (`switch`)**：`cfgfc switch <ProjectName>`。将当前终端 (基于 PPID) 的操作锁定到指定项目上。
- **列出资源 (`list`)**：`cfgfc list`。展示配置树，可以分级查看资源。
  - `cfgfc list` (无状态下列出所有 Project；switch 状态下列出当前 Project 的所有 Column 和 Mode)
  - `cfgfc list -p <ProjectName> -c <ColumnName>`：列出某个栏目下的所有 Setting。
  - `cfgfc list -m <ModeName>`：查看某模式的具体内容。

### 4. 应用配置 (`apply`)

- **应用模式 (Mode)**：`cfgfc apply -m <ModeName>`
  - 系统遍历模式中声明的 Column，根据设定的 `strategy` 决定是先清空该栏目的旧链接（全量）还是直接附加（增量）。
  - 对于模式中未声明的 Column，默认执行清空（撤销）操作。
- **应用单栏目 (Column)**：`cfgfc apply -c <ColumnName> -s <SettingName>`
  - 如果需要传入多个子配置，可通过逗号分隔，如：`-s "Skill-A,Skill-B"`
- **底层行为**：防冲突检测 -> 撤销需清理的软链接 -> 解析路径变量并创建新软链接 -> 写入新状态与时间戳日志。

### 5. 重置与回退 (`reset` / `revert`)

- **重置环境 (`reset`)**：`cfgfc reset -p <ProjectName>`
  - **底层行为**：读取 `Backup/current_state.json` 中的软链接列表，将它们从系统中安全移除（撤销该项目当前产生的所有链接），并清空状态文件，将目标环境彻底还原为无配置的干净状态。

- **回退历史 (`revert`)**：`cfgfc revert -p <ProjectName>`
  - **底层行为**：读取 `Backup/history.log` 与历史状态记录，解析出上一次执行 `apply` 操作前的软链接映射状态。系统会先安全清理当前的软链接，随后按照历史记录重新建立之前的软链接，将环境精准回退到上一个配置版本，而非单纯清空。

## 使用方法样例

### 场景：配置并应用 OpenCode 的 Max 模式

1. **新建项目与栏目**：

    ```bash
    cfgfc new -p OpenCode
    cfgfc switch OpenCode   # 切换上下文绑定当前终端进程，后续操作锁定 OpenCode，省略 -p
    cfgfc new -p OpenCode -c "opencode.json"
    cfgfc new -p OpenCode -c "Skills"
    ```

2. **填充内容与新建模式模板**：
    - 用户打开文件管理器，将 `CLAUDE.json` 和 `GPT.json` 拖入 `~/.configfacilitator/SettingWarehouse/OpenCode/Column/opencode.json/` 目录下。将具体的技能文件夹拖入 `Skills` 目录下。
    - 用户打开对应的 `SettingIndex.jsonc` 填写 `"defaultTarget"` 或 `"target"`，指定软链接生成的绝对路径（可使用 `~` 等变量）。
    - 生成模式模板：

        ```bash
        cfgfc new -p OpenCode -m Max
        ```

    - 用户使用文本编辑器打开生成的 `~/.configfacilitator/SettingWarehouse/OpenCode/Mode/ModeIndex.jsonc`，补全映射关系和覆盖策略。

3. **扫描同步与检视**：

   ```bash
   cfgfc sync
   cfgfc list -c "Skills"
   ```

   *(控制台输出：扫描完成。列表输出 Skills 下存在的各个技能配置状态。)*

4. **应用配置**：
   - 方式 A（应用完整模式，按照 ModeIndex 设定的覆盖策略自动处理）：

        ```bash
        cfgfc apply -m Max
        ```

   - 方式 B（应用单个栏目，例如只临时换一下模型配置；多运用逗号隔开）：

        ```bash
        cfgfc apply -c "opencode.json" -s "GPT.json"
        cfgfc apply -c "Skills" -s "Skill-A,Skill-B"
        ```

    *(控制台输出：已成功清理旧链接/保留增量，并将相关配置软链接至目标路径...)*
