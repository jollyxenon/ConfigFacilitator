## Why

ConfigFacilitator currently states that the repository has no `LICENSE` file, which leaves reuse, redistribution, and contribution terms unclear. Adding an explicit MIT License makes the Go CLI easy to adopt while preserving copyright attribution requirements.

## What Changes

- Add a repository-root `LICENSE` file containing the standard MIT License text.
- Update the root README license section to identify MIT as the project license.
- Update English and Chinese user-facing documentation so license information is consistent across published docs.

## Capabilities

### New Capabilities
- `repository-licensing`: Defines how the repository declares its open-source license and how user-facing docs surface that license.

### Modified Capabilities
- `docs-and-hardening`: Documentation parity requirements now include keeping license information aligned in English and Chinese docs.

## Impact

- Affected files: root `LICENSE`, root `README.md`, and relevant English/Chinese docs under `docs/`.
- No CLI command behavior, runtime APIs, data formats, dependencies, or build outputs change.
- Implementation is documentation- and metadata-only; validation should confirm the files exist and the Go test/compile baselines still pass.
