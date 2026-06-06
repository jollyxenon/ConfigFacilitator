# ConfigFacilitator npm package

This package installs the `cfgfc` command by downloading the matching prebuilt Go binary from the GitHub Release whose tag matches this package version.

```bash
npm install -g @jollyxenon/cfgfc
cfgfc --help
```

The npm package is only a thin installation and dispatch layer. All command behavior is implemented by the Go binary built from `cmd/cfgfc`.
