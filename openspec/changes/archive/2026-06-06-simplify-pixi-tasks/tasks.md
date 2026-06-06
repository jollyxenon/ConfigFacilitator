## 1. Pixi Task Surface

- [x] 1.1 Update `pixi.toml` so `[tasks]` contains only `test`, `compile`, `build`, and `help`.
- [x] 1.2 Set `compile` to run the whole-project compilation check with `go build ./...`.
- [x] 1.3 Set `build` to generate the local CLI binary with `go build -o dist/cfgfc ./cmd/cfgfc`.
- [x] 1.4 Remove direct install tasks from `pixi.toml`, including `usr-install` and `global-install`.

## 2. Documentation Updates

- [x] 2.1 Update `docs/developer-setup.en.md` baseline commands and validation expectations for the four-task pixi workflow.
- [x] 2.2 Update `docs/developer-setup.zh-CN.md` with matching Chinese documentation.
- [x] 2.3 Update root and agent-facing notes that still reference the removed install tasks or old `build` semantics.

## 3. Verification

- [x] 3.1 Run `pixi run test`.
- [x] 3.2 Run `pixi run compile`.
- [x] 3.3 Run `pixi run build` and confirm the `dist/cfgfc` artifact is produced.
- [x] 3.4 Run `pixi run help`.
- [x] 3.5 Run the subcommand help sweep with the pixi-managed Go toolchain.
