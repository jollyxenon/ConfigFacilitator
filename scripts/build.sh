#!/bin/bash
# Build script for cfgfc with version injection
# Usage: ./scripts/build.sh [version]
# If no version is provided, uses "dev" as default

VERSION="${1:-dev}"
MODULE_PATH="github.com/xenon/ConfigFacilitator/internal/cli"

echo "Building cfgfc with version: $VERSION"
go build -ldflags "-X ${MODULE_PATH}.version=${VERSION}" -o dist/cfgfc ./cmd/cfgfc
