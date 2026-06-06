## 1. License Metadata

- [x] 1.1 Add a repository-root `LICENSE` file with standard MIT License text and project copyright notice.
- [x] 1.2 Verify the root `LICENSE` file is present and names MIT License.

## 2. Documentation Updates

- [x] 2.1 Update the root `README.md` license section to identify MIT License and reference `LICENSE`.
- [x] 2.2 Update matching English and Chinese docs under `docs/` wherever repository license metadata is surfaced.
- [x] 2.3 Review English and Chinese license wording for parity.

## 3. Validation

- [x] 3.1 Run `pixi run test` to confirm the documentation-only change does not break the Go test suite.
- [x] 3.2 Run `pixi run compile` to confirm the project still compiles.
- [x] 3.3 Run `openspec status --change "add-mit-license"` and confirm the change remains apply-ready before implementation completion.
