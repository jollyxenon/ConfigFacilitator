# Platform Notes

## Symlink policy

ConfigFacilitator uses real symlinks only. It does not fall back to directory junctions, hardlinks, file copies, `mklink`, PowerShell helpers, or any other substitute mechanism.

On native Windows, `cfgfc.exe` manages Windows user-directory configuration with real file and directory symlinks. Windows may require Developer Mode or Administrator privileges before it allows symlink creation. If Windows refuses a symlink, ConfigFacilitator reports the failure and stops instead of creating a junction or copy.

The source path is inspected at link creation time to confirm it exists and to let the platform infer whether the link is file-backed or directory-backed. ConfigFacilitator does not persist source type in its state files.

## Portable layout

The warehouse is resolved from the current user's home/profile directory, not beside the shell working directory. Moving the binary does not change the active warehouse root.

- Default Unix-like root: `~/.configfacilitator/`
- Default native Windows root: `%USERPROFILE%/.configfacilitator`
- Persistent override bootstrap: `~/.cfgfc-root` on Unix-like platforms and `%USERPROFILE%/.cfgfc-root` on native Windows

`cfgfc root` prints the current effective warehouse root. `cfgfc root <path>` persists a new effective root in the bootstrap file after path expansion and normalization. Changing roots does not migrate, copy, or initialize warehouse contents.

Root-level project discovery runs directly inside the effective warehouse root. Any directory there that matches the project layout, including `SettingWarehouse`, participates in discovery.

## Native Windows and WSL boundary

Native Windows `cfgfc.exe` and a Linux build running under WSL are separate runtimes. Native Windows follows `%USERPROFILE%` paths and Windows symlink permissions. WSL follows Linux path and symlink semantics for the paths it is given. ConfigFacilitator does not automatically translate between `%USERPROFILE%` and `/mnt/c/...`.

## Session context

`switch` stores the active project by parent process ID. This is a convenience feature for concurrent shells, not a hard isolation boundary.

The `.cfgfc-session/` directory lives under the effective warehouse root, so switching roots also switches the session-context store.

## Revert scope

`revert` is single-step only and restores the previous apply snapshot.

## Native Windows smoke test

To manually verify native Windows support, run `cfgfc.exe` from a Windows shell with Developer Mode enabled or Administrator privileges, then apply one file-backed setting and one directory-backed setting. Confirm both targets are real symlinks and that the warehouse root resolves to either the default `%USERPROFILE%/.configfacilitator` path or the root persisted with `cfgfc root`.
