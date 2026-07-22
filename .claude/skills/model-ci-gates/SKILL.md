---
name: model-ci-gates
description: How to run and interpret this repo's CI gates locally — check-gen drift, validate-yang/yanglint failures, buf breaking, field-number lock behavior, conformance kinds, and the naming/revision/layering checks. Use this whenever CI fails on a PR, whenever a make target errors and the cause isn't obvious, or before pushing to predict what CI will say. Also use it when a yanglint failure appears in a module you didn't touch — that cascade has a specific known cause.
---

# Running and interpreting the CI gates

Every CI job mirrors a Make target, so any red job reproduces locally.
Run the full gate with the targets below; `make all` covers most of it.

| CI job | Local command | What it proves |
|---|---|---|
| go-test | `go build ./... && go vet ./... && go test ./...` | tools + generated Go compile and pass tests |
| check-gen | `make check-gen` | committed generated artifacts match a fresh regen |
| buf | `buf lint` + `buf breaking --against .git#ref=<main-sha>` | proto style + no wire-breaking change vs main |
| yang-checks | `make check-revisions check-naming check-deviations check-augment-collisions check-events-layering validate-noi check-graduation` | governance: revisions, naming, layering, deviations |
| validate-yang | `make validate-yang` | every fixture in `yang/testdata/` gets its expected verdict from yanglint |
| yang-lint | `make yang-lint` | pyang --strict structural lint |
| conformance (×9) | `go run ./tools/conformance -driver mock -kind <kind>` | mock device satisfies the per-kind behavioral checks |

Conformance kinds: `asc`, `rsu`, `dms`, `ess`, `ramp-metering`,
`traffic-sensor`, `reversible-lane`, `perception`, `cctv`.

## Failure → diagnosis

**check-gen shows a diff.** Either you edited YANG and didn't commit the
regen, you hand-edited a generated file (never do this — fix the YANG or
the generator), or your local generator versions differ from the pins in
`.github/workflows/ci.yml` (protoc, protoc-gen-go, ygot). Re-run
`make gen`, inspect `git diff`, commit what belongs.

**validate-yang fails in modules you didn't touch.** Deterministic
cascade, not flakiness: a new `must` on a container whose leaf carries a
`default`, under a non-presence ancestor, is evaluated against the
implicit default tree of EVERY fixture. Diagnose by `git stash` +
re-run (confirms your change is the trigger), then run yanglint directly
on one failing fixture and read stderr — it names the must. Fix by adding
`presence` or conditioning the must. See `extending-a-model`.

**validate-yang fails on your own new fixture.** `invalid-*` fixtures
must fail with yanglint's data-violation exit code — a fixture that fails
to PARSE (wrong module prefix, RFC 7951 keyed by the wrong module) is a
broken test, not a passing one. Check the top-level member is prefixed by
the module that DEFINES the node.

**buf breaking fails.** You changed the wire contract: removed/renumbered
a field, or inserted an enum value mid-list (proto ordinals are
positional — append only). If the break is intended pre-1.0, it must be a
`feat!:` commit with a `BREAKING CHANGE:` footer; otherwise restructure
the change to be additive.

**field-numbers.yaml confusion.** This file is the wire lock
(message → field → tag). New fields append entries; deleted fields leave
it byte-identical because retired tags are tombstoned forever. If
check-gen shows unexpected churn HERE, the change renamed or moved proto
messages — treat as a wire-compat question, not noise.

**check-naming fails.** Subjects use the seven-token grammar
`openits.{region}.{agency}.{agency-unit}.{service}.{controller-id}.{event}`
and the guard also rejects legacy URN/ce-type shapes. The error output
names the offending string.

**check-events-layering fails.** An `-events` module imported a service
core module. Events may import only `openits-types`, `ietf-yang-types`,
`openits-nema-common`, and `*-types` modules — move the shared
typedef/grouping into the service's `-types` module instead.

**check-revisions fails.** Duplicate same-date revision statements or a
module changed without a new revision. One revision per change set.

**conformance fails.** The mock driver plus per-kind checks in
`tools/conformance/tests/` — read the named test; if you added model
surface the mock must now populate, extend the mock device data rather
than weakening the test.

**Docker/validate-yang environment.** yanglint runs via a digest-pinned
Docker image (`scripts/validate-yang.sh`). Override with `YANGLINT_IMAGE`
only for local experiments; never commit an override.
