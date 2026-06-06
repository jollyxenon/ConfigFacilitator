# Platform Notes

## Symlink policy

ConfigFacilitator uses real symlinks only. It does not fall back to directory junctions, hardlinks, file copies, `mklink`, PowerShell helpers, or any other substitute mechanism.

On native Windows, `cfgfc.exe` manages Windows user-directory configuration with real file and directory symlinks. Windows may require Developer Mode or Administrator privileges before it allows symlink creation. If Windows refuses a symlink, ConfigFacilitator reports the failure and stops instead of creating a junction or copy.

The source path is inspected at link creation time to confirm it exists and to let the platform infer whether the link is file-backed or directory-backed. ConfigFacilitator does not persist source type in its state files.

## Portable layout

The warehouse is resolved from the current user's home/profile directory, not beside the shell working directory. Moving the binary does not change the active warehouse root.

- Unix-like platforms: `~/.configfacilitator/`
- Native Windows: `%USERPROFILE%/.configfacilitator`

Root-level project discovery runs directly inside `~/.configfacilitator/`. Any directory there that matches the project layout, including `SettingWarehouse`, participates in discovery.

## Native Windows and WSL boundary

Native Windows `cfgfc.exe` and a Linux build running under WSL are separate runtimes. Native Windows follows `%USERPROFILE%` paths and Windows symlink permissions. WSL follows Linux path and symlink semantics for the paths it is given. ConfigFacilitator does not automatically translate between `%USERPROFILE%` and `/mnt/c/...`.

## Session context

`switch` stores the active project by parent process ID. This is a convenience feature for concurrent shells, not a hard isolation boundary.

## Revert scope

`revert` is single-step only and restores the previous apply snapshot.

## Native Windows smoke test

To manually verify native Windows support, run `cfgfc.exe` from a Windows shell with Developer Mode enabled or Administrator privileges, then apply one file-backed setting and one directory-backed setting. Confirm both targets are real symlinks and that the warehouse root is under `%USERPROFILE%/.configfacilitator`.
