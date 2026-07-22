#!/usr/bin/env bash
#
# Snapshot the current YANG modules (and any generated .proto
# companions) into the content-addressable schema registry tree under
# schema-registry/<module>/<revision>/.
#
# The revision tag is read from the module's own most-recent `revision`
# statement — NOT from today's date.  Modules whose declared revision
# is already present in the registry are skipped cleanly; modules whose
# declared revision is new are snapshotted.  The script therefore:
#
#   - never creates a spurious snapshot under today's date for a module
#     that has not actually changed, and
#   - never aborts the whole batch just because one module already has
#     its current revision snapshotted.
#
# "Immutable" applies to schema.yang (the revision's actual content
# contract) and schema.proto (derived 1:1 from it). A snapshot that
# predates a newly-added companion artifact type — e.g. schema.json,
# introduced after that snapshot was taken — is backfilled with just that artifact in
# place, rather than requiring the revision to be deleted and
# recreated: the backfill never touches schema.yang/schema.proto/
# README.md, so the revision's actual pinned contract is unchanged.
#
# Every snapshot always carries schema.yang. A module that declares at
# least one live `notification` statement also carries schema.proto —
# a copy of the generated proto file its notifications land in under
# api/proto/openits/v1/ (tools/yang-proto-gen packs notifications by
# service, e.g. every openits-common-* module's events land in
# common_events.proto; see tools/yang-proto-gen/pkgmap.go's
# serviceRoutes, mirrored below). Modules with no notification
# statements of their own (service-core state/config modules, *-types
# modules, shared-typedef modules) carry schema.yang only — there is
# no generated proto for their content post-refactor.
#
# A notification-bearing module also carries schema.json — built from
# the per-notification files tools/yang-proto-gen emits at
# api/proto/openits/v1/<module>.<notification>.schema.json (one file
# per live notification; see main.go's emitJSONSchemas). A module with
# exactly one live notification gets a straight copy, named
# schema.json. A module with more than one (e.g.
# openits-common-fault-events: fault-raised, fault-cleared) gets
# schema.json as a single JSON object keyed by notification name,
# each value that notification's full generated schema verbatim —
# the JSON-Schema analogue of schema.proto
# packing multiple notification messages into one file. A module whose
# notifications are all deprecated/obsolete has zero live schema.json
# files and carries no schema.json — that is expected, not an error.
#
# Usage: scripts/update-schema-registry.sh [registry-dir]
set -euo pipefail

REGISTRY="${1:-schema-registry}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

MODULES=(
    openits-types
    openits-device-diagnostics
    openits-v2x-radio
    openits-v2x-messaging
    openits-scms
    openits-vehicle-detection
    openits-cabinet-power
    openits-nema-common
    openits-signal-control
    openits-rsu
    openits-rsu-events
    openits-dms
    openits-dms-events
    openits-ess
    openits-ess-events
    openits-ramp-metering
    openits-ramp-metering-events
    openits-traffic-sensor
    openits-traffic-sensor-events
    openits-reversible-lane
    openits-reversible-lane-events
    openits-perception
    openits-perception-events
    openits-cctv
    openits-cctv-events
    openits-cctv-types
    openits-signal-control-types
    openits-dms-types
    openits-ess-types
    openits-rsu-types
    openits-v2x-radio-types
    openits-v2x-messaging-types
    openits-ramp-metering-types
    openits-perception-types
    openits-reversible-lane-types
    openits-traffic-sensor-types
    openits-common-comm-health-events
    openits-common-fault-events
    openits-common-mode-events
    openits-signal-control-events
    openits-vendor-econolite-signal-control-types
    openits-vendor-trafficvision-traffic-sensor-types
    openits-vendor-trafficvision-perception-types
)

# Augment modules contributed by vendors / agencies. Snapshotted alongside
# core modules but live under yang/augments/ rather than yang/. The
# loop below globs yang/augments/*.yang so new augments are picked up
# without touching this script.
AUGMENT_GLOB="${ROOT_DIR}/yang/augments/*.yang"

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

# read_revision prints the most-recent YANG revision date declared in
# the given module file.  YANG `revision` statements are convention-
# ordered newest-first, so the first one we see wins.
read_revision() {
    local file="$1"
    awk '
      /^[[:space:]]*revision[[:space:]]+[0-9]{4}-[0-9]{2}-[0-9]{2}/ {
        for (i = 1; i <= NF; i++) {
          if ($i ~ /^[0-9]{4}-[0-9]{2}-[0-9]{2}$/) {
            print $i
            exit
          }
        }
      }
    ' "$file"
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
# expected, not an error. Files are processed in sorted order so a
# multi-notification schema.json's key order (and bytes) is
# deterministic.
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
        -name "${m}.*.schema.json" 2>/dev/null | sort)
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

