#!/usr/bin/env python3
"""CI gate: no inline `enumeration` inside a grouping that crosses module lines.

ygot names the Go type it generates for an INLINE enumeration after a
USE-SITE path, not after the module that defines the grouping. So an inline
enumeration inside a grouping that is composed from another module belongs
to whichever composing module sorts first, and adding a second composer
silently RENAMES it out from under the first.

Nothing in the composed YANG changes when that happens, and the failure
surfaces as an unrelated module's Go build breaking — which makes it
effectively invisible in review. Hoisting the enumeration into a named
`typedef` in the grouping's own module makes the generated name
module-scoped and stable (`OpenitsCabinetPower_PowerSource`) no matter who
composes it.

Two severities:

  ACTIVE  - the grouping is already composed by 2+ modules, so consumers of
            all but one of them hold a type named after somebody else's
            tree. Fails the gate.
  LATENT  - the grouping is composed by exactly one module, and that module
            is NOT the one defining it. Stable today; the next composer
            renames it. Fails the gate, because "the next composer" is
            precisely the change that will not notice.

A grouping defined and used only within its own module is service-local:
its generated name cannot be raced, so it is allowed and not reported.

Scans yang/**/*.yang, excluding vendored IETF modules (yang/ietf/).

Exit 0 when clean; exit 1 listing offending groupings.
"""
import re
import sys
from pathlib import Path

YANG_ROOT = Path("yang")

MODULE = re.compile(r'^\s*(?:module|submodule)\s+([\w.-]+)', re.M)
PREFIX = re.compile(r'^\s*prefix\s+([\w.-]+)\s*;', re.M)
IMPORT = re.compile(r'import\s+([\w.-]+)\s*\{\s*prefix\s+([\w.-]+)\s*;')
GROUPING = re.compile(r'^[ \t]*grouping\s+([\w.-]+)\s*\{', re.M)
USES = re.compile(r'\buses\s+(?:([\w.-]+):)?([\w.-]+)\s*[;{]')
LEAF = re.compile(r'^[ \t]*leaf(?:-list)?\s+([\w.-]+)\s*\{', re.M)
INLINE_ENUM = re.compile(r'\btype\s+enumeration\s*\{')


def brace_body(text, start):
    """Body between the first '{' at/after start and its matching '}'."""
    i = text.index("{", start)
    depth = 0
    for j in range(i, len(text)):
        if text[j] == "{":
            depth += 1
        elif text[j] == "}":
            depth -= 1
            if depth == 0:
                return text[i + 1:j]
    return text[i + 1:]


def line_of(text, idx):
    return text.count("\n", 0, idx) + 1


def scan():
    files = [p for p in sorted(YANG_ROOT.rglob("*.yang")) if "ietf" not in p.parts]

    modules = {}      # module -> (path, text)
    own_prefix = {}   # module -> prefix
    imports = {}      # (module, prefix) -> imported module

    for path in files:
        text = path.read_text()
        m = MODULE.search(text)
        if not m:
            continue
        mod = m.group(1)
        modules[mod] = (path, text)
        p = PREFIX.search(text)
        if p:
            own_prefix[mod] = p.group(1)
        for im in IMPORT.finditer(text):
            imports[(mod, im.group(2))] = im.group(1)

    # (defining module, grouping) -> set of modules that `uses` it
    composers = {}
    for mod, (_, text) in modules.items():
        for u in USES.finditer(text):
            pfx, name = u.group(1), u.group(2)
            if pfx is None or pfx == own_prefix.get(mod):
                target = mod
            else:
                target = imports.get((mod, pfx))
            if target:
                composers.setdefault((target, name), set()).add(mod)

    findings = []
    for mod, (path, text) in modules.items():
        for g in GROUPING.finditer(text):
            gname = g.group(1)
            body = brace_body(text, g.start())
            offenders = []
            for lf in LEAF.finditer(body):
                leaf_body = brace_body(body, lf.start())
                if INLINE_ENUM.search(leaf_body):
                    offenders.append(lf.group(1))
            if not offenders:
                continue

            used_by = composers.get((mod, gname), set())
            external = used_by - {mod}
            if len(used_by) >= 2:
                sev = "ACTIVE"
            elif external:
                sev = "LATENT"
            else:
                continue  # service-local: cannot be raced

            findings.append((
                sev, path, line_of(text, g.start()), mod, gname,
                offenders, sorted(used_by),
            ))

    return findings


def main():
    findings = scan()
    if not findings:
        print("check-inline-enums: no cross-module grouping carries an inline enumeration")
        return 0

    findings.sort(key=lambda f: (f[0] != "ACTIVE", str(f[1])))
    for sev, path, line, mod, gname, leaves, used_by in findings:
        print(f"{path}:{line}: {sev} grouping '{gname}' has inline enumeration(s): "
              f"{', '.join(leaves)}")
        print(f"    composed by: {', '.join(used_by)}")
        print(f"    fix: hoist each into a named `typedef` in {mod}, so the generated")
        print("         type is module-scoped and cannot be renamed by another composer.")
    print()
    print(f"check-inline-enums: {len(findings)} grouping(s) need hoisting "
          "(see docs/reference/yang-reference-conventions.md)")
    return 1


if __name__ == "__main__":
    sys.exit(main())
