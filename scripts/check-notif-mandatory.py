#!/usr/bin/env python3
"""Check that notification fixtures carry every `mandatory true` leaf
their schema declares.

Why this exists: yanglint 2.1.30 (the pinned sysrepo/sysrepo-netopeer2
image) validates a notification instance with `-t notif` but does not
enforce `mandatory true` (or `must`) on it -- confirmed empirically,
including against `-t nc-notif` (the NETCONF-envelope-wrapped mode,
which DOES enforce mandatory/must but requires XML and, more
importantly, also enforces mandatory on nodes nested inside
non-presence containers pulled in by unrelated `uses` groupings --
e.g. `wire-source`'s `container source` is documented and intended as
optional-as-a-whole on notifications, but a bare non-presence
container with a mandatory descendant is itself an implicit mandatory
node per RFC 7950 7.5.7, so `-t nc-notif` would wrongly reject every
existing fixture that (correctly) omits it). Fixing that would mean
adding `presence` to `container source` in openits-types.yang, which
is a schema change outside this script's remit.

So instead: parse the YANG source directly and collect, for each
notification, the set of leaf names that are `mandatory true` AND
reachable from the notification's own top level only by crossing
`uses` (grouping inclusion, which is a transparent splice at the same
depth) -- never by descending into a `container`, `list`, `choice`, or
`case` (each of those introduces its own optionality boundary, so a
mandatory leaf nested inside one does not make the notification's
immediate instance invalid; only that container's own presence
implies it). This intentionally mirrors YANG's actual semantics for
notification instances, without inheriting the container/mandatory
interaction quirk `-t nc-notif` applies to.

A `min-elements` floor on a top-level list/leaf-list is also covered,
added when openits-work-zone-events became the first notification in
this schema family to declare one (every existing min-elements before
it lived on config/state trees already validated via -t data, so this
needed extending exactly as this docstring anticipated). It's
recognized two ways: declared directly on a list/leaf-list, or (the
case this schema family actually uses) added via
`refine "<name>" { min-elements N; ... }` on a `uses` statement, the
mechanism a notification uses to put a cardinality floor on a list it
splices in from a grouping it doesn't own (e.g.
openits-types:geo-path's `point`). This is not a `must` at all --
tools/yang-proto-gen (goyang) rejects `must` as a direct child of
`notification` even though YANG 1.1 permits it, so a notification-level
list-cardinality floor has to be expressed as min-elements instead of
`must "count(x) >= N"` in the first place; see
openits-work-zone-events.yang's `point` refine for the full rationale.

(An earlier revision of this script also evaluated one `must` shape --
a same-notification leaf-to-sibling date-and-time ordering comparison
-- for openits-work-zone-events.yang's window-end leaf. That `must` was
removed from the module (it rejected some conforming data outright: two
date-and-time values at different fractional-second precision compare
as wildly different magnitudes once digit-stripped and treated as
plain integers, so a later, valid window-end could still fail), and the
evaluator was removed along with it -- no notification in the corpus
declares a `must` any more, so keeping a must-evaluator here would be
dead code. If a future notification needs one, the extension point is
the same shape `_top_level_min_elements` uses: collect the constraint
via a walk mirroring `mandatory_leaves`'s uses-crossing, evaluate it in
Python against the fixture's parsed instance, don't try to reproduce
XPath's number-coercion semantics for anything but genuinely numeric
comparisons.)

A `refine "<leaf>" { mandatory true; }` on a `uses` is honored too,
added when openits-zone-occupancy-events refined `presence` mandatory on
zone-occupancy-changed while leaving it optional in the shared grouping
the live state also composes. The refine's target is read at the
notification's own depth, exactly like the min-elements refine below
(a path with '/' in it points below an optionality boundary and is
ignored), and `mandatory false` on a refine removes an inherited
mandatory.

Extend the min-elements handling above if a future notification needs
a floor this doesn't already cover.

Usage:
    check-notif-mandatory.py --schemas S1.yang S2.yang ... \
        -- F1.json F2.json ...

Prints one result line per fixture file (same order given):
    OK <path>              notification instance carries all its
                            schema's mandatory leaves (or the fixture's
                            top-level element isn't a known notification --
                            e.g. a config/state datastore fixture, which
                            this script doesn't apply to)
    MISSING <path> <leaf1>,<leaf2>,...
                            notification instance is missing one or more
                            mandatory leaves
Always exits 0; the caller inspects the per-line verdicts.
"""
import json
import re
import sys


