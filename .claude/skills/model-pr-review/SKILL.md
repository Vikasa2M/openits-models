---
name: model-pr-review
description: Reviewer checklist for pull requests that touch the model surface (yang/, api/, bindings/, pkg/, schema-registry/, field-numbers.yaml). Use this whenever reviewing a PR in this repo, when asked to self-review a model change before opening a PR, or when assessing whether a diff is wire-breaking. It encodes what maintainers actually bounce PRs for.
---

# Reviewing a model PR

Green CI is necessary, not sufficient — the gates can't judge modeling
quality or intent. Review in this order; the early items are the ones
that are expensive to fix after merge.

## 1. Wire compatibility (irreversible after release)

- Enum diffs: no existing member's `value` may change, and new members
  need an explicit value above the current maximum. The dangerous case is
  an enum with NO explicit values — there YANG numbers by position and a
  mid-list insert silently renumbers later members (wire break).
  `buf breaking` should catch renumbering, but verify on any enum diff;
  prefer appended members regardless for review clarity.
- No proto tag reuse: `field-numbers.yaml` diff shows only ADDED lines.
  Byte-identical on field deletion is correct (tombstones).
- Identity additions are safe (identityrefs are proto strings); identity
  RENAMES are wire-visible in JSON/identityref values — treat as breaking.
- Intended breaks are `feat!:` + `BREAKING CHANGE:` footer describing the
  wire impact, and pre-1.0 only.

## 2. Constraint coverage

- Every new/changed `must` has BOTH a `valid-*` and an `invalid-*`
  fixture in `yang/testdata/`. No invalid fixture = unverified constraint;
  ask for it.
- The invalid fixture fails for the RIGHT reason (the new must, not a
  parse error).
- No must on a defaulted-leaf container under a non-presence ancestor
  (fires on every fixture's default tree).

## 3. Modeling quality

- Placement: intent in `config`, observation in `state`/operational
  containers; `state` mirrors config, rollups live in state-only
  containers.
- Identity-vs-enum matches policy: hierarchies/extensible → identity,
  closed orthogonal sets → enum. No new enum that mirrors a kind
  identity hierarchy.
- Device-class neutrality: no vendor-specific leaves in core modules
  (vendor surface goes through augments/extension model); leaf names and
  units follow the referenced standard (`reference` statements present).
- Reuses shared typedefs/groupings (`openits-types`, platform groupings)
  instead of redefining.

## 4. Mechanical discipline

- Exactly one new `revision` per changed module, dated, descriptive; no
  duplicate same-date revisions. Watch for the sneaky variant: AMENDING an
  already-released revision's description (and thereby rewriting its
  "immutable" registry snapshot) instead of adding a new revision — diff
  the revision block, not just its presence.
- The whole tree must move together: if `yang/` changed but the module's
  `schema-registry/` snapshot didn't (or vice versa), the PR is
  internally inconsistent even when individual gates pass.
- Generated artifacts (proto, Go, asyncapi, schema-registry snapshot)
  committed IN THE SAME PR as the YANG change; no hand-edits to generated
  files (spot-check: does the diff touch generated code the YANG diff
  can't explain?).
- Events changes respect layering (events import only types-level
  modules) and fixture module-prefixing.
- No issue-tracker IDs, personal names, or internal URLs in YANG
  descriptions, fixtures, or docs — this is a public standard.

## 5. Commit/release hygiene

- Commit type matches content: `feat:`/`fix:` for model surface (drives
  the version), `chore:`/`ci:`/`docs:` for everything else. Dependabot
  merges that should cut a release need retitling to `fix(deps):` at
  squash time.
- PR is one reviewable unit: model + regen + fixtures + conformance
  together, unrelated refactors split out.
