## Why

ConfigFacilitator CLI 目前没有版本显示功能。用户无法知道他们正在使用哪个版本的工具，这给问题排查和兼容性检查带来困难。添加 `--version` 标志是 CLI 工具的标准实践，可以提升用户体验和调试能力。

## What Changes

- 添加 `cfgfc --version` 和 `cfgfc -v` 命令行标志
- 显示当前版本号（初始版本为 1.0.0）
- 版本信息通过构建时注入，支持开发版本和发布版本
- 在帮助信息中显示版本标志

## Capabilities

### New Capabilities
- `version-command`: 提供版本显示功能，支持 `--version` 和 `-v` 标志

### Modified Capabilities
- `cli-runtime-bootstrap`: 需要在帮助信息中显示版本标志，并在 CLI 参数解析中添加版本处理

## Impact

- 修改 `internal/cli/cli.go` 以添加版本参数解析
- 修改 `pixi.toml` 构建配置以支持版本注入
- 添加版本相关的测试用例
- 更新帮助信息以包含版本标志