# Glossary

Acronyms, terms, and project-specific vocabulary used across the
OpenITS documentation. This is the data-model repo; a few
deployment-side terms (poller, TMC, storage backends) are defined here
only for the context in which they appear in these docs — their
implementation lives in the deployment/collector layer, not in this
repo.

---

**Agency** — A transportation operator. State DOTs (TxDOT,
Caltrans), county / city traffic departments, toll authorities,
transit agencies. The `{agency}` token in OpenITS subjects.

**Agency unit** — A subdivision within an agency: TxDOT districts,
Caltrans regions, NYC DOT modes. The `{agency-unit}` token. The
sentinel `all` is used for un-subdivided agencies.

**ARC-IT** — Architecture Reference for Cooperative and
Intelligent Transportation. The U.S. federal-aligned reference
architecture defining ITS service packages and information flows.
OpenITS YANG modules carry `arc-it-flow` annotations.

**ASC** — Actuated Signal Controller. The roadside hardware that
runs an intersection's signal timing. NTCIP 1202.

**ASC3** — Econolite's third-generation Actuated Signal Controller
product line. Its vendor-alarm identities are the worked example in
`openits-vendor-econolite-signal-control-types`.

**ATSPM** — Automated Traffic Signal Performance Measures. A set
of measures (Purdue phase-termination, split failures, green-time
percentiles, ped-call-to-walk delay) derived from high-resolution
signal-controller event logs. OpenITS ingests high-resolution event
logs from each cabinet and exposes ATSPM dashboards (implemented
in the collector's `internal/atspm/`, not in this repo).

**AsyncAPI** — Specification for describing event-driven APIs.
OpenITS generates `asyncapi.yaml` from the in-tree CloudEvents
catalog.

**Augment** — A YANG 1.1 module that adds nodes to a core OpenITS
module without modifying it. Vendors and agencies ship augments
in their own namespace; augments graduate to core when three
independent organisations adopt them.

**BSM** — Basic Safety Message. SAE J2735 V2X message broadcast
by vehicles; received by RSUs.

**ce-id, ce-type, ce-source, ce-dataschema, ce-time** —
CloudEvents 1.0 attributes. In OpenITS, they ride in NATS message
headers (binary mode).

**ClickHouse** — Column-oriented database. OpenITS' default
history / analytics backend.

**CloudEvents** — CNCF specification for a common envelope around
event data. OpenITS uses CloudEvents 1.0 binary mode.

**Conformance kit** — `tools/conformance/`. Validates an
implementation against the OpenITS contract by subscribing to its
NATS endpoint, checking subject hierarchy, envelope shape, and
per-service assertions.

**Core** — Tier 1 of the extension model. TSC-controlled YANG
modules at `yang/openits-<service>.yang`.

**Deviation** — Tier 3 of the extension model. A YANG 1.1
`deviation` module that tightens constraints in a core module
for a specific jurisdiction.

**DMS** — Dynamic Message Sign. Variable-message signs on
freeways and arterials. NTCIP 1203.

**DSRC** — Dedicated Short-Range Communications. The legacy
V2X air interface; superseded in many deployments by C-V2X.
IEEE 1609.x.

**ESS / RWIS** — Environmental Sensor Station / Road Weather
Information System. Roadside weather sensors. NTCIP 1204.

**gNMI** — gRPC Network Management Interface. One transport for
pushing/pulling YANG-modelled data. OpenITS uses NATS, not gNMI;
the YANG models are transport-independent, so a deployment that
prefers gNMI (or NETCONF, RESTCONF, or plain HTTP) can map them
onto its preferred transport without schema changes.

**Graduation** — The process by which a Tier 2 augment moves into
Tier 1 core. Three independent NoIs (with at least one operator)
plus TSC operator-weighted majority vote.

**JetStream** — NATS' durable streaming layer. Provides at-least-once
delivery, replay, and consumer groups. The reference deployment's
multi-tier stream topology is a deployment concern, out of scope for
this model repo.

**KV** — Key-Value. OpenITS uses NATS KV (default) or Valkey
(at scale) for live state — the latest snapshot per device.

**Leaf node** — A NATS server that proxies a small client
population to a central NATS cluster. In OpenITS, every cabinet
runs a leaf node; cabinets connect to the leaf, the leaf
connects outbound to central.

**MAP** — SAE J2735 V2X message broadcasting intersection
geometry to vehicles.

**MaxTime** — McCain's signal-controller product line.

**MUTCD** — Manual on Uniform Traffic Control Devices. The U.S.
federal manual whose §4F.17 sets the yellow-change interval (3–6 s)
— the only signal-timing bound here with a genuine MUTCD basis.
Red-clear has no MUTCD minimum (engineering-determined, with a
≤6 s ceiling per §4F.17); minimum green is an engineering floor,
not a MUTCD value. OpenITS encodes all three as YANG
`must`-constraints, labeled by their actual source.

**NATS** — Open-source messaging system. OpenITS' transport.

**NETCONF** — RFC 6241 protocol for managing YANG-modelled data.
One of several transports for YANG; gNMI and RESTCONF are others.
OpenITS uses NATS, not NETCONF; the YANG models are
transport-independent and can be mapped onto NETCONF without
schema changes.

**NoI** — Notice of Implementation. A YAML file at
`schema-registry/notices/<augment>/<implementer>.yaml` declaring
public adoption of an augment. Drives the graduation rule.

**NTCIP** — National Transportation Communications for ITS
Protocol. Family of standards governing device-level ITS
communication. NTCIP 1202 (signal control), 1203 (DMS), 1204 (ESS),
1205 (CCTV), 1207 (ramp meter), 1208 (parking), 1218 (RSU).

**OpenITS** — This project. Single authority prefix used across
all layers (CE, subject, URN, YANG namespace, module name).

**Operator** — A transportation agency that owns and operates ITS
infrastructure. The TSC's operator seats carry majority weight on
YANG-changing decisions.

**Per-event Protobuf** — OpenITS' approach: each transition is
its own typed Protobuf message (FaultRaised, ModeChanged, etc.)
with its own ce-type, NATS subject, and schema revision. As
opposed to a bundled telemetry blob.

**Poller** — The edge process running in (or near) each cabinet
that polls devices and emits OpenITS events. Its reference
implementation lives in the deployment/collector layer, not this repo.

**Profile** — A scope for a conformance claim: `core`,
`core-plus-augments=<list>`, `core-plus-deviations=<list>`, or
`complete`.

**Proprietary** — Tier 4 of the extension model. Vendor-internal
modules outside the `openits.>` subject tree, on
`vendor.<vendor>.>`. Not under OpenITS governance.

**Protobuf** — Google's binary serialisation format. OpenITS'
wire payload format (CloudEvents binary mode).

**PSM** — Personal Safety Message. SAE J2735 V2X message for
vulnerable road users.

**Registry** — In OpenITS, "the schema registry" refers to
`schema-registry/<module>/<revision>/` — content-addressed
immutable snapshots of YANG and Protobuf at each revision.

**Region** — A jurisdiction at the state-or-equivalent level.
The `{region}` token in OpenITS subjects (e.g., `us-tx`).

**RSA** — Roadside Alert. SAE J2735 V2X advisory message.

**RSU** — Roadside Unit. V2X-equipped roadside hardware that
broadcasts SPaT, MAP, TIM, etc., and may receive vehicle
messages. NTCIP 1218.

**SCMS** — Security Credential Management System. The PKI
that issues V2X security certificates. RSU certificate lifecycle
events are part of OpenITS' RSU service.

**Service** — In OpenITS, a category of field infrastructure:
signal-control, dms, ess, rsu, ramp-metering, perception,
traffic-sensor, reversible-lane. The `{service}` token in OpenITS
subjects.

**SPaT** — Signal Phase and Timing. SAE J2735 V2X message
broadcast by RSUs (and originating from signal controllers).

**SRM / SSM** — Signal Request Message / Signal Status Message.
SAE J2735 V2X messages for signal priority/preemption requests.

**TIM** — Traveler Information Message. SAE J2735 V2X message
broadcast by RSUs to convey advisories, work-zone info,
construction zones, etc.

**TMC** — Traffic Management Center. The operator-side facility
that monitors and controls infrastructure. OpenITS consumers
typically run inside the TMC or analytics adjacent to it.

**TSAM** — Traffic Signal Adaptive Management. An adaptive-
control feature that consumes SPaT and other signals to drive
timing decisions. In OpenITS, TSAM events live on the
signal-control service: `tsam-mode-changed`,
`tsam-recommendation-applied`.

**TSC** — Technical Steering Committee. OpenITS' governance
body. Five operator seats, two vendor, one integrator, one
community. YANG-changing decisions require operator majority.

**TSP** — Transit Signal Priority. A common deviation use case;
transit vehicles request signal preemption to reduce dwell time.

**Valkey** — BSD-licensed Redis-compatible KV store. OpenITS'
optional swap for NATS KV at fleet scale (>10k keys).

**V2X** — Vehicle-to-Everything. Family of vehicle / roadside
wireless communications: V2V, V2I, V2P. Governed by SAE J2735
(messages) and IEEE 1609.x (protocol stack).

**YANG** — RFC 7950 schema language. OpenITS' source-of-truth
for data modelling. Modules at `yang/openits-<service>.yang`;
ygot-generated Go structs at `pkg/yang/`.

**ygot** — Open-source Go code generator from YANG. OpenITS
uses ygot for both Go struct generation and the
companion Protobuf generator output.
