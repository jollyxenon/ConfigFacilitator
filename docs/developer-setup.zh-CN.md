# 开发环境

## 工具链

- 语言：Go 1.24.4
- 环境管理器：`pixi`
- 入口文件：`cmd/cfgfc/main.go`
- npm 分发包装目录：`npm/`
- 当前开发环境：WSL 中的 Linux；原生 Windows 行为需要单独做原生验证

## 基线命令

```bash
pixi run test
pixi run compile
pixi run build
pixi run help
```

## 完整 help sweep

扫描必须覆盖每个顶层命令族和每个可运行嵌套命令，而不是已移除命令：

```bash
pixi run bash -lc '
commands=(
  "project list" "project show" "project create" "project set" "project rename" "project delete"
  "column list" "column show" "column create" "column set" "column rename" "column delete"
  "column target list" "column target add" "column target set" "column target delete"
  "setting list" "setting show" "setting create" "setting set" "setting rename" "setting delete"
  "setting target list" "setting target set" "setting target reset"
  "setting content list" "setting content read" "setting content write"
  "setting content mkdir" "setting content move" "setting content delete"
  "mode list" "mode show" "mode create" "mode set" "mode rename" "mode delete"
  "mode column list" "mode column set" "mode column delete"
  "current show" "current column list" "current column set" "current column delete"
  "use" "status" "apply mode" "apply column" "refresh" "sync" "root" "reset" "revert" "web"
  "completion bash" "completion zsh" "completion fish" "completion powershell"
)
for cmd in "${commands[@]}"; do
  go run ./cmd/cfgfc $cmd --help >/dev/null || exit 1
done
'
```

还要确认根帮助只包含当前顶层命令面，并确认 `new`、`switch`、`list`、`update` 会作为用法错误被拒绝，且不发生变更。

## CLI 生命周期冒烟测试

命令或工作流变化时，使用临时 HOME/profile，并通过 `cfgfc root <Path>` 持久化一个独立备用根目录。只使用 `cfgfc` 和 stdin/重定向覆盖：

1. Project、Column、从零开始的目标、stdin 文件 Setting、目录 Setting 与嵌套内容、Mode 和 Mode 选择；
2. `apply mode`、`apply column`、人类模式 `status`（显示 `Current: following/independent [...]`）、`current show`、`--json` envelope 和退出码；
3. 仅修改内容字节，并确认活动链接立即可见；
4. 修改元数据后执行 `refresh`，`full` 选择加入新 Setting，以及修改被跟随 Mode 的选择时自动重新规划；
5. 重命名活动 Setting/Column/Mode/Project，并确认 canonical 上下文和引用更新；
6. 内容和资源删除，验证 `--yes`、`--cascade`、`--force-targets` 相互独立；
7. `reset` 和单步 `revert`；
8. 外部来源消失、sync 按 Project 隔离事务、索引与空 Current 重建、Mode/runtime 引用无法解析、apply/refresh 失败、不支持 prune 参数，以及通过资源命令显式重建；
9. prepared 事务诊断、重启恢复、回滚和并发变更锁；
10. 文件型与目录型目标所有权/漂移回收，并确认只处理已记录路径；
11. Web UI 冒烟：启动 `cfgfc web`，打开 `http://127.0.0.1:38031`，演练 `/api/snapshot` 和一次 `/api/command` 写入，确认过期 `revision` 返回 `409`、端口被占用时报持久化错误、Ctrl-C 干净退出。

主要生命周期不得通过直接编辑索引或来源来创建。只有明确测试 sync 互操作的部分才允许从外部删除文件；重建资源必须使用 cfgfc，不能声称 sync 会恢复已删除元数据。

## 文档与 OpenSpec 检查

仅文档变化至少运行：

```bash
openspec validate replace-editor-workflows-with-cli --strict
openspec status --change replace-editor-workflows-with-cli --json
```

保持 `README.md`、全部中英文文档对、`skills/configfacilitator-usage/SKILL.md`、`AGENTS.md` 和生成的 CLI 帮助一致。中英文页面必须描述相同的命令、示例、安全规则和开发流程。

## npm 包与发布流程

```bash
pixi run build
cd npm
npm pack --dry-run
CFGFC_BINARY_PATH=../dist/cfgfc npm install -g .
cfgfc --help
CFGFC_TEST_PLATFORM=freebsd CFGFC_TEST_ARCH=x64 node install.js
```

最后一个安装命令是用于检查不支持平台组合提示的预期失败路径。

预期发布顺序：

1. 把 `npm/package.json` 版本设为 `X.Y.Z`。
2. 推送 Git tag `vX.Y.Z`。
3. 由 GoReleaser 发布 `cfgfc_X.Y.Z_linux_amd64.tar.gz`、`checksums.txt` 等产物。
4. 只有匹配的 release 产物存在后，才发布 npm 包。

## 原生平台验证

Linux/WSL 测试通过 pixi 运行，但原生 Windows 路径解析、文件/目录符号链接、权限和目标回收必须使用原生 `cfgfc.exe` 验证。Windows 和 WSL 是不同运行环境，不能相互替代。
