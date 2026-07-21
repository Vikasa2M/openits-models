#!/usr/bin/env bash
# Build the curated release bundle for a given tag.
#
#   scripts/build-release-bundle.sh v0.1.0
#
# Produces, in the current directory:
#   openits-models-<tag>.zip
#   openits-models-<tag>.tar.gz
#   SHA256SUMS
#
# The bundle is a clean subset for non-Go / other-language consumers: the
# models and specs, without the Go-generated pkg/, tooling, tests, or docs.
# (Go consumers use the tag directly via `go get`; GitHub's auto source
# archive covers "the whole repo".)  Used by both the manual tag-triggered
# release workflow and the release-please asset-upload job so the two paths
# produce byte-identical bundles.
set -euo pipefail

TAG="${1:?usage: build-release-bundle.sh <tag>}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
OUT_DIR="$(pwd)"

STAGE_PARENT="$(mktemp -d)"
trap 'rm -rf "$STAGE_PARENT"' EXIT
STAGE="$STAGE_PARENT/openits-models-${TAG}"

mkdir -p "$STAGE/api"
cp -r "$ROOT_DIR/yang"            "$STAGE/yang"
cp -r "$ROOT_DIR/api/proto"       "$STAGE/api/proto"
rm -rf "$STAGE/api/proto/yang"    # drop the ygot-generated extension tree
cp -r "$ROOT_DIR/schema-registry" "$STAGE/schema-registry"
cp -r "$ROOT_DIR/bindings"        "$STAGE/bindings"
cp "$ROOT_DIR/CHANGELOG.md" \
   "$ROOT_DIR/LICENSE" "$ROOT_DIR/NOTICE" "$STAGE/"

( cd "$STAGE_PARENT" && zip -qr "$OUT_DIR/openits-models-${TAG}.zip" "openits-models-${TAG}" )
tar -czf "$OUT_DIR/openits-models-${TAG}.tar.gz" -C "$STAGE_PARENT" "openits-models-${TAG}"

# sha256sum on Linux/CI; shasum on macOS.
if command -v sha256sum >/dev/null 2>&1; then
    SHA256=(sha256sum)
else
    SHA256=(shasum -a 256)
fi
( cd "$OUT_DIR" && "${SHA256[@]}" "openits-models-${TAG}.zip" "openits-models-${TAG}.tar.gz" > SHA256SUMS )

echo "built:"
( cd "$OUT_DIR" && ls -1 "openits-models-${TAG}.zip" "openits-models-${TAG}.tar.gz" SHA256SUMS )
