# Platform Notes

## Symlink policy

ConfigFacilitator uses real symlinks only. It does not fall back to directory junctions or file copies.

## Portable layout

The warehouse is resolved at `~/.configfacilitator/SettingWarehouse/`, not beside the shell working directory. Moving the binary does not change the active warehouse root.

## Session context

`switch` stores the active project by parent process ID. This is a convenience feature for concurrent shells, not a hard isolation boundary.

## Revert scope

`revert` is single-step only and restores the previous apply snapshot.
