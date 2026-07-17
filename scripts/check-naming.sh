#!/usr/bin/env bash
set -euo pipefail

# CI guard — fails if forbidden legacy naming reappears in non-historical files.
#
# What's forbidden, and where:
#   * tc.{telemetry,command,event,health}.* ce-type forms — replaced by openits.<service>.<event>.v<n>
#   * urn:tc:* dataschema forms — replaced by ce-dataschema URLs
#   * 5-token subject hierarchy ({telemetry|cmd|health|ack|event}.{region}.{site}.{type}.{id})
#     — replaced by 7-token openits.{region}.{agency}.{agency-unit}.{service}.{controller-id}.{event}
#   * the bare word "portunus" / "Portunus" outside historical / generated /
#     this-script files (reserved for retrospectives in docs/phase-*-retrospective.md
#     and absolute-path comments in generated YANG output).
#
# Historical files exempt from the check:
#   * docs/phase-*-retrospective.md       (factual record of what happened)
#   * docs/plans/**                       (plan / governance docs may reference legacy
#                                          for migration context)
#   * .git/**                             (commit history)
#   * pkg/**                              (generated; absolute paths in comments)
#   * api/**.pb.go                        (defensive — if something ends up here)
#   * scripts/check-naming.sh             (this file)

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

EXIT=0

declare -a EXCLUDES=(
  --exclude-dir=.git
  --exclude-dir=.idea
  --exclude-dir=node_modules
  --exclude-dir=bin
  # Generated proto/ygot trees carry absolute-path comments that include
  # the legacy "portunus" project directory. They regenerate on any
  # machine and are not part of the hand-authored wire contract.
  --exclude-dir=pkg
  # Gitignored local planning/tooling scratch (not part of the shipped
  # repo). Superpowers review diffs retain pre-rebrand "portunus" paths.
  --exclude-dir=.superpowers
  --exclude-dir=superpowers
  --exclude='phase-*-retrospective.md'
  --exclude='check-naming.sh'
)
# NB: yang/ (incl. yang/augments and yang/deviations) and schema-registry/
# are intentionally NOT excluded — the anti-legacy-naming guard must scan the
# hand-authored YANG and its registry snapshots, which is where legacy forms
# would actually regress (this is how the pre-P3d SCREAMING_SNAKE identities
# slipped a naming check).

# Files that intentionally reference the legacy "Portunus" name as part of
# documenting the OpenITS rebrand history. The check applies to the rest of the
# repository.
plan_filter() {
  grep -v '^docs/plans/' \
    | grep -v '^docs/phase-' \
    | grep -v '^docs/architecture\.md:1[0-9]:' \
    | grep -v '^README\.md:[0-9]:' \
    | grep -v '^README\.md:1[0-9]:'
}

echo "== check 1: legacy tc.* ce-type forms =="
HITS="$(grep -rEn "${EXCLUDES[@]}" 'tc\.(telemetry|command|event|health)\.' . 2>/dev/null \
  | sed 's|^./||' | plan_filter || true)"
if [ -n "$HITS" ]; then
  echo "$HITS"
  echo "ERROR: legacy tc.* ce-type form found above. ce-types must use the openits.<service>.<event>.v<N> shape."
  EXIT=1
fi

echo "== check 2: urn:tc:* schema URN form =="
HITS="$(grep -rEn "${EXCLUDES[@]}" 'urn:tc:' . 2>/dev/null \
  | sed 's|^./||' | plan_filter || true)"
if [ -n "$HITS" ]; then
  echo "$HITS"
  echo "ERROR: legacy urn:tc:* form found above. Schema references must use the openits ce-dataschema URL form."
  EXIT=1
fi

echo "== check 3: 5-token subject hierarchy =="
# Match {telemetry|cmd|health|ack|event}.{a}.{b}.{c}.{d} where each {x} is a placeholder
# token wrapped in braces. The 5-token form was telemetry.{region}.{site}.{type}.{id}.
HITS="$(grep -rEn "${EXCLUDES[@]}" '\b(telemetry|cmd|health|ack|event)\.\{[a-z_-]+\}\.\{[a-z_-]+\}\.\{[a-z_-]+\}\.\{[a-z_-]+\}' . 2>/dev/null \
  | sed 's|^./||' | plan_filter || true)"
if [ -n "$HITS" ]; then
  echo "$HITS"
  echo "ERROR: legacy 5-token subject form found above. Subjects must use the 7-token openits.{region}.{agency}.{agency-unit}.{service}.{controller-id}.{event} shape."
  EXIT=1
