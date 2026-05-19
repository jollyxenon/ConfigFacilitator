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
```

## 验证要求

- 使用 `pixi run test` 运行完整 Go 测试套件。
- 使用 `pixi run build` 确认项目仍可编译。
- 使用 `pixi run help` 验证根命令面。
- 如果改动涉及命令行为，还要针对临时的可执行文件同级 `SettingWarehouse/` 做一次真实 CLI smoke test。

## 文档维护流程

当改动影响用户可见行为、命令面、项目结构或开发工作流时，需要在同一次修改中同步更新 `docs/` 下对应的英文和中文文档。
