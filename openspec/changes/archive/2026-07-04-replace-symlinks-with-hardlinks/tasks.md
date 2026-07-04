## 1. Linker Backend

- [x] 1.1 Replace managed link creation with a hard-link helper that validates the source exists and is a regular file before calling `os.Link`.
- [x] 1.2 Replace symlink ownership checks with filesystem-identity checks between recorded source and target, treating missing sources, non-regular targets, and legacy symlinks as drift or unmanaged targets.
- [x] 1.3 Update target removal paths so deleting an owned hard-link target removes only the target directory entry and never removes the warehouse source file.
- [x] 1.4 Update native Windows hard-link failure diagnostics to report source, target, original error, hard-link limits, and the no-fallback policy.

## 2. Lifecycle Semantics

- [x] 2.1 Update apply and single-column apply flows to create hard links for regular-file mappings and fail clearly for directory or non-regular source mappings before mutating that target.
- [x] 2.2 Update update, reset, and revert flows so state validation, rollback, and forced recovery use hard-link ownership semantics instead of symlink target strings.
- [x] 2.3 Preserve current-state and history file compatibility for source-target mapping data while changing runtime ownership validation to filesystem identity.
- [x] 2.4 Keep forced target reclamation for files, symlinks, and directories, but ensure force never converts directory-backed source mappings into supported activations.

## 3. Tests

- [x] 3.1 Replace symlink target assertion helpers with hard-link identity and shared-content assertions in linker and CLI tests.
- [x] 3.2 Add coverage for apply, update, reset, revert, and force flows using regular-file hard links.
- [x] 3.3 Add coverage for directory source rejection, non-regular source rejection, legacy symlink drift, missing source drift, and cross-device or platform hard-link failures where practical.
- [x] 3.4 Update native Windows tests to expect hard-link creation diagnostics and remove Developer Mode/Admin symlink guidance expectations.

## 4. Documentation

- [x] 4.1 Update English and Chinese README, platform notes, architecture docs, command docs, and examples from symlink-backed behavior to hard-link-backed regular-file behavior.
- [x] 4.2 Document hard-link limitations, especially unsupported directory-backed mappings, cross-device or cross-volume failures, and target edits mutating the warehouse source file.
- [x] 4.3 Update `ConfigFacilitatorDemanding.md` and any help or error text that promises symlink-only behavior.

## 5. Verification

- [x] 5.1 Run `pixi run test` and `pixi run compile`.
- [x] 5.2 Run `pixi run help` and the subcommand help sweep for all registered commands.
- [x] 5.3 Run a real CLI smoke test with a temporary home/profile that verifies hard-link identity, shared content, directory-source rejection, and forced target reclamation.
