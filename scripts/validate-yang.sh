#!/usr/bin/env bash
# Validates golden YANG instance data against the openits modules
# using yanglint (libyang) in Docker.  Asserts each fixture produces
# its expected outcome: `valid-*` must parse cleanly; `invalid-*` must
# fail with yanglint exit code 7 (data violates schema / must).
set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
# Pinned by digest: `latest` is a mutable tag, and this runs inside CI.
# Digest of sysrepo/sysrepo-netopeer2:latest (yanglint 2.1.30) as of 2026-07-21.
IMAGE="${YANGLINT_IMAGE:-sysrepo/sysrepo-netopeer2@sha256:36cc1d841c97f118d62f775024d60fb1ce210c3bca6b32135c1a9e5a358a1cc9}"

cd "$ROOT_DIR"

SCHEMAS=(
    yang/openits-types.yang
    yang/openits-device-diagnostics.yang
    yang/openits-cabinet-power.yang
    yang/openits-v2x-radio.yang
    yang/openits-v2x-messaging.yang
    yang/openits-v2x-radio-types.yang
    yang/openits-v2x-messaging-types.yang
    yang/openits-scms.yang
    yang/openits-vehicle-detection.yang
    yang/openits-common-fault-events.yang
    yang/openits-common-comm-health-events.yang
    yang/openits-common-mode-events.yang
    yang/openits-nema-common.yang
    yang/openits-signal-control-types.yang
    yang/openits-signal-control.yang
    yang/openits-signal-control-events.yang
    yang/openits-dms-types.yang
    yang/openits-dms.yang
    yang/openits-dms-events.yang
    yang/openits-ess-types.yang
    yang/openits-ess.yang
    yang/openits-ess-events.yang
    yang/openits-rsu-types.yang
    yang/openits-rsu.yang
    yang/openits-rsu-events.yang
    yang/openits-ramp-metering-types.yang
    yang/openits-ramp-metering.yang
    yang/openits-ramp-metering-events.yang
    yang/openits-traffic-sensor-types.yang
    yang/openits-traffic-sensor.yang
    yang/openits-traffic-sensor-events.yang
    yang/openits-reversible-lane-types.yang
    yang/openits-reversible-lane.yang
    yang/openits-reversible-lane-events.yang
    yang/openits-perception-types.yang
    yang/openits-perception.yang
    yang/openits-perception-events.yang
    yang/openits-cctv-types.yang
    yang/openits-cctv.yang
    yang/openits-cctv-events.yang
    yang/openits-vendor-trafficvision-traffic-sensor-types.yang
    yang/openits-vendor-trafficvision-perception-types.yang
    yang/augments/trafficvision-traffic-sensor-camera.yang
    yang/augments/trafficvision-perception-incident-media.yang
)

# Guard: every module that declares a notification must be in
# SCHEMAS, else its notifications go unvalidated. Fail loudly if not.
missing=()
for m in yang/*.yang; do
    grep -qE '^[[:space:]]*notification[[:space:]]' "$m" || continue
    case " ${SCHEMAS[*]} " in
        *" $m "*) ;;                    # present — ok
        *) missing+=("$m") ;;
    esac
done
if [ ${#missing[@]} -gt 0 ]; then
    echo "validate-yang: notification-bearing module(s) missing from SCHEMAS:" >&2
    printf '  %s\n' "${missing[@]}" >&2
    echo "Add them to the SCHEMAS array so their notifications are validated." >&2
    exit 1
fi

# Guard: every notification declared in a loaded module must have
# at least one valid-<name>.json fixture, so a newly-added notification
# cannot ship fixture-less and slip past this suite. Fixtures are keyed by
# bare notification name, so also assert name uniqueness across modules —
# otherwise two notifications would map to one fixture and only one would
# actually be exercised.
notif_names=$(grep -hoE '^[[:space:]]*notification[[:space:]]+[a-z][a-z0-9-]*[[:space:]]*[{;]' "${SCHEMAS[@]}" \
    | sed -E 's/^[[:space:]]*notification[[:space:]]+([a-z0-9-]+).*/\1/' | sort)
dups=$(printf '%s\n' "$notif_names" | uniq -d)
if [ -n "$dups" ]; then
    echo "validate-yang: notification name(s) declared in more than one module:" >&2
    printf '  %s\n' $dups >&2
    echo "Fixtures are keyed by bare notification name; rename to keep names unique" >&2
    echo "or switch to module-qualified fixture names." >&2
    exit 1
fi
nofix=()
for n in $(printf '%s\n' "$notif_names" | uniq); do
    [ -f "yang/testdata/valid-$n.json" ] || nofix+=("$n")
