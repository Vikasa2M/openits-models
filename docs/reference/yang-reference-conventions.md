# YANG authoring & citation conventions

This is the conventions reference for authoring OpenITS YANG modules. It
covers two things: the **authoring doctrine** every module follows (where
`must` constraints belong, the config/state idiom, identity-vs-enum, and
module placement), and the **citation conventions** for how modules cite
normative sources in `reference` substatements.

## Contents

Authoring doctrine — the load-bearing rules for writing a module:

- [Constraint placement (`must`) doctrine](#constraint-placement-must-doctrine)
- [Identity vs. enum: which axis gets which](#identity-vs-enum-which-axis-gets-which)
- [Module placement: the ≥3-service rule](#module-placement-the-3-service-rule)
- [Config/state idiom](#configstate-idiom)

Citation conventions — how modules cite normative sources:

- [Normative-source citation formats](#normative-source-citation-formats)
- [Revision-block references](#revision-block-references)
- [Application rules](#application-rules)

## Citing normative sources

This section defines how OpenITS YANG modules cite normative sources
in `reference` substatements. The conventions cover both **revision-block
references** (the `reference` substatement of a `revision` statement)
and **node-level references** (the `reference` substatement of any
schema node: leaf, container, list, identity, typedef, grouping).

A canonical citation form lets adopter tooling extract a structured
list of every standard OpenITS depends on, surfaces drift when a cited
spec revision changes, and gives contributors a single answer to "how
do I cite this?".

## TL;DR

For node-level references, use the formats in [§ Normative-source
citation formats](#normative-source-citation-formats) below. For
revision-block references, allow both normative-source and OpenITS
PR citations (see [§ Revision-block references](#revision-block-references)).
This document is the convention; applying it codebase-wide is
tracked follow-up work.

## Why have a convention

OpenITS YANG modules port concepts from a small set of normative sources
(NTCIP, NEMA TS-2, SAE J2735, IEEE 1609.x, ATSPM Indiana enumeration,
ARC-IT). Citations let downstream consumers verify that a YANG
construct faithfully reflects its source:

- A `phase-number` typedef restricted to 1-16 should cite NEMA TS-2
  phase numbering.
- A `wire-source` choice case for NTCIP polling should cite NTCIP 1202
  / 1203 / 1204.
- An `indiana-code` leaf inside the wire-source grouping should cite
  the ATSPM Indiana enumeration.

When citations are buried in `description` prose, tooling can't
extract them. The YANG `reference` substatement is the structured
mechanism; this doc defines its content.

## Normative-source citation formats

Each format is a single string suitable for inclusion in a
`reference "...";` substatement. Place node-level references
immediately after the node's `description` (where applicable) or as
the last substatement before the node's closing brace.

### NTCIP

```yang
reference "NTCIP 1202:2019 §4.2.3.";
```

- **Form:** `NTCIP <doc-number>:<year> §<section>.`
- **Year** is the publication year of the cited revision (not the
  date of the revision being authored).
- **Section** uses the `§` symbol followed by the canonical section
  number from the NTCIP document.
- **Examples:**
  - `reference "NTCIP 1202:2019 §4.2.3.";` (signal-control)
  - `reference "NTCIP 1203:2010 §5.4.";` (DMS)
  - `reference "NTCIP 1204:2017 §3.2.";` (ESS / RWIS)
- **When to use:** A typedef, leaf, or grouping whose semantics or
  value range port from a specific NTCIP MIB object.

### NEMA TS-2

```yang
reference "NEMA TS-2 §6.3.2 (phase numbering).";
```

- **Form:** `NEMA TS-2 §<section> (<short-context>).`
- **Short context** in parens names the concept being cited (e.g.,
  "phase numbering", "intervals", "ring structure"). Optional but
  recommended for readability.
- **When to use:** Phase numbering ranges, ring/barrier structure,
  interval semantics, signal-controller convention.

### SAE J2735

```yang
reference "SAE J2735 §6.2.7 (DSRC.MessageFrame.messageId).";
```

- **Form:** `SAE J2735 §<section> (<asn1-path>).`
- **ASN.1 path** in parens names the J2735 ASN.1 type or message-id
  being referenced. Optional but recommended.
- **When to use:** V2X message types, BSM/SPaT/MAP/TIM/PSM/SRM/SSM
  identifiers, DSRC channel constants.

### IEEE 1609.x

```yang
reference "IEEE 1609.3-2020 §6.2 (WSMP packet).";
```

- **Form:** `IEEE <standard>:<year> §<section> (<short-context>).`
- **Standard** includes the sub-number (1609.2, 1609.3, 1609.4).
- **When to use:** WAVE/WSMP layer references, certificate management,
  multi-channel operation.

### ATSPM Indiana enumeration

```yang
reference "ATSPM Indiana enumeration code 185 (vendor-specific alarm).";
```

- **Form:** `ATSPM Indiana enumeration code <code> (<short-context>).`
- **Short context** in parens names what the code means. Required
  because the Indiana enumeration's canonical document is sparse;
  the parens supply the context tooling needs.
- **When to use:** Identities that map 1:1 to Indiana enumeration HR
  event codes; sub-event taxonomy bases.

### ARC-IT

```yang
reference "ARC-IT 9.1 Service Package SU01 (Traffic Signal Control).";
```

- **Form:** `ARC-IT <version> Service Package <package-id> (<package-name>).`
- **Version** is the ARC-IT version cited (e.g., 9.1).
- **When to use:** Service-package alignment, information-flow
  references; whenever a module ports a concept from the ARC-IT
  conceptual model.

### MUTCD

```yang
reference "MUTCD 11th ed. 4F.17 (yellow/red clearance).";
```

- **Form:** `MUTCD <edition> <section> (<short-context>).`
- **When to use:** Phase-timing constraints, pedestrian-interval
  minimums, traffic-control-device rules. Cite the actual basis:
  §4F.17 is Guidance for the yellow-change interval only (3–6 s);
  it sets no minimum for red-clear and no minimum-green value at
  all. Don't cite MUTCD for a constraint that is really an
  engineering floor.

### IETF RFCs

```yang
reference "RFC 7950 §11.";
```

- **Form:** `RFC <number> §<section>.`
- **When to use:** YANG specification, revision discipline (RFC 7950
  §11 for additive-only), IETF reference modules.

## Revision-block references

**Principle:** a `reference` substatement should cite something **useful to
a downstream consumer** — a normative spec, a section, a section number.
Bookkeeping references (e.g., "OpenITS audit X batch") provide zero adopter
value and pollute the citation surface; do not add them.

This is a deliberate deviation from RFC 8407 §4.8, which RECOMMENDS a
`reference` substatement on every revision. We adopt the recommendation
when a useful source applies and decline it when one doesn't. Pyang
`--lint` will flag the deviation; the warning is informational.

Per the project decision, revision blocks MAY cite a normative source,
an OpenITS PR ID, or both — whichever applies. The two patterns:

### Normative-source citation (revision ports from a spec)

```yang
revision 2026-04-19 {
  description
    "Initial revision. Minimum viable model covering identity,
     message buffers, active message, environment sensors, pixel
     and lamp status, and faults.";
  reference
    "NTCIP 1203 v03; ARC-IT 9.1 Service Package TI06 (Traffic Information
     Dissemination).";
}
```

Use when the revision delivers content ported from one or more
normative sources. Multiple sources are joined by `;` in citation order.

### OpenITS PR citation (project-internal revision)

```yang
revision 2026-05-17 {
  description
    "Add Traffic Signal Adaptive Management (TSAM) notifications:
     tsam-mode-changed and tsam-recommendation-applied.";
  reference
    "OpenITS PR #N: TSAM HR-event split.";
}
```

Use for revisions that re-organize, rename, fix bugs, or otherwise
derive from project-internal decisions rather than external specs. PR
number `#N` is the GitHub PR number that merged the revision.

**Use only when the PR is a known, externally-followable artifact.**
The PR number must be discoverable in the project's public PR history.
Citing internal audit document IDs (e.g., "OpenITS YANG audit U5-1")
when those documents are not published in the repo provides no consumer
value; leave `reference` absent rather than dangle.

### Concatenated form (both apply)

```yang
revision 2026-05-23 {
  description
    "Move preemption-type and controller-mode typedefs out to
     openits-signal-control-types.";
  reference
    "OpenITS PR #N: typedef placement convention.";
}
```

The concatenated form is also used to combine normative-source
citations with the OpenITS PR that delivered them:

```yang
reference
  "ARC-IT 9.1 Service Package SU01 (Traffic Signal Control);
   OpenITS PR #N: phase-state-change HR notifications.";
```

## Application rules

### Where to attach a reference

- **Module-level** `reference` (top of the module, after `description`):
  applies to the module as a whole; cite the foundational spec(s) the
  module ports from.
- **Revision-block** `reference`: applies to that revision's changes.
- **Node-level** `reference` (on typedef, identity, leaf, container,
  list, grouping, notification, augment): applies to the node's
  semantics.

### When `reference` is NOT required

- Nodes that are OpenITS-original conventions (no ported source).
  Example: the `wire-source` grouping in `openits-types` is an
  OpenITS-original abstraction — no `reference` needed.
- Nodes whose enclosing container already carries a `reference` and
  the sub-nodes inherit semantics by composition.
- Trivial wrappers (`leaf reason { type string; }`) where the source
  spec isn't load-bearing.

When in doubt, prefer including the reference; tooling silently
ignores duplicates and overly-narrow citations.

### Spec revision currency

Cited spec revisions SHOULD be the current published revision at the
time the YANG revision is authored. When a newer revision of a cited
spec ships, the YANG module's next revision SHOULD update its
references in lockstep — or document why the older revision remains
authoritative.

A periodic standards-revision verification pass catches stale
citations.

## Worked examples

### Example: typedef ported from NEMA TS-2

```yang
typedef phase-number {
  type uint8 {
    range "1..16";
  }
  description
    "NEMA-style phase number. Conforming controllers number ring-1
     phases 1-8 and ring-2 phases 9-16, but the type does not enforce
     ring assignment — controllers MAY use the full 1-16 space.";
  reference "NEMA TS-2 §6.3.2 (phase numbering).";
}
```

### Example: identity ported from Indiana enumeration

```yang
identity fault-vendor-alarm {
  base sc-fault-event-kind;
  description
    "Vendor-specific alarm. Vendor modules MAY derive more specific
     identities from this for their own vendor-alarm-N codes.";
  reference "ATSPM Indiana enumeration code 185 (vendor-specific alarm).";
}
```

### Example: choice case ported from NTCIP

```yang
case ntcip-oid {
  description
    "NTCIP MIB objects polled via SNMP. OID + stringified raw value.";
  reference
    "NTCIP 1202:2019 §4 (signal-control MIB);
     NTCIP 1203:2010 §4 (DMS MIB);
     NTCIP 1204:2017 §3 (ESS MIB).";

  leaf oid {
    type string;
    description "Dotted-numeric OID.";
  }
  leaf oid-value {
    type string;
    description "Stringified raw value the OID held at poll time.";
  }
}
```

### Example: revision-block with both citation forms

```yang
revision 2026-04-19 {
  description
    "(M9) Extract phase-number typedef and phase-timing grouping to
     openits-nema-common so ramp metering and signal control share
     one source of truth for the MUTCD-derived and engineering-floor
     phase-timing constraints.";
  reference
    "MUTCD 11th ed. 4F.17 (yellow/red clearance), 4E.06 (pedestrian
     intervals); ARC-IT 9.1 Service Package TI01; NTCIP 1202 v03;
     OpenITS PR #N: nema-common extraction.";
}
```

## Tooling

A future audit/lint script can verify:

- Every cited spec revision is current (compares against a maintained
  list of published revisions per spec family).
- Every `reference` string matches one of the canonical forms above.
- Every cite-worthy node (typedef, identity, etc. with a `description`
  citing a spec in prose) has a corresponding `reference` substatement.

The audit script doesn't exist yet; it's tracked follow-up work.

## Constraint placement (`must`) doctrine

`must` is a schema-validation gate: the datastore rejects an edit that
makes the constraint evaluate to `false`. That makes it the right tool
for **intent** — a request the model refuses to accept — and the wrong
tool for **observation**. A device's readback can be wrong (a stopped
meter, a conflicting signal head, a facility with no reported
direction); when a `must` sits on that readback, the collector cannot
represent the anomaly it exists to report, because the datastore
rejects the very data the anomaly consists of. This section states
where a `must` belongs and what it needs when it does.

### 1. `must` = intent → config-true only

Attach `must` to `config true` (RFC 7950 §7.21.1, the `config`
statement) trees only — configuration the model asks a controller to
honor or refuse. Never attach `must` to a `config false` subtree. A
misbehaving or faulted device's telemetry must stay representable so
it can be read back, diagnosed, and reported; a `config false` `must`
makes exactly the anomaly the fault surface exists to report into
invalid data instead.

```yang
/* Wrong: rejects the exact stopped-meter condition the fault
 * surface exists to report. */
container state {
  config false;
  leaf current-release-rate-vph {
    type release-rate-vph;
    must "not(derived-from-or-self(../mode, 'mode-active')) or . > 0" {
      error-message "A meter in active mode must release at a positive rate.";
    }
  }
}

/* Right: readback stays representable; the condition is documented
 * in prose and mapped to the fault surface that reports it. */
container state {
  config false;
  leaf current-release-rate-vph {
    type release-rate-vph;
    description
      "Release rate the meter is commanding right now (config-false
       readback). A meter reading 0 vph while mode=active is a real,
       reportable stopped-meter fault — emitted via
       openits-common-fault-events:fault-raised, not rejected here.";
  }
}
```

Where the config-false condition matters operationally, say so in the
leaf or container `description` and name the notification or fault
category that reports it (`fault-raised`, a specific
`*-conflict-detected` event, etc.) instead of encoding it as a `must`.

### 2. Presence requirements go on the parent, never the optional leaf

RFC 7950 §7.5.3 (the `must` statement) evaluates a `must` once per
node **in the accessible tree** — a `must` attached to an optional
leaf simply never runs when that leaf is absent, so `must ". >= 1"` on
an optional leaf enforces nothing about whether the leaf was supplied.
To require presence (optionally combined with a value constraint), put
the `must` on the **parent container**, referencing the child by name:

```yang
container timing {
  must "yellow-change" {
    error-message "Every signalized phase requires a yellow-change interval.";
  }
  leaf yellow-change { type decimal64 { fraction-digits 1; } }
}
```

RFC 7950 §7.5.4.3 shows this container-level pattern in its own
worked `must`/`error-message` example — it is the standard, not a
local workaround.

### 3. Don't restate a type's range as a `must`

If a leaf's type already constrains its value (`range`, `length`, an
enumeration), a `must` re-checking that same bound is dead code — it
can never fire independently of the type. `openits-types:percentage`
already ranges `0..100`; a leaf of that type does not also need
`must ". <= 100"`. Only add a `must` for a constraint the type cannot
express: a relationship between sibling nodes, a condition on an
ancestor, presence of another leaf, or a cross-list reference.

### 4. Every `must` needs an `error-message` and an `invalid-*` fixture

A `must` without RFC 7950 §7.5.4.1's `error-message` substatement
gives a field technician nothing to act on when a write is rejected;
always pair the two. A `must` without a corresponding
`yang/testdata/invalid-*.json` fixture proving it actually fires is
unverified — `make validate-yang` (via yanglint) is how this repo
proves a constraint binds, not a reading of the XPath expression.
Symmetrically: when a `must` is **removed** (per rule 1, most often),
delete the `invalid-*.json` fixture that exercised it — that data is
now legal — and add a `valid-*.json` fixture carrying the same
anomaly, so the datastore's newfound tolerance for it is itself
positively tested rather than merely assumed.

## Identity vs. enum: which axis gets which

A classification axis is either a YANG `identity` hierarchy or a closed
`enumeration`. Choose by extensibility, not by size:

- **Vendor-extensible axis ⇒ identity.** If a vendor or a future standard
  revision can legitimately add a new member without the core being wrong,
  model it as an identity so the addition needs no core revision (a vendor
  module derives from the base). Examples: `controller-mode`, `detector-type`,
  the fault/event-kind taxonomy, DMS/ESS/incident modes.
- **Closed physical set ⇒ enum.** If the members are a fixed, exhaustive
  physical reality that a vendor cannot extend, keep it an `enumeration`.
  Examples: `interval-type` (a signal head shows exactly these intervals),
  `lcs-indication`, `sensor-health`, `precipitation-type`,
  `pavement-condition`, `visibility-situation`.

When in doubt, ask whether a conforming vendor could ship a valid new member
tomorrow. If yes, it is an identity.

## Module placement: the ≥3-service rule

`openits-types` holds only primitives used by **three or more**
services. A typedef or identity used by fewer services belongs in the
capability module for the domain it serves, not in the shared types
module. This is why the V2X message-type, radio-fault, and
channel-fault identities and the `dsrc-channel` / `tx-power-dbm`
typedefs live in `openits-v2x-messaging` and `openits-v2x-radio`
rather than `openits-types`: V2X is a single domain, so its
domain-specific content belongs with its own capability modules.
Keeping the shared module scoped this way keeps its surface area
proportional to what is genuinely cross-cutting.

## Config/state idiom

Service models use an OpenConfig-style `config`/`state` split:
intended configuration lives in `config` containers (read-write);
operational data lives in `state` containers (`config false`). A
`state` container re-exposes the applied value of every `config` leaf
(the mirror) plus operational-only leaves.

### The trichotomy — pair only where both axes exist

Do not mechanically wrap every node in a `config`/`state` pair. A node
gets the treatment its data actually has:

- **Paired `config`/`state`** — a node with BOTH intended configuration
  and operational state (a commanded mode; a sensor with a configurable
  sample interval and live readings).
- **State-only** (`config false`, no paired `config`) — pure telemetry
  with no intended configuration (environmental observations, live fault
  inventories, per-component health). Do NOT create an empty `config {}`
  beside it.
- **Config-only** (config-true, no `state` mirror) — pure configuration
  templates with no per-entry operational state (a library of metering
  plans, a schedule table). Do NOT create an empty `state {}` beside it.

The test is per node: *does this thing have intended config? does it
have operational state?* Pair only when the answer to both is yes.

### Mechanics

- **Define config leaves once, `uses` them in both containers.** Put the
  config leaves in a grouping and `uses` it in the `config` container and
  again in the `state` container (where it is the applied mirror). Zero
  source duplication.
- **List keys are leafrefs into `../config/<key>`.** A keyed list under
  the idiom carries `leaf <key> { type leafref { path "../config/<key>";
  } }`; the real typed key leaf lives in the entry's `config` container.
- **No `must` on `config false`.** Constraints belong on config-true trees
  only (see the `must` doctrine above); a misbehaving device's telemetry
  must stay representable.

### Commands are config-writes

A device is commanded by writing its `config`, not by invoking an RPC or
action — the natural shape for an asynchronous pub/sub bus. The operator
writes the intended value to `config`; the device acts; `state` reports
the applied/actual value. **Whether a command took effect is read as
`config` vs `state` divergence** (`config/mode` = "active", `state/mode`
= "off" ⇒ not applied). There is deliberately **no `command-status`
field** in the model: the applied/not-applied signal is the divergence
itself, a device persistently not honoring a command is reported as a
fault, and why a specific write was refused is a runtime concern for the
device or collector, not schema surface.

Group a commanded surface in a dedicated functional container (e.g.
`control`) with its own `config`/`state`, kept separate from the
set-once identity `config`/`state`. Read-only services have no such
container — its presence signals that a device is commandable.

Fire-and-forget operational device ops with no persistent desired state —
reboot, certificate refresh/rotate, run-self-test — do **not** belong in the
data model. They are operational RPCs (the gNOI plane: `System.Reboot`,
Certificate Management), modeled outside this repo, not as YANG `action`s. The
data model carries only desired state the device reconciles to (config) and
observed state (config false). If a "command" has persistent desired state — an
operator decision the device honors until some condition clears (e.g. an SRM
approve/deny) — it is a config-write, not an operational RPC.

### Generated proto naming

Each service's YANG generates into its own proto package
(`openits.<service>.v1`) and its own Go package, so common message names
stay clean and independent across services (every service may have its
own `Detector`, `Lane`, `Diagnostics`). Within a service, `config`/`state`
containers are parent-qualified (`ControlConfig`, `PhaseState`); other
nested list/container names take their bare name unless two collide
inside the same service package, in which case both are parent-qualified
(`PavementSensor` / `DiagnosticsSensor`).

### Geography: `geo-location` vs `geo-point`

`openits-types` splits geography into two groupings: `geo-point`
(latitude/longitude only) and `geo-location` (`uses geo-point` plus
optional elevation and heading). Choosing between them is a physical
question, not a stylistic one:

- **`geo-location`** — anything with a physical position, including
  every device identity. A device sits somewhere with a real elevation
  and (often) a mounting heading, even when a given deployment leaves
  those leaves unset; `openits-types:device-identity-config` — the
  grouping every service's identity `config`/`state` uses — carries
  `geo-location` for exactly this reason.
- **`geo-point`** — only where a location is genuinely 2D-only: a
  polygon vertex, or another observation coordinate that has no
  elevation/heading of its own to carry.

Do not use `geo-point` for a device identity as a way to save two
optional leaves — the identity is still a physical thing sitting at an
elevation, even if that value is usually absent on the wire.

## Module prefixes

Module prefixes are an internal, non-wire-visible convention. The suite
uses a mix of service short-codes (`openits-sc`, `openits-rm`,
`openits-ts`, `openits-rl`, `openits-pn`) and full names
(`openits-dms-types`, `openits-cctv-types`, ...). This is deliberately
**not** normalized before v1: the prefix appears in every `import` and
every qualified reference, so a rename is a large repo-wide edit with no
external (wire) benefit — RFC 7951 identityref values are keyed by module
*name*, not prefix. Revisit at v1.

## Related documents

- [Extension model](../06-extension-model.md) — Core / Augments /
  Deviations / Proprietary tiers; revision discipline; graduation rule.
- [Standards alignment](../05-standards-alignment.md) — Project-wide
  view of which standards OpenITS aligns with.
- [The data model](../data-model.md) — the module family and taxonomy
  these conventions document.
