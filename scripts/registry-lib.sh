#!/usr/bin/env bash
#
# Helpers shared by scripts/update-schema-registry.sh (which writes
# snapshots) and scripts/check-revisions.sh (which compares the live tree
# against them), so both build a module's JSON Schema companion the same
# way. Source it; it expects ROOT_DIR to be set.

# PROTO_ROUTE_PREFIXES/PROTO_ROUTE_FILES mirror tools/yang-proto-gen's
# pkgmap.go serviceRoutes table: a YANG module-name prefix maps to the
# generated proto file (under api/proto/openits/v1/) that module's
# notifications are packed into. Keep in sync with pkgmap.go if a
# service is added there. No two prefixes match the same module name,
# so match order does not matter.
PROTO_ROUTE_PREFIXES=(
    "openits-common-"
    "openits-signal-control"
    "openits-dms"
    "openits-ess"
    "openits-rsu"
    "openits-ramp-metering"
    "openits-perception"
    "openits-traffic-sensor"
    "openits-reversible-lane"
    "openits-cctv"
    "openits-zone-occupancy"
    "openits-work-zone"
)
# Paths are relative to api/proto/openits/. The generator emits nested
# per-service dirs (api/proto/openits/<service>/v1/events.proto), so each
# route is "<service>/v1/events.proto"; the module's per-notification
# schema.json files sit alongside in the same <service>/v1 directory.
PROTO_ROUTE_FILES=(
    "common/v1/events.proto"
    "signal_control/v1/events.proto"
    "dms/v1/events.proto"
    "ess/v1/events.proto"
    "rsu/v1/events.proto"
    "ramp_metering/v1/events.proto"
    "perception/v1/events.proto"
    "traffic_sensor/v1/events.proto"
    "reversible_lane/v1/events.proto"
    "cctv/v1/events.proto"
    "zone_occupancy/v1/events.proto"
    "work_zone/v1/events.proto"
)

# declares_notification returns success if the given YANG file declares
# at least one top-level `notification` statement (as opposed to merely
# mentioning the word "notification" in prose, e.g. a description).
declares_notification() {
    grep -qE '^[[:space:]]*notification[[:space:]]+[A-Za-z0-9_-]+[[:space:]]*\{' "$1"
}

# proto_file_for_module prints the api/proto/openits/v1/*.proto basename
# that moduleName's notifications are generated into, per
# PROTO_ROUTE_PREFIXES/PROTO_ROUTE_FILES. Returns failure (and prints
# nothing) if no route matches.
proto_file_for_module() {
    local m="$1"
    local i
    for i in "${!PROTO_ROUTE_PREFIXES[@]}"; do
        if [[ "${m}" == "${PROTO_ROUTE_PREFIXES[$i]}"* ]]; then
            echo "${PROTO_ROUTE_FILES[$i]}"
            return 0
        fi
    done
    return 1
}

# write_schema_json builds dest/schema.json for module m from its live
# (non-deprecated/non-obsolete) per-notification files under
# api/proto/openits/v1/<m>.<notification>.schema.json. One live
# notification -> a straight copy, named schema.json. More than one
# (e.g. openits-common-fault-events: fault-raised, fault-cleared,
# controller-fault-event) -> schema.json becomes a single JSON object
# keyed by notification name, each value that notification's full
# generated schema verbatim — the JSON-Schema analogue of schema.proto
# packing multiple notification messages into one file. Zero live
# notifications (all deprecated/obsolete) writes nothing — that is
# expected, not an error. Files are processed in byte order (LC_ALL=C)
# so a multi-notification schema.json's key order, and its bytes, do not
# depend on the locale of whoever runs the script: under en_US collation
# "rsu-tim-broadcast.schema.json" sorts before
# "rsu-tim-broadcast-state-changed.schema.json", under C after it, and the
# released snapshots were written in C order.
write_schema_json() {
    local m="$1" dest="$2"
    local proto_file json_dir
    proto_file="$(proto_file_for_module "${m}")" || return 0
    json_dir="${ROOT_DIR}/api/proto/openits/$(dirname "${proto_file}")"
    local json_files=()
    local line
    while IFS= read -r line; do
        json_files+=("${line}")
    done < <(find "${json_dir}" -maxdepth 1 \
        -name "${m}.*.schema.json" 2>/dev/null | LC_ALL=C sort)
    if [[ ${#json_files[@]} -eq 1 ]]; then
        cp "${json_files[0]}" "${dest}/schema.json"
    elif [[ ${#json_files[@]} -gt 1 ]]; then
        {
            echo "{"
            local first=1 jf notif
            for jf in "${json_files[@]}"; do
                notif="$(basename "${jf}" .schema.json)"
                notif="${notif#"${m}".}"
                if [[ "${first}" -eq 0 ]]; then
                    echo ","
                fi
                first=0
                printf '  "%s": ' "${notif}"
                cat "${jf}"
            done
            echo
            echo "}"
        } > "${dest}/schema.json"
    fi
}
