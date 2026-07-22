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

We use [**Conventional Commits**](https://www.conventionalcommits.org/) — the
changelog and version bumps are generated from them by release-please, so the
prefix matters:

| Prefix | Effect | Use for |
|--------|--------|---------|
| `feat:` | minor bump | a new capability/module, a new field or event |
| `fix:` | patch bump | a bug fix in a model or a tool |
| `feat!:` or `BREAKING CHANGE:` footer | minor (pre-1.0) / major (≥1.0) | a wire/JSON or YANG-contract break |
| `docs:` `chore:` `test:` `refactor:` `ci:` | no release | everything else |

Example: `feat(dms): add travel-time route table augment`.

- Keep PRs focused; separate model changes from tooling changes where practical.
- **Don't hand-edit `CHANGELOG.md`** — release-please regenerates it from commit
  messages. Write a good commit subject instead. See
  [`docs/versioning.md`](docs/versioning.md).
- Ensure every CI job is green before requesting review.

## AI-assisted contributions

The repo ships agent skills under [`.claude/skills/`](.claude/skills/) that
encode the contribution workflows above — extending a model, adding events or
services, interpreting CI gates, and the review checklist. Claude Code picks
them up automatically; other agent tools (Codex, Cursor, etc.) get the same
guidance via the root [`AGENTS.md`](AGENTS.md), which indexes those skill
files — they're all plain markdown. AI-assisted or not, the same
bar applies: you are responsible for the change, and every gate must be green.

## Security

Do **not** open a public issue for security vulnerabilities. Report them
privately — see [`SECURITY.md`](SECURITY.md).

## Code of conduct

Participation is governed by our [Code of Conduct](CODE_OF_CONDUCT.md).

## License

By contributing, you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE), consistent with the rest of the repository.
