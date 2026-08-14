## Purpose

为本地 cfgfc Web UI 提供与 CLI 创建语义一致的 Column、Setting、Mode 创建入口，使用户可以在浏览器中完成资源初始化并立即继续配置。

## ADDED Requirements

### Requirement: Web UI SHALL create Column resources

在 Project 资源作用域中，系统 SHALL 提供创建 Column 的入口。创建表单 MUST 要求 canonical 名称，并 MAY 接受展示名称、描述和 aliases；提交成功后 SHALL 创建一个零 target 的 Column 及其空 Setting 索引，并刷新页面快照。

#### Scenario: 创建 Column

- **WHEN** 用户在某个 Project 中提交有效的 Column canonical 名称和可选元数据
- **THEN** Web UI SHALL 创建该 Column，显示成功反馈，并将用户定位到新 Column 的资源视图
- **AND** 新 Column 的 target 数量 SHALL 为零，Setting 列表 SHALL 为空

#### Scenario: Column 名称无效或冲突

- **WHEN** 用户提交空白、保留名称、已有 canonical 名称或与现有 alias 冲突的 Column 名称
- **THEN** 创建 SHALL 被拒绝并显示错误
- **AND** 仓库索引和文件系统 SHALL 不发生部分变更

### Requirement: Web UI SHALL create file- and directory-backed Settings

在选定 Column 的资源作用域中，系统 SHALL 提供创建 Setting 的入口。表单 MUST 要求 canonical 名称和 `file`/`directory` 类型；MAY 接受展示名称、描述和 aliases。文件型 Setting MAY 接受 UTF-8 初始内容，目录型 Setting SHALL 创建为空目录；创建出的 Setting SHALL 使用当前 Column 的 target 数量初始化继承数组。

#### Scenario: 创建带初始内容的文件型 Setting

- **WHEN** 用户选择 `file`，填写有效名称并提交 UTF-8 初始内容
- **THEN** 系统 SHALL 创建文件型 Setting，内容字节 SHALL 与提交内容一致，并刷新快照显示文件型 Setting

#### Scenario: 创建空目录型 Setting

- **WHEN** 用户选择 `directory`，填写有效名称并提交
- **THEN** 系统 SHALL 创建目录型 Setting 的空目录，创建表单 SHALL 不要求文件内容
- **AND** 用户 SHALL 能从内容编辑视图继续管理该目录内容

#### Scenario: Setting 创建参数无效

- **WHEN** 用户未选择类型、名称冲突、aliases 无效，或目录型 Setting 提交不支持的初始内容
- **THEN** 创建 SHALL 被拒绝并显示明确错误
- **AND** 不得留下不完整的 Setting 来源或索引条目

### Requirement: Web UI SHALL create empty Mode resources

在 Project 的 Mode 作用域中，系统 SHALL 提供创建 Mode 的入口。创建表单 MUST 要求 canonical 名称，并 MAY 接受展示名称、描述和 aliases；创建成功后 Mode SHALL 不包含 Column selection，页面 SHALL 刷新并定位到新 Mode。

#### Scenario: 创建空 Mode

- **WHEN** 用户提交有效的 Mode canonical 名称和可选元数据
- **THEN** 系统 SHALL 创建没有 Column selection 的 Mode，并打开新 Mode 的选择页面
- **AND** 用户 SHALL 能在该页面继续配置 Column strategy 和 Setting selection

#### Scenario: Mode 名称冲突

- **WHEN** 用户提交与已有 Mode canonical 名称或 alias 冲突的名称
- **THEN** 创建 SHALL 被拒绝并显示错误，已有 Mode selection SHALL 保持不变

### Requirement: Web resource creation API SHALL use warehouse mutation contracts

Web API SHALL 接受 `column.create`、`setting.create` 和 `mode.create` 命令。除只读命令外，每个命令 MUST 要求当前 warehouse `revision`；revision 不匹配时 SHALL 返回 HTTP 409 和 `revision_conflict` envelope。成功响应 MUST 返回包含新快照的标准成功 envelope；失败响应 SHALL 使用现有错误 envelope，并保证资源索引、来源和事务提交保持原子性。

#### Scenario: 使用当前 revision 成功创建

- **WHEN** Web 客户端提交受支持的创建命令和当前 `/api/snapshot` 返回的 revision
- **THEN** 服务 SHALL 执行对应资源创建并返回新的完整 snapshot 和成功消息

#### Scenario: 使用过期 revision 创建

- **WHEN** 另一个 CLI 或 Web 请求先修改仓库，随后客户端使用旧 revision 创建资源
- **THEN** 服务 SHALL 返回 HTTP 409、错误码 `revision_conflict`
- **AND** SHALL 不创建资源，客户端草稿数据 SHALL 可继续保留供重新提交

