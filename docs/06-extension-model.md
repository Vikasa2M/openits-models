# Extension model

OpenITS is designed to grow. The mechanism by which it grows is the
single most important governance feature of the project; it
determines whether the standard becomes useful over time or becomes
a snapshot of one moment's thinking.

This document describes the four-tier extension model and the
graduation rule that lets vendor and agency contributions move into
core when the market signals consensus.

## The four tiers

### Tier 1 — Core

**Path.** `yang/openits-<service>.yang`
**Namespace.** `urn:openits:yang:<service>`
**Owner.** Technical Steering Committee (TSC), operator-weighted
majority on YANG-changing decisions.
**Stability.** Breaking changes require a 2-year deprecation window
during which both old and new revisions publish in parallel.
**Contents.** The minimum every conforming implementation MUST
support — semantic model derived from the relevant device-level
standard, structural shape agreed by TSC, MUTCD-style safety
constraints expressed as YANG `must`.

A conforming implementation that claims `openits-signal-control`
support implements every `config true` leaf as writable and emits
every notification listed in `openits-signal-control-events`
when the underlying condition occurs.

### Tier 2 — Augments

**Path.** `yang/augments/<contributor>-<service>-<feature>.yang`
**Namespace.** `urn:<contributor>:yang:<service>-<feature>`
**Owner.** The contributor (vendor or agency); listed in the
module's `contact` statement.
**Stability.** The contributor decides; community can fork if the
contributor abandons.
**Contents.** Net-new nodes added via YANG 1.1 `augment`
statements. **Never modifies existing core nodes.** Wire-compatible:
a consumer that doesn't load the augment ignores the unknown
nodes (Protobuf's unknown-field tolerance does the same on the
wire).

Examples of legitimate augments:

- `siemens-signal-control-vehicle-counts.yang` — per-phase vehicle
  count statistics from Siemens detectors.
- `caltrans-signal-control-corridor-id.yang` — Caltrans-specific
  corridor identifier on each controller.
- `econolite-dms-marquee-mode.yang` — Econolite-specific marquee
  animation mode.

A consumer reading messages can ignore augments it doesn't
recognise. A producer that ships an augment SHOULD carry both the
core fields and the augment fields in every relevant message —
augments are additive, not replacements.

#### Tier 2 sub-pattern: identity-only extension

Some contributions don't add new schema-tree nodes; they refine
identity hierarchies. A vendor module declares specialized
identities derived from open identity slots in a core module,
without `augment` statements. A consumer that doesn't load the
vendor module sees only the base identity; one that does sees the
specific derived identity in the same `identityref` field.

This is the right pattern when:

- The core leaves an open identity slot for vendor refinement
  (e.g., `fault-vendor-alarm` in `openits-signal-control-types`).
- The contributor maps wire-level codes 1:1 onto specific
  identities (e.g., the Econolite ASC3 vendor-alarm slots 1-8 from
  Indiana code 185).
- No new fields are needed; the existing payload shape already
  carries an `identityref` that points into the extended hierarchy.

**Worked example in-tree:**
[`yang/openits-vendor-econolite-signal-control-types.yang`](../yang/openits-vendor-econolite-signal-control-types.yang)
declares 8 econolite-vendor-alarm-N identities derived from
`openits-signal-control-types:fault-vendor-alarm`. The module
contains no `augment` statement; it extends only the identity
space.

Filename + namespace conventions match Tier 2:

| Field | Value |
|---|---|
| Path | `yang/openits-vendor-<vendor>-<service>-types.yang` |
| Namespace | `urn:<vendor>:yang:<service>-types` |
| Prefix | Vendor's choice, conventionally `openits-vendor-<vendor>-<service>-types` |
| Contact | Vendor; the project carries the Econolite example as a conformance-reference under NoI |

Identity-only extension does not require its own NoI graduation
flow distinct from augments; the same three-implementer rule
applies. Graduation typically means promoting the open identity
slot to a richer identity hierarchy in the core module.

### Tier 3 — Deviations

