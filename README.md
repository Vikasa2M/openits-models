# openits-models

The OpenITS data-model layer: vendor-neutral model definitions and the code
generated from them, published as a standalone Go module. Components such as
collectors consume it as a dependency.

Module path: `github.com/Vikasa2M/openits-models`

## Layout

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

## Design & rationale

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

## Using it

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

## Regeneration

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

## Provenance

`openits-models` is self-contained: the YANG modules are the source of truth,
and every other artifact (protobuf, Go, JSON Schema, AsyncAPI, schema-registry
snapshots) is generated from them by in-repo tooling. No other repository is
required to build, generate, or validate this repo. The collector and other
consumers depend on this module; this module depends on none of them.

## Contributing, versioning & releases

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

## License

Licensed under the [Apache License 2.0](LICENSE). See [`NOTICE`](NOTICE) for
attribution, including the vendored IETF YANG modules under `yang/ietf/`.