### Requirement: Creation UI SHALL preserve form state on recoverable failure

创建失败（包括校验错误、revision 冲突和后端拒绝）时，Web UI SHALL 保留用户已经填写的表单字段，并显示与错误 envelope 对应的可理解提示；成功后 SHALL 清除该表单并以服务端返回的 snapshot 为唯一状态来源。

#### Scenario: 创建被后端拒绝后修改并重试

- **WHEN** 用户提交创建请求失败并关闭错误提示
- **THEN** 表单 SHALL 仍包含原名称、类型、元数据和内容
- **AND** 用户修改错误字段后 SHALL 能使用最新 revision 再次提交

#### Scenario: 创建成功后更新导航

- **WHEN** 创建请求成功返回新 snapshot
- **THEN** 新资源 SHALL 出现在对应 Project 的 Column/Mode 导航和资源数据中
- **AND** UI SHALL 不依赖手工刷新浏览器或再次执行 `sync`

### Requirement: Web UI SHALL edit existing Column and Setting Index records

在已选 Project/Column 的资源视图中，系统 SHALL 提供编辑已有 Column/Setting Index 的入口。编辑 SHALL 支持展示名称、描述和 aliases；Column SHALL 支持 target 数量、默认目标目录和默认目标名；Setting SHALL 支持每个 target 位置的目录/名称覆盖以及分别恢复继承。所有修改 SHALL 保留未编辑的扩展字段，并在成功后返回新的完整 snapshot。

#### Scenario: 编辑 Column Index 元数据和默认 targets

- **WHEN** 用户修改已有 Column 的展示元数据或默认 target 数组并提交
- **THEN** 系统 SHALL 原子更新对应 Column/Setting Index，校验 target 数组长度和项目级目标冲突
- **AND** 页面 SHALL 显示更新后的 target 数量、目录和名称，不需要手工执行 `sync`

#### Scenario: 编辑 Setting Index 覆盖并恢复继承

- **WHEN** 用户为 Setting 的某个 target 位置设置目录或名称覆盖，或选择恢复继承
- **THEN** 系统 SHALL 只修改该 Setting 的对应覆盖字段，另一字段和其他位置保持不变
- **AND** 目标规划 SHALL 使用更新后的继承/覆盖结果

#### Scenario: Index 编辑失败不产生部分写入

- **WHEN** 用户提交无效 target 数组、空目标目录/名称、重复目标或过期 revision
- **THEN** 系统 SHALL 拒绝请求并返回现有错误 envelope
- **AND** 相关 Index、来源、Current 和表单草稿 SHALL 不产生部分变更

### Requirement: Web UI SHALL rename Column and Setting canonical names

在 Column 或 Setting 详情中，系统 SHALL 提供独立的 canonical 重命名入口。重命名 MUST 复用现有事务性 rename 领域语义，更新来源路径、对应 Index 键、Mode/Current/历史引用，并在目标被占用或漂移时遵循现有 `forceTargets` 授权流程。

#### Scenario: 重命名 Column 或 Setting

- **WHEN** 用户提交新的有效 canonical 名称
- **THEN** 系统 SHALL 原子移动存在的来源、更新相关 Index 和引用，并以新名称刷新导航与详情
- **AND** 原名称不再作为 canonical 资源键保留

#### Scenario: 重命名冲突或 revision 过期

- **WHEN** 新名称已被占用、目标不安全且用户拒绝授权，或请求 revision 已过期
- **THEN** 重命名 SHALL 被拒绝且不得留下部分移动、索引或引用更新
- **AND** Web UI SHALL 保留当前编辑输入并显示明确错误

### Requirement: Web resource Index editing API SHALL use warehouse mutation contracts

Web API SHALL 接受 Index 元数据/target 编辑和 Column/Setting rename 命令。所有可变命令 MUST 要求当前 warehouse `revision`，成功 SHALL 返回完整 snapshot；错误 SHALL 复用现有错误 envelope，并通过事务领域函数保证 Index、来源、Current、Mode 和历史引用的一致性。

#### Scenario: 使用当前 revision 保存 Index 编辑

- **WHEN** Web 客户端提交 Index 更新或 Column/Setting rename 命令，并携带当前 `/api/snapshot` 返回的 revision
- **THEN** 服务 SHALL 调用对应事务领域函数并返回新的完整 snapshot
- **AND** revision 不匹配时 SHALL 返回 HTTP 409、错误码 `revision_conflict`，且不执行任何 Index 或来源变更