**Path.** `yang/deviations/openits-<service>-<deviation-name>.yang`
**Namespace.** `urn:openits:yang:<service>-<deviation-name>`
(deviations stay under `openits` because they refine the
*standard*, not introduce new content).
**Owner.** TSC reviews any deviation that lands in
`yang/deviations/`; the deviation's `contact` field names the
proposing party.
**Stability.** Deviation modules carry their own revision dates;
enforcement is per-implementation (a controller declares which
deviations it adheres to).
**Contents.** YANG 1.1 `deviation` statements that **tighten**
constraints in the base module — narrower ranges, additional
`must` rules, mandatory leaves that were optional. May also include
narrowly-scoped `augment` statements when the deviation needs new
nodes to express its constraint. Wire-compatible (deviations don't
change the schema; they change validation).

Examples of legitimate deviations:

- `openits-signal-control-mutcd-strict.yang` — enforces yellow ≥
  4.0s instead of core's 3.0s minimum.
- `openits-signal-control-tsp.yang` — adds TSP-specific constraints.
- `openits-rsu-fcc-part90.yang` — narrows allowed channels to the
  FCC Part 90 5.9 GHz ITS band.

Deviations are how a jurisdiction or use case says "in our context,
the standard is *this*, not the maximum permissive set."

This tier is enforced, not just documented. A worked example ships
in-tree at
[`yang/deviations/openits-signal-control-mutcd-strict.yang`](../yang/deviations/openits-signal-control-mutcd-strict.yang),
and `make check-deviations` validates every module under
`yang/deviations/` resolves against its base module and checks that its
`deviate` statements tighten rather than loosen it. The check is
strongest for the common cases: `deviate add` always adds a constraint
(tightening) and passes; `not-supported` and `delete` of a `must` remove
a constraint (loosening) and fail the run. A `deviate replace` on a range
can narrow *or* widen and cannot be classified from the AST alone, so it
is reported as a note for reviewer attention rather than auto-passed; the
Docker-based tightening proof (a `must`-violating fixture that the base
accepts but base+deviation must reject) is the empirical backstop for
those cases.

### Tier 4 — Proprietary

**Path.** Outside `yang/` entirely; lives in the vendor's own
repository.
**Namespace.** Vendor's own (e.g.,
`urn:siemens:yang:proprietary-diagnostics`).
**Owner.** Vendor; not under any OpenITS governance.
**Stability.** Vendor's choice.
**Contents.** Anything the vendor wants to keep private — internal
diagnostics, factory calibration data, license-gated features.

Proprietary modules MAY ride on a vendor-specific NATS subject
prefix (`vendor.<vendor>.>`), NOT on `openits.>`. They keep the
vendor's data out of OpenITS conformance entirely. A vendor's
poller binary can publish *both* `openits.*` (core + augments)
and `vendor.<vendor>.*` (proprietary) on the same NATS connection.

## The graduation rule

This is the mechanism that makes Tier 2 (augments) *flow into*
Tier 1 (core) when the market signals consensus.

> An augment graduates into the next minor revision of the core
> module when **three independent implementations** — by different
> organisations, with implementation receipts on file — ship the
> same augment in a wire-compatible way, and the TSC passes a
> graduation motion (operator-weighted majority).

Three is the magic number used by IEEE 802 working groups, IETF
(rough consensus + running code), and several CNCF graduation
paths. It's enough to prove "this isn't one vendor's idea," small
enough that progress is achievable.

### Implementation receipts (Notices of Implementation)

A **Notice of Implementation (NoI)** is a one-page YAML file
submitted as a PR to
`schema-registry/notices/<augment-name>/<implementer>.yaml`:

```yaml
augment: siemens-signal-control-vehicle-counts
revision: 2026-04-19
implementer: caltrans
implementer_contact: noi@vikasa.io
implementer_type: operator
deployment_scale: 1200
first_observed: 2026-08-12
notes: >
  Caltrans is consuming the per-phase vehicle counts in District 7
  SCATS coordination.
```

NoI submission is a public commitment, machine-checkable for
graduation eligibility, and creates a paper trail. The
`implementer_type` field distinguishes operators from vendors
because the graduation rule requires at least one operator NoI.

### Graduation review

When an augment has 3+ NoIs from independent implementers, anyone
may open a **graduation PR** that:

1. Inlines the augment's nodes into the core module at the next
   minor revision.
2. Adds a deprecation notice to the augment module pointing at the
   new core revision.
3. Keeps the augment module in-tree for the full 2-year deprecation
   window.
4. Updates the graduation log with the date, augment, NoIs counted,
   and TSC motion record.

