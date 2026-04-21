#!/usr/bin/env bash
set -euo pipefail

# Fetches the latest tagged release of default configs from the predictable-yaml-configs repo.
# Used by go generate to populate internal/embedded/configs/ before build.

CONFIGS_REPO="${CONFIGS_REPO:-https://github.com/snarlysodboxer/predictable-yaml-configs}"
CONFIGS_TAG="${CONFIGS_TAG:-latest}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEST_DIR="${SCRIPT_DIR}/../internal/embedded/configs"

mkdir -p "${DEST_DIR}"

# If CONFIGS_TAG is "latest", resolve it to the actual latest tag
if [ "${CONFIGS_TAG}" = "latest" ]; then
    # Extract owner/repo from URL
    REPO_PATH=$(echo "${CONFIGS_REPO}" | sed 's|.*github.com/||' | sed 's|\.git$||')
    CONFIGS_TAG=$(curl -sL "https://api.github.com/repos/${REPO_PATH}/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"//;s/".*//')
    if [ -z "${CONFIGS_TAG}" ]; then
        echo "ERROR: Could not determine latest tag from ${CONFIGS_REPO}." >&2
        exit 1
    fi
fi

echo "Fetching configs from ${CONFIGS_REPO} at ${CONFIGS_TAG}..."

TARBALL_URL="${CONFIGS_REPO}/archive/${CONFIGS_TAG}.tar.gz"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

if ! curl -sL --fail "${TARBALL_URL}" | tar xz -C "${TMP_DIR}"; then
    echo "ERROR: Could not fetch configs from ${TARBALL_URL}." >&2
    exit 1
fi

# Find the extracted directory (tarball has a top-level dir)
EXTRACTED_DIR=$(find "${TMP_DIR}" -mindepth 1 -maxdepth 1 -type d | head -1)
if [ -z "${EXTRACTED_DIR}" ]; then
    echo "ERROR: Tarball extraction produced no directory." >&2
    exit 1
fi

# Copy only YAML files to the destination
rm -f "${DEST_DIR}"/*.yaml "${DEST_DIR}"/*.yml
found_files=false
for f in "${EXTRACTED_DIR}"/*.yaml "${EXTRACTED_DIR}"/*.yml; do
    if [ -f "$f" ]; then
        cp "$f" "${DEST_DIR}/"
        found_files=true
    fi
done

if [ "${found_files}" = false ]; then
    echo "ERROR: No YAML files found in ${CONFIGS_REPO} at ${CONFIGS_TAG}." >&2
    exit 1
fi

echo "Embedded configs updated from ${CONFIGS_REPO} at ${CONFIGS_TAG}."
