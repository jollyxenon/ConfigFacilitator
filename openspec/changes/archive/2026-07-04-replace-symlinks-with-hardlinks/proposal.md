## Why

ConfigFacilitator currently activates every managed target through real filesystem symlinks. That keeps ownership easy to inspect, but it also makes native Windows usage fragile because symlink creation can require Developer Mode or Administrator privileges. Users who keep the same warehouse reachable from Windows and Linux need a connection mechanism that behaves the same way on both runtimes without relying on Windows symlink permissions.

## What Changes

- Replace managed file-target connection creation with filesystem hard links created from warehouse source files to target paths.
- Update ownership detection, state validation, reset, revert, update, and forced recovery so they no longer depend on symlink mode bits or `readlink` targets.
- Treat directory-backed mappings as unsupported by the hard-link backend and fail with clear guidance, because normal hard links cannot represent directories on supported platforms.
- Preserve the existing no-fallback safety model: do not silently use symlinks, directory junctions, copies, `mklink`, PowerShell helpers, or platform-specific substitutes when a hard link cannot be created.
- Update English and Chinese documentation, platform notes, command wording, architecture notes, and tests to describe hard-link-backed file mappings and directory limitations.

## Capabilities

### New Capabilities

### Modified Capabilities

- `symlink-state-lifecycle`: replace symlink-backed managed state semantics with hard-link-backed file semantics, including ownership detection, apply/update/reset/revert behavior, and force recovery.
- `windows-native-symlinks`: retire the Windows symlink-only runtime contract in favor of native hard-link creation for files, with explicit no-fallback and unsupported-directory diagnostics.

## Impact

- Affected code: `internal/linker/engine.go`, linker ownership helpers, linker tests, CLI smoke tests, and any helper/test names that currently assert symlink targets.
- Affected behavior: file mappings create hard links; directory mappings fail instead of creating directory symlinks; ownership checks can no longer read a link target path from the filesystem.
- Affected state: current source-target mappings can keep the same schema, but runtime ownership validation must compare filesystem identity and stored source metadata rather than symlink target strings.
- Affected documentation: `README.md`, `docs/`, `ConfigFacilitatorDemanding.md`, platform notes, architecture docs, and command examples that mention symlinks or Windows symlink permissions.
