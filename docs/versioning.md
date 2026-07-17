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
consumers pull the tag directly (`go get github.com/openits/openits-models@vX.Y.Z`),
so cutting a release is: land the changes, update the changelog, push a tag.

## Go module semver

We follow [Semantic Versioning](https://semver.org/):

- **v0.x (current).** Pre-1.0. The public Go API and the model contract may
  still change between minor versions. Breaking changes bump the **minor**
  (`0.1.0 → 0.2.0`); backward-compatible additions and fixes bump the
  **patch** (`0.1.0 → 0.1.1`). This signals to consumers that the standard is
  still stabilizing.
- **v1.0.0 and beyond.** Once the model is stable, `v1.0.0` commits to the
  usual semver guarantees. A breaking Go API change then requires a new major
  **and** a `/v2` module path (Go's [semantic import
  versioning](https://go.dev/ref/mod#major-version-suffixes)) — deliberately
  expensive, which is why we stay on v0.x until the contract has settled.

## YANG revisions vs. the module version

A module's `revision` date is *not* the release version. It records when that
module's schema last changed and is enforced by `make check-revisions`: if a
module's content differs from its last snapshot under
`schema-registry/<module>/<revision>/`, CI fails until the revision is bumped
(or the change is deliberately re-snapshotted). So a single `v0.2.0` release
may contain modules at many different revision dates — that is expected. The
tag versions *the collection*; the revision versions *each module*.

## Wire compatibility

`buf breaking` (config in `buf.yaml`, gate in CI) compares every PR against
`main` under the `WIRE_JSON` ruleset. Field numbers are locked in
`field-numbers.yaml`. Practical rules for editing generated-from-YANG protos:

- Add new fields/messages/enums by **appending** — never renumber, never
  reuse a retired field number, never insert into the middle of an enum
  (ygot ordinals are positional).
- `identityref` leaves render as proto **strings**, so adding new identities
  is non-breaking.

If `buf breaking` flags a genuinely necessary wire break, that is a
minor-version bump on the v0.x line (or a major + `/v2` post-1.0).

## Cutting a release

1. Ensure `main` is green (all CI jobs pass).
2. Move the `## [Unreleased]` items in [`CHANGELOG.md`](../CHANGELOG.md) into a
   new `## [X.Y.Z] - YYYY-MM-DD` section, and update the compare/tag links at
   the bottom.
3. Commit that on `main`.
4. Tag and push:
   ```
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```
5. The [`release`](../.github/workflows/release.yml) workflow re-runs the build
   + tests, then creates a GitHub Release whose body is the matching CHANGELOG
   section. Tags with a pre-release suffix (e.g. `v0.2.0-rc.1`) are marked as
   pre-releases automatically.

## ⚠️ Prerequisite: module path must match the repo URL

The Go module path in `go.mod` is `github.com/openits/openits-models`, but the
repository currently lives at `github.com/Vikasa2M/openits-models`. For
`go get github.com/openits/openits-models@vX.Y.Z` to resolve, **one of these
must be true before the first tag is consumable**:

- the repo is moved/mirrored to `github.com/openits/openits-models`, **or**
- a vanity import redirect is served at that path, **or**
- `go.mod`'s module path is changed to match the actual repo location (and
  every consumer's import updated in lockstep).

Tagging works regardless, but downstream `go get` of the current module path
will fail until this is resolved.