# write_manifest (re)writes dest/MANIFEST.json for module m at revision
# rev, listing whichever of schema.yang/schema.proto/schema.json are
# actually present in dest. Used both for a brand-new snapshot and to
# refresh the files list after a same-revision backfill (e.g. schema.json
# added to a snapshot that predates it).
write_manifest() {
    local m="$1" rev="$2" dest="$3"
    cat > "${dest}/MANIFEST.json" <<EOF
{
  "module": "${m}",
  "revision": "${rev}",
  "files": [
    "schema.yang"$([[ -f "${dest}/schema.proto" ]] && echo ",
    \"schema.proto\"")$([[ -f "${dest}/schema.json" ]] && echo ",
    \"schema.json\"")
  ]
}
EOF
}

wrote_any=0

for m in "${MODULES[@]}"; do
    src_yang="${ROOT_DIR}/yang/${m}.yang"
    if [[ ! -f "${src_yang}" ]]; then
        echo "Warning: ${src_yang} not found — skipping ${m}" >&2
        continue
    fi

    rev="$(read_revision "${src_yang}")"
    if [[ -z "${rev}" ]]; then
        echo "Error: ${m} has no revision statement; refusing to snapshot" >&2
        exit 1
    fi

    dest="${REGISTRY}/${m}/${rev}"
    if [[ -d "${dest}" ]]; then
        # Existing, revision-immutable snapshot. schema.yang/schema.proto
        # are already correct for this revision and are never rewritten
        # here. The one thing worth backfilling in place is a missing
        # schema.json — additive, derived, and never part of what
        # check-revisions.sh (or the ce-dataschema contract) pins.
        if declares_notification "${src_yang}" && [[ ! -f "${dest}/schema.json" ]]; then
            write_schema_json "${m}" "${dest}"
            if [[ -f "${dest}/schema.json" ]]; then
                write_manifest "${m}" "${rev}" "${dest}"
                echo "Backfilled schema.json into ${dest}"
                wrote_any=1
                continue
            fi
        fi
        echo "Skip  ${m}@${rev} (already present; revisions are immutable)"
        continue
    fi
    mkdir -p "${dest}"

    cp "${src_yang}" "${dest}/schema.yang"

    # .proto (+ .json) companions — only for modules that declare
    # notifications of their own. The generated proto lives under
    # api/proto/openits/v1/, packed per service (see
    # PROTO_ROUTE_PREFIXES/PROTO_ROUTE_FILES above), not per module —
    # this is the successor to the deleted api/proto/yang/** tree,
    # which had no 1:1 per-module mapping to the new generator's
    # output.
    if declares_notification "${src_yang}"; then
        proto_file="$(proto_file_for_module "${m}")" || {
            echo "Error: ${m} declares notifications but matches no proto route in PROTO_ROUTE_PREFIXES (see tools/yang-proto-gen/pkgmap.go)" >&2
            exit 1
        }
        src_proto="${ROOT_DIR}/api/proto/openits/${proto_file}"
        if [[ ! -f "${src_proto}" ]]; then
            echo "Error: ${m} declares notifications but ${src_proto} is missing; run 'make yang-proto-gen' first" >&2
            exit 1
        fi
        cp "${src_proto}" "${dest}/schema.proto"

        write_schema_json "${m}" "${dest}"
    fi

    write_manifest "${m}" "${rev}" "${dest}"

    cat > "${dest}/README.md" <<EOF
# ${m} — revision ${rev}

Immutable snapshot of the \`${m}\` YANG module at revision ${rev}.
Referenced from openits CloudEvents \`ce-dataschema\` URLs of the form
\`https://schemas.open-its.org/${m}/${rev}/\`.
EOF

    echo "Wrote ${dest}"
    wrote_any=1
done

# Augments — shared loop body via inline shell. Augments live under
# yang/augments/<contributor>-<service>-<feature>.yang and snapshot to
# schema-registry/<contributor>-<service>-<feature>/<revision>/.
for src_yang in ${AUGMENT_GLOB}; do
    [[ -f "${src_yang}" ]] || continue
    base="$(basename "${src_yang}" .yang)"
    rev="$(read_revision "${src_yang}")"
    if [[ -z "${rev}" ]]; then
        echo "Error: augment ${base} has no revision statement" >&2
        exit 1
    fi
    dest="${REGISTRY}/${base}/${rev}"
    if [[ -d "${dest}" ]]; then
        echo "Skip  ${base}@${rev} (already present; revisions are immutable)"
        continue
    fi
    mkdir -p "${dest}"
    cp "${src_yang}" "${dest}/schema.yang"
    cat > "${dest}/MANIFEST.json" <<EOF
{
  "module": "${base}",
  "revision": "${rev}",
  "kind": "augment",
  "files": ["schema.yang"]
}
EOF
    cat > "${dest}/README.md" <<EOF
# ${base} — revision ${rev}

Immutable snapshot of the augment module \`${base}\` at revision ${rev}.
Augments add nodes to a core OpenITS module without modifying it. See
[../../docs/plans/yang-extension-model.md](../../docs/plans/yang-extension-model.md).
EOF
    echo "Wrote ${dest}"
    wrote_any=1
done

if [[ "${wrote_any}" -eq 0 ]]; then
    echo "Registry up to date — no new revisions to snapshot."
else
    echo "Registry updated at ${REGISTRY}."
fi
