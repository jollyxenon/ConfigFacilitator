## Why

`cfgfc sync` currently rejects a Setting index when the directory and name target arrays do not have matching lengths. Authors therefore must manually keep four independently authored arrays aligned, even though their intended number of target positions is one shared property.

## What Changes

- Add required top-level `targetNumber` metadata to each `SettingIndex.jsonc` to declare the number of target positions.
- Make `cfgfc sync` normalize `defaultTargetDir`, `defaultTargetName`, and every Setting entry's `targetDir` and `targetName` to exactly `targetNumber` entries before writing the index.
- When an array is longer than `targetNumber`, retain its leading entries and remove the rest.
- When an array is shorter than `targetNumber`, repeat its value when all existing entries are identical; otherwise append empty-string inheritance entries until it reaches the declared length.
- Remove the current length-mismatch rejection and all legacy/fallback behavior for deriving target counts from existing arrays.
- Update generated templates, validation, tests, and bilingual documentation to make `targetNumber` and sync normalization the canonical target-array contract.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `target-dir-name-resolution`: Define `targetNumber` as the required top-level target-position count and require sync to normalize all target arrays to that count.

## Impact

- Affects Setting index parsing, serialization, generated scaffolds, and sync reconciliation in `internal/index`, `internal/scaffold`, and `internal/syncer`.
- Affects target resolution in `internal/planner`, which will consume normalized arrays instead of deriving or repairing their length.
- Affects existing Go tests and English/Chinese JSONC examples and contract documentation.
- Existing Setting indexes without `targetNumber` are intentionally unsupported and must be updated; no compatibility path or fallback is provided.
