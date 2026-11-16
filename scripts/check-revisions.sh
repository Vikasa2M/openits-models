#!/usr/bin/env bash
#
# Fail if any YANG module's current content, wire shape or JSON Schema
# differs from what is snapshotted under its declared revision. Catches
# the "content changed but revision did not" class of bug, including the
# case where the YANG is untouched but the wire moved anyway: a change to
# the YANG-to-proto mapping, or to a shared type the module imports.
#
# For each module under yang/*.yang:
#   - read the module's most-recent revision statement
#   - if schema-registry/<module>/<revision>/schema.yang exists, compare
#     the snapshot against the live module:
#       schema.yang  against yang/<module>.yang
#       schema.shape against api/shapes/<module>.shape (make gen); a
#                    module with root messages whose snapshot predates
#                    schema.shape fails until it is backfilled
#       schema.json  against the JSON Schema rebuilt from api/proto
#     - all identical → OK
#     - any different → FAIL (bump the revision)
#   - if no snapshot exists for this revision → OK (first time for
#     this revision; next `./scripts/update-schema-registry.sh` will snapshot)
#   - FAIL if the module carries two (or more) revision statements
#     for the same date — the revision log is the compatibility
#     contract and must stay lintable and legible
#   - FAIL if a first-party module (yang/openits-*.yang; vendor
#     modules under yang/openits-vendor-* keep their own) doesn't
#     carry the "OpenITS Working Group (proposal)" organization and
#     contact strings
#
# Exit 0 if every module is consistent; 1 if any drift is detected.
set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
REGISTRY="${1:-${ROOT_DIR}/schema-registry}"

read_revision() {
    awk '
      /^[[:space:]]*revision[[:space:]]+[0-9]{4}-[0-9]{2}-[0-9]{2}/ {
        for (i = 1; i <= NF; i++) {
          if ($i ~ /^[0-9]{4}-[0-9]{2}-[0-9]{2}$/) {
            print $i
            exit
          }
        }
      }
    ' "$1"
}

