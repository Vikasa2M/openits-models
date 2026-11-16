# Versioning & releasing

`openits-models` has three version axes that move independently. This document
explains what each one means, how they relate, and the exact steps to cut a
release.

## The three axes

| Axis | Where it lives | Governs | Cadence |
|------|----------------|---------|---------|
| **Go module version** | git tag `vX.Y.Z` | The importable Go API (`pkg/proto`, `pkg/yang`) and everything shipped in the tag. | Per release. |
| **YANG module revision** | `revision` statement in each `yang/*.yang` | The schema contract of one module. Bumped only when that module's content changes. | Per-module, as edited. |
| **Protobuf wire compatibility** | field numbers in `field-numbers.yaml` + `buf breaking` | On-the-wire and JSON-tag stability of the generated messages. | Enforced continuously; never "released" on its own. |

The **Go module version is the release**. There is no binary artifact — Go
consumers pull the tag directly (`go get github.com/Vikasa2M/openits-models@vX.Y.Z`),
so cutting a release is: land the changes, then merge the release PR that
release-please maintains (see [Cutting a release](#cutting-a-release)); that
merge writes the changelog and pushes the tag. The module path is the repo
URL, so the tag is immediately consumable.

## Go module semver

We follow [Semantic Versioning](https://semver.org/):

- **v0.x (current).** Pre-1.0. The public Go API and the model contract may
  still change between minor versions. Breaking changes and new features
  bump the **minor** (`0.7.0 → 0.8.0`); fixes bump the **patch**
  (`0.8.0 → 0.8.1`). This signals to consumers that the standard is
  still stabilizing.
- **v1.0.0 and beyond.** Once the model is stable, `v1.0.0` commits to the
  usual semver guarantees. A breaking Go API change then requires a new major
  **and** a `/v2` module path (Go's [semantic import
  versioning](https://go.dev/ref/mod#major-version-suffixes)) — deliberately
  expensive, which is why we stay on v0.x until the contract has settled.

## YANG revisions vs. the module version

A module's `revision` date is *not* the release version. It records when that
module's contract last changed. So a single `v0.8.0` release may contain
modules at many different revision dates — that is expected. The tag versions
*the collection*; the revision versions *each module*.

This is the same split OpenConfig uses: one repository-wide release for the
set of models that work together, with each module carrying its own version
inside it. Modules are not released separately. Shared types and the common
events reach every service, so separate releases would still move together,
and a consumer would then need a published table of which module versions work
with which. A break in one service after 1.0 is carried by a new proto package
and `ce-type` major for that service (see
[Extension model](06-extension-model.md)), not by a new major for the whole
repository.

## What a revision pins

Every event names its schema in `ce-dataschema` as `<module>/<revision>`, so a
revision has to determine the bytes on the wire, not only the YANG. The
snapshot under `schema-registry/<module>/<revision>/` pins all of it:

| File | Pins | Present for |
|------|------|-------------|
| `schema.yang` | the module source | every module |
| `schema.proto` | the messages the module itself declares, as a compilable file; messages sibling modules declare in the same generated file are left out | modules with root messages (live notifications, or a config/state tree) |
| `schema.shape` | the canonical wire shape of everything those messages reach, shared types included: types by full name, fields in number order, comments and declaration order dropped | the same modules |
| `schema.json` | the JSON Schema of each live notification, shared definitions inlined | modules with live notifications |

`schema.shape` and `schema.proto` are rendered from what `protoc` compiles
(`scripts/wire-shapes.sh`, part of `make gen`, writing `api/shapes/`), not from
the YANG. So the shape moves whenever the wire moves, whatever caused it:

- a YANG edit;
- a change to the YANG-to-proto mapping in `tools/yang-proto-gen`;
- a change to a shared message or enum the module reaches through an import,
  such as `openits/types/v1/types.proto` or the service's `types.proto`.

`make check-revisions` compares the live module against the snapshot of its
current revision: `schema.yang`, `schema.shape` and the rebuilt `schema.json`
must all be identical. A change that moves any of them needs a new revision,
**including a change that leaves the module's YANG untouched**. Its revision
statement says what moved on the wire and why. A generator change that alters
the output for many modules therefore bumps all of them; the revision log is
where a consumer reads that the meaning of their payload changed.

The mapping itself is a versioned contract: the
[YANG-to-wire mapping specification](wire-mapping-spec.md), pinned by test
vectors the generator must reproduce byte for byte. A change to the mapping
is a change to that specification, made in the same change set as the
revisions of every module whose output it alters.

Released snapshots are immutable. `make check-revision-history` fails a PR
that edits or removes any file of a snapshot on `main`. It also fails one that
adds a file to such a snapshot, unless the file is an artifact type introduced
after the snapshot was taken. Today that is only `schema.shape`, which
`scripts/backfill-shapes.sh` adds only after rendering the module's shape at
the commit that took the snapshot and at every release since, and finding one
shape throughout. Every snapshot must also be complete (`schema.yang`,
`MANIFEST.json`, `README.md`, plus `schema.proto` and `schema.json` for a
module with live notifications), so a snapshot is written whole or not at all.

`schema-registry/index.json` carries these facts per snapshot under
`snapshots.<module>.<revision>`:

- `current`: whether this is the revision this release's producers compile;
- `ceTypes`: listed for the current revision;
- `shapeDigest`: the sha256 of `schema.shape`;
- `messages`: each root message's own closure digest.

A consumer reads these instead of recomputing them. The conformance kit uses
them to check that each event's `ce-dataschema` names a published snapshot,
and that the payload message the producer compiled has the shape that snapshot
pins.

### Revisions that shipped more than one wire shape

Before `schema.shape`, nothing tied a revision to the generated proto, and
these module revisions were carried by releases whose producers compiled
different wire shapes. Their snapshots are immutable and cannot be corrected.
A consumer that keys behavior on one of them must also know the release it
came from.

| Module revision | Shapes, by release |
|---|---|
| `openits-common-fault-events` 2026-07-21 | v0.2.1–v0.5.0 \| v0.6.0 |
| `openits-common-mode-events` 2026-07-21 | v0.2.1–v0.5.0 \| v0.6.0 |
| `openits-common-comm-health-events` 2026-09-02 | v0.5.0 \| v0.6.0 |
| `openits-ess-events` 2026-07-21 | v0.2.1–v0.5.0 \| v0.6.0 |
| `openits-traffic-sensor-events` 2026-07-21 | v0.2.1–v0.5.0 \| v0.6.0 |
| `openits-cctv-events` 2026-08-05 | v0.3.0–v0.5.0 \| v0.6.0 |
| `openits-ramp-metering-events` 2026-08-05 | v0.3.0–v0.5.0 \| v0.6.0 |
| `openits-rsu-events` 2026-08-05 | v0.3.0–v0.5.0 \| v0.6.0 |
| `openits-zone-occupancy-events` 2026-09-05 | v0.5.0 \| v0.6.0 |
| `openits-cctv` 2026-08-05 | v0.3.0–v0.5.0 \| v0.6.0–v0.7.0 |
| `openits-ess` 2026-08-05 | v0.3.0–v0.5.0 \| v0.6.0–v0.7.0 |
| `openits-traffic-sensor` 2026-08-05 | v0.3.0–v0.5.0 \| v0.6.0–v0.7.0 |
| `openits-ramp-metering` 2026-08-05 | v0.3.0 \| v0.4.0–v0.5.0 \| v0.6.0–v0.7.0 |
| `openits-perception` 2026-08-15 | v0.3.0 \| v0.4.0–v0.5.0 |
| `openits-rsu` 2026-08-05 | v0.3.0 \| v0.4.0–v0.5.0 |

The v0.6.0 split is the field-presence change in the
[v0.6.0 migration note](migration/v0.6.0.md#1-field-presence-scalars-became-optional):
the YANG did not change, so no revision moved, but scalar leaves gained proto3
`optional`. The bytes of a set, non-zero field are the same, but absent and
zero became distinguishable and the generated Go fields became pointers. The
earlier splits in the service config/state modules are generated enum and
message changes on the state side, which no snapshot pinned at the time.
Every revision current today has had one shape since it was snapshotted.

### `openits-version`

Every first-party module carries an `openits-types:openits-version` semver
stamp, the counterpart of `openconfig-version`. `make check-revision-history`
enforces it against the base branch, as OpenConfig's CI enforces
`openconfig-version`:

- A module whose newest revision changed must raise `openits-version`.
- The raise is sized by the module's wire shape diff: a removed or altered
  field or enum value is breaking (MAJOR from 1.0 on; MINOR before it, since
  a 0.x version may break on MINOR); a new field, message or enum value needs
  at least MINOR; no wire change allows PATCH. A YANG-only change that alters
  meaning without altering the wire (a tighter range or `must`) is sized in
  review.
- MAJOR matches the `ce-type` major of every event the module defines: a 0.x
  or 1.x module emits `.v1`, a 2.x module `.v2`.

## Wire compatibility

`buf breaking` (config in `buf.yaml`, gate in CI) compares every PR against
`main` under the `WIRE_JSON` ruleset. Field numbers are locked in
`field-numbers.yaml`. Practical rules for editing generated-from-YANG protos:

- Add new fields/messages/enums by **appending** — never renumber, never
  reuse a retired field number, never insert into the middle of an enum
  (ygot ordinals are positional).
- `identityref` leaves render as proto **strings** carrying
  `defining-module:identity-name`, so **adding** new identities is
  non-breaking — but **renaming** an identity, or moving it to a different
  module, changes the string every conforming producer emits and is
  **wire-breaking**, exactly as if a field had been renumbered. (Some older
  module revision histories describe identityref renames as "wire-neutral";
  those notes are frozen in their revision statements and are superseded by
  this rule.)

If `buf breaking` flags a genuinely necessary wire break, that is a
minor-version bump on the v0.x line (or a major + `/v2` post-1.0).

## Migration notes

A release that changes the wire contract carries a migration note under
[`docs/migration/`](migration/), written before the release is cut and linked
from the changelog entry. The changelog says *what* changed; the migration
note says what a consumer has to do about it, with the old-to-new mapping for
every renamed or retyped value in one place rather than scattered across
module revision statements.

Pre-1.0 this is the norm rather than the exception: the project takes wire
breaks deliberately while they are cheap (see
[Extension model](06-extension-model.md) for what they cost after 1.0), so
each such release owes consumers one document that collects them.

- [v0.6.0](migration/v0.6.0.md) — field presence, seven enum-to-identity
  conversions, DMS activation errors, scalar retypes, enum name un-prefixing.
- [v0.7.0](migration/v0.7.0.md) — `sequence-epoch` on every event;
  `mode-changed` field names `previous-mode` and `mode`.
- [v0.8.0](migration/v0.8.0.md) — one fault shape, device health and
  `observed-at`, mode/value binding, signal-control operation split.
- [v0.9.0](migration/v0.9.0.md) — signal-control pedestrian detector table,
  detector event field renames, unit-suffixed duration and percentage names,
  preemption force-off split, unit alarm status 1 rename.

## Cutting a release

Releases are driven by **[release-please](https://github.com/googleapis/release-please)**
off [Conventional Commits](https://www.conventionalcommits.org/). You do not
tag or edit the changelog by hand for the normal cadence.

1. Land PRs on `main` with Conventional Commit messages — `fix:` (→ patch),
   `feat:` (→ minor), and `feat!:` / a `BREAKING CHANGE:` footer (→ minor while
   pre-1.0; major once ≥ 1.0). `docs:`/`chore:`/`test:`/`refactor:` don't
   trigger a release on their own.
2. [`release-please.yml`](../.github/workflows/release-please.yml) watches
   `main` and maintains a standing **release PR** that bumps the version in
   [`.release-please-manifest.json`](../.release-please-manifest.json) and
   rewrites [`CHANGELOG.md`](../CHANGELOG.md) from those commits.
3. When the changelog and version look right, **merge the release PR**. Merging
   it is what tags `vX.Y.Z` and creates the GitHub Release with generated notes;
   the `upload-assets` job in the same workflow then builds the curated bundle
   from the tagged tree and attaches it (below): `.tar.gz`, `.zip`, and
   `SHA256SUMS`.

Maintainers do not hand-tag normal releases: merging the release PR is the
only routine way a version is cut.

The release PR is authored by the release-please bot. Depending on the org's
GitHub Actions policy for bot/first-time-contributor PRs, the CI runs on it may
land in `action_required` and need a maintainer to approve them before they
run — approve from the PR's Checks tab or via
`gh api -X POST repos/<owner>/<repo>/actions/runs/<id>/approve`.

Config lives in [`release-please-config.json`](../release-please-config.json)
(`release-type: go`, `bump-minor-pre-major`).

**Only model-contract changes drive versions.** The config's `exclude-paths`
ignores commits that touch nothing but `.claude/`, `.github/`, `docs/`,
`scripts/`, `tools/`, or root meta/build files (`AGENTS.md`, `CLAUDE.md`,
README, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, LICENSE, NOTICE, Makefile,
buf configs, `.gitignore`, the changelog, release-please's own files). The
tracked surface is what a consumer's version pin actually protects: `yang/`,
`api/`, `bindings/`, `pkg/`, `schema-registry/`, `field-numbers.yaml`, and
`go.mod`/`go.sum`. Note the sources-vs-artifacts distinction: a generator fix
in `tools/` releases not because `tools/` changed but because its regenerated
output under `api/`/`pkg/` did.

### Pinning an exact version

release-please derives its baseline from
[`.release-please-manifest.json`](../.release-please-manifest.json), **not** from
git tags. If the manifest names a version that has no corresponding tag,
release-please assumes that version already shipped and proposes the *next* one.
So the manifest — not any tag — is the source of truth for "where we are."

To pin an exact next version (a deliberate jump), put a `Release-As: X.Y.Z`
footer on a commit that touches the tracked surface; release-please then
proposes exactly that version in the release PR. An empty commit, or one that
touches only excluded paths, does **not** work: `exclude-paths` drops commits
with no tracked files before release-please reads their footers.

### Emergency manual tag (maintainers only)

This is not a release path for normal use. If release-please itself is broken
and a release cannot wait, a maintainer can push a semver tag by hand: the
[`release`](../.github/workflows/release.yml) workflow re-runs the gate,
creates the Release from the matching `CHANGELOG.md` section, and attaches the
bundle (`-rc` / pre-release suffixes are marked pre-release automatically).
The changelog section and the manifest must already name that version, or
release-please will propose it again afterwards.

```
git tag vX.Y.Z
git push origin vX.Y.Z
```

release-please tags with `GITHUB_TOKEN`, which does not trigger this tag-based
workflow, so the automated and manual paths never double-fire.

## Release artifacts

The **tag itself is the release** — Go consumers pull it directly with
`go get`, and GitHub auto-attaches a full-repo source archive.

For non-Go / other-language implementers, the release workflow also attaches a
**curated bundle** — the models and specs without the Go-generated `pkg/`,
tooling, tests, or docs:

- `openits-models-vX.Y.Z.zip` and `openits-models-vX.Y.Z.tar.gz`, each
  containing `yang/`, `api/proto/` (minus the ygot-generated extension tree),
  `schema-registry/`, the `bindings/` tree (the NATS reference profile plus its
  `bindings/nats/asyncapi.yaml`), `CHANGELOG.md`, `LICENSE`, and `NOTICE` under a
  top-level `openits-models-vX.Y.Z/` directory. (Exact set:
  [`scripts/build-release-bundle.sh`](../scripts/build-release-bundle.sh).)
- `SHA256SUMS` — checksums for both archives, for consumers who vendor them.

There are no compiled binaries to publish (this repo builds none) and no
package-registry publishing (npm/PyPI) until a consumer needs it.
