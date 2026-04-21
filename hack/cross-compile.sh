#!/bin/bash
set -euo pipefail

# Fetch latest embedded default configs
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
"${SCRIPT_DIR}/fetch-default-configs.sh"

# Build for all target platforms
GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -o predictable-yaml-linux-amd64
GOARCH=arm64 GOOS=linux CGO_ENABLED=0 go build -o predictable-yaml-linux-arm64
GOARCH=amd64 GOOS=darwin CGO_ENABLED=0 go build -o predictable-yaml-darwin-amd64
GOARCH=arm64 GOOS=darwin CGO_ENABLED=0 go build -o predictable-yaml-darwin-arm64
GOARCH=amd64 GOOS=windows CGO_ENABLED=0 go build -o predictable-yaml-windows-amd64.exe