The graduation review checklist:

- [ ] Three NoIs from independent implementers (different
      `implementer` ids, no shared parent organisation per
      `schema-registry/notices/organizations.yaml`)
- [ ] At least one NoI is `implementer_type: operator`
- [ ] All three NoIs reference the same augment revision
- [ ] No other augment in `yang/augments/` conflicts with the
      proposed graduation (collision check via
      `make check-augment-collisions`)
- [ ] The augment's wire format unchanged from the third NoI's
      revision (no last-minute "let's add one more leaf")
- [ ] AsyncAPI spec, conformance kit, ARC-IT inventory,
      schema-registry snapshot all updated in the same PR
- [ ] Graduation motion passes the GOVERNANCE.md voting rule
      (operator-weighted majority for YANG changes)

### What graduation does NOT mean

- It does NOT remove the augment from `yang/augments/`. The module
  stays for the deprecation window.
- It does NOT force re-implementation. Implementations using the
  augment-namespace nodes continue to work; their messages just
  have field names that match the new core for free if the proto
  generator preserves field numbers.
- It does NOT confer ownership. The TSC owns core; the original
  contributor's name stays in the commit history and the
  contributor list.

### What if an augment never graduates?

That's fine. Some augments serve a small market and never reach
3 implementers. They live in `yang/augments/` as long as their
owner maintains them. If the owner abandons them, anyone can pick
up maintenance via a PR to update the `contact` field, or another
implementer can fork them into their own namespace.

The system isn't trying to graduate everything. It's trying to
ensure that **what's in core is what the industry has converged on**.

## Authorship rules

