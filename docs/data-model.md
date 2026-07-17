# The data model

This document describes the OpenITS data-model layer for implementers:
how the YANG module family is organised, how the event taxonomy works,
how wire provenance and safety constraints are carried in the schema,
and how the schema maps onto encodings and transports. Read the
system architecture doc (in the source monorepo) first for how events
move; this document covers what the events *are*.

[04 — Design decisions](04-design-decisions.md) explains *why* the
model is shaped this way; this document explains the shape itself and
shows the machinery working. The two recurring sources of
lessons-learned are **NTCIP** (whose data model was bound to SNMP as a
transport, so replacing the transport meant replacing the model) and
**OpenConfig** (a decade of vendor-neutral network modeling whose
telemetry side succeeded and whose one-size-fits-all configuration
side fractured). OpenITS borrows what worked, avoids what didn't, and
diverges where ITS is genuinely different — each divergence is called
out below.

## The module family

The YANG tree is many small modules with a clean import graph, not one
mega-module per device. This is a deliberate OpenConfig lesson:
OpenConfig converged on ~30 focused modules (`openconfig-bgp`,
`openconfig-interfaces`, …) after the mega-module pattern failed.

| Layer | Modules | Role |
|-------|---------|------|
| Foundation | `openits-types` | Cross-service typedefs, the `device-event-kind` identity root, the `wire-source` provenance grouping, the `arc-it-flow` extension. |
| Shared domain | `openits-nema-common` | NEMA-adjacent types shared by signal-control and ramp-metering: `phase-number`, `phase-timing` with MUTCD-derived and engineering-floor `must`-constraints. |
| Service core | `openits-signal-control`, `openits-dms`, `openits-ess`, `openits-rsu`, `openits-ramp-metering`, `openits-perception`, `openits-traffic-sensor`, `openits-reversible-lane` | Per-service state tree: identity, configuration, operational status, faults. |
| Service types | `openits-{signal-control,dms,ess,rsu,ramp-metering,perception,traffic-sensor,reversible-lane}-types` | Per-service event-kind identity hierarchies (plus, for signal-control, shared typedefs), importable without pulling in the service core. |
| Event modules | `openits-signal-control-{phase,detector,overlap,pedestrian,preemption,coordination,tsp,tsam,raw}-events` | One module per behavioral concern, each ~50–180 lines, each with its own revision cycle. |
| Cross-service events | `openits-common-{fault,mode,comm-health}-events` | Generic notifications (fault-raised / fault-cleared / mode-changed / …) that every service emits with a service-derived `kind` identity. The generation-1 per-service fault/mode notifications are deprecated in favor of these. |
| Vendor identity modules | `openits-vendor-econolite-signal-control-types` | Vendor-derived identities filling open extension slots. See [06 — Extension model](06-extension-model.md). |
| Augments | `yang/augments/*` | Net-new nodes in contributor namespaces. |

Two structural rules keep the family healthy:

- **Notifications live in companion modules**, split from the service
  core (a ygot proto-backend constraint that turned out to be good
  hygiene anyway — the state tree and the event surface version
  independently).
- **Per-concern event modules stay small.** Adding a TSP event never
  forces a revision of the phase-event module. This is the opposite of
  NTCIP's single-MIB-per-device shape, where every change rides one
  monolithic document.

## The event taxonomy

Every event carries a `kind` leaf: an `identityref` into a hierarchy
rooted at `openits-types:device-event-kind`. Two shapes of sub-identity
coexist below the root, depending on whether the behavioral class is
genuinely cross-service:

- **Cross-service classes** (`fault-event-kind`, `mode-event-kind`,
  `comm-health-event-kind`) are **dual-classified**: a service's
  sub-identity derives from *both* the behavioral class and its own
  service root, so it's filterable at either altitude.
- **Service-only classes** (signal-control's phase, detector, overlap,
  pedestrian, preemption, coordination, TSP, TSAM, and external-I/O
  concerns) have no cross-service peer to dual-base on — "every phase
  event, any service" was never a real query, since only signal-control
  has phases — so they derive solely from their service root.