fi

echo "== check 4: bare 'portunus' / 'Portunus' outside historical files =="
# Allow the bare name in plan docs (migration context), retrospectives (history),
# and generated code (pkg/). Anywhere else is regression.
HITS="$(grep -rIn "${EXCLUDES[@]}" -E '(^|[^a-zA-Z0-9_])(p|P)ortunus([^a-zA-Z0-9_]|$)' . 2>/dev/null \
  | sed 's|^./||' | plan_filter || true)"
if [ -n "$HITS" ]; then
  echo "$HITS"
  echo "ERROR: 'portunus' / 'Portunus' found in non-historical files. Use 'openits' / 'OpenITS'."
  EXIT=1
fi

echo "== check 5: every notification uses event-header + a mandatory kind identityref =="
# Structural check: every 'notification { ... }' in a hand-authored top-level
# module must 'uses <prefix>:event-header;' (whatever local prefix that
# module happens to import openits-types under) and declare a mandatory
# 'leaf kind' of 'type identityref'. This is done via a small pyang output
# plugin (real YANG grammar) rather than text/brace matching: notification
# blocks can contain single-line sub-blocks, and revision/description prose
# elsewhere in these files routinely contains the bare word "notification"
# at the start of a line (e.g. "notification statements)." in a wrapped
# description) — line-anchored regex or hand-rolled brace counting produces
# false positives/negatives on this codebase. pyang's parser respects
# quoting/comments and gives an unambiguous statement tree.
#
# Scope: only yang/*.yang (hand-authored top-level modules) is fed to pyang
# below. yang/augments/**, yang/deviations/**, and yang/ietf/** are
# deliberately NOT globbed in — augments/deviations extend existing nodes
# rather than declaring their own notifications, and ietf/ holds third-party
# imports, not openits modules. Enforce that assumption structurally (not
# just by convention) so a future notification added under one of those
# trees can't silently go unchecked by pyang's glob:
NOTIF_ELSEWHERE="$(grep -rlE '^[[:space:]]*notification[[:space:]]+[A-Za-z0-9_-]+[[:space:]]*[{;]' \
  yang/augments yang/deviations yang/ietf 2>/dev/null | sort || true)"
if [ -n "$NOTIF_ELSEWHERE" ]; then
  echo "$NOTIF_ELSEWHERE"
  echo "ERROR: notification declared outside yang/*.yang — augments/deviations/ietf aren't scanned by check 5's pyang glob. Move it under yang/*.yang or widen the glob below."
  EXIT=1
fi

if command -v pyang >/dev/null 2>&1; then
  PLUGIN_DIR="$(mktemp -d)"
  NOTIF_ERR_FILE="$(mktemp)"
  trap 'rm -rf "$PLUGIN_DIR"; rm -f "$NOTIF_ERR_FILE"' EXIT
  cat > "$PLUGIN_DIR/openits_notif_check.py" <<'PYEOF'
"""pyang plugin: verify every notification uses openits-types:event-header
and declares a mandatory 'kind' identityref leaf."""
from pyang import plugin

def pyang_plugin_init():
    plugin.register_plugin(OpenitsNotifCheckPlugin())

class OpenitsNotifCheckPlugin(plugin.PyangPlugin):
    def add_output_format(self, fmts):
        self.multiple_modules = True
        fmts['openits-notif-check'] = self

    def setup_fmt(self, ctx):
        ctx.implicit_errors = False

    def emit(self, ctx, modules, fd):
        checked = 0
        bad = 0
        for module in modules:
            # Resolve the *actual* local prefix this module imports
            # openits-types under, rather than assuming every module
            # spells it "openits-types:" — a module is free to import it
            # under any prefix (e.g. 'import openits-types { prefix ot;
            # }'), and a hardcoded string would false-negative on such a
            # module's otherwise-correct 'uses ot:event-header;'.
            hdr_prefix = _imported_prefix(module, 'openits-types')
            expected_uses = ('%s:event-header' % hdr_prefix
                              if hdr_prefix else None)
            # openits-types itself may reference the grouping unprefixed
            # (a local 'uses event-header;').
            local_uses = ('event-header'
                          if module.arg == 'openits-types' else None)
            for notif in _find(module, 'notification'):
                checked += 1
                name = notif.arg
                has_hdr = any(
                    s.keyword == 'uses'
                    and s.arg in (expected_uses, local_uses)
                    for s in notif.substmts)
                kind_leaf = next(
                    (s for s in notif.substmts
                     if s.keyword == 'leaf' and s.arg == 'kind'), None)
                has_type = kind_leaf is not None and any(
                    s.keyword == 'type' and s.arg == 'identityref'
                    for s in kind_leaf.substmts)
                has_mandatory = kind_leaf is not None and any(
                    s.keyword == 'mandatory' and s.arg == 'true'
                    for s in kind_leaf.substmts)
                errs = []
                if not has_hdr:
                    want = expected_uses or local_uses
                    if want:
                        errs.append("missing 'uses %s;'" % want)
                    else:
                        errs.append(
                            "module does not import openits-types; cannot "
                            "verify 'uses <prefix>:event-header;'")
                if kind_leaf is None:
                    errs.append("missing 'leaf kind { ... }'")
                else:
                    if not has_type:
                        errs.append("'leaf kind' is not 'type identityref'")
                    if not has_mandatory:
                        errs.append("'leaf kind' is not 'mandatory true'")
                if errs:
                    bad += 1
                    for e in errs:
                        fd.write("%s: notification %s: %s\n" %
                                 (module.pos.ref, name, e))
        fd.write("NOTIF_CHECKED=%d NOTIF_BAD=%d\n" % (checked, bad))

