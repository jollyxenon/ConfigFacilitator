# Proposal: 在 Web UI 中增加 Column、Setting、Mode 创建与 Index 编辑能力

## Why

cfgfc 的本地 Web UI 已经能够浏览、编辑内容、调整 Mode 选择并删除资源，但创建资源和修改已有 Column/Setting Index 仍必须回到 CLI。这样用户无法在同一个界面完成资源初始化、目标配置、元数据维护和 canonical 重命名；详情页虽然展示了 Index 状态，关键字段却仍是只读。

## What Changes

- 在 Web UI 中提供创建 Column 的入口和表单。
- 在 Web UI 中提供创建 Setting 的入口和表单，支持选择 `file` 或 `directory`，并允许文件型 Setting 使用 UTF-8 初始内容创建；目录型 Setting 默认创建为空目录。
- 在 Web UI 中提供创建 Mode 的入口和表单。
- 三类创建表单支持与 CLI 创建命令一致的通用元数据：canonical 名称、展示名称、描述和逗号分隔 aliases。
- Web API 增加 `column.create`、`setting.create`、`mode.create` 命令，沿用已有 revision 冲突检查、事务持久化、稳定错误 envelope 和成功后返回完整 snapshot 的约定。
- 创建成功后 Web UI 自动刷新快照，并定位到新资源；创建失败时保留表单输入并展示可理解的错误。
- 新建 Column 继续遵循现有领域语义：初始为零个 target；新建 Mode 初始没有 Column selection；创建表单不重复实现 target 配置和 Mode selection。
- 在 Web UI 中提供已有 Column/Setting Index 编辑入口：通用元数据、Column 默认 target 数组、Setting target 覆盖与继承恢复。
- 在 Web UI 中提供 Column/Setting canonical 重命名入口，复用现有事务性 rename 语义并更新来源路径、索引和引用。

## Capabilities

### New Capabilities

- `web-resource-creation`: 通过本地 Web UI 和其 API 创建 Column、Setting、Mode。
- `web-resource-index-editing`: 通过本地 Web UI 和其 API 修改已有 Column/Setting Index 及 canonical 名称。

### Modified Capabilities

无。现有 CLI 创建、Index 修改和 rename 命令及资源存储语义不变；本变更只增加 Web 入口和对应 API 命令。

## Impact

- 影响 `internal/web/server.go` 的 API 命令注册、请求映射和创建执行逻辑。
- 影响 `internal/web/static/index.html`、`internal/web/static/app.js`、`internal/web/static/styles.css` 的入口、表单和状态反馈。
- 增加 Web API 和事务/快照回归测试，覆盖创建、Index 编辑、目标继承、canonical 重命名、revision 冲突、名称/别名校验和编辑后的导航。
- 不增加第三方前端依赖，不改变 CLI 命令面、仓库 JSONC 结构或已有删除/应用授权规则。
