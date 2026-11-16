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
# Every file of a snapshot is immutable once it is on main:
# check-revision-history fails any edit or removal, and any addition other
# than an artifact type introduced after the snapshot was taken (today only
# schema.shape; see scripts/backfill-shapes.sh, which verifies a backfill
# against history before writing it). A snapshot is therefore written
# complete or not at all.
#
# Every snapshot always carries schema.yang. A module that puts messages on
# the wire — a live `notification`, or a config/state tree on
# tools/yang-proto-gen's configStateRoutes allowlist — also carries:
#
#   schema.proto  the messages the module itself declares, as a compilable
#                 file (api/shapes/<module>.proto). Messages sibling modules
#                 declare in the same generated file are left out.
#   schema.shape  the canonical wire shape of everything its root messages
#                 reach, shared types included (api/shapes/<module>.shape).
#                 check-revisions compares the live shape against this file,
#                 so a revision's wire cannot move without a new revision.
#
# Both come from scripts/wire-shapes.sh (part of `make gen`), which renders
# them from the compiled protos. Snapshots taken before schema.shape existed
# carry, as schema.proto, a copy of the whole service events file instead,
# sibling modules' messages included; they are immutable and stay that way.
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
    openits-zone-occupancy-types
    openits-zone-occupancy
    openits-zone-occupancy-events
    openits-work-zone-types
    openits-work-zone-events
    openits-cabinet-power
    openits-schedule
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
    openits-vendor-cameleon-reversible-lane-types
)

# Augment modules contributed by vendors / agencies. Snapshotted alongside
# core modules but live under yang/augments/ rather than yang/. The
# loop below globs yang/augments/*.yang so new augments are picked up
# without touching this script.
AUGMENT_GLOB="${ROOT_DIR}/yang/augments/*.yang"

# Deviation modules are versioned/snapshotted like augments: they are
# published contract surface (check-revisions scans yang/deviations/),
# but are not core modules. The loop below globs yang/deviations/*.yang
# so new deviation modules are picked up automatically.
DEVIATION_GLOB="${ROOT_DIR}/yang/deviations/*.yang"

# shellcheck source=registry-lib.sh
source "${SCRIPT_DIR}/registry-lib.sh"

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

# write_manifest writes dest/MANIFEST.json for a new snapshot of module m
# at revision rev, listing whichever of schema.yang/schema.proto/
# schema.shape/schema.json are present in dest.
write_manifest() {
    local m="$1" rev="$2" dest="$3"
    cat > "${dest}/MANIFEST.json" <<EOF
{
  "module": "${m}",
  "revision": "${rev}",
  "files": [
    "schema.yang"$([[ -f "${dest}/schema.proto" ]] && echo ",
    \"schema.proto\"")$([[ -f "${dest}/schema.shape" ]] && echo ",
    \"schema.shape\"")$([[ -f "${dest}/schema.json" ]] && echo ",
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
        echo "Skip  ${m}@${rev} (already present; revisions are immutable)"
        continue
    fi
    # Resolve every companion BEFORE creating anything on disk. A module
    # whose shape or JSON Schema is missing must fail here, not after
    # schema.yang has been written: a partial directory would be treated as
    # an existing, immutable snapshot by the branch above and never be
    # completed. That is how openits-zone-occupancy-events' 2026-08-15
    # snapshot was left with schema.yang alone for two releases.
    src_shape="${ROOT_DIR}/api/shapes/${m}.shape"
    src_own="${ROOT_DIR}/api/shapes/${m}.proto"
    if declares_notification "${src_yang}" && [[ ! -f "${src_shape}" ]]; then
        echo "Error: ${m} declares notifications but ${src_shape} is missing; run 'make gen' first" >&2
        exit 1
    fi
    if [[ -f "${src_shape}" && ! -f "${src_own}" ]]; then
        echo "Error: ${src_shape} has no companion ${src_own}; run 'make gen'" >&2
        exit 1
    fi
    if declares_notification "${src_yang}"; then
        proto_file_for_module "${m}" >/dev/null || {
            echo "Error: ${m} declares notifications but matches no proto route in PROTO_ROUTE_PREFIXES (see tools/yang-proto-gen/pkgmap.go)" >&2
            exit 1
        }
    fi

    mkdir -p "${dest}"

    cp "${src_yang}" "${dest}/schema.yang"

    if [[ -f "${src_shape}" ]]; then
        cp "${src_own}" "${dest}/schema.proto"
        cp "${src_shape}" "${dest}/schema.shape"
    fi
    if declares_notification "${src_yang}"; then
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

# Deviations — mirrors the augment loop above. Deviation modules live
# under yang/deviations/<subject>.yang and snapshot to
# schema-registry/<subject>/<revision>/. Like augments, deviations never
# declare notifications of their own, so there is no proto/schema.json
# to carry — schema.yang + MANIFEST.json + README.md only.
for src_yang in ${DEVIATION_GLOB}; do
    [[ -f "${src_yang}" ]] || continue
    base="$(basename "${src_yang}" .yang)"
    rev="$(read_revision "${src_yang}")"
    if [[ -z "${rev}" ]]; then
        echo "Error: deviation ${base} has no revision statement" >&2
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
  "kind": "deviation",
  "files": ["schema.yang"]
}
EOF
    cat > "${dest}/README.md" <<EOF
# ${base} — revision ${rev}

Immutable snapshot of the deviation module \`${base}\` at revision ${rev}.
Deviations tighten a core OpenITS module to conform to an external
reference (e.g. MUTCD, FCC band plan) for a given jurisdiction, without
modifying the core module. See
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
