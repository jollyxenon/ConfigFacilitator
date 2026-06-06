## Context

ConfigFacilitator is a Go CLI developed through pixi, with Go supplied as a pixi dependency rather than assumed to exist globally on contributor machines. The current task list includes compilation checks, help execution, and Unix/system installation helpers in the same public surface, while the intended user distribution path is future package-manager installation through Scoop or npm.

## Goals / Non-Goals

**Goals:**
- Make pixi tasks describe development and release-build actions only.
- Keep the task surface small and platform-conscious: `test`, `compile`, `build`, and `help`.
- Preserve a clear distinction between compilation validation and building a distributable CLI binary.
- Update English and Chinese docs so contributors understand the revised workflow.

**Non-Goals:**
- Implement Scoop, npm, GoReleaser, or release automation in this change.
- Change runtime CLI behavior or command semantics.
- Add platform-specific cross-compilation tasks beyond the local `build` artifact.

## Decisions

- Use `compile = "go build ./..."` for whole-project compilation checks.
  - Rationale: `compile` communicates that the command validates buildability without promising a distributable output.
  - Alternative considered: keep this behavior under `build`; rejected because `build` is more naturally understood as producing an artifact.
- Use `build = "go build -o dist/cfgfc ./cmd/cfgfc"` for local CLI artifact generation.
  - Rationale: release wrappers and package-manager flows need a concrete binary, and `dist/` clearly separates build outputs from source files.
  - Alternative considered: keep `build-cli`; rejected to preserve the requested four-task surface.
- Remove `usr-install` and `global-install` from `pixi.toml`.
  - Rationale: direct installation tasks encode platform assumptions and conflict with the planned Scoop/npm distribution story.
  - Alternative considered: rename to a more explicit private helper; rejected because the requested outcome is to keep only four tasks.
- Keep `help = "go run ./cmd/cfgfc --help"`.
  - Rationale: it remains a cheap source-run smoke test for the root command surface.

## Risks / Trade-offs

- [Risk] Existing contributors may still try `pixi run build` as a compile-only check. → Mitigation: update developer docs and agent notes to point to `pixi run compile` for compile validation.
- [Risk] `dist/cfgfc` is Unix-style and does not include `.exe` on Windows. → Mitigation: treat this as a local build task for the active platform; future release tooling should own multi-platform artifact naming.
- [Risk] Removing install helpers may inconvenience local Unix users. → Mitigation: local installation is outside the public pixi task surface and should be handled by future package-manager workflows or ad-hoc developer commands.
