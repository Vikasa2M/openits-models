# 04 — Design decisions

This document explains the major architectural choices and the
reasoning behind each. The format is consistent: **what we chose,
what we considered, why we chose it, what we'd revisit if we got
this wrong**.

These choices were made in the open and are open to revision; nothing
here is treated as untouchable. Bring evidence and a working group;
the TSC will hear arguments.

## YANG as the data-model contract

**What we chose.** Every service category is described by a YANG
module that defines its shape, constraints, and notifications. The
YANG models are the single source of truth from which Go structs
(via ygot), Protobuf messages, and AsyncAPI are derived.

**What we considered.** Avro, JSON Schema, Cap'n Proto schemas,
hand-rolled Protobuf-only. Each was rejected for a specific reason:

- **Avro** — strong schema evolution semantics but no widespread
  tooling for the transport-layer integrations we needed (NATS,
  CloudEvents). Tooling gap is real-cost.
- **JSON Schema** — universal but lacks the formal constraint
  language YANG ships with (`must` expressions for cross-leaf rules,
  identities for taxonomies, the `augment` and `deviation` machinery
  the extension model depends on).
- **Cap'n Proto** — clean, fast, but schema language is less
  expressive than YANG and ecosystem alignment with networking
  industry tooling is weaker.
- **Protobuf-only** — would have worked at the wire layer but
  forces every constraint to live in code. The phase-timing rules —
  MUTCD's yellow-change mandate (3–6 s, §4F.17, the only one of the
  three with a genuine MUTCD basis), the engineering-determined
  red-clear ceiling (≤6 s; MUTCD sets no minimum), and the
  engineering-floor minimum green (not a MUTCD value at all) —
  would have to be hand-implemented per validator instead of
  declared once in the YANG.

**Why YANG.** Three reasons stand out:

1. **Constraint expressiveness.** YANG `must` and `when`
   statements let safety-critical rules live next to the data
   shape. A consumer that loads the YANG gets the rules; a
   validator runs them at every boundary.

2. **Transport-independence.** YANG is a schema language; the
   transport that carries YANG-modelled data is a separate
   decision. NTCIP's hard lesson — locking the data model to a
   specific transport (SNMP) — makes any data model that picks
   YANG-the-schema while leaving the transport open more
   resilient. OpenITS uses NATS today; a future deployment that
   wants gNMI, NETCONF, plain HTTP, or something not yet
   invented can adapt the same YANG models without touching the
   schema.

3. **Augment / deviation machinery.** YANG 1.1's `augment` and
   `deviation` statements are exactly the shape the extension
   model needs. Vendors add `augment` modules in their own
   namespace; jurisdictions tighten constraints with `deviation`
   modules; the core stays unmodified. No other schema language
   has this baked in.

**What we'd revisit.** If ygot's tooling becomes a maintenance
burden and YANG's expressiveness advantages over Protobuf don't
earn their keep in practice (the constraint language goes
unused; the augment/deviation machinery doesn't see external
adoption), a future major-version cycle could move to
Protobuf-only schemas with constraint rules expressed as
separate validator code. The wire format wouldn't change; only
the source-of-truth location would. Transport choices stay
independent of this — they're a separable decision from the
schema language.

## CloudEvents as the envelope

**What we chose.** Every NATS message is a CloudEvents 1.0
envelope in binary mode. The CE attributes ride in NATS message
headers; the body is binary Protobuf.

**What we considered.** Custom envelope (NATS headers with our
own attribute names), structured-mode CloudEvents (JSON envelope
with embedded payload), no envelope at all (subject + raw bytes).

**Why CloudEvents.**

1. **Vocabulary already exists.** `ce-type`, `ce-source`,
   `ce-dataschema`, `ce-id`, `ce-time` are well-defined. We don't
   re-invent the metadata-versioning conversation; we inherit
   the CNCF working group's answers.