def strip_and_split(text):
    """Split a YANG block body into top-level (keyword, argument, subbody)
    statements, skipping quoted-string and comment content so embedded
    braces/semicolons in description text don't confuse block matching."""
    stmts = []
    i, n = 0, len(text)

    def skip_ws_comments(i):
        while i < n:
            c = text[i]
            if c in " \t\r\n":
                i += 1
            elif text[i : i + 2] == "//":
                j = text.find("\n", i)
                i = n if j == -1 else j + 1
            elif text[i : i + 2] == "/*":
                j = text.find("*/", i)
                i = n if j == -1 else j + 2
            else:
                break
        return i

    while True:
        i = skip_ws_comments(i)
        if i >= n:
            break
        start = i
        while i < n and text[i] not in " \t\r\n{};":
            i += 1
        keyword = text[start:i]
        if not keyword:
            i += 1
            continue
        i = skip_ws_comments(i)

        arg_parts = []
        while i < n and text[i] not in "{;":
            if text[i] in "\"'":
                quote = text[i]
                i += 1
                buf = []
                while i < n and text[i] != quote:
                    if quote == '"' and text[i] == "\\" and i + 1 < n:
                        buf.append(text[i + 1])
                        i += 2
                        continue
                    buf.append(text[i])
                    i += 1
                i += 1  # closing quote
                arg_parts.append("".join(buf))
                i = skip_ws_comments(i)
                if i < n and text[i] == "+":
                    i = skip_ws_comments(i + 1)
                    continue
                # Not a string-concatenation continuation, so the
                # argument is done and the loop is about to exit for
                # good (break, below) -- leave `i` at its
                # already-whitespace-skipped position so the `{`/`;`
                # check right after this loop sees the subbody opener
                # even when it's separated from the closing quote by
                # whitespace (e.g. `refine "point" { ... }`, a common
                # YANG style). Previously this reset `i` to right after
                # the closing quote, which made that check silently
                # fail to find a subbody whenever there was
                # intervening whitespace -- the subbody's statements
                # then got parsed as this one's siblings instead of
                # its children.
                break
            else:
                wstart = i
                while i < n and text[i] not in " \t\r\n{;":
                    i += 1
                arg_parts.append(text[wstart:i])
                i = skip_ws_comments(i)
                break
        argument = "".join(arg_parts) if arg_parts else None

        subbody = None
        if i < n and text[i] == "{":
            depth = 1
            i += 1
            bstart = i
            while i < n and depth > 0:
                c = text[i]
                if c in "\"'":
                    quote = c
                    i += 1
                    while i < n and text[i] != quote:
                        if quote == '"' and text[i] == "\\" and i + 1 < n:
                            i += 2
                            continue
                        i += 1
                    i += 1
                elif text[i : i + 2] == "//":
                    j = text.find("\n", i)
                    i = n if j == -1 else j + 1
                elif text[i : i + 2] == "/*":
                    j = text.find("*/", i)
                    i = n if j == -1 else j + 2
                elif c == "{":
                    depth += 1
                    i += 1
                elif c == "}":
                    depth -= 1
                    i += 1
                else:
                    i += 1
            subbody = text[bstart : i - 1]
        elif i < n and text[i] == ";":
            i += 1
        stmts.append((keyword, argument, subbody))
    return stmts


def bare(keyword):
    return keyword.split(":")[-1]


_KIND_GUARD_RE = re.compile(r"derived-from-or-self\(\s*\.\./kind\s*,\s*'([^']+)'\s*\)")


