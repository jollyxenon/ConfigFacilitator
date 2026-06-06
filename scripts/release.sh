#!/bin/bash
# Cross-compile cfgfc for all supported platforms, package archives,
# generate checksums, and (optionally) create a GitHub release.
#
# Usage:
#   ./scripts/release.sh              # build + package only
#   ./scripts/release.sh --publish    # also create GH release + upload assets

set -euo pipefail

VERSION="${CFGFC_VERSION:?Set CFGFC_VERSION, e.g. 0.1.0}"
MODULE_PATH="github.com/xenon/ConfigFacilitator/internal/cli"
DIST_DIR="dist"
BINARY_BASE="cfgfc"

# Platform matrix: GOOS GOARCH
TARGETS=(
  "linux   amd64"
  "linux   arm64"
  "darwin  amd64"
  "darwin  arm64"
  "windows amd64"
  "windows arm64"
)

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

echo "==> Building cfgfc v${VERSION} for all platforms"

for target in "${TARGETS[@]}"; do
  read -r goos goarch <<< "$target"
  archive="${BINARY_BASE}_${VERSION}_${goos}_${goarch}"

  echo "  -> ${goos}/${goarch}"
  bin_name="$BINARY_BASE"
  [[ "$goos" == "windows" ]] && bin_name="${BINARY_BASE}.exe"

  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -ldflags "-s -w -X ${MODULE_PATH}.version=${VERSION}" \
    -o "${DIST_DIR}/${bin_name}" ./cmd/cfgfc

  pushd "$DIST_DIR" > /dev/null
  if [[ "$goos" == "windows" ]]; then
    7z a -tzip "${archive}.zip" "$bin_name" > /dev/null
  else
    tar -czf "${archive}.tar.gz" "$bin_name"
  fi
  rm -f "$bin_name"
  popd > /dev/null
done

echo "==> Generating checksums"
pushd "$DIST_DIR" > /dev/null
sha256sum *.tar.gz *.zip > checksums.txt
popd > /dev/null

echo "==> Artifacts:"
ls -lh "$DIST_DIR"

if [[ "${1:-}" == "--publish" ]]; then
  echo "==> Creating git tag v${VERSION}"
  git tag -a "v${VERSION}" -m "Release v${VERSION}"
  git push origin "v${VERSION}"

  echo "==> Creating GitHub release"
  gh release create "v${VERSION}" \
    --repo jollyxenon/ConfigFacilitator \
    --title "v${VERSION}" \
    --notes "Release v${VERSION}" \
    "${DIST_DIR}"/*.tar.gz "${DIST_DIR}"/*.zip "${DIST_DIR}/checksums.txt"

  echo "==> GitHub release v${VERSION} published"
fi
