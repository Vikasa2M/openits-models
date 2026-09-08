# Zone occupancy — carrying live state over a message fabric

`openits-zone-occupancy` models live occupancy as operational state
(`zones/zone`, `config false`) and declares one notification,
`zone-occupancy-interval-report`, for periodic aggregates. There is no
notification carrying the live condition, so a consumer on a message fabric
has no vendor-neutral way to receive it.

This is a proposal, not a decision. It argues that the gap is real, that the
repo has already settled the shape twice in other domains, and that the fix is
additive.

## Why this is live

A downstream platform needed live occupancy on a NATS fabric, found nothing in
the model, and defined a private non-OpenITS JSON event for it. That works, and
its keys mirror the `zones/zone` leaves character for character — but it leaves
the highest-volume signal in the domain outside the standard.

The acquisition path makes the transport binding the load-bearing interface.
Today a collector polls NTCIP/vendor APIs and converts to the model. Later a
vendor publishes to the fabric directly. **Those two must produce the same
event stream**, or every consumer is rewritten at the transition. That is
precisely what a standard is for, and it is the part currently unspecified.

## The repo has already answered this twice

OpenITS has a consistent two-notification convention per domain:

| Shape | Purpose |
|---|---|
| `<x>-interval-report` | Periodic aggregate over a closed window |
| `<x>-state-changed` | The live condition, emitted on transition |

Both instances exist today:

- `openits-traffic-sensor-events:queue-state-changed`
- `openits-work-zone-events:zone-state-changed`

And `queue-state-changed` states the doctrine in its own description:

> A queue-detection zone changed state (queue started, cleared, or its
> persisted duration was re-reported). **The `traffic-sensor/queues` container
> is the digital-twin rollup of these events.**

That is the architecture: **change events are primary, the `config false`
container is derived from them.** Zone occupancy has the interval report and is
missing its counterpart. The gap is not a missing "reading" concept — it is the
missing half of a pattern the repo already uses.

## Recommendation

Add `zone-occupancy-changed` to `openits-zone-occupancy-events`, modeled on
`queue-state-changed`:

1. **New identity** `zoc-zone-occupancy-changed` under the existing
   `zone-occupancy-event-kind` base in `openits-zone-occupancy-types`.
2. **Extract a grouping.** The `zones/zone` leaves are currently inline in
   `openits-zone-occupancy`. Move them to
   `openits-zone-occupancy-types:zone-occupancy-state`, and have **both** the
   state container and the new notification `uses` it.
3. **Notification body**: `kind` identityref, `uses openits-types:event-header`,
   `zone-id`, `uses zone-occupancy-state`.
4. **Restate the doctrine** in the description: `zone-occupancy/zones` is the
   digital-twin rollup of these events.

Step 2 is not new ground. `openits-traffic-sensor` did exactly this: the
`queue-zone-state` grouping lives in the types module and is used by both the
state container (`openits-traffic-sensor/2026-08-05:574`) and the notification
(`openits-traffic-sensor-events/2026-07-21:175`), recorded in its revision as
an "additive mirror fix ... converged on the shared grouping." Zone occupancy
should converge the same way.

## Why `-changed` and not `-reading`

A notification is for a noteworthy occurrence. A periodic readback is telemetry,
and modeling telemetry as a notification is the objection a standards reviewer
will raise first. Three reasons the change framing survives it:

- **It already accommodates re-reporting.** `queue-state-changed` explicitly
  covers "or its persisted duration was re-reported." A device or collector that
  re-announces an ongoing condition is within the contract.
- **It is acquisition-neutral.** A polling collector emits on diff; a vendor
  emits on transition. Same stream, same model, across exactly the migration
  described above.
- **Naming it `reading` would standardize today's implementation.** The model
  should describe the phenomenon, not the fact that we currently reach it by
  polling. A `reading` notification would bake the collector's polling cadence
  into a public standard and be very hard to walk back once vendors implement.

## Alternatives considered

**YANG-Push (RFC 8641) on the `zones` container.** In a NETCONF/RESTCONF
deployment this is the correct answer to "stream this state," and it needs no
new model — a `config false` container is already a valid subscription target.
It is the wrong fit here: it presumes a datastore and subscription negotiation,
and the fabric is pub/sub CloudEvents with neither. Worth recording explicitly,
because a reviewer will ask why the standard mechanism was not used.

**Leave the transport private to each implementer.** The phenomenon is already
modeled and already tagged with an ARC-IT flow (*ITS Roadway Equipment → TMC*).
Standardizing the data but not the transport means every implementer invents an
incompatible binding for the most frequent message in the domain — the failure a
standard exists to prevent.

**Do nothing.** Defensible only if live occupancy is considered out of scope for
the fabric. It is not: the interval report is already there, and it is the
coarser of the two signals.

## Open questions for review

1. **`measured-at` vs `occurred-at`.** `event-header` carries `occurred-at`;
   `zones/zone` carries `measured-at`. For a change notification these can
   legitimately differ — a poll at T detects a transition that happened at T−δ.
   Decide whether the grouping keeps `measured-at`, or whether it collapses into
   the header, and document which one a consumer should trust for ordering.
2. **Cadence and de-duplication.** Does the model constrain re-report interval,
   or leave it to a deployment profile? `queue-state-changed` leaves it open;
   consistency argues for the same, but occupancy is higher-volume.
3. **Doctrine placement.** The readback-representability text in
   `openits-zone-occupancy` (presence without a count, a straddling vehicle
   exceeding capacity, a class breakdown that does not sum) governs the
   notification equally. Restate on the notification, or reference it?
4. **Identity naming.** Confirm `zoc-zone-occupancy-changed` matches the `zoc-`
   convention set by `zoc-zone-occupancy-interval-report`.

## Cost and compatibility

Additive and backward compatible. No wire change to
`zone-occupancy-interval-report`. The grouping extraction is a mechanical
refactor of the existing state container with no semantic change. Downstream,
a platform already emitting a private JSON mirror of these leaves converts by
swapping its decoder, with its storage schema unchanged.
