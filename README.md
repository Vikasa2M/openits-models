# openits-models

The OpenITS data-model layer, extracted from the Vikasa monorepo as a
standalone, importable Go module. This repo owns the vendor-neutral model
definitions and the code generated from them; downstream components
(e.g. `openits-collector`) consume it as a dependency.

Module path: `github.com/openits/openits-models`

## Layout

| Path | Contents |
|------|----------|
| `yang/` | YANG source modules (the source of truth for the model). |
| `api/proto/` | Protobuf sources: hand-authored `command.proto`/`device.proto` plus generated `openits/v1/*.proto` (event payloads + shared types), produced from `yang/` by the in-repo YANG→proto generator (`tools/yang-proto-gen`). |
| `pkg/proto/openits/v1/` | Generated Go protobuf types (package `openitspb`). |
| `pkg/yang/openits/` | Generated ygot Go types (package `openits`). |
| `schema-registry/` | AsyncAPI/JSON-schema registry entries per service. |
| `asyncapi.yaml` | Published AsyncAPI document (see note below). |
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
- [`docs/07-conformance.md`](docs/07-conformance.md),
  [`docs/glossary.md`](docs/glossary.md),
  [`docs/reference/yang-reference-conventions.md`](docs/reference/yang-reference-conventions.md).

## Using it

```go
import (
    openitspb "github.com/openits/openits-models/pkg/proto/openits/v1"
    openits   "github.com/openits/openits-models/pkg/yang/openits"
)
```

## Regeneration

```
make gen             # yang -> proto (tools/yang-proto-gen) -> pkg/proto (protoc); yang -> pkg/yang (ygot)
make check-gen       # gen, then fail if output drifts from what's committed (freshness gate)
make yang-proto-gen  # yang -> api/proto/openits/v1/*.proto (event payloads + shared types) + field-numbers.yaml lock
make proto           # api/proto -> pkg/proto (protoc)
make yang            # yang -> pkg/yang (ygot fakeroot structs)
make validate-yang check-revisions check-naming
make validate-noi check-graduation check-augment-collisions
```

`command.proto`/`device.proto` under `api/proto/` are hand-curated and
untouched by `make yang-proto-gen`.

Requires `protoc`/`buf`, `ygot`/`goyang`, and (optionally) `pyang` and
`yanglint`.

> **asyncapi.yaml** is carried here as a published artifact, regenerated
> in-repo from the YANG-derived ce-type catalog by `make asyncapi`.

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

CI (`.github/workflows/ci.yml`) runs the full gate on every push and PR:
generation freshness, `go build`/`vet`/`test`, `buf` lint + breaking-change
detection, YANG governance checks, `yanglint` instance-data validation, and the
conformance harness across all device kinds.

## License

Licensed under the [Apache License 2.0](LICENSE). See [`NOTICE`](NOTICE) for
attribution, including the vendored IETF YANG modules under `yang/ietf/`.
