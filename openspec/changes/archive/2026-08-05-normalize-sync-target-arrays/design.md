## Context

`SettingIndex.jsonc` stores the two column-level default arrays plus two arrays on each Setting. Today, `internal/syncer` derives a generated target count from the default arrays and rejects mismatched defaults, while `internal/planner` independently rejects mismatched effective directory/name arrays. The target-count source is therefore implicit, and sync cannot repair malformed authored array lengths.

This change makes the target-position count explicit. It intentionally changes the editable index contract: every current Setting index must declare `targetNumber`; indexes that omit it are invalid rather than being interpreted through a legacy count rule.

## Goals / Non-Goals

**Goals:**
- Store and serialize one explicit top-level target-position count.
- Normalize all four target-array kinds during sync with the exact truncate, repeat, and empty-fill rules.
- Keep target inheritance and final target validation after sync intact.
- Cover parsing, scaffold, sync, planning, and bilingual documentation with tests and examples.

**Non-Goals:**
- Migrating existing indexes or accepting indexes that omit `targetNumber`.
- Inferring the target count from default arrays or restoring any prior count fallback.
- Changing the meaning of empty strings in target arrays, target-name fallback after inheritance, or duplicate-target validation.
- Normalizing arrays outside `cfgfc sync`.

## Decisions

1. **Add `TargetNumber int` to `index.SettingIndex`, and require it during parsing.**
   - `parseSettingIndex` will recognize only the top-level `targetNumber` field, decode it as an integer, and reject a missing, negative, fractional, or otherwise invalid value. `marshalSettingIndex` will always emit the count, including zero.
   - Rationale: an `int` permits an explicit zero count, while checking the raw JSON map for the field distinguishes zero from absence.
   - Alternatives considered: treating zero as missing (rejected because it makes valid zero-length normalization unrepresentable), or using default-array length when missing (rejected by the no-fallback requirement).

2. **Normalize every target array in sync through one pure helper.**
   - The helper accepts an array and `targetNumber`. It returns a leading slice when the input is too long; preserves an equal-length array; repeats the one common existing string when a non-empty short array is uniform; and otherwise appends empty strings. An empty short array is filled with empty strings. A zero count always returns an empty array.
   - `rewriteSettingIndex` will normalize `DefaultTargetDir` and `DefaultTargetName` before it rewrites entries, then normalize both target arrays for every non-missing Setting that is persisted.
   - Rationale: one helper makes the four fields obey one deterministic, directly testable rule and ensures no target-array mismatch reaches the synced file.
   - Alternatives considered: normalizing only default arrays (rejected because setting overrides could still be malformed), or repairing lengths in the planner (rejected because the requested behavior belongs to sync and would hide unsynced authored state).

3. **Remove count generation and keep planner validation as a consumer-side safeguard.**
   - Remove `generatedTargetCount` and its default-array length rejection. Sync will use only `SettingIndex.TargetNumber` and will not derive any target count. Planner continues to zip and validate the effective arrays; it does not grow, truncate, or infer them.
   - When sync discovers a new Setting whose target arrays have no values, normalization writes empty placeholders at every declared position. Existing inheritance then resolves those placeholders using defaults and the existing warehouse-name fallback for an empty default target name.
   - Rationale: sync owns persisted-shape normalization, while planning remains responsible only for resolving and validating paths.

4. **Emit explicit arrays in normalized output and scaffold a valid current-format index.**
   - Sync serialization will retain all four array fields even when `targetNumber` is zero, so the rewritten document visibly satisfies the declared cardinality. The new-column template and trailing example will use `targetNumber: 1` and one-element arrays.
   - Rationale: omitting empty arrays at count zero would make the synced representation ambiguous and violate the declared exact-length contract.

5. **Update all repository-owned fixtures and user documentation to the breaking schema.**
   - Every test fixture and documented `SettingIndex.jsonc` example will include `targetNumber`; no fixture will exercise a compatibility path. English and Chinese documentation will describe `targetNumber` as the authoritative count, the sync normalization rules, and the need to sync after changing it.

## Risks / Trade-offs

- [Risk] The breaking required field makes all old indexes fail to load before sync can repair them. → Mitigation: this is intentional; templates, test fixtures, and docs provide only the new shape, and no migration/fallback is implemented.
- [Risk] Extending a non-uniform array with empty strings can cause apply-time validation errors if inherited defaults are also empty. → Mitigation: retain existing planning diagnostics; sync repairs cardinality, not target validity.
- [Risk] Reducing `targetNumber` permanently removes trailing authored values during sync. → Mitigation: document the destructive truncation rule and test it explicitly.
- [Risk] Target count zero yields no usable mapping for a selected Setting. → Mitigation: preserve the existing planner error for a resolved empty target set rather than silently succeeding.
