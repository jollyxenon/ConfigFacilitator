## 1. npm Package Skeleton

- [x] 1.1 Create a dedicated `npm/` package directory with `package.json`, `bin/cli.js`, and `install.js`.
- [x] 1.2 Configure `package.json` with the package name, version, `bin` mapping for `cfgfc`, package `files`, supported `os`/`cpu`, and `postinstall` script.
- [x] 1.3 Implement `bin/cli.js` to resolve `cfgfc` or `cfgfc.exe`, forward all CLI arguments, inherit stdio, and preserve the Go process exit status.

## 2. Binary Download Installer

- [x] 2.1 Implement platform and architecture mapping from Node.js values to the release asset naming convention.
- [x] 2.2 Implement release URL construction from the npm package version and documented GitHub release tag format.
- [x] 2.3 Implement archive download, extraction, executable placement under `npm/bin/`, and Unix executable permissions.
- [x] 2.4 Add clear installer errors for unsupported platform tuples, failed downloads, missing binaries inside archives, and extraction failures.
- [x] 2.5 Add checksum verification if generated release checksums are available; otherwise document checksum hardening as a follow-up.

## 3. Release Asset Configuration

- [x] 3.1 Add GoReleaser configuration that builds `cfgfc` from `./cmd/cfgfc` for Linux, macOS, and Windows on amd64 and arm64.
- [x] 3.2 Ensure archive names and binary names match the npm installer's expected convention.
- [x] 3.3 Add or document a release workflow that publishes GitHub Release assets before npm publication.
- [x] 3.4 Ensure release metadata keeps npm package versions aligned with GitHub tags such as `vX.Y.Z`.

## 4. Documentation

- [x] 4.1 Update the root README with npm installation instructions and the relationship between npm and the Go binary.
- [x] 4.2 Update English documentation under `docs/` with npm install, local package validation, and release prerequisites.
- [x] 4.3 Update Chinese documentation under `docs/` with equivalent npm install, validation, and release guidance.
- [x] 4.4 Update developer notes if the release workflow changes maintainer responsibilities.

## 5. Verification

- [x] 5.1 Run `pixi run test` to confirm existing Go behavior remains unchanged.
- [x] 5.2 Run `pixi run compile` to confirm Go compilation still succeeds.
- [x] 5.3 Run npm package validation from `npm/`, including `npm pack --dry-run`.
- [x] 5.4 Run a local npm install smoke test and verify `cfgfc --help` executes through the npm wrapper.
- [x] 5.5 Verify installer failure messages for at least one unsupported or intentionally invalid platform/download path.