2. **Vendor portability.** Tools that already understand
   CloudEvents (Knative, Argo Events, OPA, the OpenTelemetry
   exporter family) handle our messages without OpenITS-specific
   adapters.

3. **Binary mode keeps wire size small.** Structured mode would
   wrap the Protobuf payload in a JSON envelope — a few hundred
   bytes per message. At 10k cabinets × ~5 events/sec × 86,400
   seconds, that's a real cost. Binary mode pays nothing; the CE
   attributes are NATS headers and would be sent anyway.

4. **Deterministic `ce-id`.** Computing the id as a ULID derived
   from a content hash of the event —
   `SHA-256(ce-source ‖ ce-type ‖ stable-time ‖ payload)`, see
   `docs/ce-id-spec.md` — makes retries idempotent at the
   storage layer (ClickHouse `ReplacingMergeTree(ce_id, ce_time)`
   deduplicates for free). Most CloudEvents deployments use random
   UUIDs and bolt deduplication on later; we get it as a side effect
   of the id policy. (An earlier `controller-id + sequence` scheme
   was replaced because it required a durable counter; the content
   hash is restart-invariant.)

**What we'd revisit.** If a future major version of CloudEvents
breaks binary mode or introduces requirements that make NATS
header-based encoding awkward, the envelope policy is changeable.
The envelope is one layer; the rest of the design holds.

## Per-event Protobuf, not bundled telemetry

**What we chose.** Every transition is its own typed Protobuf
message: `FaultRaised`, `ModeChanged`, `OperationalStatus`, etc.
Each has its own ce-type, its own NATS subject, its own schema
revision.

**What we considered.** The original design had a bundled
`DeviceTelemetry` message with a oneof for the per-device type
(ASCData / RSUData / DMSData) and another oneof inside that for
events. One stream of homogeneous-shape messages.

**Why per-event.** Three problems with the bundled approach
became obvious in the first months of integration:

1. **Storage shape.** ClickHouse partitions cleanly when the
   table schema matches the message shape. With a bundled
   message, the table is a wide soup of nullable columns —
   slow queries and ugly schema migration on any change.

2. **Subscription granularity.** Consumers that only care about
   faults shouldn't have to subscribe to a firehose of operational
   status updates and filter at parse time. Per-event messages let
   subscribers narrow with NATS subject filters before parsing
   anything.

3. **Schema evolution.** Adding a field to one event family
   shouldn't force a re-snapshot of every other event family's
   schema. Per-event messages decouple revisions.

The cost is a longer ce-type catalog (62 ce-types as of writing)
and a wider Protobuf source tree. That cost is borne by the
project; consumers see a cleaner subscription model.

One scoping note: per-event *Protobuf* is the reference encoding,
not part of the standard's identity. The contract is the YANG model;
Protobuf-over-NATS is one binding of it. A deployment that serves
the same events as RFC 7951 JSON over HTTPS, or streams them over
gNMI, is carrying the same standard — what must be preserved across
bindings (schema reference, deterministic event id, wire provenance)
is spelled out in [the data model](data-model.md).

**What we'd revisit.** If a use case appears that demands
multi-event atomic publishes (e.g., a transactional state-change
spanning multiple devices), we'd add a *correlation* mechanism via
extension headers rather than bundling. Bundling is the wrong
answer to that problem; the bundled-telemetry experiment proved it.

## NATS as the transport

**What we chose.** NATS JetStream for durable streams, NATS KV for
live state, NATS leaf nodes at the edge with the central cluster as
the hub.

**What we considered.** Kafka, RabbitMQ, MQTT, gRPC streaming,
plain HTTP webhooks.

**Why NATS.**

1. **Leaf nodes are unique to NATS.** A Kafka deployment requires
   either a broker per cabinet (operationally hard) or a relay
   model that's not native (operationally fragile). NATS leaf
   nodes are first-class: each cabinet runs a tiny NATS instance
   that proxies to the central cluster, with subject mapping for
   address-space isolation. This is exactly the shape ITS needs.