| Tier | Contributor | Required NATS subject prefix | Required namespace |
|------|-------------|------------------------------|--------------------|
| Core | TSC | `openits.>` | `urn:openits:yang:<service>` |
| Augments | Any (vendor / agency / community) | `openits.>` (augments ride on the core service's subject tree) | `urn:<contributor>:yang:<service>-<feature>` |
| Deviations | Any (TSC review on merge) | `openits.>` | `urn:openits:yang:<service>-<deviation>` |
| Proprietary | Vendor | `vendor.<vendor>.>` (NOT `openits.>`) | Vendor's own |

The subject-prefix rule matters: a consumer subscribing to
`openits.>` gets all standard + augment + deviation traffic, and
only that. Vendor proprietary traffic is on a separate prefix and
a consumer that wants it must subscribe explicitly. This protects
the integrity of "openits-conformant" as a label.

## Versioning across tiers

| Change type | Effect on `ce-type` | Effect on `ce-dataschema` | Wire compat |
|-------------|---------------------|---------------------------|-------------|
| Add an optional leaf to a core module | unchanged (still `vN`) | bumps revision date | yes |
| Remove or rename a leaf in a core module | bumps to `vN+1`; both `vN` and `vN+1` publish in parallel for 2 years | bumps revision date | no |
| Add an augment | unchanged for core consumers; new `ce-type` if augment introduces new events | augment module has its own revision; core unchanged | yes |
| Add a deviation | unchanged | core unchanged; deviation has its own revision | yes |
| Graduate an augment into core | unchanged on the wire (field numbers preserved); core revision bumps; augment enters deprecation | core revision bumps | yes |

This means the wire format is stable across nearly all change
types. The only breaking case is a removal or rename in core,
which is heavily gated and rare.

## Tooling that enforces the model

The repository ships tools that make the extension model real:

| Tool | What it does |
|------|--------------|
| `make yang` | Regenerate Go + Protobuf from YANG modules. |
| `make validate-yang` | Validate `must`-constraints with libyang. |
| `make check-revisions` | Catch content-changed-without-revision-bump (covers `yang/`, `yang/augments/`, and `yang/deviations/`). |
| `./scripts/update-schema-registry.sh` | Snapshot YANG and proto into `schema-registry/<module>/<revision>/`. |
| `make validate-noi` | Validate every NoI YAML against the schema. |
| `make check-graduation` | Report per-augment NoI counts, operator presence, eligibility. |
| `make check-augment-collisions` | Warn on YANG path collisions between augments. |
| `make check-deviations` | Validate every `yang/deviations/*` module resolves against its base; classify `deviate` statements (add = tightening; `not-supported` / `delete`-of-`must` = loosening, fail; `replace` = note) plus a Docker tightening proof over the `invalid-*-under-*` fixtures. |
| `make asyncapi` | Regenerate `asyncapi.yaml` from the taxonomy-derived ce-type catalog, embedding each message's JSON Schema payload; see [Design decisions](04-design-decisions.md#generated-asyncapi-not-hand-maintained). |
| `make asyncapi-check` | `make asyncapi` plus a `git diff --exit-code`, failing CI when the spec is out of date. |

A contributor adding an augment runs `make yang-lint`,
`make check-revisions`, and `make check-augment-collisions`. If the
augment is graduation-track, they also file an NoI; `make
check-graduation` reports its progress against the rule. Since
AsyncAPI is generated, not hand-maintained, `make asyncapi-check`
joins this list automatically — a new notification's payload appears
in `asyncapi.yaml` on the next regeneration with no separate edit.

## Walkthrough: how an augment graduates

The walkthrough is fictional but mechanically correct. The in-tree
pedagogical exemplar of an augment lives at
[`yang/augments/example-signal-control-vehicle-counts.yang`](../yang/augments/example-signal-control-vehicle-counts.yang)
under the `urn:example:` placeholder namespace — when a real
contributor ships the augment, they swap `example` for their own
vendor / agency name throughout.

**Phase 1 — vendor ships in their namespace.** Siemens ships a
poller that emits per-phase vehicle counts. They author
`yang/augments/siemens-signal-control-vehicle-counts.yang` with
namespace `urn:siemens:yang:signal-control-vehicle-counts`, send
a PR with the YANG file, and submit an NoI at
`schema-registry/notices/siemens-signal-control-vehicle-counts/siemens.yaml`.
They start emitting
`openits.us-tx.txdot.d07.signal-control.ctrl-001.vehicle-count-snapshot`
events.

**Phase 2 — second implementer adopts.** Caltrans deploys 1,200
cabinets running pollers configured to consume the augment for
SCATS coordination. They submit an NoI at
`schema-registry/notices/siemens-signal-control-vehicle-counts/caltrans.yaml`
with `implementer_type: operator`. Operator NoI is significant;
the graduation rule requires at least one.

**Phase 3 — third implementer adopts.** Econolite ships a
competing controller that emits the same augment fields
wire-compatibly (same Protobuf field numbers). They submit
`schema-registry/notices/siemens-signal-control-vehicle-counts/econolite.yaml`.
Now there are three NoIs from three independent organisations,
including one operator. Graduation eligible.

**Phase 4 — graduation PR.** Anyone (typically the original
author) opens a PR that:

- Inlines `vehicle-count-snapshot` as a notification under
  `openits-signal-control-events.yang`
- Inlines the per-phase counter container under
  `/signal-controller/phases/phase/state/vehicle-counts`
- Bumps `openits-signal-control` revision
- Adds a deprecation notice to
  `siemens-signal-control-vehicle-counts.yang`
- Updates the conformance kit with new tests under the standard
  suite
- Updates AsyncAPI to register the event under the standard
  channel

**Phase 5 — TSC review.** Operator-weighted majority votes per
`GOVERNANCE.md`. If it passes, merge. Siemens' name remains in
the commit history; the `contact` of the deprecated augment
module retains attribution.

**Phase 6 — community catches up.** Implementations using the
Siemens augment namespace continue to work for two years. New
implementations target the new core revision. After two years,
the augment module is removed from `yang/augments/`.

This is the loop. It doesn't have to spin fast. What matters is
that it can spin at all.

## What this means for vendors and agencies

**Vendors:** ship innovation in your namespace today. If three
independent operators care, your contribution becomes part of the
standard, and you get the credit. If only you care, your customers
still get value and you keep the IP you want.

**Agencies:** if your jurisdiction has unique requirements (CA's
stricter MUTCD, NYC's signal preemption rules), file them as
deviations. Your vendors' products either claim conformance with
your deviation or they don't — the contract becomes
machine-checkable.

**TSC:** governs the floor (core), not the ceiling. The ceiling
moves on its own. You ratify what the market has already decided.

**Implementers:** subscribe to `openits.>`, get the standard. If
you want vendor-specific extensions, subscribe to
`vendor.<vendor>.>` separately. Conformance is testable; coverage
is documented; nothing surprising can show up in `openits.>`
without going through the model above.
