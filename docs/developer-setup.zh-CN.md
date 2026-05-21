# 开发环境

## 工具链

- 语言：Go 1.24.4
- 环境管理器：`pixi`
- 入口文件：`cmd/cfgfc/main.go`

## 基线命令

```bash
pixi run test
pixi run build
pixi run help
pixi run bash -lc 'for cmd in new sync switch list apply reset revert; do go run ./cmd/cfgfc "$cmd" --help; done'
```

## 验证要求

- 使用 `pixi run test` 运行完整 Go 测试套件。
- 使用 `pixi run build` 确认项目仍可编译。
- 使用 `pixi run help` 验证根命令面。
- 使用子命令 help sweep，验证每个已注册命令都能通过 `cfgfc <command> --help` 返回结构化帮助。
- 如果改动涉及命令行为，还要针对临时的 `~/.configfacilitator/` 做一次真实 CLI smoke test。

## 文档维护流程

当改动影响用户可见行为、命令面、项目结构或开发工作流时，需要在同一次修改中同步更新 `docs/` 下对应的英文和中文文档。