def _imported_prefix(module, modname):
    """Return the local prefix `module` binds `modname` to via its own
    'import' statement, or None if it doesn't import it. This is a
    structural (syntax-tree) lookup, not a guess at naming convention, so
    it stays correct regardless of which prefix an importing module picks."""
    for imp in module.substmts:
        if imp.keyword == 'import' and imp.arg == modname:
            for s in imp.substmts:
                if s.keyword == 'prefix':
                    return s.arg
    return None

def _find(stmt, keyword):
    out = []
    if stmt.keyword == keyword:
        out.append(stmt)
    for s in stmt.substmts:
        out.extend(_find(s, keyword))
    return out
PYEOF
  NOTIF_STATUS=0
  NOTIF_RAW="$(pyang --plugindir "$PLUGIN_DIR" -p yang -p yang/ietf -f openits-notif-check yang/*.yang 2>"$NOTIF_ERR_FILE")" || NOTIF_STATUS=$?
  NOTIF_STDERR="$(cat "$NOTIF_ERR_FILE")"
  # NOTIF_RAW is only the plugin's own output (violations + the
  # NOTIF_CHECKED=/NOTIF_BAD= summary line, written via fd.write). pyang's
  # own stderr diagnostics (e.g. an unrelated deprecation warning on some
  # other module) are captured separately in NOTIF_STDERR so they can never
  # be misparsed as a notification violation — they only fail this check
  # via a nonzero pyang exit status (fail-closed on real errors), not
  # merely by having written something to stderr.
  NOTIF_SUMMARY="$(printf '%s\n' "$NOTIF_RAW" | grep -E '^NOTIF_CHECKED=[0-9]+ NOTIF_BAD=[0-9]+$' || true)"
  NOTIF_VIOLATIONS="$(printf '%s\n' "$NOTIF_RAW" | grep -v -E '^NOTIF_CHECKED=[0-9]+ NOTIF_BAD=[0-9]+$' || true)"
  if [ "$NOTIF_STATUS" -ne 0 ] || [ -z "$NOTIF_SUMMARY" ]; then
    [ -n "$NOTIF_RAW" ] && echo "$NOTIF_RAW"
    [ -n "$NOTIF_STDERR" ] && echo "$NOTIF_STDERR" >&2
    echo "ERROR: notification event-header/kind structural check did not complete (pyang exited $NOTIF_STATUS)."
    EXIT=1
  elif [ -n "$NOTIF_VIOLATIONS" ]; then
    echo "$NOTIF_VIOLATIONS"
    echo "ERROR: every notification MUST 'uses openits-types:event-header;' and declare a mandatory 'leaf kind { type identityref ... }' — see notification(s) above."
    EXIT=1
  else
    echo "$NOTIF_SUMMARY"
    if [ -n "$NOTIF_STDERR" ]; then
      echo "note: pyang wrote the following to stderr (not treated as a notification violation):"
      echo "$NOTIF_STDERR"
    fi
  fi
  rm -rf "$PLUGIN_DIR"
  rm -f "$NOTIF_ERR_FILE"
  trap - EXIT
else
  echo "pyang not installed; skipping notification event-header/kind structural check"
fi

if [ $EXIT -eq 0 ]; then
  echo
  echo "naming check: OK"
fi

exit $EXIT
