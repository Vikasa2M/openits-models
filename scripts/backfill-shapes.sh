#!/usr/bin/env bash
#
# Backfill schema.shape into the snapshot of each module's CURRENT revision,
# but only where that is a true statement about the revision.
#
# schema.shape is the one artifact check-revision-history lets a released
# snapshot gain (see tools/check-revision-history/registry.go). Writing
# today's shape into a snapshot taken weeks ago would bless any drift that
# happened in between, so for every module whose current-revision snapshot
# lacks the file this script renders the module's shape, with today's
# renderer, at:
#
#   - the commit that created the snapshot,
#   - every release tag that contains that commit, and
#   - the working tree (api/shapes/, from `make gen`).
#
# If all agree, the revision has had one wire shape since it was taken and
# the file is written. If any disagree, the revision shipped (or is about to
# ship) more than one wire shape: the module needs a new revision, and the
# script names the points that differ instead of writing anything for it.
#
# Older, non-current revisions are left alone: they have no live tree to
# compare against, and some already shipped in more than one shape.
#
# Usage: scripts/backfill-shapes.sh [--dry-run]
# Run `make gen` first so api/shapes/ is current.
set -euo pipefail

DRY_RUN=0
[[ "${1:-}" == "--dry-run" ]] && DRY_RUN=1

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
REGISTRY="${ROOT_DIR}/schema-registry"
cd "$ROOT_DIR"

TMP="$(mktemp -d)"
WORKTREES=()
cleanup() {
    local wt
    for wt in "${WORKTREES[@]}"; do
        git worktree remove --force "$wt" >/dev/null 2>&1 || true
    done
    rm -rf "$TMP"
}
trap cleanup EXIT

TOOL="$TMP/yang-proto-gen"
go build -o "$TOOL" ./tools/yang-proto-gen

read_revision() {
    awk '
      /^[[:space:]]*revision[[:space:]]+[0-9]{4}-[0-9]{2}-[0-9]{2}/ {
        for (i = 1; i <= NF; i++) {
          if ($i ~ /^[0-9]{4}-[0-9]{2}-[0-9]{2}$/) { print $i; exit }
        }
      }
    ' "$1"
}

# shapes_at <commit>: render every module's shape as of <commit> into
# $TMP/shapes/<commit>/ (cached). Fails if that tree cannot be rendered.
shapes_at() {
    local c="$1" out="$TMP/shapes/$1"
    if [[ -d "$out" ]]; then
        return 0
    fi
    if [[ -f "$TMP/failed-$c" ]]; then
        return 1
    fi
    local wt="$TMP/wt-$c"
    git worktree add --detach --quiet "$wt" "$c"
    WORKTREES+=("$wt")
    local protos=()
    while IFS= read -r -d '' f; do
        protos+=("${f#"$wt/api/proto/"}")
    done < <(find "$wt/api/proto/openits" -name '*.proto' -print0 | sort -z)
    mkdir -p "$out"
    if ! protoc --proto_path="$wt/api/proto" --include_imports \
            --descriptor_set_out="$TMP/$c.binpb" "${protos[@]}" 2>"$TMP/$c.err" ||
       ! "$TOOL" -shapes -yang "$wt/yang" -descriptors "$TMP/$c.binpb" -out "$out" 2>>"$TMP/$c.err"; then
        rm -rf "$out"
        touch "$TMP/failed-$c"
        return 1
    fi
}

backfilled=0
drifted=()
for live in api/shapes/*.shape; do
    m="$(basename "$live" .shape)"
    src=""
    for d in yang yang/augments yang/deviations; do
        [[ -f "$d/$m.yang" ]] && src="$d/$m.yang"
    done
    [[ -n "$src" ]] || continue
    rev="$(read_revision "$src")"
    dir="$REGISTRY/$m/$rev"
    [[ -f "$dir/schema.yang" ]] || continue   # not snapshotted yet
    [[ -f "$dir/schema.shape" ]] && continue  # already pinned

    created="$(git log --diff-filter=A --format=%H -1 -- "schema-registry/$m/$rev/schema.yang")"
    if [[ -z "$created" ]]; then
        echo "SKIP   $m@$rev: snapshot is not committed yet; it will be written complete by update-schema-registry" >&2
        continue
    fi
    points=("$created")
    while IFS= read -r t; do
        points+=("$t")
    done < <(git tag --contains "$created" --list 'v*' --sort=v:refname)

    differs=()
    for p in "${points[@]}"; do
        c="$(git rev-parse "$p^{commit}")"
        if ! shapes_at "$c"; then
            differs+=("$p (could not render: $(head -1 "$TMP/$c.err"))")
            continue
        fi
        if ! cmp -s "$live" "$TMP/shapes/$c/$m.shape"; then
            differs+=("$p")
        fi
    done

    if [[ ${#differs[@]} -gt 0 ]]; then
        echo "DRIFT  $m@$rev: today's shape differs from: ${differs[*]}" >&2
        drifted+=("$m@$rev")
        continue
    fi
    label="${points[*]}"
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "WOULD  $m@$rev (one shape at ${label// /, }, and today)"
    else
        cp "$live" "$dir/schema.shape"
        echo "PINNED $m@$rev (one shape at ${label// /, }, and today)"
    fi
    backfilled=$((backfilled + 1))
done

echo
echo "backfill-shapes: $backfilled snapshot(s) $([[ $DRY_RUN -eq 1 ]] && echo 'would be ')backfilled, ${#drifted[@]} drifted."
if [[ ${#drifted[@]} -gt 0 ]]; then
    echo "Each drifted module's current revision has carried more than one wire shape." >&2
    echo "Give it a new revision (and openits-version) recording the wire change, then" >&2
    echo "run ./scripts/update-schema-registry.sh; its new snapshot is written complete." >&2
    exit 1
fi
