# Platform Notes

## Real-symlink policy

ConfigFacilitator creates real file and directory symlinks only. It does not fall back to junctions, hardlinks, copies, `mklink`, PowerShell helpers, or other substitutes. Source existence and kind are inspected when planning or linking; source kind is not persisted in runtime state.

On native Windows, Developer Mode or Administrator privileges may be required. If symlink creation is refused, cfgfc returns a persistence failure instead of silently using another mechanism.

## Roots and runtime boundaries

- Default Unix-like root: `~/.configfacilitator/`
- Default native Windows root: `%USERPROFILE%/.configfacilitator`
- Persistent override bootstrap: `~/.cfgfc-root` or `%USERPROFILE%/.cfgfc-root`

`cfgfc root` prints the effective root. `cfgfc root <Path>` expands and normalizes a new root, then persists it for later commands. It does not migrate, copy, or initialize contents. Moving the executable does not change the root.

Direct child Project directories under the effective root, including `SettingWarehouse`, participate in discovery. `.cfgfc-session/` and `.cfgfc-transactions/` are reserved and are not discovered as resources.

## Native Windows and WSL

Native `cfgfc.exe` and a Linux binary under WSL are separate runtimes:

- native Windows uses `%USERPROFILE%`, Windows path syntax, and Windows symlink privileges;
- WSL uses Linux home, path, permission, and symlink semantics;
- cfgfc does not translate `%USERPROFILE%` to `/mnt/c/...` or convert warehouse paths between runtimes.

Choose the binary whose filesystem semantics match the targets you intend to manage. Avoid sharing active runtime/session/transaction state between native Windows and WSL processes.

## Portable target paths

Target directories may use `~`, `${VAR}`, and Windows `%VAR%`. Expansion follows the current runtime's home and environment. Target names must be a single normal file or directory name. Expanded targets must be non-empty and unique within one plan.

Setting content paths are relative to the Setting root. Absolute paths, empty paths where a child is required, `.`/`..` traversal, escaped paths, and symlink components are rejected. Directory imports accept only regular files and directories and do not follow symlinks.

## Context and transactions

`cfgfc use` stores canonical Project context by parent process ID under the effective root. This is convenience state for concurrent shells, not a security or isolation boundary. Explicit `-p/--project` takes precedence. Changing roots also changes the context store.

Mutating commands serialize through a warehouse-wide lock and recover an incomplete prepared transaction before new work. Read-only `status` reports transaction diagnostics without recovery. Do not edit or delete `.cfgfc-transactions/` manually.

## Web UI networking

`cfgfc web` binds to `127.0.0.1` only (default port `49631`), so the UI is reachable only from the same machine. The frontend is embedded in the single binary and makes no external network requests. The port can be changed with `--port`; an occupied port is a persistence failure, not an automatic fallback. Ctrl-C stops the server.

## Destructive behavior

`--force-targets` can recursively reclaim only affected target paths already recorded by the requested apply, rename, delete, refresh, reset, or revert operation. It does not confirm repository deletion (`--yes`), authorize dependent-reference repair (`--cascade`), or back up/reconstruct overwritten unmanaged content.

## Native Windows smoke test

From a Windows shell with Developer Mode enabled or Administrator privileges:

1. persist a temporary root with `cfgfc root <Path>`;
2. use resource/content commands to create one file-backed and one directory-backed Setting;
3. configure distinct targets and apply them;
4. confirm both targets are real symlinks and readable;
5. replace target ownership deliberately and confirm normal operation refuses while `--force-targets` reclaims only the recorded path;
6. confirm content relative-path and symlink-component rejection.

Run an equivalent smoke independently under WSL when WSL support is needed; do not treat one runtime's result as proof for the other.
