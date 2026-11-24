# OpenITS

OpenITS is an open data model for roadway systems. It defines what common
transportation devices and roadway events are, and what data each one reports,
in a single vendor-neutral format. Any system can publish its data in this
format, and any other system can read it.

This repository, `openits-models`, holds the model itself. The first part of
this README explains OpenITS in plain terms for agencies, managers, and other
non-engineers. Implementers can skip ahead to [For implementers](#for-implementers).

## What it covers

**Devices**

- Traffic signals
- Detectors
- Cameras
- Dynamic message signs
- Weather stations
- Ramp meters
- Reversible lanes
- Roadside units and connected-vehicle messaging

**Roadway events**

- Work zones and lane closures. These are modeled as events on the roadway,
  not as devices.

## Why it exists

Today, each vendor and system describes this data in its own format. A signal
state, a detector count, or a message sign's current display looks different
depending on which product reports it. Connecting two systems usually requires
custom translation work, and that work is repeated for every pair of systems an
agency operates.

With a shared model, each system is translated to OpenITS once. Any other
system that reads OpenITS can then use that data without additional
integration.

## What this means for agencies

- **Integration:** Each system connects to the shared model instead of to every
  other system individually.
- **Data ownership:** The model is open and free to use, so no single vendor
  controls the format. Agencies decide which data is shared and which stays
  internal.
- **Vendor comparison:** Because all products use the same data format, vendors
  are evaluated on their products and services rather than on format lock-in.
- **Operator visibility:** The model uses plain, readable terms, so engineers
  and operators can review what devices, automated systems, and AI tools are
  reporting.

## The data fabric: how OpenITS data moves

The data fabric is the shared network where data in the OpenITS format is
sent, stored, and shared. It is built on [NATS](https://nats.io), an
open-source messaging technology, and uses NATS's built-in storage feature
(JetStream) to keep data durable.

Devices in the field send their data into the fabric. Other systems, such as a
traffic management system, analytics tools, or a neighboring agency, receive
the data they are allowed to see. The fabric follows the structure of a DOT.
Each cabinet runs a local NATS that connects up to a regional cluster where
needed. Regional clusters feed the central tier. A separate DMZ handles sharing
with outside parties.

### How data moves

- **Connection:** Each roadside cabinet makes one outbound connection to the
  fabric. Nothing connects inward to the cabinet, so it works with standard
  firewalls.
- **Translation:** A small program in the cabinet converts the device's native
  data into OpenITS before sending it.
- **Lost connections:** The cabinet's local NATS stores data while the
  connection is down. When the connection returns, it sends everything it
  missed. The record has no gaps.

### Security

- **Encryption:** Connections are encrypted with mTLS (mutual TLS), and both
  sides prove who they are. There are no trusted "inside" links.
- **Scoped access:** Each cabinet has its own credentials, limited to its own
  data.

### Data sharing

- **Default:** Nothing leaves the agency unless the agency lists it. Anything
  not on the list stays internal.
- **DMZ:** Shared data passes through the DMZ, not the internal system.
  Internal naming is never exposed.
- **Outside access:** Researchers, partners, and neighboring DOTs get read-only
  access to only the data assigned to them. Access can be revoked for each
  party individually.
- **Incoming data:** Data from other agencies arrives in a separate, read-only
  area. It is never mixed into internal systems.

## Adoption

OpenITS is designed to be adopted through existing procurement and maintenance
contracts, not as a separate funded program. The core requirement is one
sentence:

> The system must publish its data in OpenITS.

- **New equipment and software:** Add the requirement to the procurement
  specification. Systems arrive already using the model.
- **Existing equipment:** A small adapter runs alongside the device's current
  software and translates its data into OpenITS. No firmware changes are
  needed, so this work can be scheduled during routine maintenance.

Example procurement language is in
[`docs/07-conformance.md`](docs/07-conformance.md).

## Relationship to existing standards

OpenITS does not replace NTCIP, SAE J2735, or ARC-IT. It sits above them as a
common data layer.

| Standard | What it does |
|----------|--------------|
| NTCIP | Defines how central systems communicate with field devices |
| SAE J2735 | Defines messages broadcast to vehicles |
| ARC-IT | National reference architecture for ITS deployments |
| **OpenITS** | Defines one consistent model where data from these sources is stored and shared |

Details are in
[`docs/05-standards-alignment.md`](docs/05-standards-alignment.md).

## License and governance

- **License:** Apache 2.0. Any agency, vendor, or researcher may use, modify,
  fork, and redistribute it, including commercially, at no cost.
- **Steward:** Vikasa.io currently maintains the model. The project is open to
  adoption by a standards body.
- **Background:** Developed through research funded by the Federal Highway
  Administration, with input from agencies and vendors.

## Status

- Versioned and in active development.
- The conformance kit currently tests against a built-in mock device. Testing
  against live equipment is planned.
- Known gaps are documented openly, for example
  [`docs/reference/dms-ntcip-1203-fidelity.md`](docs/reference/dms-ntcip-1203-fidelity.md).

## Exploring this repository with an AI assistant

If you are not an engineer, you can paste this repository's link into an AI
assistant and ask questions such as:

- What does OpenITS define, in plain English, for a state DOT?
- How does it relate to NTCIP and J2735?
- What would we write into a procurement spec?

---

## For implementers

The rest of this README covers the `openits-models` repository itself: the
OpenITS data-model layer, made up of vendor-neutral model definitions and the
code generated from them, published as a standalone Go module. Components such
as collectors consume it as a dependency.

Module path: `github.com/Vikasa2M/openits-models`

### Layout

| Path | Contents |
|------|----------|
| `yang/` | YANG source modules (the source of truth for the model). |
| `api/proto/` | Protobuf sources: hand-authored `command.proto`/`device.proto` plus generated per-service protos under `openits/<service>/v1/` (`events.proto`, `state.proto`, `types.proto`, and per-event JSON Schemas), produced from `yang/` by the in-repo YANG→proto generator (`tools/yang-proto-gen`). |
| `pkg/proto/openits/<service>/v1/` | Generated Go protobuf types per service (e.g. package `dmsv1`); `pkg/proto/openits/v1/` holds only the `command`/`device` types (package `openitspb`). |
| `pkg/yang/openits/` | Generated ygot Go types (package `openits`). |
| `schema-registry/` | AsyncAPI/JSON-schema registry entries per service. |
| `bindings/nats/` | NATS reference profile: subject grammar, CloudEvents envelope, JetStream notes (`README.md`) and the generated `asyncapi.yaml` (see note below). The transport-neutral model layer above does not depend on it. |
| `scripts/`, `tools/` | Model generation, validation, and lint tooling. |
| `docs/` | Design rationale — how the model is shaped and why. |

### Design & rationale

The model's shape is deliberate, and much of it is a direct application of
**OpenConfig's lessons**: many small, independently-versioned modules rather
than a mega-module per device; leaning into the telemetry side that OpenConfig
got right; and avoiding the one-size-fits-all configuration side that fractured.
See:

- [`docs/data-model.md`](docs/data-model.md) — the module family, event
  taxonomy, wire provenance, and a "Lessons applied" summary.
- [`docs/04-design-decisions.md`](docs/04-design-decisions.md) — *why* each
  choice was made (YANG contract, per-event proto, greenfield vs. bridged, …).
- [`docs/05-standards-alignment.md`](docs/05-standards-alignment.md) — NTCIP /
  J2735 / ARC-IT mapping.
- [`docs/06-extension-model.md`](docs/06-extension-model.md) — augments,
  deviations, and graduation.
- [`docs/08-capability-architecture.md`](docs/08-capability-architecture.md) —
  model by function; thin device profiles compose capability modules.
- [`docs/reference/dms-ntcip-1203-fidelity.md`](docs/reference/dms-ntcip-1203-fidelity.md) —
  worked example of how a service model is audited against its source
  standard: deliberate improvements vs. fidelity defects vs. coverage gaps.
- [`docs/07-conformance.md`](docs/07-conformance.md),
  [`docs/glossary.md`](docs/glossary.md),
  [`docs/reference/yang-reference-conventions.md`](docs/reference/yang-reference-conventions.md).

### Using it

```sh
go get github.com/Vikasa2M/openits-models@latest
```

```go
import (
    openitspb "github.com/Vikasa2M/openits-models/pkg/proto/openits/v1"     // command / device
    dmsv1     "github.com/Vikasa2M/openits-models/pkg/proto/openits/dms/v1" // one service's events + state
    openits   "github.com/Vikasa2M/openits-models/pkg/yang/openits"
)
```

Non-Go implementers should start from the curated release bundle instead
(`yang/`, `api/proto/`, `schema-registry/`, `bindings/` — attached to each
[GitHub release](https://github.com/Vikasa2M/openits-models/releases)).

### Regeneration

```
make gen             # all of the below: proto, Go, AsyncAPI and the ce-type catalog
make yang-proto-gen  # yang -> api/proto/openits/<service>/v1/*.proto + field-numbers.yaml lock
make proto           # api/proto -> pkg/proto (protoc)
make yang            # yang -> pkg/yang (ygot fakeroot structs)
make asyncapi        # yang -> bindings/nats/asyncapi.yaml
make catalog         # yang -> schema-registry/index.json (ce-type catalog)
./scripts/update-schema-registry.sh  # snapshot changed modules into schema-registry/
make check-gen       # gen, then fail if output drifts from what's committed (freshness gate)
make ci              # the full local gate, mirroring CI (run on a committed tree)
```

`command.proto`/`device.proto` under `api/proto/` are hand-curated and
untouched by `make yang-proto-gen`.

Requires `protoc`/`buf`, `ygot`/`goyang`, and (optionally) `pyang` and
`yanglint`.

> **`bindings/nats/asyncapi.yaml`** is carried here as a published
> artifact of the NATS reference profile, regenerated in-repo from the
> YANG-derived ce-type catalog by `make asyncapi`. See
> [`bindings/nats/README.md`](bindings/nats/README.md) for the profile
> spec and [`docs/07-conformance.md`](docs/07-conformance.md) for the
> two-tier (model vs. NATS-profile) conformance model.

### Provenance

`openits-models` is self-contained: the YANG modules are the source of truth,
and every other artifact (protobuf, Go, JSON Schema, AsyncAPI, schema-registry
snapshots) is generated from them by in-repo tooling. No other repository is
required to build, generate, or validate this repo. The collector and other
consumers depend on this module; this module depends on none of them.

### Contributing, versioning & releases

- [`CONTRIBUTING.md`](CONTRIBUTING.md) — the edit-YANG-then-regenerate workflow
  and the full set of gates CI enforces.
- [`docs/versioning.md`](docs/versioning.md) — how the Go module version,
  per-module YANG revisions, and protobuf wire compatibility relate, plus the
  step-by-step release process.
- [`CHANGELOG.md`](CHANGELOG.md) — notable changes per release.

CI (`.github/workflows/ci.yml`) runs the gate on every pull request as
grouped jobs — `go` (generation freshness, `go build`/`vet`/`test`), `buf`
(lint + breaking-change detection), `yang` (governance checks, `yang-lint`,
`yanglint` instance-data validation) and `conformance` (every device kind) —
after a `changes` job classifies the diff. The required check `ci-ok` rolls
them up. A docs-only PR skips `go`, `buf` and `conformance` but still runs
`yang`; the release-please PR skips every gate; a push to `main` runs only
the `go` job's build, vet and test steps.

### License

Licensed under the [Apache License 2.0](LICENSE). See [`NOTICE`](NOTICE) for
attribution, including the vendored IETF YANG modules under `yang/ietf/`.
