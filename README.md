# ConfigFacilitator

ConfigFacilitator is a portable Go CLI for managing Project, Column, Setting, and Mode data inside `~/.configfacilitator/`.

ConfigFacilitator 是一个便携式 Go CLI，用于管理位于 `~/.configfacilitator/` 中的 Project、Column、Setting 和 Mode。

Project directories are discovered directly under `~/.configfacilitator/`, including names such as `SettingWarehouse`.

项目目录会直接从 `~/.configfacilitator/` 根目录下发现；`SettingWarehouse` 这样的名称也会按普通项目目录处理。

## Purpose / 用途

- English docs: [docs/README.en.md](docs/README.en.md)
- 中文文档: [docs/README.zh-CN.md](docs/README.zh-CN.md)

## Installation / 安装

Build with Go 1.24.4 or run through `pixi`:

```bash
pixi run build
```

使用 Go 1.24.4 构建，或通过 `pixi` 运行：

```bash
pixi run build
```

## License / 开源协议

This repository does not currently include a `LICENSE` file.

本仓库当前未包含 `LICENSE` 文件。
