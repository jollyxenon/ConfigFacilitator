## Context

The repository currently documents that no `LICENSE` file exists. ConfigFacilitator is a Go CLI intended for external use, so leaving license terms unspecified creates unnecessary ambiguity for users, packagers, and contributors.

The change is limited to repository metadata and documentation. It must preserve the existing CLI behavior, Go module dependencies, pixi workflow, and English/Chinese documentation parity expectations.

## Goals / Non-Goals

**Goals:**
- Declare MIT License terms with a standard root `LICENSE` file.
- Replace the current README notice with a clear MIT License statement.
- Keep English and Chinese documentation aligned wherever license information is surfaced.
- Validate that the documentation-only change does not disturb the Go test and compile baselines.

**Non-Goals:**
- Changing CLI behavior, command output, warehouse formats, or symlink logic.
- Adding a new dependency or packaging pipeline.
- Introducing contributor license agreements or dual licensing.

## Decisions

- Use the standard MIT License text in `LICENSE`.
  - Rationale: MIT is concise, permissive, widely recognized for CLI tools, and only requires preserving copyright and license notices.
  - Alternative considered: Apache-2.0, which adds explicit patent language but is more verbose than needed for this small CLI.
  - Alternative considered: GPL-3.0, which enforces copyleft but would reduce adoption flexibility for a configuration-management utility.
- Keep license documentation lightweight.
  - Rationale: The authoritative license terms belong in `LICENSE`; README and docs should link or name the license rather than duplicating the full legal text.
  - Alternative considered: Copying the full license into every docs page, which increases drift risk and makes bilingual docs harder to maintain.
- Treat license information as a documentation-parity concern.
  - Rationale: The project already requires English/Chinese user-facing docs to stay aligned for workflow changes; license visibility should follow the same parity rule.

## Risks / Trade-offs

- [Risk] A non-standard license text could create ambiguity. → Mitigation: Use the canonical MIT License wording and only customize the copyright line.
- [Risk] Documentation could drift between English and Chinese pages. → Mitigation: Update corresponding English and Chinese docs in the same task and verify the license references during review.
- [Risk] Metadata-only changes may skip validation. → Mitigation: Run the standard `pixi run test` and `pixi run compile` baselines after implementation.

## Migration Plan

1. Add the root `LICENSE` file.
2. Update README and matching docs to reference MIT License.
3. Run validation commands.

Rollback is removing the `LICENSE` file and reverting the documentation edits; no data migration is needed.

## Open Questions

- None. MIT License has been selected for this change.
