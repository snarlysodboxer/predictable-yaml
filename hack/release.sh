#!/usr/bin/env bash
set -euo pipefail

# --- Gather info ---

CURRENT_VERSION=$(grep '^\s*version = ' flake.nix | head -1 | sed 's/.*version = "//;s/".*//')
CURRENT_CONFIGS_REV=$(grep '^\s*rev = ' flake.nix | head -1 | sed 's/.*rev = "//;s/".*//')
echo "Current predictable-yaml version: ${CURRENT_VERSION}"
echo "Current predictable-yaml-configs rev: ${CURRENT_CONFIGS_REV}"
echo ""

# --- Prompt for configs tag ---

read -rp "predictable-yaml-configs tag to embed (enter to keep '${CURRENT_CONFIGS_REV}'): " CONFIGS_TAG
CONFIGS_TAG="${CONFIGS_TAG:-${CURRENT_CONFIGS_REV}}"

# --- Prompt for new version ---

read -rp "New predictable-yaml version (enter to keep '${CURRENT_VERSION}'): " NEW_VERSION
NEW_VERSION="${NEW_VERSION:-${CURRENT_VERSION}}"

echo ""
echo "Will release with:"
echo "  predictable-yaml version:      ${NEW_VERSION}"
echo "  predictable-yaml-configs tag:  ${CONFIGS_TAG}"
echo ""
read -rp "Continue? [y/N] " CONFIRM
if [[ "${CONFIRM}" != "y" && "${CONFIRM}" != "Y" ]]; then
    echo "Aborted."
    exit 0
fi

# --- Update flake.nix: configs rev (always write desired value) ---

echo ""
echo "Setting flake.nix configs rev to ${CONFIGS_TAG}..."
sed -i 's|rev = ".*"|rev = "'"${CONFIGS_TAG}"'"|' flake.nix

# --- Update flake.nix: configs hash ---

echo "Determining configs hash (nix build will fail once to report the correct hash)..."
CURRENT_CONFIGS_HASH=$(grep -A3 'predictable-yaml-configs' flake.nix | grep 'hash = ' | sed 's/.*hash = "//;s/".*//')

# Set a dummy hash to force nix to compute the real one
sed -i 's|hash = ".*"|hash = ""|' flake.nix
NEW_CONFIGS_HASH=$(nix build .#predictable-yaml 2>&1 | grep 'got:' | awk '{print $2}' || true)
if [ -z "${NEW_CONFIGS_HASH}" ]; then
    # Restore old hash so the file isn't left broken
    sed -i 's|hash = ""|hash = "'"${CURRENT_CONFIGS_HASH}"'"|' flake.nix
    echo "ERROR: Could not determine configs hash. Fix flake.nix manually and re-run." >&2
    exit 1
fi
sed -i 's|hash = ""|hash = "'"${NEW_CONFIGS_HASH}"'"|' flake.nix
echo "Configs hash: ${NEW_CONFIGS_HASH}"

# --- Update flake.nix: version (always write desired value) ---

echo "Setting flake.nix version to ${NEW_VERSION}..."
sed -i 's|version = ".*"|version = "'"${NEW_VERSION}"'"|' flake.nix

# --- Verify nix build ---

echo "Building with nix to verify..."
nix build
echo "Nix build succeeded."

# --- Fetch embedded configs and cross-compile ---

echo "Fetching embedded configs for release binaries..."
CONFIGS_TAG="${CONFIGS_TAG}" hack/fetch-default-configs.sh

echo "Cross-compiling release binaries..."
LDFLAGS="-X github.com/snarlysodboxer/predictable-yaml/cmd.Version=${NEW_VERSION}"
GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o predictable-yaml-linux-amd64
GOARCH=arm64 GOOS=linux CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o predictable-yaml-linux-arm64
GOARCH=amd64 GOOS=darwin CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o predictable-yaml-darwin-amd64
GOARCH=arm64 GOOS=darwin CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o predictable-yaml-darwin-arm64
GOARCH=amd64 GOOS=windows CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o predictable-yaml-windows-amd64.exe
echo "Binaries built."

# --- Build docker image ---

DOCKER_TAG="snarlysodboxer/predictable-yaml:${NEW_VERSION}"
DOCKER_LATEST="snarlysodboxer/predictable-yaml:latest"
echo "Building docker image ${DOCKER_TAG}..."
docker build --build-arg "VERSION=${NEW_VERSION}" -t "${DOCKER_TAG}" -t "${DOCKER_LATEST}" .
echo "Docker image built."

# --- Done ---

echo ""
echo "=========================================="
echo "Release ${NEW_VERSION} built successfully!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. commit and push and merge PR"
echo "  2. git tag -s ${NEW_VERSION}"
echo "  3. git push origin ${NEW_VERSION} # push the tag"
echo "  4. docker push ${DOCKER_TAG}"
echo "  5. docker push ${DOCKER_LATEST}"
echo "  6. gh release create ${NEW_VERSION} predictable-yaml-linux-amd64 predictable-yaml-linux-arm64 predictable-yaml-darwin-amd64 predictable-yaml-darwin-arm64 predictable-yaml-windows-amd64.exe --title ${NEW_VERSION} --generate-notes"
