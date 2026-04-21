#!/usr/bin/env bash
set -euo pipefail

# Fetches configs from the predictable-yaml-configs repo into internal/embedded/configs/.
# Run from the repo root.

CONFIGS_REPO="${CONFIGS_REPO:-https://github.com/snarlysodboxer/predictable-yaml-configs}"
CONFIGS_TAG="${CONFIGS_TAG:-latest}"

mkdir -p internal/embedded/configs

# If CONFIGS_TAG is "latest", resolve it to the most recent tag
if [ "${CONFIGS_TAG}" = "latest" ]; then
    REPO_PATH=$(echo "${CONFIGS_REPO}" | sed 's|.*github.com/||' | sed 's|\.git$||')
    API_URL="https://api.github.com/repos/${REPO_PATH}/tags?per_page=1"
    echo "Resolving latest tag from ${API_URL}..."
    API_RESPONSE=$(curl -sL "${API_URL}")
    CONFIGS_TAG=$(echo "${API_RESPONSE}" | grep '"name"' | head -1 | sed 's/.*"name": *"//;s/".*//' || true)
    if [ -z "${CONFIGS_TAG}" ]; then
        echo "ERROR: Could not determine latest tag from ${CONFIGS_REPO}." >&2
        echo "  API URL: ${API_URL}" >&2
        echo "  Response: ${API_RESPONSE}" >&2
        echo "" >&2
        echo "  Make sure the repo exists and has at least one tag." >&2
        echo "  To use a specific tag instead, run: CONFIGS_TAG=v1.0.0 $0" >&2
        exit 1
    fi
fi

echo "Fetching configs from ${CONFIGS_REPO} at ${CONFIGS_TAG}..."

TARBALL_URL="${CONFIGS_REPO}/archive/${CONFIGS_TAG}.tar.gz"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

TARBALL_FILE="${TMP_DIR}/configs.tar.gz"
HTTP_CODE=$(curl -sL -o "${TARBALL_FILE}" -w "%{http_code}" "${TARBALL_URL}")
if [ "${HTTP_CODE}" -lt 200 ] || [ "${HTTP_CODE}" -ge 300 ]; then
    echo "ERROR: Failed to download tarball (HTTP ${HTTP_CODE})." >&2
    echo "  URL: ${TARBALL_URL}" >&2
    echo "" >&2
    echo "  Make sure the repo exists and the tag '${CONFIGS_TAG}' is valid." >&2
    exit 1
fi

if ! tar xz -C "${TMP_DIR}" -f "${TARBALL_FILE}"; then
    echo "ERROR: Failed to extract tarball." >&2
    echo "  URL: ${TARBALL_URL}" >&2
    exit 1
fi

# Find the extracted directory (tarball has a top-level dir)
EXTRACTED_DIR=$(find "${TMP_DIR}" -mindepth 1 -maxdepth 1 -type d | head -1)
if [ -z "${EXTRACTED_DIR}" ]; then
    echo "ERROR: Tarball extraction produced no directory." >&2
    echo "  URL: ${TARBALL_URL}" >&2
    exit 1
fi

# Copy only YAML files to the destination
rm -f internal/embedded/configs/*.yaml internal/embedded/configs/*.yml
found_files=false
for f in "${EXTRACTED_DIR}"/*.yaml "${EXTRACTED_DIR}"/*.yml; do
    if [ -f "$f" ]; then
        cp "$f" internal/embedded/configs
        found_files=true
    fi
done

if [ "${found_files}" = false ]; then
    echo "ERROR: No YAML files found in tarball." >&2
    echo "  Repo: ${CONFIGS_REPO}" >&2
    echo "  Tag: ${CONFIGS_TAG}" >&2
    echo "  Extracted dir contents: $(ls -la "${EXTRACTED_DIR}")" >&2
    exit 1
fi

echo "Embedded configs updated from ${CONFIGS_REPO} at ${CONFIGS_TAG}."
