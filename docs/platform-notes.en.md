# Platform Notes

## Symlink policy

ConfigFacilitator uses real symlinks only. It does not fall back to directory junctions or file copies.

## Portable layout

The warehouse is resolved relative to the executable, not the shell working directory. Moving the binary changes the active warehouse root.

## Session context

`switch` stores the active project by parent process ID. This is a convenience feature for concurrent shells, not a hard isolation boundary.

## Revert scope

`revert` is single-step only and restores the previous apply snapshot.
