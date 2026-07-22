#!/usr/bin/env bash
set -euo pipefail

# Ensure GOPATH/bin is in PATH for protoc-gen-go
export PATH="$PATH:$(go env GOPATH)/bin"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
PROTO_DIR="$ROOT_DIR/api/proto"
OUT_DIR="$ROOT_DIR/pkg/proto"
OPENITS_OUT="$OUT_DIR/openits/v1"

mkdir -p "$OPENITS_OUT"

if ! command -v protoc &> /dev/null; then
    echo "Error: protoc is not installed"
    echo "Install with: brew install protobuf"
    exit 1
fi

if ! command -v protoc-gen-go &> /dev/null; then
    echo "Error: protoc-gen-go is not installed"
    echo "Install with: go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11"
    exit 1
fi

echo "Generating top-level core protobuf code (command, device)..."

# Top-level command.proto + device.proto generate into the openits Go package.
# Per-file go_package directives route them to pkg/proto/openits/v1.
protoc \
    --proto_path="$PROTO_DIR" \
    --go_out="$OUT_DIR" \
    --go_opt=paths=source_relative \
    --go_opt=Mcommand.proto=github.com/Vikasa2M/openits-models/pkg/proto/openits/v1 \
    --go_opt=Mdevice.proto=github.com/Vikasa2M/openits-models/pkg/proto/openits/v1 \
    "$PROTO_DIR"/command.proto "$PROTO_DIR"/device.proto

# protoc with paths=source_relative respects the source path. command.proto and
# device.proto live at api/proto/, so they would generate to pkg/proto/. We move
# them into openits/v1/ to match their go_package directive.
mv -f "$OUT_DIR"/command.pb.go "$OPENITS_OUT"/ 2>/dev/null || true
mv -f "$OUT_DIR"/device.pb.go  "$OPENITS_OUT"/ 2>/dev/null || true

echo "Generating openits per-service protobuf code..."

# api/proto/openits/**/*.proto is now 100% generated from YANG by
# tools/yang-proto-gen (see Makefile `gen` target); this includes
# openits/types/v1/types.proto (shared WireSource) alongside every
# per-service events.proto / state.proto. Each generated file carries its
# own per-service `option go_package` (see tools/yang-proto-gen's
# goPackageFor), so paths=source_relative routes it to
# pkg/proto/<svc-path>/v1/ automatically — one protoc invocation compiles
# every service into its own Go package. Use `find` (not bash globstar) to
# recurse into every service directory: the default /bin/bash on macOS is
# 3.2, which predates globstar. protoc v35.1 requires at least one output
# directive, which --go_out above already satisfies.
GENERATED_PROTOS=()
while IFS= read -r -d '' f; do
    GENERATED_PROTOS+=("$f")
done < <(find "$PROTO_DIR/openits" -name '*.proto' -print0)

if [ ${#GENERATED_PROTOS[@]} -eq 0 ]; then
    echo "Error: no generated .proto files found under $PROTO_DIR/openits" >&2
    exit 1
fi

protoc \
    --proto_path="$PROTO_DIR" \
    --go_out="$OUT_DIR" \
    --go_opt=paths=source_relative \
    "${GENERATED_PROTOS[@]}"

# protoc-gen-go and ygot are version-pinned (see the `go install` hints above),
# but protoc itself is a system binary whose version is stamped into every
# generated header (`//  protoc  vX.Y.Z`). A runner with a different protoc
# would therefore drift the headers and false-fail `make check-gen`. Normalize
# that one version line out; real content differences (which a genuinely
# incompatible protoc would produce) are still caught by the diff. Walk every
# generated pkg/proto/openits/**/*.pb.go recursively — not just the old flat
# openits/v1/ — since per-service output now spans one directory per service
# (this also covers command.pb.go/device.pb.go, which the block above already
# moved into $OPENITS_OUT).
while IFS= read -r -d '' f; do
    sed -E 's|(//[[:space:]]*protoc[[:space:]]+)v[0-9][0-9.]*|\1(normalized)|' "$f" >"$f.tmp" && mv "$f.tmp" "$f"
done < <(find "$OUT_DIR/openits" -name '*.pb.go' -print0)

echo "Generated protobuf code in $OUT_DIR"
