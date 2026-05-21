# ConfigFacilitator

ConfigFacilitator is a portable Go CLI for managing Project, Column, Setting, and Mode data inside `~/.configfacilitator/`.

ConfigFacilitator 是一个便携式 Go CLI，用于管理位于 `~/.configfacilitator/` 中的 Project、Column、Setting 和 Mode。

Project directories are discovered directly under `~/.configfacilitator/`, including names such as `SettingWarehouse`.

项目目录会直接从 `~/.configfacilitator/` 根目录下发现；`SettingWarehouse` 这样的名称也会按普通项目目录处理。

## Purpose / 用途

- English docs: [docs/README.en.md](docs/README.en.md)
- 中文文档: [docs/README.zh-CN.md](docs/README.zh-CN.md)

## Installation / 安装

Build locally:

```bash
pixi run build
```

Install `cfgfc` as a global command:

```bash
pixi run install-global
```

If `cfgfc` is still not found, add `GOBIN` or `$(go env GOPATH)/bin` to your `PATH`.

本地构建：

```bash
pixi run build
```

将 `cfgfc` 安装为全局命令：

```bash
pixi run install-global
```

如果执行后仍然找不到 `cfgfc`，请把 `GOBIN` 或 `$(go env GOPATH)/bin` 加到 `PATH` 中。

## License / 开源协议

This repository does not currently include a `LICENSE` file.

本仓库当前未包含 `LICENSE` 文件。
