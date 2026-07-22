---
name: extending-a-model
description: The required workflow for changing any existing YANG module in this repo — adding/renaming/removing leaves, containers, lists, enums, identities, or must constraints. Use this whenever a task touches a file under yang/, even for "small" edits like a new leaf or a description fix, and BEFORE editing: it covers the identity-vs-enum choice, config/state placement, revision discipline, fixtures, and the regeneration order that CI enforces. Skipping it is the most common way contributor PRs fail CI.
---

# Extending an existing model

Every YANG change here ripples into generated protobuf, Go bindings, JSON
Schema registry snapshots, and AsyncAPI — all checked in, all drift-gated by
CI. The workflow below exists so a change lands green in one pass.

Make the SMALLEST change that satisfies the task. Adding a leaf never
justifies restructuring how a grouping is composed or renaming its
containers — if the existing structure genuinely blocks the addition, stop
and raise it rather than folding a breaking reshape into an additive PR.

Authoritative background: `docs/data-model.md` (structure and idioms),
`docs/04-design-decisions.md` (why the conventions are what they are),
`docs/versioning.md` (revisions vs releases vs wire compatibility).

## Before you edit: three placement decisions

**1. Config, state, or operation?** Modules follow the OpenConfig-style
split: a `config` container (intended state, `config true`), a sibling
`state` container (the config mirror + derived values, `config false`).
Operational rollups that are not config readback (current mode, flash
status, active-plan) live in a state-only container such as `operation` —
NOT in `state`, which mirrors config. Put a new leaf where its writer is:
operator intent → config, device observation → state/operation.

**2. Identity or enum?** Use an `identityref` when the value set is a
classification hierarchy or vendors/agencies may extend it (device kinds,
event kinds, control sources, time sources). Use an `enumeration` for
closed, orthogonal sets the standard fully owns (interval types, cycle
states). Two hard rules:

- **Enum value stability is the wire contract.** In an enum whose members
  lack explicit `value` statements, YANG assigns values by position — a
  mid-list insert silently renumbers every later member, and the generated
  proto follows (silent wire break). Members with explicit `value`
  statements keep them verbatim in proto (see the emitEnum contract in
  `tools/yang-proto-gen/emit.go`), so the rules are: NEVER change an
  existing member's value, give every new member an explicit value above
  the current maximum, and append it at the end (position is style, but
  append keeps diffs and reviews honest).
- **Identity additions are non-breaking**: identityref leaves render as
  proto strings, so extending an identity hierarchy never moves wire tags.
  When in doubt about future extension, prefer the identity.

**3. New list?** Keys are explicit `key` leaves typed with the shared id
typedefs from `openits-types` where one fits. Follow an existing list in
the same module for the key idiom before inventing one.

## Writing must constraints

- Every new `must` gets a PAIR of fixtures (see below) — a must without an
  `invalid-*` fixture is unverified and reviewers will bounce it.
- `deref()` paths: a leafref one container deeper needs one extra `..` —
  count containers, not intuition, and let `make validate-yang` confirm.
- **Trap:** a `must` on a container whose leaf has a `default`, under a
  non-presence ancestor, is evaluated against the implicit default tree of
  EVERY fixture in the repo — unrelated modules start failing. If
  validate-yang suddenly cascades across modules after your change, this is
  why. Guard the must with a `presence` container or condition it on the
  leaf's existence.

## Revision discipline

Add ONE new `revision` statement (today's date, imperative summary) per
change set. Never add a second revision with the same date as an existing
one, and never rewrite a published revision's description.
`make check-revisions` gates this.

Edge case — the module already has a revision dated today: if that
revision has NOT shipped in a release, fold your change into it (extend
its description). If it HAS shipped (a release tag includes it), its
registry snapshot is immutable — do not amend it or let
`update-schema-registry.sh` rewrite that snapshot; flag the collision in
the PR and let a maintainer decide (usually: date the new revision the
next day).

## Fixtures

Instance-data fixtures live in `yang/testdata/`:

- `valid-<scenario>.json` — must parse cleanly.
- `invalid-<scenario>.json` — must be REJECTED (schema or must violation).

For every new constraint add both: a valid fixture exercising the feature
and an invalid one proving the constraint fires. Fixtures are RFC 7951
JSON — top-level members are keyed by the module that DEFINES the node.
Never put personal names, real agency data, or issue-tracker IDs in
fixture content; use neutral placeholder values like `field-tech-07`.

## The pipeline, in order

```
1. Edit yang/<module>.yang           (+ revision statement)
2. make gen                          # regenerates proto, Go, asyncapi
3. scripts/update-schema-registry.sh # immutable registry snapshot for the new revision
4. Add/adjust fixtures in yang/testdata/
5. make check-gen validate-yang yang-lint check-revisions check-naming
6. go build ./... && go test ./...
7. go run ./tools/conformance -driver mock -kind <affected-kind>
8. Commit the YANG edit AND all regenerated artifacts together
```

Committing the model change without its regen (or vice versa) is the #1
cause of `check-gen` failures on PRs. `field-numbers.yaml` will gain
entries for new fields — that is expected; never hand-edit it, and never
be surprised that deleting a field leaves it byte-identical (deleted tags
are tombstoned so they are never reused).

If a gate fails and the cause isn't obvious, use the `model-ci-gates`
skill to interpret it.

## Commit message

Conventional Commits drive releases: `feat:` (new model surface) → minor,
`fix:` (correction) → patch. Breaking wire changes pre-1.0 use `feat!:`
with a `BREAKING CHANGE:` footer describing the wire impact. Pure
regen/tooling churn that shouldn't release is `chore:`.
