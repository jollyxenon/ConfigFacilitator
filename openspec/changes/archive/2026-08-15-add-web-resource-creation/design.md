## Context

现有 Web Handler 已统一处理 `/api/snapshot`、`/api/command` 和 revision 校验，成功的变更命令会返回新的完整 snapshot。资源创建、Index 元数据/target 修改和 canonical rename 的事务领域函数已经存在于 `internal/mutate`，CLI 通过这些函数完成校验、索引写入、来源移动和引用更新；前端已有动态 modal host、Project/Column/Mode 导航和成功后 snapshot 替换机制。

## Goals / Non-Goals

### Goals

- 让 Web UI 的创建语义直接复用现有资源领域规则，而不是复制一份名称、alias 或文件系统校验。
- 让创建请求遵循现有 revision、错误 envelope 和原子事务约定。
- 让表单在文件型/目录型 Setting 之间动态切换，并在成功后把用户带到新资源。
- 保持当前零依赖、嵌入式、离线前端架构。
- 在同一资源视图中完成 Column/Setting Index 元数据、targets 和 canonical rename。

### Non-Goals

- 不新增 CLI 命令或兼容旧命令。
- 不在本次变更中实现 Mode selection 编辑；它继续使用现有 Mode 页面和保存流程。
- 不改变 Setting 来源内容编辑语义；Index 编辑只修改索引元数据和 target 配置。
- 不实现浏览器本地文件上传、目录上传或二进制初始内容；Setting 创建只接受 UTF-8 文本，目录型 Setting 必须为空创建。
- 不改变仓库 JSONC 格式、资源目录布局、Current 状态模型或删除授权规则。

## Decisions

### 1. Web API 直接调用现有 mutate/workflow 领域函数

在 `internal/web/server.go` 中注册创建、Index 修改和 Column/Setting rename 命令，并在执行分支中调用现有 `mutate.Create*`、`mutate.Set*`、target mutation 和 `mutate.Rename*` 函数。创建与编辑元数据时通过 `mutate.NewMetadata`/`MetadataPatch`，aliases 先按逗号拆分后交给领域校验；错误统一经过现有分类函数转换为 HTTP envelope。

选择直接调用而不是启动 CLI 子进程，因为 Handler 已拥有仓库根目录、环境和事务上下文；这样可以避免解析人类输出、重复 revision 检查，并保证 Web 与 CLI 共享同一套原子写入和错误代码。

### 2. 扩展现有 commandRequest，而不是新增 API 路由

沿用 `POST /api/command` 的 `command` 字段。创建继续使用 `name`、`displayName`、`description`、`aliases` 和 Setting 的 `column`、`kind`、`content`、`encoding` 字段；Index 编辑额外提交元数据 patch、Column target 位置或完整 target 数组、Setting target 覆盖/继承动作；rename 提交 `project`、`column`、旧引用和 `newName`。所有命令属于可变命令，因此自动要求当前 revision，并在成功后返回完整 snapshot。

Setting 的 `encoding` 只接受空值或 `utf8`。文件型 Setting 将内容转换为 `content.SourceBytes`；空内容仍创建空文件。目录型 Setting 的非空内容在 API 层作为参数错误拒绝，避免把文本误写成目录来源。

### 3. 使用现有 modal host 实现三个轻量表单

在现有资源导航和详情页增加创建与 Index 编辑入口：Project 资源页提供 Column/Setting 创建入口，Mode 分组提供 Mode 创建入口，Column/Setting 详情提供 Index 编辑和独立 rename 入口。表单复用现有 modal host 和动态样式，创建表单包含 canonical 名称、display name、description、aliases，以及 Setting 的 kind/content 字段；Index 表单同时展示 metadata 和目标位置；rename 表单只处理 canonical 名称。通过统一的表单提交函数构造 API payload，成功后清理 modal 和草稿，失败只更新错误区域，不销毁表单 DOM 或输入值。

这样无需引入前端框架或额外依赖，也不需要让后端暴露未持久化的临时资源状态。表单打开时绑定当前 Project/Column/Mode 作用域；提交前再次读取 `S.snap.revision`，避免把旧快照写入请求。

### 4. 成功定位规则显式处理三类资源

创建成功后使用服务端返回的 snapshot 作为状态基准：

- Column：切换到 Project 资源作用域，选中新 Column，Setting 为空选择。
- Setting：保持 Project 资源作用域，选中新 Column 和新 Setting，并切到内容编辑 tab；文件型可立即读取，目录型显示空目录。
- Mode：切换到新 Mode 页面，初始化为空选择。

如果响应为 revision 冲突，复用现有冲突 modal，保留创建表单输入；用户选择重新加载后仍可重新打开或重试，不自动覆盖服务端数据。

## Risks / Trade-offs

- [Risk] 只支持 UTF-8 文本会让 Web 创建无法一次导入二进制或已有目录内容 → 在表单和错误提示中明确限制；用户可先创建资源，再使用现有 CLI/content 能力导入。
- [Risk] 新建 Column 没有 target 时立即应用会因目标无法解析而失败 → 在成功提示和 Column 详情中明确“零 target”；Index 编辑保存 target 前复用 planner 校验，不伪造默认目标。
- [Risk] 浏览器打开多个窗口会产生 revision 冲突 → 复用已有 409 处理并保留表单，不做静默重试。
- [Risk] 动态 modal HTML 可能出现字段转义或事件重复绑定 → 所有服务端/用户可见文本通过现有 `esc` 或 `textContent` 写入，提交事件使用 modal 生命周期内的一次性监听。

## Migration Plan

无需数据迁移。实现后旧仓库可直接使用新 Web API；新创建和编辑后的资源继续使用现有索引、目录和事务格式，旧 CLI 无需升级步骤。若回滚，仅移除 Web API 命令和 Web 入口，不影响已经成功创建、编辑或重命名的资源（它们仍可由 CLI 管理）。

## Open Questions

无。目录型 Setting 为空创建、文件型仅支持 UTF-8 初始文本、Index 编辑覆盖 1–3、canonical rename 独立于 Index 表单，以及不重复实现 Mode selection 编辑，已作为本变更边界确定。

### 5. Index 编辑复用现有目标领域函数

Column Index 编辑使用 `mutate.SetColumn` 更新通用元数据，并使用现有 `AddColumnTarget`、`SetColumnTarget`、`DeleteColumnTarget` 修改默认 target 位置；Setting Index 编辑使用 `mutate.SetSetting` 更新通用元数据，并使用 `SetSettingTarget`/`ResetSettingTarget` 修改覆盖。每个 Web 命令均经过 revision 检查和 planner 目标校验，成功后重新生成完整 snapshot。前端按单个逻辑操作提交，避免直接写 JSONC。

### 6. Canonical rename 作为独立危险操作

Column/Setting rename 使用 `mutate.RenameColumn`/`mutate.RenameSetting`，不与普通 Index 保存混合。这样可以明确区分“修改索引字段”和“移动来源路径、重写引用”的影响范围；现有 `execCommand` 的 unsafe target 授权流程继续处理 `forceTargets`。重命名成功后，前端以服务端 snapshot 重新定位新 canonical 资源。
