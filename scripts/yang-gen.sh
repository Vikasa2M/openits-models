#!/usr/bin/env bash
set -euo pipefail

# Generate Go structs from the openits YANG modules using ygot's generator.
# (YANG -> proto is handled separately by tools/yang-proto-gen; see
# scripts/proto-gen.sh and the Makefile `gen` target.)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
YANG_DIR="$ROOT_DIR/yang"
OUT_GO_DIR="${YANG_GO_OUT:-$ROOT_DIR/pkg/yang/openits}"

export PATH="$PATH:$(go env GOPATH)/bin"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'
log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

check_ygot() {
    if ! command -v generator &>/dev/null; then
        log_warn "ygot generator not found; installing..."
        go install github.com/openconfig/ygot/generator@v0.34.0
    fi
}

check_yang_files() {
    local files=(
        "$YANG_DIR/openits-types.yang"
        "$YANG_DIR/openits-device-diagnostics.yang"
        "$YANG_DIR/openits-cabinet-power.yang"
        "$YANG_DIR/openits-schedule.yang"
        "$YANG_DIR/openits-v2x-radio.yang"
        "$YANG_DIR/openits-v2x-messaging.yang"
        "$YANG_DIR/openits-v2x-radio-types.yang"
        "$YANG_DIR/openits-v2x-messaging-types.yang"
        "$YANG_DIR/openits-scms.yang"
        "$YANG_DIR/openits-vehicle-detection.yang"
        "$YANG_DIR/openits-zone-occupancy-types.yang"
        "$YANG_DIR/openits-zone-occupancy.yang"
        "$YANG_DIR/openits-zone-occupancy-events.yang"
        "$YANG_DIR/openits-work-zone-types.yang"
        "$YANG_DIR/openits-work-zone-events.yang"
        "$YANG_DIR/openits-signal-control-types.yang"
        "$YANG_DIR/openits-dms-types.yang"
        "$YANG_DIR/openits-ess-types.yang"
        "$YANG_DIR/openits-rsu-types.yang"
        "$YANG_DIR/openits-ramp-metering-types.yang"
        "$YANG_DIR/openits-common-comm-health-events.yang"
        "$YANG_DIR/openits-common-fault-events.yang"
        "$YANG_DIR/openits-common-mode-events.yang"
        "$YANG_DIR/openits-signal-control-events.yang"
        "$YANG_DIR/openits-nema-common.yang"
        "$YANG_DIR/openits-signal-control.yang"
        "$YANG_DIR/openits-rsu.yang"
        "$YANG_DIR/openits-rsu-events.yang"
        "$YANG_DIR/openits-dms.yang"
        "$YANG_DIR/openits-dms-events.yang"
        "$YANG_DIR/openits-ess.yang"
        "$YANG_DIR/openits-ess-events.yang"
        "$YANG_DIR/openits-ramp-metering.yang"
        "$YANG_DIR/openits-ramp-metering-events.yang"
        "$YANG_DIR/openits-traffic-sensor.yang"
        "$YANG_DIR/openits-traffic-sensor-events.yang"
        "$YANG_DIR/openits-reversible-lane.yang"
        "$YANG_DIR/openits-reversible-lane-events.yang"
        "$YANG_DIR/openits-perception.yang"
        "$YANG_DIR/openits-perception-events.yang"
        "$YANG_DIR/openits-cctv-types.yang"
        "$YANG_DIR/openits-cctv.yang"
        "$YANG_DIR/openits-cctv-events.yang"
        "$YANG_DIR/ietf/ietf-inet-types.yang"
        "$YANG_DIR/ietf/ietf-yang-types.yang"
    )
    for f in "${files[@]}"; do
        if [[ ! -f "$f" ]]; then
            log_error "Missing YANG file: $f"
            exit 1
        fi
    done
}

