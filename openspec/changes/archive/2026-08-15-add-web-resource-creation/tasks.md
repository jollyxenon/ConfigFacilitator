## 1. Web API 创建命令

- [x] 1.1 扩展 `internal/web/server.go` 的 command payload 和命令注册表，加入 `column.create`、`setting.create`、`mode.create`，保留现有 revision 检查、成功 snapshot 响应和错误 envelope 语义。
- [x] 1.2 实现共享 Web 元数据转换与校验：将展示名称、描述和 aliases 转换为 `mutate.Metadata`，复用 canonical/alias 领域校验，不复制 CLI 规则。
- [x] 1.3 接入三类资源的事务创建：Column 创建零 target，Mode 创建空 selections，Setting 根据 `file`/`directory` 创建来源；只接受空值或 UTF-8 初始内容，拒绝目录型非空内容和不支持的 encoding。
- [x] 1.4 为 API 增加回归测试，覆盖三类成功创建、文件内容字节、目录 Setting、名称/alias 冲突、错误时无部分写入，以及过期 revision 返回 409 且不创建资源。

## 2. Web UI 创建入口与表单

- [x] 2.1 在 `internal/web/static/index.html` 和现有导航/资源区域增加 Column、Setting、Mode 创建入口，并明确当前 Project/Column 作用域。
- [x] 2.2 在 `internal/web/static/app.js` 中实现可复用的创建表单 modal：canonical 名称、展示名称、描述、aliases；Setting 表单增加类型和 UTF-8 内容，并在切换到 directory 时隐藏或清空不适用内容字段。
- [x] 2.3 实现创建请求提交、字段校验、错误展示和 revision 冲突处理；失败时保留表单内容，成功时以服务端 snapshot 替换状态并清理表单。
- [x] 2.4 实现成功后的定位与内容初始化：Column 选中新 Column，Setting 选中新 Column/Setting 并打开内容编辑，Mode 打开新 Mode；确保无需手工刷新或 sync。
- [x] 2.5 在 `internal/web/static/styles.css` 及动态 modal 样式中补齐表单、字段错误、类型切换和窄窗口布局，保持现有零依赖嵌入式 UI 与可键盘操作性。

## 3. 文档与验证

- [x] 3.1 更新中英文 Web/命令文档，说明 Web UI 创建入口、三类资源的初始状态、Setting UTF-8/空目录限制和 revision 冲突行为，保持现有 CLI 说明不变。
- [x] 3.2 运行 Web 包及完整 Go 测试、编译和前端静态回归检查；覆盖创建后导航、失败保留表单和重复提交不会生成重复资源。
- [x] 3.3 运行 OpenSpec 严格校验，确认 proposal、spec、design、tasks 与实现范围一致。

## 4. Web API Index 编辑与重命名

- [x] 4.1 扩展 Web command payload 和注册表，加入 Column/Setting 元数据更新、Column/Setting target 编辑和 canonical rename 命令；保持 revision、snapshot 和错误 envelope 语义。
- [x] 4.2 接入 `mutate.SetColumn`、`mutate.SetSetting` 及现有 Column/Setting target mutation，覆盖 Column target 增删改、Setting target 覆盖/继承恢复，并保留目标规划校验。
- [x] 4.3 接入 `mutate.RenameColumn` 与 `mutate.RenameSetting`，复用来源移动、Index 键、Mode/Current/历史引用和 `forceTargets` 事务语义。
- [x] 4.4 增加 API 回归测试，覆盖元数据更新、Column target 增删改、Setting 覆盖/恢复继承、rename 成功、冲突、过期 revision 和失败时无部分写入。

## 5. Web UI Index 编辑与重命名

- [x] 5.1 在 Column/Setting 详情中增加 Index 编辑和 canonical rename 入口，明确 Column 默认 targets 与 Setting 覆盖 targets 的区别。
- [x] 5.2 实现可复用 Index 编辑 modal：元数据字段、Column target 数组增删改、Setting 每位置目录/名称覆盖与独立继承恢复。
- [x] 5.3 实现编辑提交、错误展示、revision 冲突和成功后 snapshot/导航更新；失败时保留当前表单输入。
- [x] 5.4 实现 Column/Setting rename modal，支持现有 unsafe target/forceTargets 授权流程，并在成功后定位新 canonical 资源。
- [x] 5.5 补齐窄窗口和键盘可操作样式，保持零依赖嵌入式前端。

## 6. 文档与验证

- [x] 6.1 更新中英文 README、命令、架构、开发环境、AGENTS 和 usage Skill，说明 Index 编辑字段、target 继承、rename 事务和 revision 冲突。
- [x] 6.2 运行完整 Go 测试、编译、构建、前端回归、OpenSpec 严格校验和差异检查。