class Schema:
    def __init__(self):
        self.groupings = {}       # (module, name) -> subbody text
        self.notifications = {}   # (module, name) -> subbody text
        self.prefix_map = {}      # module -> {prefix: target-module}
        self.identity_bases = {}  # (module, name) -> [(module, base-name), ...]

    def load(self, path):
        text = open(path, encoding="utf-8").read()
        stmts = strip_and_split(text)
        mod_kw, mod_name, mod_body = None, None, None
        for kw, arg, sub in stmts:
            if bare(kw) in ("module", "submodule") and sub is not None:
                mod_kw, mod_name, mod_body = kw, arg, sub
                break
        if mod_body is None:
            return
        own_prefix = None
        top = strip_and_split(mod_body)
        prefixes = {}
        for kw, arg, sub in top:
            if bare(kw) == "prefix":
                own_prefix = arg
            elif bare(kw) == "import" and sub is not None:
                imp_stmts = strip_and_split(sub)
                p = None
                for ikw, iarg, _isub in imp_stmts:
                    if bare(ikw) == "prefix":
                        p = iarg
                if p:
                    prefixes[p] = arg
        if own_prefix:
            prefixes[own_prefix] = mod_name
        self.prefix_map[mod_name] = prefixes

        for kw, arg, sub in top:
            k = bare(kw)
            if k == "grouping" and sub is not None:
                self.groupings[(mod_name, arg)] = sub
            elif k == "notification" and sub is not None:
                self.notifications[(mod_name, arg)] = sub
            elif k == "identity" and sub is not None:
                bases = []
                for ik, iarg, _isub in strip_and_split(sub):
                    if bare(ik) == "base" and iarg:
                        tgt = self.resolve(mod_name, iarg)
                        if tgt:
                            bases.append(tgt)
                self.identity_bases[(mod_name, arg)] = bases

    def resolve(self, from_module, ref):
        """Resolve a possibly-prefixed grouping reference to (module, name)."""
        if ":" in ref:
            pfx, name = ref.split(":", 1)
            target = self.prefix_map.get(from_module, {}).get(pfx)
            if target is None:
                return None
            return (target, name)
        return (from_module, ref)

    @staticmethod
    def _refined_mandatory(uses_sub):
        """{child: bool} for every `refine "<child>" { mandatory true|false; }`
        directly on a uses statement whose target is a top-level child of
        the grouping (no '/' in the refine path)."""
        out = {}
        if uses_sub is None:
            return out
        for ik, iarg, isub in strip_and_split(uses_sub):
            if bare(ik) != "refine" or not iarg or isub is None:
                continue
            child = iarg.strip('"\'')
            if "/" in child:
                continue
            for rk, rarg, _rsub in strip_and_split(isub):
                if bare(rk) == "mandatory" and rarg in ("true", "false"):
                    out[child] = rarg == "true"
        return out

    def mandatory_leaves(self, module, body, visited=None):
        if visited is None:
            visited = frozenset()
        result = set()
        for kw, arg, sub in strip_and_split(body):
            k = bare(kw)
            if k == "leaf" and sub is not None:
                if any(
                    bare(ik) == "mandatory" and iarg == "true"
                    for ik, iarg, _isub in strip_and_split(sub)
                ):
                    result.add(arg)
            elif k == "uses":
                target = self.resolve(module, arg)
                inherited = set()
                if target and target in self.groupings and target not in visited:
                    gbody = self.groupings[target]
                    inherited = self.mandatory_leaves(
                        target[0], gbody, visited | {target}
                    )
                for child, mand in self._refined_mandatory(sub).items():
                    (inherited.add if mand else inherited.discard)(child)
                result |= inherited
            # container / list / choice / case / anydata / anyxml / leaf-list:
            # each introduces its own optionality boundary (or, for
            # leaf-list, cannot itself be `mandatory`) -- deliberately not
            # descended into. See module docstring.
        return result

    def notification_mandatory_leaves(self, notif_name):
        """Look up a notification by its bare name across all loaded
        modules and return its top-level mandatory leaf-name set, or
        None if no notification with that name was loaded."""
        for (module, name), body in self.notifications.items():
            if name == notif_name:
                return self.mandatory_leaves(module, body)
        return None

    def missing_mandatory(self, module, body, instance, visited=None):
        """Instance-aware mandatory-leaf check. Returns a sorted list of
        missing mandatory leaf paths given a schema block body and the
        corresponding instance dict.

        Extends mandatory_leaves() with one rule: a `container` that is
        PRESENT in the instance is descended into, and its own mandatory
        leaves become required (path-qualified as 'container/leaf'). This
        makes a mandatory leaf inside a *presence* container enforceable
        once the container is instantiated -- e.g. wire-source's
        `container source` carries a mandatory `decoder`, so an instance
        that includes `source` but omits `decoder` is flagged. A container
        that is ABSENT stays optional (a presence container is optional by
        definition; a non-presence container's mandatory descendants are
        only synthesized when the container is actually instantiated), so
        this never newly rejects a fixture that omits an optional subtree."""
        if visited is None:
            visited = frozenset()
        if not isinstance(instance, dict):
            instance = {}
        present = {k.split(":")[-1]: v for k, v in instance.items()}
        missing = []
        for kw, arg, sub in strip_and_split(body):
            k = bare(kw)
            if k == "leaf" and sub is not None:
                if any(
                    bare(ik) == "mandatory" and iarg == "true"
                    for ik, iarg, _isub in strip_and_split(sub)
                ):
                    if arg not in present:
                        missing.append(arg)
            elif k == "uses":
                target = self.resolve(module, arg)
                sub_missing = []
                if target and target in self.groupings and target not in visited:
                    sub_missing = self.missing_mandatory(
                        target[0], self.groupings[target], instance,
                        visited | {target}
                    )
                refined = self._refined_mandatory(sub)
                sub_missing = [m for m in sub_missing if refined.get(m, True)]
                for child, mand in refined.items():
                    if mand and child not in present and child not in sub_missing:
                        sub_missing.append(child)
                missing += sub_missing
            elif k == "container" and sub is not None and arg in present:
                for m in self.missing_mandatory(module, sub, present[arg], visited):
                    missing.append(f"{arg}/{m}")
            # list / choice / case / anydata / anyxml / leaf-list and absent
            # containers: not descended into (see docstring / mandatory_leaves).
        return sorted(missing)

    def notification_missing(self, notif_name, instance):
        """Instance-aware missing-mandatory list for a notification, or
        None if no notification with that bare name was loaded."""
        for (module, name), body in self.notifications.items():
            if name == notif_name:
                return self.missing_mandatory(module, body, instance)
        return None

    def _derived_from_or_self(self, ident, ancestor, seen=None):
        """True if identity `ident` is `ancestor` or transitively derives
        from it, per the loaded `base` graph."""
        if ident == ancestor:
            return True
        if seen is None:
            seen = set()
        if ident in seen:
            return False
        seen.add(ident)
        for base in self.identity_bases.get(ident, []):
            if self._derived_from_or_self(base, ancestor, seen):
                return True
        return False

    def _guarded_leaves(self, module, body, visited=None):
        """Map leaf-name -> set of allowed kind identities (module,name) for
        every notification leaf whose `when` is a pure
        derived-from-or-self(../kind, 'X') guard (one or more OR'd clauses).
        Crosses `uses` at the same depth; other `when` forms are ignored
        (they reference sibling data this instance-only checker cannot
        evaluate). yanglint's pinned build does not enforce `when` on a
        notification instance, so this reproduces that one guard shape."""
        if visited is None:
            visited = frozenset()
        out = {}
        for kw, arg, sub in strip_and_split(body):
            k = bare(kw)
            if k == "leaf" and sub is not None:
                for ik, iarg, _isub in strip_and_split(sub):
                    if bare(ik) == "when" and iarg:
                        ids = _KIND_GUARD_RE.findall(iarg)
                        if ids:
                            allowed = {t for t in (self.resolve(module, r) for r in ids) if t}
                            out[arg] = allowed
            elif k == "uses":
                target = self.resolve(module, arg)
                if target and target in self.groupings and target not in visited:
                    out.update(
                        self._guarded_leaves(
                            target[0], self.groupings[target], visited | {target}
                        )
                    )
        return out

    def notification_when_violations(self, notif_name, instance):
        """List of reason strings for kind-guarded leaves that are present
        in the instance but whose 'kind' is not derived-from-or-self any of
        the leaf's allowed kinds. Empty if none / notification not found."""
        if not isinstance(instance, dict):
            return []
        for (module, name), body in self.notifications.items():
            if name != notif_name:
                continue
            guarded = self._guarded_leaves(module, body)
            if not guarded:
                return []
            present = {k.split(":")[-1] for k in instance.keys()}
            kind_val = None
            for k, v in instance.items():
                if k.split(":")[-1] == "kind":
                    kind_val = v
                    break
            if kind_val is None:
                return []
            if ":" in kind_val:
                pfx, iname = kind_val.split(":", 1)
                kind_id = (self.prefix_map.get(module, {}).get(pfx, module), iname)
            else:
                kind_id = (module, kind_val)
            reasons = []
            for leaf, allowed in guarded.items():
                if leaf in present and not any(
                    self._derived_from_or_self(kind_id, a) for a in allowed
                ):
                    reasons.append(f"{leaf}(kind={kind_val}-not-in-guard)")
            return sorted(reasons)
        return []

    def _top_level_min_elements(self, module, body, visited=None):
        """Collect min-elements floors reachable at a notification's own
        top level: {child-name: floor}. Two sources: a min-elements
        substatement declared directly on a top-level list/leaf-list,
        and a `refine "<name>" { min-elements N; ... }` substatement on
        a `uses` statement -- the mechanism a notification uses to put a
        cardinality floor on a list it splices in from a grouping it
        doesn't own (e.g. openits-types:geo-path's `point`; see
        openits-work-zone-events.yang). Also crosses plain `uses` at the
        same depth to inherit any min-elements already native to the
        grouping's own list/leaf-list (a local refine, if present, wins
        over an inherited value). Never descends into container/list/
        choice/case, for the same optionality-boundary reason
        mandatory_leaves() doesn't."""
        if visited is None:
            visited = frozenset()
        out = {}
        for kw, arg, sub in strip_and_split(body):
            k = bare(kw)
            if k in ("list", "leaf-list") and sub is not None:
                for ik, iarg, _isub in strip_and_split(sub):
                    if bare(ik) == "min-elements" and iarg and iarg.isdigit():
                        out[arg] = int(iarg)
            elif k == "uses":
                target = self.resolve(module, arg)
                inherited = {}
                if target and target in self.groupings and target not in visited:
                    inherited = self._top_level_min_elements(
                        target[0], self.groupings[target], visited | {target}
                    )
                refined = {}
                if sub is not None:
                    for ik, iarg, isub in strip_and_split(sub):
                        if bare(ik) == "refine" and iarg and isub is not None:
                            child = iarg.split("/")[0]
                            for rk, rarg, _rsub in strip_and_split(isub):
                                if bare(rk) == "min-elements" and rarg and rarg.isdigit():
                                    refined[child] = int(rarg)
                for child, floor in inherited.items():
                    out.setdefault(child, floor)
                out.update(refined)
        return out

    def notification_min_elements_violations(self, notif_name, instance):
        """List of reason strings for min-elements floors (see
        _top_level_min_elements) the instance violates. yanglint's
        pinned build does not enforce min-elements on a notification
        instance under -t notif any more than must/mandatory --
        confirmed empirically alongside those two."""
        if not isinstance(instance, dict):
            return []
        for (module, name), body in self.notifications.items():
            if name != notif_name:
                continue
            present = {k.split(":")[-1]: v for k, v in instance.items()}
            floors = self._top_level_min_elements(module, body)
            reasons = []
            for child, floor in floors.items():
                val = present.get(child)
                count = len(val) if isinstance(val, list) else (0 if val is None else 1)
                if count < floor:
                    reasons.append(f"{child}(min-elements={floor},got={count})")
            return sorted(reasons)
        return []


