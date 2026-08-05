## 1. Setting index contract

- [x] 1.1 Add required `targetNumber` parsing, validation, and stable serialization to `internal/index.SettingIndex`; ensure the current-format serializer represents zero-length target arrays explicitly.
- [x] 1.2 Update index parsing and serialization tests for valid positive/zero counts and missing, negative, fractional, and non-numeric count failures; update all existing Setting-index fixtures to the breaking schema.

## 2. Sync-time target-array normalization

- [x] 2.1 Replace default-array-derived target-count generation in `internal/syncer` with a single normalization helper driven only by `SettingIndex.TargetNumber`.
- [x] 2.2 Normalize both default arrays and every persisted non-missing Setting's directory/name arrays before sync writes `SettingIndex.jsonc`; remove the length-mismatch rejection and legacy count fallback.
- [x] 2.3 Add sync tests covering truncation, uniform-value broadcasting, varied-array empty filling, empty-array filling, zero target count, and newly discovered Settings at multiple target positions.
- [x] 2.4 Adjust planner and CLI fixtures/tests to use explicit `targetNumber` while retaining planner validation for unresolved or invalid final targets.

## 3. Scaffold and documentation

- [x] 3.1 Update the Setting-index scaffold body and trailing example to show `targetNumber: 1` and the current normalized target-array contract.
- [x] 3.2 Update English and Chinese JSONC guides, architecture descriptions, and examples to explain `targetNumber`, sync normalization, destructive truncation, uniform-array broadcasting, and empty-string filling.

## 4. Verification

- [x] 4.1 Run `pixi run test` and `pixi run compile`.
- [x] 4.2 Run `pixi run help` and `pixi run bash -lc 'for cmd in new sync switch root list apply update reset revert; do go run ./cmd/cfgfc "$cmd" --help; done'`.
- [x] 4.3 Run a temp-home CLI smoke test that creates a current-format warehouse, changes `targetNumber`, runs `cfgfc sync`, and verifies the persisted arrays follow every normalization rule.
