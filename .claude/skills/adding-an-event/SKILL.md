---
name: adding-an-event
description: The workflow for adding or changing a YANG notification (event) — layering rules, kind identities vs enums, subject naming, fixtures, and the AsyncAPI/ce-type regeneration. Use this whenever a task adds a notification, adds fields to an existing event, introduces a new event kind, or touches any *-events.yang module — the layering and classification rules here are enforced by dedicated CI gates and differ from ordinary model edits.
---

# Adding or changing an event

Events are the repo's wire-visible pub/sub surface: each notification
becomes a proto message, a JSON Schema registry snapshot, an AsyncAPI
channel, and a ce-type. The conventions below keep that surface coherent.
Read `bindings/nats/README.md` for how events ride the wire (subjects,
CloudEvents envelope, deterministic ce-id) and `docs/ce-id-spec.md` for
event identity.

## Where the event lives

- Service-specific domain events go in that service's `*-events.yang`
  module (e.g. one consolidated events module per service).
- Faults and mode changes are NOT service events — they are emitted via
  the shared `openits-common-fault-events` / `openits-common-mode-events`
  notifications, classified by a `kind` identityref. Adding "my-service
  fault-raised" as a new notification is wrong; add a new fault `kind`
  identity instead.

## Layering (CI: check-events-layering)

Events modules may import ONLY: `openits-types`, `ietf-yang-types`,
`openits-nema-common`, and `*-types` modules. Never the service core. If
the event needs a typedef or grouping that lives in the core module, move
it to the service's `-types` module first (note: moving an enumeration
typedef between modules renames its generated Go type — source-level
breaking for Go consumers, wire-neutral; mention it in the PR).

## Classification: kind identities, not parallel enums

If the notification is classified by a `kind` identityref, do NOT add an
enum that mirrors the kind hierarchy (a "reason" enum whose values map
1:1 to kinds). The identity hierarchy IS the classification — parallel
enums drift and got deliberately removed repo-wide. Enums are fine for
truly orthogonal axes (what-was-selected, source-class) that don't mirror
kinds.

## Naming

The notification's last name token becomes the subject's `{event}` token:
`openits.{region}.{agency}.{agency-unit}.{service}.{controller-id}.{event}`.
Use hyphen-form past-tense event names (`fault-raised`,
`plan-applied`-style). `make check-naming` gates the grammar.

## Fixtures

RFC 7951 keys a top-level notification by its DEFINING module — for a
consolidated events module the fixture's top-level member is
`<events-module-name>:<notification>`, not the service core module. Add a
`valid-*` fixture per new notification and `invalid-*` fixtures for any
constraint. Mandatory event fields are additionally checked by
`scripts/check-notif-mandatory.py`.

## Pipeline

Same as `extending-a-model` (revision statement, `make gen`,
`scripts/update-schema-registry.sh`, gates), plus event-specific checks:

```
make gen                 # includes asyncapi regen — one channel per ce-type
make asyncapi-check      # drift gate for bindings/nats/asyncapi.yaml
make check-events-layering
go run ./tools/conformance -driver mock -kind <service-kind>
```

New event fields append to `field-numbers.yaml` (never renumber). A new
notification means the conformance mock should emit it if the per-kind
checks assert on it — extend the mock, don't skip the check.