2. **Subject hierarchy is native.** Kafka has topics, not
   hierarchies. The seven-token OpenITS subject would map to
   29 of the wildcards Kafka doesn't have. NATS' wildcard
   filtering — `>` for "rest of subject", `*` for "one token" —
   is exactly the dispatch tool consumers want.

3. **JetStream is sufficient at our scale.** A Kafka deployment
   for ITS-scale fleet (10k+ leafs, single-digit-thousand
   events/sec sustained) is justifiable but operationally heavier
   than JetStream's three-node-cluster footprint. The break-even
   for Kafka's complexity is well above where ITS deployments
   typically land.

4. **The auth model fits.** NATS' NKey/JWT scheme with
   account-level isolation maps cleanly to multi-agency
   deployments. Each agency gets its own account; subject
   permissions enforce isolation; cross-agency reads are explicit
   exports rather than ambient namespace overlap.

   *Reference-deployment status.* Multi-agency account isolation
   is a design property of NATS that the demo deployment does not
   exercise — the 4-tier lab uses a single `$G` account end-to-end.
   The shipped `cmd/keygen` produces a single-agency account; the
   dev compose stack runs a flat namespace; per-agency accounts,
   scoped JWT permissions, and cross-agency export syntax are
   future deployment work. The wire contract is stable; only the
   deployment topology becomes more partitioned.

**What we'd revisit.** If a future deployment scenario demands
features NATS doesn't have well-tested support for (regional
replication with conflict resolution, very high-throughput single
streams, extreme retention windows), the storage layer is
pluggable. The wire format is independent of the transport. A
Kafka backend could be added as a parallel deployment without
touching the wire contract.

## Greenfield, not bridged

**What we chose.** No backwards-compatibility commitment to any
prior pre-OpenITS data format. The standard ships clean.

**What we considered.** Carrying along TMDD compatibility,
existing TMC vendor formats, or the ad-hoc per-agency JSON shapes
that some agencies have already deployed.

**Why greenfield.**

1. **The compatibility cost dominates the design.** Every
   compatibility commitment forces a constraint on the new
   format. Multiply by the number of legacy formats and the
   resulting design is the union of all of them — i.e., the
   problem we're trying to fix.

2. **Bridging at the consumer side is cheap.** An operator that
   wants to expose OpenITS *and* a legacy format runs a small
   bridge process. The bridge is a few hundred lines of code per
   format, owned by the operator who needs it.

3. **The migration story for new deployments is irrelevant.**
   Operators standing up new central infrastructure don't have a
   legacy format to be compatible with. They have a free choice;
   greenfield-OpenITS is a coherent answer to that choice.

**What we'd revisit.** Nothing. This decision is foundational; if
we got it wrong, the project is wrong from the ground up. The
willingness to ship clean-slate is part of the project's value
proposition.

## Operator-weighted governance

**What we chose.** The Technical Steering Committee has five
operator seats, two vendor seats, one integrator seat, and one
community-at-large seat. YANG-changing decisions require operator
majority approval; other decisions require simple majority.

**What we considered.** Equal-weight voting, vendor-weighted
governance (faster for shipping new content), academic
governance (a federal advisory committee).

**Why operator-weighted.**

1. **Operators bear the deployment cost.** A standard that's
   easy for vendors to ship but hard for agencies to operate
   transfers cost from vendors to operators. Operator weight
   forces vendors to design for deployment, not just for
   capability.

2. **Operators are downstream of vendors.** A standard ratified
   by vendors alone tends to encode vendor product roadmaps,
   not operator needs. Operator weight inverts the dynamic.

3. **Vendor influence remains substantial.** Two vendor seats on
   a nine-seat TSC is a real voice; vendors can write augments,
   propose graduations, and engage operator seats with
   well-grounded arguments. They cannot ratify against operators.

**What we'd revisit.** If the operator seats can't be filled
(fewer than three agencies engaging with the TSC), the project
isn't viable as a standard and should be honest about being a
reference implementation only. The governance shape is a forcing
function: if operators don't want to govern, the standard isn't
real.

