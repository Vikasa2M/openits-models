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
so cutting a release is: land the changes, update the changelog, push a tag.
The module path is the repo URL, so the tag is immediately consumable.

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
   from the tagged tree and attaches it (below). Verified on `v0.1.0`: the
   Release carried `.tar.gz`, `.zip`, and `SHA256SUMS`.

The release PR is authored by the release-please bot. Depending on the org's
GitHub Actions policy for bot/first-time-contributor PRs, the CI runs on it may
land in `action_required` and need a maintainer to approve them before they
run — approve from the PR's Checks tab or via
`gh api -X POST repos/<owner>/<repo>/actions/runs/<id>/approve`.

Config lives in [`release-please-config.json`](../release-please-config.json)
(`release-type: go`, `bump-minor-pre-major`).

**Only model-contract changes drive versions.** The config's `exclude-paths`
ignores commits that touch nothing but `.github/`, `docs/`, `scripts/`,
`tools/`, or root meta/build files (README, LICENSE, Makefile, buf configs,
the changelog, release-please's own files). The tracked surface is what a
consumer's version pin actually protects: `yang/`, `api/`, `bindings/`,
`pkg/`, `schema-registry/`, `field-numbers.yaml`, and `go.mod`/`go.sum`.
Note the sources-vs-artifacts distinction: a generator fix in `tools/`
releases not because `tools/` changed but because its regenerated output
under `api/`/`pkg/` did.

### Pinning an exact version / bootstrap

release-please derives its baseline from
[`.release-please-manifest.json`](../.release-please-manifest.json), **not** from
git tags. If the manifest names a version that has no corresponding tag,
release-please assumes that version already shipped and proposes the *next* one.
So the manifest — not any tag — is the source of truth for "where we are."

To pin an exact next version (the first release, or any deliberate jump), land
an **empty commit with a `Release-As: X.Y.Z` footer**; release-please then
proposes exactly that version in the release PR. This is how `v0.1.0` was
actually cut:

```
git commit --allow-empty -m "chore: release 0.1.0" -m "Release-As: 0.1.0"
```

A hand-pushed semver tag remains available as an escape hatch: the
[`release`](../.github/workflows/release.yml) workflow re-runs the gate, creates
the Release from the matching `CHANGELOG.md` section, and attaches the bundle
(`-rc` / pre-release suffixes are marked pre-release automatically):

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
