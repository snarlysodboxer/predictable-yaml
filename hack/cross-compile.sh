#!/usr/bin/env bash
set -euo pipefail

# Fetch latest embedded default configs
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo "Fetching embedded default configs..."
if ! "${SCRIPT_DIR}/fetch-default-configs.sh"; then
    echo "" >&2
    echo "ERROR: Failed to fetch default configs. Cannot build without embedded configs." >&2
    echo "  See errors above for details." >&2
    exit 1
fi

# Build for all target platforms
echo "Building release binaries..."
GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -o predictable-yaml-linux-amd64
GOARCH=arm64 GOOS=linux CGO_ENABLED=0 go build -o predictable-yaml-linux-arm64
GOARCH=amd64 GOOS=darwin CGO_ENABLED=0 go build -o predictable-yaml-darwin-amd64
GOARCH=arm64 GOOS=darwin CGO_ENABLED=0 go build -o predictable-yaml-darwin-arm64
GOARCH=amd64 GOOS=windows CGO_ENABLED=0 go build -o predictable-yaml-windows-amd64.exe
echo "Done."

echo "Don't forget to build the docker image and push it!"