# read_meta <file> <keyword>: prints the first quoted string of the named
# top-level statement (e.g. "organization" or "contact"), whether it's
# written single-line (`organization "X";`) or multiline (`organization`
# on one line, `"X";` on the next).
read_meta() {
    awk -v kw="$2" '
      found && match($0, /"[^"]*"/) { print substr($0, RSTART+1, RLENGTH-2); exit }
      $1 == kw { found=1; if (match($0, /"[^"]*"/)) { print substr($0, RSTART+1, RLENGTH-2); exit } }
    ' "$1"
}

# shellcheck source=registry-lib.sh
source "${SCRIPT_DIR}/registry-lib.sh"

JSON_TMP="$(mktemp -d)"
trap 'rm -rf "$JSON_TMP"' EXIT

FAIL=0
DRIFT=()

for src in "${ROOT_DIR}"/yang/*.yang "${ROOT_DIR}"/yang/augments/*.yang "${ROOT_DIR}"/yang/deviations/*.yang; do
    [ -e "$src" ] || continue
    m="$(basename "$src" .yang)"
    rev="$(read_revision "$src")"
    if [ -z "$rev" ]; then
        echo "FAIL  $m has no revision statement" >&2
        FAIL=1
        continue
    fi

    dup="$(grep -E '^[[:space:]]*revision[[:space:]]+[0-9]{4}-[0-9]{2}-[0-9]{2}' "$src" \
            | awk '{print $2}' | sort | uniq -d)"
    if [ -n "$dup" ]; then
        echo "FAIL  $m has duplicate revision statement(s) for: $(echo $dup | tr '\n' ' ')" >&2
        FAIL=1
    fi

    # Every core openits module carries an openits-version stamp (vendor
    # modules under yang/openits-vendor-* keep their own versioning).
    case "$src" in
        */yang/openits-vendor-*) ;;
        */yang/openits-*.yang)
            if ! grep -q 'openits-version "' "$src"; then
                echo "FAIL  $m has no openits-version stamp" >&2
                FAIL=1
            fi
            org="$(read_meta "$src" organization)"
            con="$(read_meta "$src" contact)"
            if [ "$org" != "OpenITS Working Group (proposal)" ]; then
                echo "FAIL  $m organization is '$org' (expected 'OpenITS Working Group (proposal)')" >&2
                FAIL=1
            fi
            if [ "$con" != "OpenITS Working Group (proposal)" ]; then
                echo "FAIL  $m contact is '$con' (expected 'OpenITS Working Group (proposal)')" >&2
                FAIL=1
            fi
            ;;
    esac

    snap="${REGISTRY}/${m}/${rev}/schema.yang"
    if [ ! -f "$snap" ]; then
        echo "NEW   $m@$rev (no snapshot yet — run ./scripts/update-schema-registry.sh)"
        continue
    fi

    dir="${REGISTRY}/${m}/${rev}"
    problems=()
    if ! diff -q "$src" "$snap" >/dev/null 2>&1; then
        problems+=("YANG differs from ${dir}/schema.yang")
    fi

    # The wire: what producers compiled from this revision must still be
    # what the snapshot pins, whether the YANG changed or not. A generator
    # change, or a change to a shared type the module imports, moves the
    # shape with the YANG untouched.
    live_shape="${ROOT_DIR}/api/shapes/${m}.shape"
    if [ -f "$live_shape" ]; then
        if [ ! -f "${dir}/schema.shape" ]; then
            problems+=("snapshot predates schema.shape; run ./scripts/backfill-shapes.sh")
        elif ! cmp -s "$live_shape" "${dir}/schema.shape"; then
            problems+=("wire shape differs: diff ${dir}/schema.shape api/shapes/${m}.shape")
        fi
    elif [ -f "${dir}/schema.shape" ]; then
        problems+=("snapshot pins a wire shape but the module no longer has root messages")
    fi

    # The JSON Schema companion, rebuilt exactly as the snapshot script
    # builds it. Shared definitions are inlined per notification, so this
    # also catches a range or pattern change in an imported typedef.
    if [ -f "${dir}/schema.json" ]; then
        rm -f "${JSON_TMP}/schema.json"
        write_schema_json "$m" "$JSON_TMP"
        if [ ! -f "${JSON_TMP}/schema.json" ]; then
            problems+=("snapshot pins schema.json but the module has no live notification")
        elif ! cmp -s "${JSON_TMP}/schema.json" "${dir}/schema.json"; then
            problems+=("JSON Schema differs from ${dir}/schema.json")
        fi
    fi

    if [ "${#problems[@]}" -eq 0 ]; then
        echo "OK    $m@$rev"
    else
        for p in "${problems[@]}"; do
            echo "DRIFT $m@$rev — $p" >&2
        done
        DRIFT+=("$m@$rev")
        FAIL=1
    fi
done

if [ "$FAIL" -ne 0 ]; then
    echo >&2
    echo "check-revisions: drift detected in ${#DRIFT[@]} module(s):" >&2
    if [ "${#DRIFT[@]}" -gt 0 ]; then
        for d in "${DRIFT[@]}"; do
            echo "  - $d" >&2
        done
    fi
    echo >&2
    echo "Fix by either:" >&2
    echo "  (a) bumping the module's revision (and openits-version) to record" >&2
    echo "      the change, then running ./scripts/update-schema-registry.sh;" >&2
    echo "      a wire-only change still needs a revision, whose description" >&2
    echo "      says what moved and why, or" >&2
    echo "  (b) if the snapshot exists only on this branch (not yet on main)," >&2
    echo "      deleting it and re-running update-schema-registry. A snapshot" >&2
    echo "      on main is immutable; check-revision-history rejects edits." >&2
    exit 1
fi

echo
echo "check-revisions: all modules consistent with their snapshots."