## Augments before fixes, deviations before forks

**What we chose.** A four-tier extension model:

1. **Core** — TSC-controlled YANG modules. Stable, with 2-year
   deprecation windows.
2. **Augments** — vendor or agency additions in their own
   namespace. Add nodes; never modify existing ones.
3. **Deviations** — TSC-reviewed constraint refinements. Tighten
   rules for specific jurisdictions.
4. **Proprietary** — vendor-internal modules outside the
   `openits.>` subject tree.

A graduation rule: when three independent organisations adopt
the same augment (with at least one operator), the augment
becomes eligible to fold into core.

**What we considered.** A two-tier model (core + proprietary), a
permissive model where any vendor change goes into core, and a
strict model where nothing extends except via TSC vote.

**Why four tiers.**

1. **Two-tier doesn't graduate.** Vendors ship in their
   proprietary namespace forever; the standard never grows
   organically.

2. **Permissive is a soup.** Every vendor's idea ends up in
   core; consumers get a wide surface most don't care about and
   the schema becomes hard to maintain.

3. **Strict-only is too slow.** TSC review of every new field
   would mean shipping cycles measured in quarters. Vendors
   don't wait; they fork. The standard fragments.

The four-tier model with a graduation rule is the
goldilocks solution: vendors ship fast in their own namespace,
operators see a stable core, and a documented path moves
proven content into the core when the market signals consensus.

**What we'd revisit.** The "three independent organisations,
one of which is an operator" rule is a chosen number with
limited evidence. If it's too restrictive (augments stagnate)
or too permissive (low-quality content graduates), the rule is
adjustable by TSC vote. The shape (multi-organisation, operator
present) is the durable part.

## JetStream for events + live state, ClickHouse for OLAP

**What we chose.** JetStream is the durable record of every event
and — via NATS KV, which is a thin last-per-subject layer over a
JetStream stream — the live-state backend. ClickHouse is added on
top for analytical / OLAP queries that don't fit JetStream's
stream model.

**What we considered.** A single backend (ClickHouse only,
serving both "latest state" and "historical analytics");
TimescaleDB instead of ClickHouse; PostgreSQL for everything; or
JetStream-only with no SQL store at all.

**Why this layering.**

1. **NATS KV is JetStream.** It's a JetStream stream with
   last-per-subject semantics and a friendly key/value API. When
   OpenITS writes "latest snapshot per device" to NATS KV, it's
   writing to a JetStream stream — same durability, same replay,
   same fault-tolerance as the event log. The KV is an
   ergonomics layer, not a separate system. Calling these
   "two backends" obscures that.

2. **JetStream covers the live-state and replay use cases.**
   Most of what an OpenITS consumer wants — "give me the latest
   state of controller X," "replay the last hour of faults" —
   maps directly to JetStream primitives (last-per-subject,
   sequence-number replay). For deployments that only need those
   access patterns, JetStream + NATS KV is sufficient and the
   third backend is unnecessary.

3. **OLAP queries don't fit streams.** "For each district, count
   fault-raised events by category over the last 30 days" is a
   columnar aggregation. Doing it via JetStream replay means
   streaming millions of events to a consumer and aggregating in
   application code; doing it in ClickHouse pushes the
   aggregation into the storage engine where it belongs.
   ClickHouse's `ReplacingMergeTree(ce_id, ce_time)` gives free
   deduplication on retried publishes as a side benefit.

4. **Valkey is the live-state escape hatch, not a parallel
   backend.** NATS KV's `List(prefix)` becomes a full-keyspace
   scan past ~10k keys. Valkey (BSD-licensed Redis-compatible)
   provides cursor-based prefix scans that scale to 100k+ keys.
   The switch is a config flip; both implement the same `Store`
   interface in the reference codebase.