# Generate Go structs. ygot's Go backend does not support `notification`
# statements, so exclude the notifications companion module. (Notifications
# are carried by the generated per-event protobuf messages instead — see
# tools/yang-proto-gen.)
generate_go() {
    log_info "Generating Go code from openits YANG modules..."
    mkdir -p "$OUT_GO_DIR"

    # .gitattributes forces LF on checkout, so ygot reads yang/ directly.
    # A temp staging copy is unnecessary and puts an absolute temp path
    # into the generated header.
    local yang_src="$YANG_DIR"
    local yang_paths="$yang_src:$yang_src/ietf"

    generator \
        -path="$yang_paths" \
        -output_file="$OUT_GO_DIR/openits.go" \
        -package_name=openits \
        -generate_fakeroot \
        -fakeroot_name=Device \
        -shorten_enum_leaf_names \
        -typedef_enum_with_defmod \
        -enum_suffix_for_simple_union_enums \
        -generate_rename \
        -generate_append \
        -generate_getters \
        -generate_delete \
        -generate_leaf_getters \
        -include_schema \
        -ignore_unsupported \
        -exclude_modules=ietf-inet-types,ietf-yang-types,openits-device-diagnostics,openits-cabinet-power,openits-schedule,openits-v2x-radio,openits-v2x-messaging,openits-scms,openits-vehicle-detection,openits-zone-occupancy,openits-zone-occupancy-events,openits-dms-events,openits-ess-events,openits-rsu-events,openits-ramp-metering-events,openits-common-comm-health-events,openits-common-fault-events,openits-common-mode-events,openits-signal-control-events,openits-traffic-sensor-events,openits-reversible-lane-events,openits-perception-events,openits-cctv-events,openits-work-zone-events \
        "$yang_src/openits-types.yang" \
        "$yang_src/openits-device-diagnostics.yang" \
        "$yang_src/openits-cabinet-power.yang" \
        "$yang_src/openits-schedule.yang" \
        "$yang_src/openits-v2x-radio.yang" \
        "$yang_src/openits-v2x-messaging.yang" \
        "$yang_src/openits-v2x-radio-types.yang" \
        "$yang_src/openits-v2x-messaging-types.yang" \
        "$yang_src/openits-scms.yang" \
        "$yang_src/openits-vehicle-detection.yang" \
        "$yang_src/openits-zone-occupancy-types.yang" \
        "$yang_src/openits-zone-occupancy.yang" \
        "$yang_src/openits-zone-occupancy-events.yang" \
        "$yang_src/openits-work-zone-types.yang" \
        "$yang_src/openits-work-zone-events.yang" \
        "$yang_src/openits-signal-control-types.yang" \
        "$yang_src/openits-dms-types.yang" \
        "$yang_src/openits-ess-types.yang" \
        "$yang_src/openits-rsu-types.yang" \
        "$yang_src/openits-ramp-metering-types.yang" \
        "$yang_src/openits-nema-common.yang" \
        "$yang_src/openits-signal-control.yang" \
        "$yang_src/openits-rsu.yang" \
        "$yang_src/openits-dms.yang" \
        "$yang_src/openits-ess.yang" \
        "$yang_src/openits-ramp-metering.yang" \
        "$yang_src/openits-traffic-sensor.yang" \
        "$yang_src/openits-reversible-lane.yang" \
        "$yang_src/openits-perception.yang" \
        "$yang_src/openits-cctv-types.yang" \
        "$yang_src/openits-cctv.yang"

    normalize_go_header "$OUT_GO_DIR/openits.go"
    log_info "Generated: $OUT_GO_DIR/openits.go"
}

# ygot writes machine-specific absolute paths into the generated file's header
# comment: the GOPATH module-cache path of its own generator, and the absolute
# path of every YANG input file. Both differ per checkout location / CI runner,
# so `make check-gen` (git diff after regen) would spuriously fail elsewhere.
# Normalize them to stable, repo-relative forms. Only the header comment is
# touched; real content drift is still caught.
normalize_go_header() {
    local f="$1"
    # ygot embeds absolute input paths in the leading block comment (through
    # the closing */ before `package`). Rewrite only that header. A whole-file
    # substitution would also rewrite any .../yang/ string that ends up inside
    # an embedded YANG description.
    local win_root=""
    if command -v cygpath >/dev/null 2>&1; then
        win_root=$(cygpath -m "$ROOT_DIR" 2>/dev/null || true)
    fi
    local header body
    header=$(mktemp)
    body=$(mktemp)
    awk -v hdr="$header" -v bod="$body" '
        !done && /^\*\// { print > hdr; done=1; next }
        !done { print > hdr; next }
        { print > bod }
    ' "$f"
    local sed_args=(
        -e 's|by [^ ]*/github.com/openconfig/ygot|by github.com/openconfig/ygot|'
        -e "s|${ROOT_DIR}/||g"
        -e 's|\\|/|g'
        # ygot prints -path as one include entry. The script passes
        # <yang>:<yang>/ietf, so Linux CI emits "yang:yang/ietf/...".
        # A greedy ".../yang/" collapse turns the Windows form into
        # "yang/ietf/..." and fails check-gen. Force the one canonical line.
        -e 's|^\t- .*ietf.*$|\t- yang:yang/ietf/...|'
    )
    if [ -n "$win_root" ]; then
        sed_args+=(-e "s|${win_root}/||g")
    fi
    sed "${sed_args[@]}" "$header" >"$header.norm"
    cat "$header.norm" "$body" >"$f.tmp"
    # Never ship CR in the generated Go.
    tr -d '\r' <"$f.tmp" >"$f"
    rm -f "$header" "$header.norm" "$body" "$f.tmp"
}

main() {
    log_info "Starting YANG code generation..."
    check_yang_files

    case "${1:-go}" in
        go)
            check_ygot
            generate_go
            ;;
        *)
            echo "Usage: $0 [go]"
            exit 1
            ;;
    esac

    log_info "YANG code generation complete."
}

main "$@"
