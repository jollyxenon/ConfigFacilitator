# Developer Setup

## Tooling

- Language: Go 1.24.4
- Environment manager: `pixi`
- Entry point: `cmd/cfgfc/main.go`
- npm distribution wrapper: `npm/`
- Current development environment: Linux under WSL; native Windows behavior requires separate native verification

## Baseline commands

```bash
pixi run test
pixi run compile
pixi run build
pixi run help
```

## Complete help sweep

The sweep must cover every top-level family and every runnable nested command, not removed commands:

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

Also verify root help contains only the current top-level surface and that `new`, `switch`, `list`, and `update` are rejected as usage errors without mutation.

## CLI lifecycle smoke

For command/workflow changes, use a temporary HOME/profile and persist a separate alternate root with `cfgfc root <Path>`. Use only `cfgfc` plus stdin/redirection to cover:

1. Project, Column, zero-based target, file Setting from stdin, directory Setting and nested content, Mode, and Mode selections;
2. `apply mode` and `apply column`, human `status` (shows `Current: following/independent [...]`), `current show`, and `--json` envelopes/exit codes;
3. byte-only content write showing immediate visibility through an active link;
4. metadata change plus `refresh`, a `full` selection adding a new Setting, and automatic re-planning when the selections of a followed Mode change;
5. active Setting/Column/Mode/Project rename and canonical context/reference updates;
6. content and resource deletion with independent `--yes`, `--cascade`, and `--force-targets` behavior;
7. `reset` and one-step `revert`;
8. external disappearance, per-Project sync transaction isolation, index and empty-Current rebuild, unresolved Mode/runtime references, apply/refresh failure, unsupported prune flags, and explicit resource recreation;
9. prepared-transaction diagnostics, restart recovery, rollback, and concurrent mutation locking;
10. file-backed and directory-backed target ownership/drift reclamation limited to recorded paths;
11. Web UI smoke: start `cfgfc web`, open `http://127.0.0.1:38031`, exercise `/api/snapshot` and a `/api/command` write, confirm a stale `revision` returns `409`, a used port fails with a persistence error, and Ctrl-C exits cleanly.

Do not use direct index/source editing to build the primary lifecycle. External file removal is allowed only for the explicit sync interoperability portion; recreate resources through cfgfc rather than claiming sync restores deleted metadata.

## Documentation and OpenSpec checks

Docs-only changes should at least run:

```bash
openspec validate replace-editor-workflows-with-cli --strict
openspec status --change replace-editor-workflows-with-cli --json
```

Keep `README.md`, all English/Chinese document pairs, `skills/configfacilitator-usage/SKILL.md`, `AGENTS.md`, and generated CLI help aligned. English and Chinese pages must describe the same commands, examples, safety rules, and developer workflow.

## npm package and release workflow

```bash
pixi run build
cd npm
npm pack --dry-run
CFGFC_BINARY_PATH=../dist/cfgfc npm install -g .
cfgfc --help
CFGFC_TEST_PLATFORM=freebsd CFGFC_TEST_ARCH=x64 node install.js
```

The final installer command is an expected failure path for unsupported tuple messaging.

Expected release order:

1. Set `npm/package.json` version to `X.Y.Z`.
2. Push Git tag `vX.Y.Z`.
3. Let GoReleaser publish assets such as `cfgfc_X.Y.Z_linux_amd64.tar.gz` and `checksums.txt`.
4. Publish npm only after matching release assets exist.

## Native platform verification

Run Linux/WSL tests through pixi, but validate native Windows path parsing, file/directory symlinks, permissions, and target reclamation with native `cfgfc.exe`. Windows and WSL are different runtimes and one does not substitute for the other.
