#!/usr/bin/env python3
"""CI gate: every enumeration member in first-party YANG must carry an
explicit `value` statement.

YANG assigns implicit enum values positionally (RFC 7950 9.6.4.2), so a
mid-list insert into a value-less enum silently renumbers every later
member — and the generated proto follows (a silent wire break). Explicit
values make the wire contract reviewable: a diff that changes an existing
`value` is visibly breaking, and a new member must state its value above
the current maximum.

Scans yang/**/*.yang, excluding the vendored IETF modules (yang/ietf/),
which are upstream-controlled and never edited here.

Exit 0 when clean; exit 1 listing offending members as file:line enum-name.
"""
import re
import sys
from pathlib import Path

ENUM_OPEN = re.compile(r'(^|\s)enumeration\s*\{')
MEMBER = re.compile(r'\benum\s+("[^"]+"|\'[^\']+\'|[^\s;{}]+)\s*(;|\{)')
VALUE = re.compile(r'\bvalue\s+-?\d+\s*;')


def check(path):
    text = path.read_text()
    findings = []
    i = 0
    while True:
        m = ENUM_OPEN.search(text, i)
        if not m:
            break
        depth, j = 1, m.end()
        while depth > 0:
            c = text[j]
            if c == '{':
                depth += 1
            elif c == '}':
                depth -= 1
            j += 1
        block = text[m.end():j - 1]
        k = 0
        while True:
            mm = MEMBER.search(block, k)
            if not mm:
                break
            name, delim = mm.group(1), mm.group(2)
            if delim == ';':
                body, k = '', mm.end()
            else:
                d, e = 1, mm.end()
                while d > 0:
                    c = block[e]
                    if c == '{':
                        d += 1
                    elif c == '}':
                        d -= 1
                    e += 1
                body, k = block[mm.end():e - 1], e
            if not VALUE.search(body):
                line = text.count('\n', 0, m.end() + mm.start()) + 1
                findings.append((line, name))
        i = j
    return findings


def main():
    root = Path(__file__).resolve().parent.parent
    bad = 0
    for path in sorted((root / 'yang').rglob('*.yang')):
        if 'ietf' in path.parts:
            continue
        for line, name in check(path):
            rel = path.relative_to(root)
            print(f'{rel}:{line}: enum member "{name}" has no explicit value statement', file=sys.stderr)
            bad += 1
    if bad:
        print(f'check-enum-values: {bad} enum member(s) missing explicit values '
              f'(implicit positional numbering is a silent wire-break hazard)', file=sys.stderr)
        return 1
    print('check-enum-values: all first-party enum members carry explicit values')
    return 0


if __name__ == '__main__':
    sys.exit(main())
