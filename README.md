# ConfigFacilitator

ConfigFacilitator is a portable Go CLI for managing a configuration warehouse through Project, Column, Setting, and Mode resources. Normal setup, content editing, target configuration, application, rename, deletion, and reconciliation are CLI-only; direct index editing is not required.

ConfigFacilitator 是一个便携式 Go CLI，通过 Project、Column、Setting 和 Mode 资源管理配置仓库。常规的创建、内容编辑、目标配置、应用、重命名、删除和同步流程都只使用 CLI，不需要直接编辑索引。

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

Use `setting content` to inspect or change Setting bytes. Existing managed symlinks expose byte changes immediately; use `refresh` when current metadata or persisted apply intent must be replanned.

使用 `setting content` 查看或修改 Setting 内容。已有受管符号链接会立即反映内容字节变化；只有当前元数据或已持久化应用意图需要重新规划时，才使用 `refresh`。

`sync` reconciles changes made by Git or another external tool. When an indexed Project directory, Column directory, or Setting file/directory disappears, sync immediately removes its metadata from the corresponding Index without recreating the source. Sync does not implicitly cascade Mode/runtime references; later apply or refresh fails if those references cannot resolve. There is no `sync --prune --yes` workflow, and recreating a former source path does not restore deleted metadata.

`sync` 用于同步 Git 或其他外部工具造成的变化。已索引的 Project 目录、Column 目录或 Setting 文件/目录消失时，sync 会立即从对应 Index 移除其元数据，且不会重建来源。Sync 不会隐式级联 Mode/runtime 引用；后续 apply 或 refresh 若无法解析这些引用会失败。不存在 `sync --prune --yes` 流程，重新创建旧来源路径也不会恢复已删除元数据。

Destructive controls are independent: `--yes` confirms resource, Column-target, or Setting-content deletion; `--cascade` permits dependent-reference repair during resource deletion; and `--force-targets` permits reclamation of recorded drifted or occupied targets.

三个破坏性控制彼此独立：`--yes` 确认资源、Column 目标或 Setting 内容删除；`--cascade` 允许资源删除时修复依赖引用；`--force-targets` 允许回收已记录但发生漂移或被占用的目标。

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
