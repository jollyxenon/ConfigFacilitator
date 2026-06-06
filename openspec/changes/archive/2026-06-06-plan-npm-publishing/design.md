## Context

ConfigFacilitator is a Go CLI with the executable entry point at `cmd/cfgfc/main.go`. The project is currently developed through pixi-managed Go tasks (`pixi run test`, `pixi run compile`, `pixi run build`) and produces a local binary at `dist/cfgfc`. There is no root `package.json`, no npm package metadata, and no release automation for cross-platform npm consumers.

The npm distribution should be a packaging layer, not a rewrite. npm users should be able to install `cfgfc` without a local Go toolchain, while existing Go users and contributors continue using the current pixi workflow.

## Goals / Non-Goals

**Goals:**

- Provide `npm install -g ...` support for installing the `cfgfc` command.
- Keep the Go binary as the source of CLI behavior and avoid duplicating command logic in JavaScript.
- Use prebuilt release assets so npm installation does not require Go on the target machine.
- Support Linux, macOS, and Windows on x64 and arm64 where Go builds are available.
- Add verification steps for npm packaging, binary download, and CLI smoke tests.
- Document release prerequisites and installation instructions in both English and Chinese docs.

**Non-Goals:**

- Rewriting the CLI in Node.js.
- Publishing platform-specific npm optional dependency packages in the first iteration.
- Changing existing command semantics, warehouse layout, or pixi development tasks.
- Implementing auto-update behavior after npm installation.

## Decisions

1. **Use a dedicated npm package directory.**

   The npm-specific files will live under `npm/` instead of the repository root. This keeps the Go repository root focused on Go/pixi files and avoids implying that Node.js is required for normal development.

   Alternative considered: root-level `package.json`. That would simplify `npm publish` from the repository root, but it makes the project appear Node-first and risks accidental publication of non-package files.

2. **Use a Node.js `bin` wrapper that executes the Go binary.**

   npm's `bin` entry will point to a JavaScript wrapper such as `bin/cli.js`. The wrapper resolves `bin/cfgfc` or `bin/cfgfc.exe`, forwards `process.argv.slice(2)`, inherits stdio, and exits with the underlying process status.

   Alternative considered: pointing `bin` directly to a binary file. This is less portable before install-time binary selection and is harder to handle consistently across Windows and Unix packages.

3. **Download prebuilt binaries during npm `postinstall`.**

   The install script will map `process.platform` and `process.arch` to the release asset naming convention, download the correct archive from GitHub Releases, extract the `cfgfc` executable into `npm/bin/`, and mark it executable on Unix systems.

   Alternative considered: compiling from source during `postinstall`. This was rejected because npm users should not need Go or pixi installed. Another alternative, platform-specific optional dependency packages, is more robust for offline installs but requires a larger publishing workflow and can be added later.

4. **Use GoReleaser-style cross-platform release assets.**

   Release automation should build `cfgfc` from `./cmd/cfgfc` for `linux`, `darwin`, and `windows` on `amd64` and `arm64`. The npm downloader should use a stable asset naming convention and, where feasible, verify checksums before installing.

   Alternative considered: manually uploading binaries. Manual upload is error-prone and makes npm installation reliability dependent on human release steps.

5. **Keep release publishing explicit and verifiable.**

   The first implementation should include local validation commands such as `npm pack --dry-run`, `npm install -g ./npm`, `cfgfc --help`, and the existing `pixi run test` / `pixi run compile`. Full automated npm publishing can be wired to GitHub Actions if npm credentials are configured.

## Risks / Trade-offs

- **GitHub Release asset missing or renamed** → Keep the downloader's expected naming convention documented and align it with release automation.
- **Network failures during `postinstall`** → Produce clear error messages explaining the required release URL and platform tuple. A future optional-dependencies layout can improve offline reliability.
- **Checksum verification complexity** → Prefer using generated checksums when available; if omitted initially, document this as a release hardening follow-up.
- **Windows path and executable differences** → Use `cfgfc.exe` on `win32`, avoid shell execution in the wrapper, and test `npm install -g ./npm` on Windows when possible.
- **Version mismatch between npm and GitHub Releases** → The install script should derive the release tag from `package.json` version and documentation should require publishing npm only after release assets for the same version exist.