```
device-event-kind                          (root, openits-types)
├── fault-event-kind                       (cross-service behavioral class)
├── mode-event-kind
├── comm-health-event-kind
└── signal-control-event-kind              (service root, sc-types)
      ├── sc-fault-event-kind              (dual-classified)
      ├── sc-mode-event-kind               (dual-classified)
      ├── sc-phase-event-kind              (signal-control-only)
      ├── sc-detector-event-kind           (signal-control-only)
      │     ...

sc-fault-event-kind                        (service × behavior, dual-classified)
  base openits-types:fault-event-kind;     ← behavioral parent
  base signal-control-event-kind;          ← service parent

sc-phase-event-kind                        (signal-control-only, single-classified)
  base signal-control-event-kind;          ← service parent only, no
                                              cross-service behavioral peer

phase-gap-out                              (leaf identity)
  base sc-phase-event-kind;                "Indiana 4: phase terminated by gap-out."
```

Because YANG identities support multiple bases, `sc-fault-event-kind`
derives from *both* the behavioral class and the service root. A
consumer can therefore filter at any altitude with
`derived-from-or-self()` — no string matching, no code tables:

| Question | Filter |
|----------|--------|
| Every fault, fleet-wide, any service | `derived-from-or-self(kind, 'openits-types:fault-event-kind')` |
| Every signal-control event, any behavior | `derived-from-or-self(kind, 'openits-sc-types:signal-control-event-kind')` |
| One vendor's alarm slot | `derived-from-or-self(kind, 'openits-vendor-econolite-sc-types:econolite-vendor-alarm-3')` |

This is the extensible-taxonomy machinery NTCIP's flat OID space never
had: an OID identifies one object on one device family, and grouping
OIDs into "all faults" is consumer-side tribal knowledge. Here the
classification is in the schema, and adding a new identity is an
additive, non-breaking change (RFC 7950 §11).

The fault taxonomy is connected cross-service, not aspirational:
every service family — signal-control, DMS, ESS, RSU, ramp-metering,
perception, traffic-sensor, and reversible-lane — derives its fault
identities from `fault-event-kind`, so the fleet-wide filter in the
table above genuinely spans the fleet.

It also avoids OpenConfig's lowest-common-denominator trap. OpenConfig
tried to make divergent vendor implementations look like one model and
accumulated deviations until the unified model fractured. OpenITS
never forces services through a universal envelope: each service
derives its *own* sub-bases, and the cross-service queries fall out of
the identity graph rather than from shared message shape. (The same
reasoning rejected a universal "fact envelope" — see the notification
taxonomy section of [04 — Design decisions](04-design-decisions.md).)

**Identities, not enums, wherever vendors plausibly extend.** An enum
is closed; an identity hierarchy is open. Event kinds, fault classes,
and vendor alarm slots are identities. Physically fixed sets
(severity levels, GPS fix modes, precipitation types) stay enums.

## Vendor extension is a worked example, not prose

Most standards describe their extension story; OpenITS ships one
running end-to-end:

- `openits-signal-control-types` leaves an open slot:
  `fault-vendor-alarm` ("vendor modules MAY derive more specific
  identities from this").
- `openits-vendor-econolite-signal-control-types` derives eight
  alarm-slot identities from that slot — and the ASC3 decoder rewrites
  the generic identity to the specific slot identity when the wire
  param byte names a known slot.
- A consumer without the vendor module loaded sees the base identity
  and loses nothing; a consumer with it sees the specific slot in the
  same `identityref` field.
- `yang/augments/example-signal-control-vehicle-counts.yang` is the
  in-tree pedagogical augment for the add-new-nodes case, and
  `tools/check-augment-collisions` polices path collisions between
  augments.

The governance side of this — namespaces, NoIs, the graduation rule —
is [06 — Extension model](06-extension-model.md).

## Wire provenance is structural

Every event decoded from a wire encoding carries the
`openits-types:wire-source` grouping in its payload: a decoder-family
name plus a typed, per-family identifier —

- `indiana` — ATSPM Indiana-enumeration HR event code + param,
- `ntcip-oid` — the polled OID and its raw value,
- `j2735` — the SAE J2735 messageId.

New decoder families add new `choice` cases additively. And when a
decoder encounters a wire row it *cannot* type, it emits the
`unmapped-event` notification (`openits-signal-control-events`)
carrying the same wire-source — nothing decoded off the wire is ever
silently dropped, and an unmapped code can be interpreted offline once
a typed mapping lands.

This is the deliberate inversion of NTCIP's failure mode. NTCIP made
the OID *be* the model; OpenITS makes the OID **payload metadata** on
a transport-neutral model. Every typed event can be traced back to the
exact OID, HR code, or J2735 message it came from — in the payload,
identically visible over any transport, not in tribal decoder
knowledge.

## Safety constraints live in the schema, once

The phase-timing safety limits are YANG `must` expressions in
`openits-nema-common:phase-timing`, with citation-grade error messages.
Yellow change is the one MUTCD-mandated bound — a 3.0-6.0 second range
(*"Yellow change must be 3.0-6.0 seconds per MUTCD 11th ed. section
4F.17 paragraph 13"*). Red-clear is a ceiling only: MUTCD 11th ed.
§4F.17 paragraph 6 sets no minimum, so the schema caps it at 6.0
seconds and leaves the value itself to engineering practice. Min-green
is not a MUTCD value at all — it is an engineering floor (`>= 1`
second) that only forbids a zero-length serve. Both signal-control and
ramp-metering `uses` the same grouping, so none of these limits can
drift between services.

This is the concrete payoff of choosing a schema language with a
constraint vocabulary: any validator that loads the YANG enforces the
rules at any boundary — poller, central ingest, conformance kit, a
future config-push path — without each one re-implementing MUTCD. In
a Protobuf-only world these rules would be hand-coded per validator;
here they are declared once next to the data they constrain.

## Config and state

State is marked `config false` in dedicated `state` containers;
configuration is the default tree. OpenITS does **not** mirror every
config leaf under a duplicated `state` subtree the way OpenConfig
does. That OpenConfig convention exists to expose *intended vs
applied* configuration over transports that predate NMDA (RFC 8342);
OpenITS is telemetry-first over an event transport, where the
distinction is carried by events (a config change is observable as an
event) rather than by tree duplication. If a future deployment needs
NMDA-style datastores over NETCONF/gNMI, the models work unchanged —
the convention is a packaging choice, not a capability limit.

## Encodings and transports: the schema is the contract

**The YANG models are the contract. Everything else is a binding.**

The reference binding — what the in-tree implementation ships — is:

- **Encoding:** per-event Protobuf, generated from the YANG via ygot.
- **Envelope:** CloudEvents 1.0 binary mode.
- **Transport:** NATS JetStream, with the subject hierarchy described
  in the system architecture doc (in the source monorepo).

None of these is the only valid choice, and the model layer is
deliberately ignorant of all three:

| Layer | Reference binding | Equally valid bindings |
|-------|-------------------|------------------------|
| Encoding | Protobuf (ygot-generated) | JSON (RFC 7951), XML, CBOR |
| Envelope | CloudEvents binary mode | CloudEvents structured mode, NETCONF notification framing |
| Transport | NATS JetStream | gNMI subscribe, NETCONF, RESTCONF, plain HTTP webhooks, Kafka |

A vendor that already ships YANG-modelled state over gNMI can be
bridged without touching the schema; an agency that wants a plain
HTTPS feed for a partner can serve RFC 7951 JSON validated against
the same schema-registry snapshot. What any binding must preserve:

1. **Schema identity** — a resolvable reference to the immutable
   schema-registry revision (the reference binding uses
   `ce-dataschema`).
2. **Event identity** — the deterministic content-derived id that
   makes retries idempotent (`ce-id` in the reference binding).
3. **Provenance** — `wire-source` rides in the payload precisely so it
   survives any transport unchanged.

### The same data, three ways

A quick proof rather than an assertion. First, a `fault-raised`
notification (module `openits-common-fault-events`) in the reference
binding — the full envelope is in the system architecture doc's
wire-format section (in the source monorepo):

```
NATS subject:   openits.us-tx.txdot.d07.signal-control.i35-exit-214.fault-raised
ce-type:        openits.signal-control.fault-raised.v1
ce-dataschema:  https://schemas.openits.example.org/openits-common-fault-events/<revision>/
NATS body:      [binary Protobuf — FaultRaised]
```

The same event served as **RFC 7951 JSON** — from a RESTCONF event
stream (RFC 8040 §6) or a plain HTTPS webhook; only the framing
differs:

```json
{
  "ietf-restconf:notification": {
    "eventTime": "2026-04-28T14:32:10.123Z",
    "openits-common-fault-events:fault-raised": {
      "kind": "openits-signal-control-types:fault-power-failure",
      "source-device-id": "i35-exit-214",
      "fault-id": "pwr-0182",
      "severity": "major",
      "occurred-at": "2026-04-28T14:32:09.871Z",
      "source": { "decoder": "indiana", "indiana-code": 182 }
    }
  }
}
```

And the *state tree* of the same service over **gNMI** — a vendor or
bridge that implements the model can serve it to any standard gNMI
client with no OpenITS-specific code:

```sh
gnmic -a controller.example:9339 subscribe \
  --path "openits-signal-control:/signal-controller/state" \
  --mode stream --stream-mode on-change
# → updates arrive as the same RFC 7951 JSON, e.g.
#   { "mode": "flash", "flash-active": true,
#     "flash-cause": "flash-mmu-conflict", ... }
```

Note what stayed identical across all three: the YANG-defined field
names and types, the identityref `kind` (still filterable with
`derived-from-or-self()`), and the `wire-source` provenance — it
lives in the payload, so it survives every transport unchanged. An
XML rendering (NETCONF notifications, RFC 7950 §7.16.3) follows
mechanically from the same schema and is omitted because it would
demonstrate nothing new. None of the alternate bindings required a
schema change; each is an integration-layer choice an operator or
vendor makes per deployment.

This separation is the central NTCIP lesson, restated: NTCIP bound its
data model to SNMP, so the model aged with the transport. OpenITS
picks a schema language with no transport opinion, then makes
transport choices per deployment. See also
[05 — Standards alignment](05-standards-alignment.md) ("YANG — schema
language, not transport").

The pub/sub *interface* that documents all of this for a consumer —
one channel/operation/message per ce-type — is `asyncapi.yaml` at the
repository root, and it is generated in-repo (`make asyncapi`), not
hand-maintained: the ce-type catalog is derived from the same
YANG event-kind taxonomy as the reference binding above, and each
message's payload is that notification's actual JSON Schema, embedded
directly rather than a URL pointer to the schema registry (see
[04 — Design decisions](04-design-decisions.md#generated-asyncapi-not-hand-maintained)).
A consumer reading `asyncapi.yaml` sees the same shape the `fault-raised`
example above shows in JSON — there is no second, hand-copied
description of the payload to drift out of sync with the schema.

## Lessons applied — a summary

| Lesson | Source | Where it lives in OpenITS |
|--------|--------|---------------------------|
| Don't bind the model to a transport | NTCIP | YANG contract + per-deployment bindings; `wire-source` keeps OIDs as payload metadata |
| Small per-concern modules, never mega-modules | OpenConfig | Nine signal-control event modules; companion notification modules |
| Model telemetry confidently; beware unified config | OpenConfig | Telemetry-first trees; no universal event envelope; per-service identity sub-bases |
| Taxonomies must be vendor-extensible | OpenConfig (augment-as-escape-valve) | Identity hierarchies with open slots; the Econolite worked example |
| Constraints belong in the schema | NTCIP (absence thereof) | MUTCD `must` rules in `openits-nema-common`, shared by two services |
| Evolve additively; version immutably | Both | RFC 7950 §11 discipline; dated revisions; immutable schema registry; `ce-type` major versions |

And the deliberate divergences from OpenConfig, for readers steeped in
its conventions:

- **Identity naming is kebab-case** (per RFC 8407), not OpenConfig's
  `UPPER_SNAKE`. Every identity in the tree is kebab-case today —
  there is no legacy carve-out.
- **No mirrored config/state subtrees**, as described above.
- **A semver stamp, borrowed from `openconfig-version` practice, not
  omitted.** The `openits-types:openits-version` extension (argument
  `semver`) lets a module declare a MAJOR.MINOR.PATCH content
  version independent of its YANG revision date — MAJOR bumps track
  the same breaking-change events that bump `ce-type`. It coexists
  with, rather than replaces, the other two versioning signals:
  dated revisions order changes chronologically, `ce-type` majors
  gate the wire, and `openits-version` gives tooling an explicit
  compatibility-intent signal neither of the other two carries on
  its own. Not yet stamped on every module — the newer service
  families (perception, traffic-sensor, reversible-lane) carry it;
  backfilling the rest is open work.

## Related documents

- **Architecture** (in the source monorepo) — how events move.
- [04 — Design decisions](04-design-decisions.md) — why these choices.
- [05 — Standards alignment](05-standards-alignment.md) — NTCIP / J2735 / ARC-IT mapping.
- [06 — Extension model](06-extension-model.md) — augments, deviations, graduation.
- [08 — Capability architecture](08-capability-architecture.md) — model by function; thin device profiles compose capability modules.
- [YANG reference-citation conventions](reference/yang-reference-conventions.md) — how modules cite normative sources.