done
if [ ${#nofix[@]} -gt 0 ]; then
    echo "validate-yang: notification(s) with no valid-<name>.json fixture:" >&2
    for n in "${nofix[@]}"; do echo "  $n  (expected yang/testdata/valid-$n.json)" >&2; done
    echo "Add a valid fixture per notification so every notification is exercised." >&2
    exit 1
fi

if ! command -v docker &>/dev/null; then
    echo "docker not found; install Docker or yanglint natively" >&2
    exit 2
fi

if ! command -v python3 &>/dev/null; then
    echo "python3 not found; needed by check-notif-mandatory.py" >&2
    exit 2
fi

# Fixtures come in two shapes: config/state datastore data (the default
# yanglint "data" tree type), and bare notification instances (which
# yanglint only accepts under "-t notif"). A fixture "passes" yanglint if
# it validates clean in EITHER mode; it "fails" if it fails in both. (Trying
# notif mode unconditionally is equivalent to the old "retry only on the
# 'Unexpected notification element' error" — a data-tree fixture that fails
# for a real reason also fails as a notif, and vice versa.)
#
# Batching: this runs in ONE container that loops over every fixture, rather
# than a `docker run` per fixture. Container start/stop on the emulated image
# dominated the runtime (~160 fixtures x a container each); yl_ok then reads
# the batched result instead of shelling out to docker per call.
yl_ok() { grep -qxF "YLPASS $1" "$YL_OUT"; }

# yanglint 2.1.30 (pinned image) does not enforce `mandatory true` (or
# `must`) when validating a notification instance under "-t notif" --
# confirmed empirically, including against the alternative "-t nc-notif"
# mode (which does enforce mandatory/must but also wrongly rejects
# several already-valid fixtures here due to an unrelated
# container/mandatory interaction with the wire-source grouping; see
# scripts/check-notif-mandatory.py's docstring for the full writeup).
# check-notif-mandatory.py parses the YANG source directly instead, and
# reports per-fixture whether a notification instance carries every
# mandatory leaf its schema declares (silently OK for non-notification
# fixtures, which this check doesn't apply to). Run it once up front
# for every fixture rather than once per fixture, since it re-parses
# the whole schema set each invocation.
MANDATORY_OUT="$(mktemp)"
YL_OUT="$(mktemp)"
trap 'rm -f "$MANDATORY_OUT" "$YL_OUT"' EXIT
python3 "$SCRIPT_DIR/check-notif-mandatory.py" --schemas "${SCHEMAS[@]}" -- \
    yang/testdata/valid-*.json yang/testdata/invalid-*.json > "$MANDATORY_OUT"

# Single-container yanglint pass: validate every fixture inside ONE container.
# Emits "YLPASS <file>" / "YLFAIL <file>" per fixture, read by yl_ok. SCHEMAS
# is passed via the environment to avoid quoting the module list into sh.
docker run --rm -e "SCHEMAS=${SCHEMAS[*]}" -v "$PWD:/w" -w /w "$IMAGE" sh -c '
    for f in yang/testdata/valid-*.json yang/testdata/invalid-*.json; do
        [ -e "$f" ] || continue
        if yanglint -f json -p yang $SCHEMAS "$f" >/dev/null 2>&1 \
           || yanglint -t notif -f json -p yang $SCHEMAS "$f" >/dev/null 2>&1; then
            echo "YLPASS $f"
        else
            echo "YLFAIL $f"
        fi
    done
' > "$YL_OUT" 2>/dev/null

mandatory_ok() {
    grep -qxF "OK $1" "$MANDATORY_OUT"
}

mandatory_missing() {
    grep -F "MISSING $1 " "$MANDATORY_OUT" | cut -d' ' -f3-
}

# Combines the yanglint type/pattern/range check with the mandatory-leaf
# check above: a fixture only "validates clean" if both agree.
check_fixture() {
    local file="$1"
    yl_ok "$file" || return 1
    mandatory_ok "$file" || return 1
    return 0
}

FAIL=0
for f in yang/testdata/valid-*.json; do
    [ -e "$f" ] || continue
    if check_fixture "$f"; then
        echo "  PASS  $(basename "$f") (validates clean)"
    else
        if ! yl_ok "$f"; then
            echo "  FAIL  $(basename "$f") should have validated clean" >&2
        else
            echo "  FAIL  $(basename "$f") should have validated clean (missing mandatory leaf: $(mandatory_missing "$f"))" >&2
        fi
        FAIL=1
    fi
done

for f in yang/testdata/invalid-*.json; do
    [ -e "$f" ] || continue
    case "$(basename "$f")" in
    invalid-*-under-*.json)
        # Deviation-tier fixture: only invalid once its named deviation
        # module (yang/deviations/*.yang) is ALSO loaded, which this
        # script's fixed base-only SCHEMAS set never does. `make
        # check-deviations` validates this family with its own
        # base+deviation yanglint pass.
        echo "  SKIP  $(basename "$f") (deviation-tier fixture; see make check-deviations)"
        continue
        ;;
    esac
    if ! check_fixture "$f"; then
        if ! yl_ok "$f"; then
            echo "  PASS  $(basename "$f") (rejected as expected)"
        else
            echo "  PASS  $(basename "$f") (rejected as expected: missing mandatory leaf: $(mandatory_missing "$f"))"
        fi
    else
        echo "  FAIL  $(basename "$f") should have been rejected" >&2
        FAIL=1
    fi
done

if [ "$FAIL" -ne 0 ]; then
    echo "validate-yang: failures above" >&2
    exit 1
fi
echo "validate-yang: all fixtures produced expected outcome"
