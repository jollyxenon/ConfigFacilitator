## Context

ConfigFacilitator currently treats a target as managed only when `os.Lstat(target)` reports a symlink and `os.Readlink(target)` resolves to the recorded source path. This model is simple and supports both files and directories, but native Windows symlink creation can require Developer Mode or Administrator privileges. The requested behavior replaces that symlink-only activation model with hard-link-backed file activation so Windows and Linux use the same ordinary file-link mechanism.

Hard links do not encode a destination path and normal platform APIs do not hard-link directories. The design therefore changes both creation and ownership validation: file mappings are validated by filesystem identity, while directory mappings fail before target mutation with an explicit unsupported-directory diagnostic.

## Goals / Non-Goals

### Goals

- Create managed file targets with native filesystem hard links from the warehouse source file to the target path.
- Validate managed ownership without symlink mode bits or readable link targets.
- Preserve the existing safety model for unmanaged paths, drift, rollback, force recovery, reset, update, and revert.
- Fail clearly for directory-backed mappings, missing sources, cross-device or cross-volume hard-link failures, and unsupported platform behavior.
- Update specs, tests, and English/Chinese docs so hard-link behavior is the documented contract.

### Non-Goals

- Do not introduce fallback mechanisms such as symlinks, directory junctions, copies, `mklink`, PowerShell helpers, or platform-specific substitutes.
- Do not support directory activation in this change, because normal hard links cannot represent directories.
- Do not migrate or rewrite warehouse content layout.
- Do not change target planning semantics except where planned mappings point at directory sources that can no longer be activated.

## Decisions

### File mapping creation

Replace the symlink creation path with a hard-link creation path:

- Inspect the source path before target mutation.
- Require the source to exist and be a regular file.
- Reject directory sources with an actionable error that names the source and target.
- Reject non-regular sources unless the existing project already has a more specific accepted source type.
- Create the target with `os.Link(source, target)` after parent-directory preparation and conflict handling.

Native Windows and Unix-like builds use the same Go hard-link API. If `os.Link` fails because paths are on different filesystems or volumes, because the platform denies the operation, or because the target filesystem does not support hard links, the CLI reports the original failure context and states that no fallback was attempted.

### Ownership detection

Replace symlink target comparison with file-identity comparison:

- A recorded mapping is owned when both recorded source and target exist as regular files and `os.SameFile(sourceInfo, targetInfo)` reports that they are the same filesystem file.
- A missing target is treated as absent.
- A missing source for a recorded mapping is ownership drift, because the engine can no longer prove the target is a hard link to the recorded source.
- A target that exists but is not the same file as the source is unmanaged or drifted, depending on whether it is encountered as a planned target conflict or recorded current-state validation.

This keeps the existing state schema useful: source-target pairs remain the source of truth, and ownership is derived from the current filesystem rather than from persisted inode or file-id metadata.

### Mutating operations

Apply, update, revert, rollback, and reset continue to use the same high-level replace algorithm:

- Validate recorded current mappings before mutation unless force semantics allow reclaiming drifted targets.
- Remove only targets proven to be owned by their recorded source when force is disabled.
- Remove planned target conflicts only when force is enabled.
- Commit `current_state.json`, apply intent, and restore snapshots after successful mutation as before.
- Roll back to the last confirmed managed mapping set if mutation succeeds but later persistence fails.

Removing a hard-link target removes only that target directory entry. It does not delete the warehouse source file or other hard links to the same file.

### Force recovery

Force-enabled operations keep recursive directory reclamation for occupied target paths. This is still needed when a planned file hard link must reclaim an unmanaged directory at the destination. After reclaiming the directory, the engine creates the requested hard-link file target if the source is a regular file.

Force does not make directory-backed mappings supported. If the source itself is a directory, the operation fails before creating a replacement target even when force is enabled.

### Legacy symlink state

Existing users may have `current_state.json` entries created by older symlink-backed versions. This change does not need a separate state schema migration, but the first mutating operation after upgrade must treat existing symlink targets as not owned by the new hard-link backend unless an explicit compatibility path is implemented.

For safety, the implementation should prefer clear failure over silently deleting old symlinks in non-force mode. Users can run forced reset/apply if they intentionally want cfgfc to reclaim old symlink targets and recreate file hard links.

## Risks / Trade-offs

- Directory mappings become unsupported. This is the largest behavior loss and must be called out in docs and diagnostics.
- Hard links generally cannot cross filesystems or Windows volumes. Users whose warehouse and target live on different devices will receive a hard-link creation failure instead of a symlink.
- Hard links share file content. Editing the target edits the warehouse source file because both paths name the same underlying file.
- Hard links do not expose their source path through filesystem metadata, so troubleshooting must rely on recorded state plus filesystem identity checks.
- Non-force upgrade paths from symlink-managed state may require manual cleanup or force recovery.

## Migration Plan

1. Update the OpenSpec delta specs for `symlink-state-lifecycle` and `windows-native-symlinks` to define hard-link-backed file behavior and directory-source rejection.
2. Replace linker creation and ownership helpers with hard-link creation and `os.SameFile` validation.
3. Update linker and CLI tests from symlink target assertions to hard-link identity and shared-content assertions.
4. Add tests for directory-source rejection, cross-device or generic hard-link failure messaging where practical, and no-fallback wording.
5. Update English and Chinese documentation, platform notes, architecture notes, examples, and demand notes.
6. Run the standard pixi-managed test and compile commands plus targeted CLI smoke tests.

## Open Questions

- Should the project keep the capability name `symlink-state-lifecycle` for history continuity, or should a later spec cleanup rename it to a link-backend-neutral capability?
- Should a future change introduce an explicit copy or directory strategy for directory-backed settings, or should directory settings remain permanently unsupported under the hard-link contract?
