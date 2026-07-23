#!/usr/bin/env bash
#
# Fail if any YANG module's current content differs from the content
# snapshotted under its declared revision.  Catches the "content
# changed but revision did not" class of bug that the M7 and M8
# retrospectives flagged as a recurring discipline gap.
#
# For each module under yang/*.yang:
#   - read the module's most-recent revision statement
#   - if schema-registry/<module>/<revision>/schema.yang exists, diff
#     the snapshot against the live file
#     - identical  → OK
#     - different  → FAIL (bump the revision, or if the change really
#                    is a same-revision edit, re-snapshot deliberately)
#   - if no snapshot exists for this revision → OK (first time for
#     this revision; next `make update-schema-registry` will snapshot)
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
        echo "NEW   $m@$rev (no snapshot yet — run make update-schema-registry)"
        continue
    fi

    if diff -q "$src" "$snap" >/dev/null 2>&1; then
        echo "OK    $m@$rev"
    else
        echo "DRIFT $m@$rev — content differs from snapshot" >&2
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
    echo "  (a) bumping the module's revision to record the change, then" >&2
    echo "      running ./scripts/update-schema-registry.sh, or" >&2
    echo "  (b) if the change is genuinely a same-revision edit (e.g. a" >&2
    echo "      same-day pre-publication refactor), deliberately delete" >&2
    echo "      the stale snapshot and re-run update-schema-registry." >&2
    exit 1
fi

echo
echo "check-revisions: all modules consistent with their snapshots."
