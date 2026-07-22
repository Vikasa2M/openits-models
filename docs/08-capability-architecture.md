# Capability-first module architecture

This document records the organizing principle for the openits module
family: **model by capability (function), not by device type.** A
physical device is described by the *union* of the capability modules
it implements, composed onto a thin device profile.

It is written in the same shape as [Design decisions](04-design-decisions.md):
what we chose, what we considered, why, and what we'd revisit.

> **Status: this is the target architecture, partially realized.** The
> principle is settled and the first capabilities ship today (see
> [Migration](#migration) for exactly what is built). Several capability
> and thin-profile modules named below are *planned*, not yet in `yang/`;
> they are marked **(planned)**. The service families that have not been
> re-laid yet still ship as the monolithic service cores described in
> [the data model](data-model.md) — that doc describes the tree as it is
> today; this doc describes the shape it is moving toward.

## What we chose

Four layers, organized by *function* rather than by *device type*:

| Layer | Modules | Role |
|-------|---------|------|
| **Foundation** | `openits-types` | Cross-service scalars, identities, and shared groupings (`event-header`, `fault-entry`, `geo-location`, `device-hardware`, `comm-link-state`, `wire-source`). |
| **Platform** | `openits-device-diagnostics`, and the platform groupings in `openits-types` | What *every* field device has regardless of function: identity, location, device hardware, communications health, compute diagnostics (CPU/memory/disk/temperature, uptime/restart), the active-fault inventory. |
| **Capability** | Built: `openits-v2x-radio`, `openits-v2x-messaging`, `openits-scms`, `openits-vehicle-detection`. Planned: `openits-signal-timing`, `openits-dms-display`, `openits-ramp-control`, `openits-incident-detection`, `openits-environmental-sensing`, `openits-reversible-lane-control`. | One coherent function each — the unit a vendor advertises ("this unit does SCMS", "this sensor does incident detection"). Versioned independently. |
| **Device profile** | Built as a thin, capability-composing profile: `openits-rsu`. Still monolithic service cores pending migration: `openits-signal-control`, `openits-dms`, `openits-ess`, `openits-traffic-sensor`, `openits-perception`, `openits-ramp-metering`, `openits-reversible-lane` (their thin-profile target names would be e.g. `openits-signal-controller`, `openits-ramp-meter`). | *Thin* — an identity plus `uses`/composition of the capabilities that device type has. Optional capabilities are gated by `feature` / `if-feature`. |

A device profile is a composition, not a monolith. An RSU:

```
container rsu {                                   // a singleton device, like every service
  container config { uses openits-platform:device-identity; }        // id, name, location
  container state { config false; uses openits-platform:device-identity; }  // applied mirror
  uses openits-v2x-radio:radios;                 // channels, radio-tech (DSRC / C-V2X)
  uses openits-v2x-messaging:broadcast;          // SPaT / MAP / TIM / BSM
  uses openits-scms:security;                    // SCMS certificate inventory
  uses openits-device-diagnostics:diagnostics;   // platform health
  uses openits-platform:faults;                  // active-fault inventory
  if-feature onboard-detection {                 // an RSU MAY do onboard analytics
    uses openits-vehicle-detection:detection;
  }
}
```

## What we considered

- **By device type (the status quo).** One self-contained module per
  device (`openits-rsu`, `openits-dms`, …), each redefining every
  function that device performs. This is where the model started, and
  it produced the 2,289-line `openits-rsu` mega-module and repeated
  copy-paste of device-diagnostics, comms-health, geo, and fault
  inventories across seven services.
- **One giant model with everything.** Rejected for the same reason
  OpenConfig rejected mega-modules: no independent versioning, an
  enormous review surface, and a namespace that never converges.
- **Augment-everything.** Compose device trees purely from capability
  modules that `augment` a bare device root. Powerful, but for *core*
  content it scatters one device's tree across many modules and makes
  the whole tree hard to read in one place. We keep `augment` for its
  existing role — *vendor* extension (Tier 2, see
  [Extension model](06-extension-model.md)) — not as the core
  composition mechanism.

## Why capability-first

1. **Functions cut across device types; device types don't cut across
   functions.** Compute diagnostics, comms health, GPS/time, and vehicle
   detection each recur on several device types. Modeling by device type
   forces a copy of each shared function per device (proven: the
   generic diagnostics block was inlined in RSU and needed by every
   service; per-approach vehicle analytics sat in RSU but is really the
   sensing services' function). Modeling by capability defines each once.

2. **It is the OpenConfig telemetry lesson applied to state.** OpenConfig
   models *functions* (`openconfig-bgp`, `openconfig-interfaces`) plus a
   generic *platform* (`openconfig-platform`); a device is the union of
   the features it implements. openits already does this for the *event*
   taxonomy (event-kind identities are functional). Capability-first
   makes the *state tree* mirror the event taxonomy — one organizing
   principle, not two.

3. **Independent versioning falls out.** A change to `v2x-messaging`
   does not bump `signal-timing`. Each capability carries its own
   revision and `openits-version`; each device profile pins the
   capabilities it composes.

4. **Conformance becomes capability-scoped.** "This unit conforms to
   `openits-v2x-radio@<rev>` and `openits-scms@<rev>`" is a precise,
   testable claim — closer to how vendors actually describe products
   than "it's an RSU."

## How YANG expresses the composition

Three mechanisms, each for a distinct case:

- **`grouping` + `uses`** — the default, for a capability a device
  profile owns in its own tree and namespace. The device tree stays
  readable in one place; the TSC owns both the capability and the
  profile. `openits-device-diagnostics` is the first worked example.
- **`feature` + `if-feature`** — for a capability a device *may or may
  not* have (onboard detection on an RSU; an integrated radio on a
  future signal controller). This is the idiomatic YANG answer to
  "can this device have that function?".
- **`augment`** — reserved for *optional / vendor* capabilities added
  without touching the core profile. Unchanged from the Tier-2
  extension model.

Rule of thumb: **core mandatory → grouping; core optional →
`if-feature`; vendor → augment.**

## Guardrails

- **Right altitude.** A capability is a coherent function a vendor would
  advertise, not a one-leaf module. Target ~8–12 capabilities across the
  family, not 40. If a "capability" is only ever used by one device and
  never advertised separately, it stays inline in that profile.
- **Profiles stay thin but readable.** A device profile is a short
  composition; if it grows its own large inline subtree, that subtree is
  a missing capability module.
- **The platform layer is not a dumping ground.** Only genuinely
  universal device concerns (identity, location, diagnostics, comms
  health, faults) live in Platform; anything function-specific is a
  Capability.

## What we'd revisit

If capability boundaries prove unstable — a "capability" repeatedly
splits or merges as devices are added — that is a signal the altitude
is wrong, and the fix is to re-draw the capability, not to abandon the
principle. The principle (function over device type, thin profiles) is
the durable part; the specific capability list is expected to evolve as
device families are added. Reverting to device-type monoliths is not on
the table: it is the shape this document exists to replace.

## Migration

This supersedes the "decompose the RSU mega-module slice by slice"
framing with "re-lay the family along capability lines."

Built today: the platform brick `openits-device-diagnostics`, the
capability modules `openits-v2x-radio`, `openits-v2x-messaging`,
`openits-scms`, and `openits-vehicle-detection`, and `openits-rsu` as
the prototype thin device profile composing them. The remaining device
families (signal-control, dms, ess, ramp-metering, traffic-sensor,
perception, reversible-lane) still ship as monolithic service cores and
follow the same composition as the capability modules they need are
carved out.

## Related documents

- [data model](data-model.md) — the module family and event taxonomy.
- [Design decisions](04-design-decisions.md) — the load-bearing choices.
- [Extension model](06-extension-model.md) — augments / deviations / graduation.
