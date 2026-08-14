# ConfigFacilitator

ConfigFacilitator is a portable Go CLI and local Web UI for managing a configuration warehouse through Project, Column, Setting, and Mode resources. Normal setup, content editing, target configuration, application, rename, deletion, and reconciliation use the CLI or Web UI; direct index editing is not required.

ConfigFacilitator 是一个便携式 Go CLI 和本地 Web UI，通过 Project、Column、Setting 和 Mode 资源管理配置仓库。常规的创建、内容编辑、目标配置、应用、重命名、删除和同步流程可使用 CLI 或 Web UI，不需要直接编辑索引。

## Install / 安装

```bash
npm install -g @jollyxenon/cfgfc
cfgfc --help
```

The npm package is a thin installer for the matching prebuilt Go release binary.

npm 包是一个轻量安装包装层，用于安装匹配版本的预编译 Go 二进制文件。

## CLI lifecycle / CLI 生命周期

```bash
cfgfc root ~/.configfacilitator-work
cfgfc project create OpenCode --aliases oc
cfgfc use OpenCode
cfgfc column create Models
cfgfc column target add Models --dir ~/.config/opencode --name opencode.json
printf '%s' '{"model":"example"}' | cfgfc setting create Main.json -c Models --kind file --stdin
cfgfc mode create Default
cfgfc mode column set Default Models --strategy cover --setting Main.json
cfgfc apply mode Default
cfgfc status
```

Use `setting content` to inspect or change Setting bytes. Existing managed symlinks expose byte changes immediately; use `refresh` when the Current state must be replanned from current metadata.

使用 `setting content` 查看或修改 Setting 内容。已有受管符号链接会立即反映内容字节变化；只有需要根据当前元数据重新规划 Current 状态时，才使用 `refresh`。

`sync` reconciles changes made by Git or another external tool. When an indexed Project directory, Column directory, or Setting file/directory disappears, sync immediately removes its metadata from the corresponding Index without recreating the source. Sync does not implicitly cascade Mode/runtime references; later apply or refresh fails if those references cannot resolve. There is no `sync --prune --yes` workflow, and recreating a former source path does not restore deleted metadata.

`sync` 用于同步 Git 或其他外部工具造成的变化。已索引的 Project 目录、Column 目录或 Setting 文件/目录消失时，sync 会立即从对应 Index 移除其元数据，且不会重建来源。Sync 不会隐式级联 Mode/runtime 引用；后续 apply 或 refresh 若无法解析这些引用会失败。不存在 `sync --prune --yes` 流程，重新创建旧来源路径也不会恢复已删除元数据。

Destructive controls are independent: `--yes` confirms resource, Column-target, or Setting-content deletion; `--cascade` permits dependent-reference repair during resource deletion; and `--force-targets` permits reclamation of recorded drifted or occupied targets.

三个破坏性控制彼此独立：`--yes` 确认资源、Column 目标或 Setting 内容删除；`--cascade` 允许资源删除时修复依赖引用；`--force-targets` 允许回收已记录但发生漂移或被占用的目标。

## Web UI / Web 界面

`cfgfc web` serves a local Web UI on `127.0.0.1` (default port `38031`). The frontend is embedded in the single binary — no separate install, no network access, fully offline. Open http://127.0.0.1:38031 in a browser; the UI talks to the backend through `/api/snapshot`, `/api/command`, and `/api/preview`. Write commands carry the warehouse-wide `revision`; a stale revision returns HTTP `409`. An occupied port is a persistence failure, not an automatic fallback; press Ctrl-C to exit.

`cfgfc web` also provides creation buttons for Column, Setting, and Mode in the selected Project, plus Index editors on existing Column/Setting details. The editors update display metadata, descriptions, aliases, Column default targets, and Setting target overrides/inheritance; canonical renames are separate transactional actions that update sources and references. Column starts with zero targets, Mode starts without selections, and Web-created Settings support empty directories or UTF-8 file content. Mutating Web requests carry the current warehouse-wide `revision`; a stale revision returns HTTP `409` without partial changes.

`cfgfc web` 也会在选定 Project 中提供创建 Column、Setting 和 Mode 的按钮，并在已有 Column/Setting 详情中提供 Index 编辑。编辑器可以修改展示元数据、描述、别名、Column 默认目标和 Setting 目标覆盖/继承；canonical 重命名是独立的事务操作，会同步更新来源和引用。Column 初始没有 target，Mode 初始没有选择；Web 创建的 Setting 支持空目录或 UTF-8 文件内容。Web 变更请求携带全仓当前 `revision`，过期时返回 HTTP `409`，不会留下部分变更。

## Warehouse root / 仓库根目录

The default root is `~/.configfacilitator/`. `cfgfc root` prints the effective root; `cfgfc root <Path>` persists another root without moving, copying, or initializing existing contents. Direct child directories, including a directory named `SettingWarehouse`, may be discovered as Projects.

默认根目录为 `~/.configfacilitator/`。`cfgfc root` 查看当前生效根目录；`cfgfc root <Path>` 持久化切换根目录，但不会移动、复制或初始化现有内容。根目录下的直接子目录都可能被发现为 Project，其中也包括名为 `SettingWarehouse` 的目录。

## Documentation / 文档

- English: [docs/README.en.md](docs/README.en.md)
- 中文：[docs/README.zh-CN.md](docs/README.zh-CN.md)
- Agent guidance / Agent 指南：[skills/configfacilitator-usage/SKILL.md](skills/configfacilitator-usage/SKILL.md)

## Development / 开发

```bash
pixi run test
pixi run compile
pixi run build
pixi run help
```

See [Developer Setup](docs/developer-setup.en.md) or [开发环境](docs/developer-setup.zh-CN.md) for the complete help sweep, smoke tests, and release checks.

完整的帮助扫描、冒烟测试和发布检查见 [Developer Setup](docs/developer-setup.en.md) 或 [开发环境](docs/developer-setup.zh-CN.md)。

## License / 开源协议

MIT License. See [LICENSE](LICENSE).

使用 MIT License，见 [LICENSE](LICENSE)。