**What we'd revisit.** A small-scale deployment (single agency,
single district, < 100 cabinets) can reasonably skip ClickHouse.
JetStream + NATS KV covers the live-state and replay use cases;
operators who already have an analytics platform (their existing
data lake, BigQuery, Snowflake) can export to it via a separate
sink rather than running ClickHouse adjacent to OpenITS. The
reference deployment includes ClickHouse because it's the
shortest path to dashboards for new POCs; production deployments
should choose deliberately.

## Imutable schema registry, no `latest` alias

**What we chose.** Every YANG revision lives at
`schema-registry/<module>/<YYYY-MM-DD>/`. Once published, the
directory's contents never change. Consumers cite the exact
revision in `ce-dataschema`. There is no `latest` symlink.

**What we considered.** A `latest` alias for convenience. A
mutable registry where bug fixes get applied in-place. A
revision-numbering scheme (v1, v2, v3) instead of dates.

**Why immutable, dated, no latest.**

1. **Reproducibility.** A research paper or a production
   deployment that pinned `2026-04-19` in 2026 must still
   resolve to the same content in 2032. Mutable registries break
   this; `latest` aliases break it silently.

2. **Bug fixes mean new revisions.** If a constraint is wrong,
   we ship `2026-05-01` with the fix, deprecate `2026-04-19`,
   and consumers migrate. Migration is explicit and dated.
   No silent semantic drift.

3. **Date-stamped revisions are self-explaining.** A consumer
   reading the URL knows when the schema was published. A
   numbering scheme requires a separate lookup table to map
   numbers to dates.

**What we'd revisit.** Nothing. This is one of the most
copied-from-prior-art decisions in the project; OpenAPI's
versioning patterns and the IETF RFC publication model both
point in the same direction. Reproducibility is non-negotiable
for an industry standard.

## Generated AsyncAPI, not hand-maintained

**What we chose.** The `asyncapi.yaml` at the repository root is
generated in-repo by `make asyncapi`
(`tools/yang-proto-gen -asyncapi`): the ce-type catalog is derived
from the YANG event-kind taxonomy (`BuildCatalog` — the identity
graph plus each service's notifications, not a hand-maintained list
imported from the collector), and each ce-type's message payload is
its notification's JSON Schema (`EmitJSONSchema`, the P2b-1 backend),
embedded directly rather than referenced by URL
(`schemaFormat: application/schema+json;version=draft-2020-12`).
`make asyncapi-check` (`make asyncapi` plus `git diff --exit-code --
asyncapi.yaml`) is the CI drift gate. This retires the earlier
collector-generate-and-copy-back workflow described in prior
revisions of this doc.

**What we considered.** Hand-maintained AsyncAPI (the original
approach), no AsyncAPI at all (rely on the schema registry alone),
or generating from Protobuf descriptors instead of the
CloudEvents catalog.

**Why generated from the catalog.**

1. **Hand-maintained drifts.** The original signal-control-only
   AsyncAPI was hand-edited and grew stale within months. New
   ce-types landed in code without spec updates. Consumers
   reading the spec saw an outdated catalogue.

2. **The catalog is the right source.** The ce-type registry
   knows every event the system emits; the Protobuf descriptors
   know the payload shapes; together they're the complete
   AsyncAPI surface. Generating from anywhere else duplicates
   information.

3. **Drift detection in CI.** `make asyncapi-check` fails a
   contributor's CI run the moment a new or changed notification's
   payload isn't reflected in the committed `asyncapi.yaml` — one CI
   run, not "discovered three months later when a consumer notices."
   Deriving the catalog from the taxonomy also produces a strict
   superset of the collector's hand-maintained catalog (surfacing
   gen-1→gen-2 migration gaps as an explicit delta report, not silent
   drift).

**What we'd revisit.** Nothing on the URL-pointer-vs-typed-payload
question — that's done: payloads are the embedded JSON Schema, not
a `$ref` to the schema registry. If AsyncAPI's tooling ecosystem
produces a generator that walks Protobuf descriptors directly with
better fidelity than the YANG-derived one, we'd adopt that; the
current generator is a few hundred lines of Go, replacing it is
cheap.

