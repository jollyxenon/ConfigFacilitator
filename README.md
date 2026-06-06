# ConfigFacilitator

ConfigFacilitator is a portable Go CLI for managing Project, Column, Setting, and Mode data inside `~/.configfacilitator/`.

ConfigFacilitator 是一个便携式 Go CLI，用于管理位于 `~/.configfacilitator/` 中的 Project、Column、Setting 和 Mode。

Project directories are discovered directly under `~/.configfacilitator/`, including names such as `SettingWarehouse`.

项目目录会直接从 `~/.configfacilitator/` 根目录下发现；`SettingWarehouse` 这样的名称也会按普通项目目录处理。

## Purpose / 用途

- English docs: [docs/README.en.md](docs/README.en.md)
- 中文文档: [docs/README.zh-CN.md](docs/README.zh-CN.md)

## Development build / 开发构建

Use pixi to run the Go toolchain. Check compilation with:

```bash
pixi run compile
```

Build a local CLI binary at `dist/cfgfc` with:

```bash
pixi run build
```

使用 pixi 运行 Go 工具链。检查编译：

```bash
pixi run compile
```

在 `dist/cfgfc` 生成本地 CLI 二进制文件：

```bash
pixi run build
```

## License / 开源协议

ConfigFacilitator is licensed under the MIT License. See [LICENSE](LICENSE) for the full terms.

ConfigFacilitator 使用 MIT License 开源。完整条款见 [LICENSE](LICENSE)。
