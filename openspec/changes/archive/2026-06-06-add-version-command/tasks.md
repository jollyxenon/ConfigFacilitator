## 1. 版本变量和解析实现

- [x] 1.1 在 `internal/cli/cli.go` 中添加包级版本变量 `var version = "dev"`
- [x] 1.2 创建 `isVersionArg()` 函数，检查 `--version` 或 `-v` 参数
- [x] 1.3 在 `run()` 函数中添加版本检查逻辑，在帮助检查之前
- [x] 1.4 实现版本显示功能，输出版本号到标准输出

## 2. 构建配置更新

- [x] 2.1 修改 `pixi.toml` 中的构建命令，添加 `-ldflags` 支持版本注入
- [x] 2.2 创建构建脚本或任务，支持指定版本号构建

## 3. 测试用例

- [x] 3.1 添加 `TestRunShowsVersionWithVersionFlag` 测试用例
- [x] 3.2 添加 `TestRunShowsVersionWithVFlag` 测试用例
- [x] 3.3 添加 `TestVersionFlagTakesPrecedence` 测试用例
- [x] 3.4 添加 `TestDefaultVersionIsDev` 测试用例

## 4. 帮助信息更新

- [x] 4.1 更新 `writeRootHelp()` 函数，在帮助信息中显示 `--version` 标志
- [x] 4.2 验证帮助信息包含版本标志

## 5. 集成验证

- [ ] 5.1 运行现有测试确保没有回归
- [x] 5.2 手动测试 `cfgfc --version` 和 `cfgfc -v` 功能
- [x] 5.3 验证版本注入构建是否正常工作