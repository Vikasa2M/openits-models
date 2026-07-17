# 05 — Standards alignment

OpenITS is a layer above the device-level standards, not a
replacement for them. This document maps OpenITS to the standards
it consumes, complements, or aligns with — for procurement
clauses, research citations, and to settle the recurring "is this
NTCIP / J2735 / 1609 / TMDD" question.

## NTCIP — the device side

**National Transportation Communications for ITS Protocol.** The
family of standards (NTCIP 1202, 1203, 1204, 1205, 1207, 1208,
1218, etc.) that governs device-level communication for U.S.
roadside infrastructure.

| OpenITS service | NTCIP coverage | Relationship |
|------------------|----------------|--------------|
| `signal-control` | NTCIP 1202 | Functional — OpenITS YANG describes the same conceptual model; SNMP OIDs are mapped at translation time. |
| `dms`            | NTCIP 1203 | Functional — same pattern. |
| `ess`            | NTCIP 1204 | Functional. |
| `cctv`           | NTCIP 1205 | Functional (queued for the next service-package addition). |
| `ramp-metering`  | NTCIP 1207 | Functional. |
| `parking`        | NTCIP 1208 | Reference (no OpenITS service yet; future augment territory). |
| `rsu`            | NTCIP 1218 | Functional, with SAE J2735 message types as broadcast payloads. |

The relationship is **functional, not wire-compatible**. Devices
still speak NTCIP MIBs over SNMP; the OpenITS poller handles
translation at the cabinet edge. The OpenITS YANG models are
deliberately transport-independent: a vendor that ships native
YANG over gNMI, NETCONF, RESTCONF, or plain HTTP can be bridged
the same way without touching the schema. The lesson NTCIP
illustrates — locking a data model to a specific transport — is
the lesson OpenITS most wants not to repeat.

## SAE J2735 — V2X messages

**SAE J2735** specifies the V2X message set: SPaT, MAP, BSM,
TIM, SRM, SSM, PSM, and others. RSUs broadcast these to
vehicles over DSRC or C-V2X.

OpenITS' `rsu` service models the lifecycle of these messages
on the RSU side:

- **TIM** (Traveler Information Message) lifecycle is
  represented as `rsu-tim-loaded`, `rsu-tim-cleared`, and
  `rsu-tim-broadcast-state-changed` events.
- **SRM** (Signal Request Message) reception triggers
  `rsu-srm-received` and `rsu-srm-status-change` events when
  the RSU forwards a request to the controller.
- **SPaT** broadcast state is observable through the RSU's
  channel and message-count telemetry.
- **Certificate lifecycle** (SCMS-managed) surfaces as
  `rsu-certificate-expiring` events.

The actual J2735 message payloads are not OpenITS' concern;
they ride the V2X air interface to vehicles. OpenITS is the
management plane around the broadcast — what's loaded, what's
broadcasting, what failed to load.

## IEEE 1609 — DSRC / WAVE protocol stack

**IEEE 1609.x** governs the DSRC / WAVE protocol stack the RSU
uses for V2X transmission. OpenITS depends on the security
model (1609.2) for SCMS certificate handling and the channel
model (1609.4) for multi-channel operation, but does not
re-specify either.

Channel-fault and certificate-related events on the `rsu`
service surface 1609-layer state to the management plane.

## ARC-IT — federal architecture vocabulary

**Architecture Reference for Cooperative and Intelligent
Transportation.** The U.S. federal-aligned reference architecture
that defines:

- **Service packages** — the named bundles of capability that
  agencies plan around (TI01 for signal control, TI03 for
  metering, TI06 for DMS, etc.).