## Naming committed early: `openits`

**What we chose.** A single authority prefix, `openits`, used
consistently across `ce-type`, NATS subjects, URN identifiers,
YANG namespaces, and module names.

**What we considered.** `tc.*` (the original working name's
prefix), `us.dot.its.*` (federally-aligned), per-agency
prefixes.

**Why `openits`.**

1. **Authority neutrality.** A federal prefix presumes a
   federal owner that may or may not materialise. A per-agency
   prefix doesn't generalise across deployments.
   `openits` is the project; the prefix is grep-replaceable
   in a single PR if the project is later donated to a
   standards body that wants its own.

2. **Consistency reduces cognitive load.** A consumer who has
   read one ce-type can predict the shape of the rest. The
   subject and the URN follow the same pattern. The YANG
   module name and the proto package name are predictable.

3. **Single source for a future rename.** All five layers (CE,
   subject, URN, YANG, module) reference the same string. A
   future foundation-driven rename touches one constant in the
   reference implementation and one entry in the governance
   doc; the wire format updates mechanically.

**What we'd revisit.** If a foundation accepts donation and
prefers a different prefix, the rename is mechanical. The
durable decision is "single authority, consistently applied,"
not the specific string.

## Notification taxonomy: many typed events, not one envelope

**What we chose.** Every openits event is its own typed YANG
notification with its own ce-type. There is no universal
"fact-envelope" or base grouping that other notifications inherit
from. Universal scalars live in `openits-types`, but no inheritance
discipline is required of new modules. Cross-event metadata
(observer, occurred-at, causality) rides in CloudEvents extension
attributes, not in the per-event payload.

**What we considered.** Early in the design we proposed an
`openits-fact` module with a `fact-envelope` grouping that every
service's notifications would `uses`. The envelope carried
`fact-id`, `trace-id`, `caused-by[]`, `occurred-at`, `observed-at`,
`observer`, `subjects[]`, `certainty`, `classification` —
opinionated metadata on every event in every service. The thinking
was that if we make the discipline mandatory, every implementer of
openits inherits it for free, and cross-service correlation becomes
trivial.

**Why we chose many small typed events instead.** OpenConfig spent
a decade learning these lessons; we don't need to re-learn them.
(The resulting machinery — the event-kind identity hierarchy, wire
provenance, and per-concern module layout — is documented for
implementers in [the data model](data-model.md).)

1. **The lowest-common-denominator trap.** OpenConfig tried to make
   Cisco BGP and Juniper BGP look like one BGP. Vendor semantics
   diverged; the unified model lost expressiveness; vendors added
   deviations until the unified model fractured into N+1 schemas
   anyway. *Applied here:* the first time DMS or RSU needs a field
   that doesn't fit our universal envelope, we'll add a deviation.
   Three deviations later we've reinvented the per-service model
   with extra steps.

2. **Module boundaries matter; mega-modules don't work.** OpenConfig
   deliberately split into ~30 small modules with clean import
   graphs (`openconfig-bgp`, `openconfig-interfaces`,
   `openconfig-platform`) — not one model importing everything.
   The mega-module pattern was tried and abandoned. *Applied
   here:* a required-inheritance `openits-fact` is the mega-module
   antipattern. `openits-types` is the right shape — shared
   scalars + URN typedefs, no required-inheritance.

3. **Telemetry models work where configuration models struggle.**
   OpenConfig's telemetry side (interface counters, BGP session
   state) is broadly successful in production. The configuration
   side fractured because vendor semantics didn't compose. *Applied
   here:* openits models telemetry and observations — the part
   OpenConfig got right. Following that pattern means small typed
   notifications per concern, not one universal envelope.

4. **Ship the smallest necessary envelope; evolve on consumer
   pull.** OpenConfig's most useful innovations (sample-interval
   semantics, state-tree mirroring config-tree) emerged from operator
   demand, not upfront speculative design. *Applied here:* we
   speculated about `caused-by`, `certainty`, `classification` on
   every event. No consumer is asking for them yet. We add CE
   extensions when a real use case appears, not preemptively.

