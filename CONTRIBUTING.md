# Contributing to openits-models

Thanks for your interest in the OpenITS data-model layer. This repo owns the
vendor-neutral YANG models and every artifact generated from them (protobuf,
Go, JSON Schema, AsyncAPI, schema-registry snapshots). The YANG modules are the
**source of truth** — you edit YANG, then regenerate; you never hand-edit
generated output.

## The golden rule: edit YANG, then regenerate

```
make gen          # regenerate all artifacts from yang/
make check-gen    # regenerate and fail if anything drifts from what's committed
```

CI runs `make check-gen`, so a PR that edits generated files without editing
their YANG source (or forgets to regenerate) will fail. Exceptions:
`api/proto/command.proto` and `api/proto/device.proto` are hand-authored core
protos and are not touched by generation.

## Before you open a PR

Run the full gate locally — these are exactly the jobs CI enforces:

```
make check-gen                       # freshness (needs protoc, protoc-gen-go, ygot)
go build ./... && go vet ./... && go test ./...
buf lint                             # protobuf lint
make check-revisions check-naming    # revision bumps + naming discipline
make check-deviations check-augment-collisions check-events-layering
make validate-noi check-graduation   # notice-of-intent + graduation
make validate-yang                   # golden instance data vs. schema (Docker)
make yang-lint                       # pyang --strict (optional; needs pyang)

# conformance, per kind:
go run ./tools/conformance -driver mock -kind asc
```

See the [`Makefile`](Makefile) for every target and the
[README](README.md#regeneration) for what each generates.

## Toolchain

- **Go** — version pinned in [`go.mod`](go.mod).
- **protoc** + `protoc-gen-go@v1.36.11` (see `scripts/proto-gen.sh`).
- **ygot generator** `@v0.34.0` (see `scripts/yang-gen.sh`).
- **buf** for proto lint / breaking-change detection.
- **Docker** for `make validate-yang` (runs `yanglint` in a container).
- **pyang** (optional) for `make yang-lint`.

## Conventions that CI enforces

- **Revision bumps.** Changing a module's content requires bumping its
  `revision` date. See [`docs/versioning.md`](docs/versioning.md).
- **Wire compatibility.** Extend protos by appending fields/messages/enums;
  never renumber or reuse field numbers. `buf breaking` runs against `main`.
- **Naming.** No legacy ce-type / URN / subject forms (`make check-naming`).
- **Events layering.** `*-events.yang` modules may import only shared types /
  IETF types / `openits-nema-common` / a `*-types` module — never a service
  core or another events module.

## Adding a new service

Use the scaffolder:

```
go run ./tools/openits-new-service --help
```

It generates the core / types / events YANG skeleton in the house style. Then
follow the regenerate-and-gate loop above.

## Commits & PRs

- Keep PRs focused; separate model changes from tooling changes where practical.
- Update [`CHANGELOG.md`](CHANGELOG.md) under `## [Unreleased]` for anything
  user-facing.
- Ensure every CI job is green before requesting review.

## Security

Do **not** open a public issue for security vulnerabilities. Report them
privately — see [`SECURITY.md`](SECURITY.md).

## Code of conduct

Participation is governed by our [Code of Conduct](CODE_OF_CONDUCT.md).

## License

By contributing, you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE), consistent with the rest of the repository.