def main(argv):
    if "--" not in argv:
        print("usage: check-notif-mandatory.py --schemas S... -- F...", file=sys.stderr)
        return 2
    sep = argv.index("--")
    schema_args = argv[1:sep]
    if schema_args and schema_args[0] == "--schemas":
        schema_args = schema_args[1:]
    fixtures = argv[sep + 1 :]

    schema = Schema()
    for path in schema_args:
        schema.load(path)

    for fpath in fixtures:
        with open(fpath, encoding="utf-8") as f:
            try:
                data = json.load(f)
            except ValueError as e:
                print(f"ERROR {fpath} invalid JSON: {e}")
                continue
        if not isinstance(data, dict) or len(data) != 1:
            print(f"OK {fpath}")
            continue
        (top_key,) = data.keys()
        notif_name = top_key.split(":")[-1]
        instance = data[top_key]
        missing = schema.notification_missing(notif_name, instance)
        if missing is None:
            # Not a known notification (e.g. a config/state datastore
            # fixture) -- out of scope for this check.
            print(f"OK {fpath}")
            continue
        problems = (
            list(missing)
            + schema.notification_when_violations(notif_name, instance)
            + schema.notification_min_elements_violations(notif_name, instance)
        )
        if problems:
            print(f"MISSING {fpath} {','.join(problems)}")
        else:
            print(f"OK {fpath}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