5. **Don't conflate state and events.** OpenConfig formalized
   separate state vs config paths, and that turned out important —
   operators always need to ask "what is, separate from what was
   configured?". *Applied here:* keep current-state (digital-twin
   pattern, NATS KV) separate from event-stream. Don't pretend
   they're the same shape under a unified envelope.

6. **Augmentation is a release valve, not a design pattern.**
   OpenConfig's augment mechanism became how vendors fled the LCD
   trap. By the time you're augmenting, the unified model has
   failed at its goal. *Applied here:* we use augments for
   first-class vendor / agency / community extensions per
   `06-extension-model.md`, but the *base* model has to be
   well-shaped enough that augments are rare additions, not
   constant workarounds.

7. **Don't model abstractions that fight reality.** Early
   OpenConfig had elaborate ACL match-criteria models that no two
   vendors implemented compatibly. *Applied here:* if a "subjects
   array" or "certainty score" on every event would be set to
   defaults 99% of the time and ignored downstream, it's an
   abstraction that fights reality. Single-subject events are the
   norm; force-fitting them through a multi-subject envelope wastes
   wire bytes and consumer attention.

**What this means concretely.**

- Each per-event message gets its own YANG notification (`fault-raised`,
  `phase-state-change`, `detector-transition`, ...) and its own
  ce-type (`openits.<service>.<event-name>.v1`).
- CloudEvents headers carry universal metadata: `ce-id` (replay-safe),
  `ce-source` (URN), `ce-subject` (NATS-subject mirror), `ce-time`,
  `ce-traceparent`.
- Two opinionated CE extension attributes (added 2026-05-15) carry
  the things CE doesn't natively express:
  - `ce-occurred-at` — wall-clock time in the world (separate from
    `ce-time` which becomes "observed-at by the emitter")
  - `ce-observer` — narrower attribution than `ce-source`
    ("cabinet-001-poller" vs "central-tsam-classifier-v3.2")
- New CE extensions (`ce-caused-by`, `ce-confidence`,
  `ce-classification`) land when consumers actually pull them.
  Not before.

**What we'd revisit.** If a real federation use case requires
cross-event correlation that CE headers can't express (e.g., V2X
trust chains with cryptographic provenance per event), we add the
extension attribute, not a universal grouping. The unit of change
is "one new CE extension"; the unit isn't "redesign every YANG
module."

## Event notifications live in a per-service companion module

**What we chose.** Every service's `notification` statements live in a single
`openits-<service>-events` companion module, imported alongside the service core
rather than inside it. Signal-control's events are one module,
`openits-signal-control-events`, with the domains (phase, pedestrian, detector,
overlap, coordination, preemption, TSP, TSAM, fault, status, raw) separated by
section comments — not eleven modules.

**What we considered.**
- Notifications inside each service core module. Rejected: ygot's proto backend
  fails on any module that contains a `notification` statement, so the core would
  not generate.
- One `-events` module per event domain (what signal-control had: 11 modules).
  Rejected: they versioned in lockstep and received identical revisions, so the
  split bought nothing while costing eleven revision histories and eleven sets of
  import/boilerplate to maintain.

**Why one companion module per service.** The generator routes notifications to
proto and AsyncAPI by module-name prefix and keys messages/ce-types by
notification name, so the number of `-events` modules is invisible to every
generated artifact — the split was pure overhead. One companion per service keeps
the core generatable, matches the layout every other service already uses, and
gives one revision history per service's event surface.

**What we'd revisit.** If the generator's proto backend gains notification
support, the companion split could be dropped entirely and notifications folded
back into each service core.

---

The decisions in this document are the load-bearing ones. Smaller
choices (file layout, naming conventions, retention windows) are
documented in their respective subsystems and are individually
movable without affecting the standard.
