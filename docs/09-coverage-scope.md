# Coverage, scope, and roadmap

This document states, deliberately and in the open, what the OpenITS data
model covers today, what it is shaped to cover next, and what is out of
scope for now. The gaps below are choices, not oversights: OpenITS is a
**device-model-first, center-to-field (C2F)** effort, so back-office and
center-to-center concerns are intentionally excluded.

Service-package identifiers are ARC-IT 9.1. Each in-tree service is
annotated with the `arc-it-flow` extension so tooling can emit an ARC-IT
coverage report (see [05 — Standards alignment](05-standards-alignment.md)).

## Covered today

| Service | Module(s) | ARC-IT 9.1 service package(s) |
|---------|-----------|-------------------------------|
| Signal control | `openits-signal-control`, `-types`, `-events`, `openits-nema-common` | Traffic Signal Control (SU01); TI01 |
| Ramp metering | `openits-ramp-metering`, `-types`, `-events` | TI03 (Traffic Metering) |
| Dynamic message signs | `openits-dms`, `-types`, `-events` | TI06 (Traffic Information Dissemination) |
| Environmental sensor station | `openits-ess`, `-types`, `-events` | MC01 (Environmental Monitoring) |
| Reversible-lane management | `openits-reversible-lane`, `-types`, `-events` | TM16 (Reversible Lane Management) |
| Traffic sensor / vehicle detection | `openits-traffic-sensor`, `-types`, `-events`, `openits-vehicle-detection` | TM01 (Infrastructure-Based Traffic Surveillance) |
| Roadside perception / AID | `openits-perception`, `-types`, `-events` | TM01 (Infrastructure-Based Traffic Surveillance) |
| CCTV / PTZ | `openits-cctv`, `-types`, `-events` | Roadway surveillance (NTCIP 1205) |
| RSU / V2X | `openits-rsu`, `-types`, `-events`, `openits-v2x-messaging(-types)`, `openits-v2x-radio(-types)`, `openits-scms` | Connected-vehicle / V2X infrastructure |
| Cabinet power / UPS | `openits-cabinet-power` | Platform (composed into device profiles) |
| Common device telemetry | `openits-common-fault-events`, `openits-common-mode-events`, `openits-common-comm-health-events`, `openits-device-diagnostics` | Cross-service (fault / mode / comm-health / diagnostics) |
| Shared foundation | `openits-types` | Cross-service types, identities, groupings |

## Planned

The model is shaped for these near-term extensions; they reuse the same
device-profile idiom (config/state pair, shared fault/mode/comm-health
events, `device-identity-config`) and add no new architecture:

- **Work-zone / portable ITS** — portable devices; depends on the
  portable-location work noted below.
- **Highway advisory radio (HAR)**.
- **Standalone gate / barrier control** and **flood / high-water
  warning** (beyond the reversible-lane interlocks already modelled).
- **Weigh-in-motion (WIM)**.
- **Roadway lighting** (NTCIP 1213 ELMS).

## Out of scope (for now)

These are back-office / center-oriented packages outside the C2F device
model. They are excluded because OpenITS models the **device** contract,
not the enterprise systems that consume it:

- **Parking management** and parking back-office.
- **Tolling / electronic payment** back-office.
- **Transit back-office** (scheduling, fare, CAD/AVL enterprise systems).
- Other **center-to-center** and enterprise packages.

A deployment is free to consume OpenITS device telemetry into any of these
systems; OpenITS simply does not model the systems themselves.

## Portable-device caveat

The identity model today assumes a **static location**: a device is
provisioned with a fixed geographic point and (now) an optional structured
`linear-reference` (route + milepost / LRS measure) and `site-id`
(added on `device-identity-config` this cycle). Genuinely portable devices
(work-zone trailers, portable DMS/CMS, temporary sensors) need a
**time-varying** location surface. That is planned work, not a limitation
of the event model — a portable device already emits the same
fault/mode/comm-health events; only its location representation needs to
become dynamic.
