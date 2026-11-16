#!/usr/bin/env bash
#
# Compile every generated .proto under api/proto/openits into one descriptor
# set and render each module's wire shape from it into api/shapes/:
#
#   api/shapes/<module>.shape  every message and enum the module's root
#                              messages reach, canonically rendered
#   api/shapes/<module>.proto  the messages the module itself declares
#
# The shape is taken from what protoc compiles, not from the YANG, so it moves
# whenever the wire moves: a YANG edit, a change to the YANG-to-proto mapping
# in tools/yang-proto-gen, or a change to a shared type a module imports.
# scripts/update-schema-registry.sh snapshots both files into each new
# revision, and scripts/check-revisions.sh fails when a module's shape no
# longer matches the snapshot of its current revision.
#
# Runs as part of `make gen`, after `proto`.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
PROTO_DIR="$ROOT_DIR/api/proto"

if ! command -v protoc &> /dev/null; then
    echo "Error: protoc is not installed" >&2
    exit 1
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

PROTOS=()
while IFS= read -r -d '' f; do
    PROTOS+=("${f#"$PROTO_DIR"/}")
done < <(find "$PROTO_DIR/openits" -name '*.proto' -print0 | sort -z)

protoc --proto_path="$PROTO_DIR" --include_imports \
    --descriptor_set_out="$TMP/openits.binpb" "${PROTOS[@]}"

cd "$ROOT_DIR"
go run ./tools/yang-proto-gen -shapes -yang yang \
    -descriptors "$TMP/openits.binpb" -out api/shapes