- **Information flows** — the named directional flows between
  ARC-IT subsystems (e.g., "Roadway Signal Controller → TMC :
  signal control plan").

OpenITS YANG modules carry an `arc-it-flow` extension annotation on
notifications and on container references where applicable. A
coverage tool (`tools/arcit-coverage`) walks the YANG tree and
produces a coverage report against the ARC-IT inventory; this is
the artefact agencies use to demonstrate federal-architecture
alignment in funding paperwork.

The annotation is informational. ARC-IT compliance is not a
prerequisite for OpenITS conformance, and OpenITS conformance does
not certify ARC-IT compliance — but the two surfaces map cleanly,
and an OpenITS deployment of a service package whose YANG carries
the right `arc-it-flow` annotations satisfies the ARC-IT
documentation requirement.

## TMDD — the alternative

**Traffic Management Data Dictionary.** A long-standing TMC-to-TMC
data-sharing standard. TMDD predates the openits-style
event-driven model and ships as a heavyweight XML message family.

OpenITS does *not* aim to replace TMDD. They serve different
purposes:

- **TMDD** is a TMC-to-TMC sharing format. It assumes both ends
  are large traffic management systems exchanging summary data
  on hourly or daily cadence.
- **OpenITS** is a device-to-consumer telemetry format. It
  assumes the consumer wants per-event detail at the cadence of
  device state changes.

A TMC that exposes both is reasonable: TMDD for cross-TMC
coordination, OpenITS for analytics and operator-facing
dashboards. A bridge process that translates OpenITS event
streams into TMDD aggregate updates is straightforward and is
expected to live in operator-side or vendor-side code, not in
the OpenITS core.

## CloudEvents — the envelope standard

**CNCF CloudEvents 1.0** specifies the metadata envelope OpenITS
uses. The choice is documented in
[Design Decisions](04-design-decisions.md). All envelope behaviour
follows the CloudEvents binary-mode specification; we contribute
no extensions beyond the standard `traceparent` extension for
W3C Trace Context propagation.

## YANG — schema language, not transport

**RFC 7950** (YANG 1.1) is the schema language. OpenITS' use of
YANG is deliberately decoupled from any specific transport.
**NETCONF** (RFC 6241) and **gNMI** are two existing transports for
YANG-modelled data; the OpenITS transport is NATS-as-CloudEvents.
Other deployments could carry the same YANG-modelled state over
RESTCONF, plain HTTP, or transports not yet invented.

The decoupling is the point. NTCIP's primary failure mode is that
it bound its data model to SNMP as the transport; replacing the
transport meant replacing the data model. OpenITS picks a schema
language (YANG) that has no transport opinion, then chooses a
transport (NATS) for the reference deployment, and treats the two
as separable.

Operators or vendors who already speak gNMI, NETCONF, or any other
YANG-compatible transport can map OpenITS schemas onto their
preferred transport without changes to the schemas themselves. The
project does not advocate any one transport as "the future."

Internally, OpenITS uses **ygot** for Go code generation from
YANG, and the standard ygot-generated proto backend for producing
companion Protobuf definitions where appropriate.

## OpenAPI / AsyncAPI — the consumer-facing contract

**AsyncAPI 3.0** describes the event-driven surface; OpenITS
generates `asyncapi.yaml` from the in-tree CloudEvents catalog.
Consumers use `@asyncapi/cli validate` to verify the spec and
`@asyncapi/generator` to scaffold cross-language clients.

OpenAPI does not appear in the wire layer (OpenITS is event-driven,
not request/response), but operator-facing administrative APIs —
configuration push, audit log queries, conformance report fetch —
are expected to use OpenAPI when they materialise.

## US DOT V2X Deployment — operational alignment

The U.S. DOT V2X deployment program emphasises:

- **Regional coordination** — multi-agency corridors that share
  data.
- **Federal investment alignment** — projects must demonstrate
  ARC-IT alignment for federal funding.
- **Vendor neutrality** — RFP language that doesn't lock the
  agency to one vendor.

OpenITS is designed to satisfy all three: subject hierarchy
encodes regional structure (`region.agency.agency-unit`),
ARC-IT annotations support funding documentation, and the
extension model preserves vendor freedom without per-agency forks.

## Where OpenITS deliberately diverges

A few places where OpenITS makes a different call than the
standards landscape would suggest:

### Greenfield wire format

Most ITS standards committees prioritise backwards compatibility.
OpenITS does not ship backwards-compatible with any prior
pre-OpenITS data format — see
[Design Decisions](04-design-decisions.md) for the reasoning.

### Operator-weighted governance

Most networking standards are vendor-driven (the IETF and IEEE
are dominated by vendor employees, with academics in support).
OpenITS gives operators majority weight on YANG-changing
decisions — closer to a public-works standard than a tech
standard.

### Schema as code

Most standards bodies separate the standard (a PDF) from
implementations. OpenITS treats the YANG modules and their
schema-registry snapshots as the standard; the PDF, if one
exists, is a derived artefact. The reference implementation
demonstrates the standard rather than illustrating it.

These divergences are deliberate. They are also reversible: if
the project is donated to a standards body that prefers
traditional-shape governance, the wire format and schema model
hold; only the procedural surface changes.
