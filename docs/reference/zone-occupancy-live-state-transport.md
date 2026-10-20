# Zone occupancy — carrying live state over a message fabric

**Status: accepted and shipped.** This is the design record for
`zone-occupancy-changed`, the notification that carries live zone
occupancy as events. It records what the model does and why.

`openits-zone-occupancy` models live occupancy as operational state
(`zones/zone`, `config false`). Its companion events module,
`openits-zone-occupancy-events`, declares two notifications: the
periodic aggregate `zone-occupancy-interval-report`, and
`zone-occupancy-changed`, the live condition emitted on transition.

## The problem it solved

Before `zone-occupancy-changed`, the only zone-occupancy notification was
the interval report. A consumer on a message fabric that needed live
occupancy had no vendor-neutral event to receive, so it had to define a
private, non-OpenITS event mirroring the `zones/zone` leaves. That left
the highest-volume signal in the domain outside the standard.

The acquisition path makes the transport binding the load-bearing
interface. A collector may poll NTCIP or vendor APIs and convert to the
model; a vendor may later publish to the fabric directly. **Those two
must produce the same event stream**, or every consumer is rewritten at
the transition. That is precisely what a standard is for.

## The convention it follows

OpenITS has a consistent two-notification convention per domain:

| Shape | Purpose |
|---|---|
| `<x>-interval-report` | Periodic aggregate over a closed window |
| `<x>-state-changed` / `<x>-changed` | The live condition, emitted on transition |

Three instances exist:

- `openits-traffic-sensor-events:queue-state-changed`
- `openits-work-zone-events:zone-state-changed`
- `openits-zone-occupancy-events:zone-occupancy-changed`

`queue-state-changed` states the doctrine in its own description:

> A queue-detection zone changed state (queue started, cleared, or its
> persisted duration was re-reported). **The `traffic-sensor/queues`
> container is the digital-twin rollup of these events.**

That is the architecture: **change events are primary, the `config
false` container is derived from them.** Zone occupancy had the interval
report and was missing its counterpart; `zone-occupancy-changed` is that
missing half.

## What shipped

1. **Kind identities.** `openits-zone-occupancy-types` declares the
   sub-base `zoc-occupancy-change-event-kind` under
   `zone-occupancy-event-kind`, with three transitions:
   `zoc-zone-occupied`, `zoc-zone-vacated` and
   `zoc-zone-occupancy-updated` (the region stayed occupied and its
   count or set of present classes changed). The notification's `kind`
   is typed to the sub-base, so the interval report's kind is not a
   valid value. A delta that maps to none of the three — confidence
   drifting, `observed-at` ticking — is not an event.
2. **A shared grouping.** The live leaves live in
   `openits-zone-occupancy-types:zone-occupancy-state`, used by both the
   `zones/zone` state list and the notification, so the two mirror each
   other leaf for leaf. On the notification `presence` is refined to
   mandatory, because every transition kind is defined in terms of it.
3. **Notification body.** `kind`, `uses openits-types:event-header`,
   `zone-id`, `uses zone-occupancy-state`. One region per event, so a
   consumer keys its twin on (`source-device-id`, `zone-id`). No
   `wire-source`: the transition is inferred or natively produced,
   never decoded from a device protocol.
4. **The doctrine is restated** in the notification's description:
   `zone-occupancy/zones` is the digital-twin rollup of these events.

The grouping extraction is the same move `openits-traffic-sensor` made:
its `queue-zone-state` grouping lives in `openits-traffic-sensor-types`
and is used by both the `queues` state container in
`openits-traffic-sensor` and the `queue-state-changed` notification.

## Why `-changed` and not `-reading`

A notification is for a noteworthy occurrence. A periodic readback is
telemetry, and modeling telemetry as a notification is the objection a
standards reviewer raises first. Three reasons the change framing
survives it:

- **It accommodates re-reporting.** An emitter MAY re-report an
  unchanged condition with the same kind and payload as the most recent
  transition; for a consumer maintaining a twin the re-report is
  idempotent.
- **It is acquisition-neutral.** A polling collector emits on diff; a
  vendor emits on transition. Same stream, same model, across exactly
  the migration described above.
- **Naming it `reading` would standardize one implementation.** The
  model describes the phenomenon, not the fact that it is sometimes
  reached by polling. A `reading` notification would bake a collector's
  polling cadence into a public standard and be very hard to walk back
  once vendors implement.

## How the review questions were settled

- **`observed-at` vs `occurred-at`.** `observed-at` stays beside the
  grouping on the `zones/zone` state list, not in the notification.
  On the event, `occurred-at` (from `event-header`) is when the
  transition took effect as far as the emitter knows: a native device
  reports the instant, a polling collector reports the poll that
  detected it. `occupied-since` is the device-asserted onset where the
  device knows it, so a consumer recovers the true dwell even from a
  polled transition. Consumers order by the event header.
- **Cadence and de-duplication.** Not modeled. Whether and how often to
  re-report is reporting policy, consistent with
  `queue-state-changed`.
- **Doctrine placement.** The readback-representability rule (nothing
  is constrained against anything else, so a misbehaving device stays
  visible) is carried by the shared grouping, so it governs the state
  list and the notification equally.
- **Identity naming.** The `zoc-` prefix follows
  `zoc-zone-occupancy-interval-report`. Rather than one
  `zoc-zone-occupancy-changed` identity, the shipped design uses three
  transition identities under a change sub-base, so `kind` carries the
  direction and there is no previous-state leaf.

## Alternatives considered

**YANG-Push (RFC 8641) on the `zones` container.** In a NETCONF/RESTCONF
deployment this is the correct answer to "stream this state," and it
needs no new model — a `config false` container is already a valid
subscription target. It is the wrong fit for the reference binding: it
presumes a datastore and subscription negotiation, and the fabric is
pub/sub CloudEvents with neither.

**Leave the transport private to each implementer.** The phenomenon was
already modeled and already tagged with an ARC-IT flow (*ITS Roadway
Equipment → TMC*). Standardizing the data but not the transport means
every implementer invents an incompatible binding for the most frequent
message in the domain — the failure a standard exists to prevent.

**Do nothing.** Defensible only if live occupancy were out of scope for
the fabric. It is not: the interval report was already there, and it is
the coarser of the two signals.

## Compatibility

The change was additive: no wire change to
`zone-occupancy-interval-report`, and the grouping extraction was a
mechanical refactor of the state list with no semantic change. A
platform that had been emitting a private JSON mirror of these leaves
converts by swapping its decoder, with its storage schema unchanged.
