# 开发环境

## 工具链

- 语言：Go 1.24.4
- 环境管理器：`pixi`，开发时由 pixi 提供 Go 工具链
- 入口文件：`cmd/cfgfc/main.go`
- npm 分发包目录：`npm/`

## 基线命令

```bash
pixi run test
pixi run compile
pixi run build
pixi run help
pixi run bash -lc 'for cmd in new sync switch list apply update reset revert; do go run ./cmd/cfgfc "$cmd" --help; done'
```

## 验证要求

- 使用 `pixi run test` 运行完整 Go 测试套件。
- 使用 `pixi run compile` 确认所有 Go package 仍可编译。
- 使用 `pixi run build` 在 `dist/cfgfc` 生成本地 CLI 二进制文件。
- 使用 `pixi run help` 验证根命令面。
- 使用上面的子命令 help sweep，验证每个已注册命令都能通过 pixi 管理的 Go 工具链返回结构化帮助。
- 如果改动涉及命令行为，还要针对临时的 `~/.configfacilitator/` 做一次真实 CLI smoke test。

## npm 包与发布流程

`npm/` 下的 npm 包只是 Go 二进制文件的薄包装层。它的 `postinstall` 脚本会从与 `npm/package.json` 版本匹配的 GitHub Release 下载对应平台的二进制文件。

发布前需要执行：

```bash
pixi run test
pixi run compile
pixi run build
cd npm
npm pack --dry-run
CFGFC_BINARY_PATH=../dist/cfgfc npm install -g .
cfgfc --help
CFGFC_TEST_PLATFORM=freebsd CFGFC_TEST_ARCH=x64 node install.js
```

预期发布顺序：

1. 确认 `npm/package.json` 版本为 `X.Y.Z`。
2. 推送 Git tag `vX.Y.Z`。
3. 由 GoReleaser 发布 GitHub Release 产物，例如 `cfgfc_X.Y.Z_linux_amd64.tar.gz` 和 `checksums.txt`。
4. 只有在匹配版本的 GitHub Release 产物存在后，才发布 npm 包。

## 文档维护流程

当改动影响用户可见行为、命令面、项目结构或开发工作流时，需要在同一次修改中同步更新 `docs/` 下对应的英文和中文文档。
