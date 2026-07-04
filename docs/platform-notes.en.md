# Platform Notes

## Hard-link policy

ConfigFacilitator uses real file hard links only. It does not fall back to symlinks, directory junctions, file copies, `mklink`, PowerShell helpers, or any other substitute mechanism.

Hard links apply only to regular files. Directory-backed mappings are unsupported and fail clearly instead of being converted to directory symlinks, junctions, or copies.

The source path is inspected at activation time to confirm it exists and is a regular file. Hard links generally cannot cross filesystems or Windows volumes, so activation fails when the warehouse source and target live on different devices, unsupported filesystems, or non-regular paths.

Editing either name mutates the same file content because the warehouse source path and the activated target path refer to the same underlying file data.

## Portable layout

The warehouse is resolved from the current user's home/profile directory, not beside the shell working directory. Moving the binary does not change the active warehouse root.

- Default Unix-like root: `~/.configfacilitator/`
- Default native Windows root: `%USERPROFILE%/.configfacilitator`
- Persistent override bootstrap: `~/.cfgfc-root` on Unix-like platforms and `%USERPROFILE%/.cfgfc-root` on native Windows

`cfgfc root` prints the current effective warehouse root. `cfgfc root <path>` persists a new effective root in the bootstrap file after path expansion and normalization. Changing roots does not migrate, copy, or initialize warehouse contents.

Root-level project discovery runs directly inside the effective warehouse root. Any directory there that matches the project layout, including `SettingWarehouse`, participates in discovery.

## Native Windows and WSL boundary

Native Windows `cfgfc.exe` and a Linux build running under WSL are separate runtimes. Native Windows follows `%USERPROFILE%` paths and Windows hard-link limits. WSL follows Linux path and hard-link semantics for the paths it is given. ConfigFacilitator does not automatically translate between `%USERPROFILE%` and `/mnt/c/...`.

## Session context

`switch` stores the active project by parent process ID. This is a convenience feature for concurrent shells, not a hard isolation boundary.

The `.cfgfc-session/` directory lives under the effective warehouse root, so switching roots also switches the session-context store.

## Revert scope

`revert` is single-step only and restores the previous apply snapshot.

## Native Windows smoke test

To manually verify native Windows support, run `cfgfc.exe` from a Windows shell, then apply one file-backed setting whose source and target stay on the same volume. Confirm the target becomes a regular file hard link, confirm no Developer Mode or Administrator symlink privilege is required, and confirm the warehouse root resolves to either the default `%USERPROFILE%/.configfacilitator` path or the root persisted with `cfgfc root`. Also verify that cross-volume, non-regular, or missing sources fail with clear diagnostics.
