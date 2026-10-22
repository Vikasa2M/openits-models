# Changelog

All notable changes to this repository are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
for the Go module. See [`docs/versioning.md`](docs/versioning.md) for how the
Go module version, per-module YANG revision dates, and protobuf wire
compatibility relate.

## [0.9.0](https://github.com/Vikasa2M/openits-models/compare/v0.8.0...v0.9.0) (2026-10-22)


### ⚠ BREAKING CHANGES

* In lcs-indication, white-two-way-left-turn-arrow (6) and white-one-way-left-turn-arrow (7) are added; flashing-yellow-x is legacy and may be reported but not commanded. Commanded lane config now refuses unknown, a green arrow without a steady red X to the opposite direction, and the MUTCD 4T.04 pairs yellow X/yellow X, white one-way/white one-way, white two-way/white one-way and white one-way/yellow X. control/state last-command-source and last-command-at are replaced by the last-command container (openits-types:command-provenance: actor-id, actor-class, schedule-id, commanded-at); proto ControlState reserves 15 and 16 and adds last_command 4. In lane-state-changed, new-state and new-direction are renamed state and direction (proto fields 3 and 4 reserved; now 12 and 13). Each reversible-lane notification's kind is constrained to its own sub-base; concrete kind identities are unchanged. Stored or in-flight protobuf carries removed fields as unknown and must be re-encoded. See docs/migration/v0.9.0.md.
* Pedestrian detectors move out of detectors/detector into pedestrian-detectors/pedestrian-detector (key pedestrian-detector-id 1..255, one mandatory call-phase, diagnostic-thresholds; state holds only active, last-activation, actuation-count and fault); the vehicle detector-type identities detector-ped-pushbutton and detector-ped-passive are removed in favour of pedestrian-pushbutton and pedestrian-passive under the new pedestrian-detector-type base. The vehicle detector config/diagnostics container is renamed diagnostic-thresholds. detector-transition channel becomes detector-id (proto DetectorTransition 2 to 11) and pedestrian-event detector-channel becomes pedestrian-detector-id, a pedestrian detector number (proto PedestrianEvent 3 to 9). The shared NEMA phase-timing leaves (min-green, max-green, passage, yellow-change, red-clear, walk, ped-clear) and 37 signal-control leaves are renamed with -s, -min or -pct suffixes, across phases, overlaps, detectors, coordination, preemption, priority, startup and the cabinet on-battery policy, ramp-metering plan timing, and the detector-report, tsam-recommendation-applied and tsp-event events; the MUTCD deviation modules target the new paths. In protobuf every renamed field moves to a new field number and the old number and name are reserved (openits.signal_control.v1, openits.ramp_metering.v1.PhaseTiming, openits.types.v1.ScOnBatteryPolicy), so v0.9.0 decoders treat old fields in stored or in-flight bytes as unknown; re-encode or translate stored data. See docs/migration/v0.9.0.md.

### Features

* align reversible-lane lane-use signals with MUTCD Chapter 4T and report each signal head change ([00f0d27](https://github.com/Vikasa2M/openits-models/commit/00f0d27303b30f0a9b9b8666173fb90b14a67363))
* give pedestrian detectors their own table and name every signal-control timing leaf with its unit ([172983d](https://github.com/Vikasa2M/openits-models/commit/172983d8877f7f5f506c6cde93630967a2ec39fc))

## [0.8.0](https://github.com/Vikasa2M/openits-models/compare/v0.7.0...v0.8.0) (2026-10-20)


### ⚠ BREAKING CHANGES

* **faults:** every service fault inventory classifies entries with a mandatory `kind` (renamed from `category`; the old proto tag is reserved). `first-observed` is mandatory on inventory entries and on fault-raised and fault-cleared, and fault ids are limited to 128 characters.
* **freshness:** the reading timestamps measured-at (zone-occupancy zones), collected-at (signal-control detector measurement), last-update (ramp-metering lane and mainline detectors) and computed-at (vehicle-detection sample basis, also composed into RSU) are renamed observed-at; in protobuf each moves to a new field number and the old number and name are reserved, so stored or in-flight data carrying the old field decodes it as unknown and must be re-encoded.
* **comm-health:** comm-lost events must carry last-contact and observed-by.
* **modes:** a mode-changed mode and previous-mode must derive strictly from the mode-value-base named by its kind (enforced by conformance), so DMS control-mode transitions no longer ride mode-changed and a bare openits-types:mode-unknown is accepted only through a family's own unknown.
* **signal-control:** operation drops flash-active (proto 2) and last-mode-change (proto 5); both numbers and names are reserved. mode-since tracks coordination-mode changes only, and cabinet-output-since tracks entry into flash or dark. monitor/fault-type is valid only when monitor status is fault. controller-mode loses mode-flash, mode-off, mode-manual, mode-preempt and mode-priority and gains openits-signal-control-types:mode-unknown: flash and dark are now cabinet-output values, manual control is control-source control-manual, and preemption and priority are the preempt-active and priority-active flags. The flash-cause identities flash-mmu-conflict and flash-fault-monitor are renamed flash-monitor and flash-controller-fault; identityref values travel as strings, so producers and consumers must change the emitted strings. operational-status-report is rebuilt on device-status-summary: flash-active (10), last-mode-change (4) and uptime-seconds (5) are removed and reserved, and uptime-s and the new fields take fresh numbers. tsam-mode-changed prior/current (2/3, reserved) become previous-mode/mode. Flash is signalled by the new cabinet-output-changed event instead of mode-changed, and a change of control authority with an unchanged mode is reported as mode-changed with mode equal to previous-mode.

### Features

* one fault shape for inventories and fault events, with a shared component reference ([2550e83](https://github.com/Vikasa2M/openits-models/commit/2550e834c3b15b5312af80d30acc443e6cce103d))
* classify fault inventory entries with kind in every service ([2611967](https://github.com/Vikasa2M/openits-models/commit/26119677abaca04f197275c147f625c1e0a2ff5f))
* let cabinet-power faults sit in signal-control and DMS fault inventories ([3afd181](https://github.com/Vikasa2M/openits-models/commit/3afd1811889ea5fe694c05009fc1e73ec040ed6f))
* add foundation health, status-summary and canonical unknown definitions ([571df4b](https://github.com/Vikasa2M/openits-models/commit/571df4bb5dd4f5a01dfef0e910063998d0225435))
* report health on every device root and name freshness observed-at everywhere ([dffc6b7](https://github.com/Vikasa2M/openits-models/commit/dffc6b7e1eb8ceb8ed6bef3afb12a8d769cb65e1))
* make comm-lost carry last-contact and its observer, and define comm-loss staleness ([26bd60c](https://github.com/Vikasa2M/openits-models/commit/26bd60cffd964a20a0d21dde60d3fa6f03dabd4a))
* check mode-changed values against their family's mode base ([df253b2](https://github.com/Vikasa2M/openits-models/commit/df253b28e58f2f9a6c1b40054b09238647ee4540))
* **reversible-lane:** add per-gate live state and the gate-status-changed notification ([95638fc](https://github.com/Vikasa2M/openits-models/commit/95638fc5a57978b385655dcbc8c61baf371ceb1e))
* split signal-control live state into cabinet output, mode, control source and monitor read status ([92e7943](https://github.com/Vikasa2M/openits-models/commit/92e79431bdfcf7205969c1172464221d43fccccd))

## [0.7.0](https://github.com/Vikasa2M/openits-models/compare/v0.6.0...v0.7.0) (2026-10-13)


### ⚠ BREAKING CHANGES

* every notification message gains the mandatory field sequence_epoch, which producers must emit and ce-id implementations must clear before hashing (beside sequence and observed-by) or ids diverge per producer and per restart; and ModeChanged.prior and ModeChanged.current are now previous_mode and mode.

### Features

* event stream identity, mode-changed renames, and the conventions lint ([f20bce7](https://github.com/Vikasa2M/openits-models/commit/f20bce79719736682f57642a701b5dffdcea5c8e))

## [0.6.0](https://github.com/Vikasa2M/openits-models/compare/v0.5.0...v0.6.0) (2026-10-13)


### ⚠ BREAKING CHANGES

* 51 wire changes across nine proto files. Optional scalars gain explicit presence, so Go consumers see pointers in struct literals and direct field assignment (reads through GetX() are unaffected). Seven classification axes emit module-qualified identity strings instead of enum names. Thirteen generated enum type and value names lose a redundant prefix. Detector delay changes JSON encoding from number to string and detector volume from string to number. plan-applied's `source` leaf is renamed `trigger`. See docs/migration/v0.6.0.md for every mapping and an upgrade checklist.

### Features

* pre-1.0 wire cleanup — field presence, identity conversions, generator fixes ([ade90ff](https://github.com/Vikasa2M/openits-models/commit/ade90ffd9c5a17767a2c4932021aa35a036154f4))


### Bug Fixes

* **deps:** bump google.golang.org/grpc from 1.83.1 to 1.83.2 ([a3ddac3](https://github.com/Vikasa2M/openits-models/commit/a3ddac3fc325eb8d724b469f87f15f23254135fd))
* **deps:** bump google.golang.org/grpc from 1.83.1 to 1.83.2 ([8560593](https://github.com/Vikasa2M/openits-models/commit/8560593cfe277737f292841f335a6d1c2e1d6a9f))
* **dms:** keep config defaults out of read-back state ([c5f8d55](https://github.com/Vikasa2M/openits-models/commit/c5f8d55d24045930b78eec3c04b4c02ec3415056))
* model defects from the pre-1.0 review ([74a6081](https://github.com/Vikasa2M/openits-models/commit/74a60810f50d44198829f3035ee203e4da4a0865))
* remove internal names from two public model descriptions ([c9735b0](https://github.com/Vikasa2M/openits-models/commit/c9735b06d134f2574e78643c1d6c3616712f59c5))
* **rsu:** make the SPaT-requires-UTC rule actually fire, and gate rejection reasons ([d7b12ad](https://github.com/Vikasa2M/openits-models/commit/d7b12ad20dc3da38bf3411e3cd4262aa72829813))
* **signal-control:** raise detector and monitor faults as themselves; correct flash and monitor definitions ([95c8685](https://github.com/Vikasa2M/openits-models/commit/95c8685c527d56b3d4ab61258315e5eeb385cf79))
* stop naming vendors as authors of modules they have not ratified ([26def2f](https://github.com/Vikasa2M/openits-models/commit/26def2fc83780a168f30811f2a027d035ced33b7))
* **types:** make the fault-severity order normative ([e972858](https://github.com/Vikasa2M/openits-models/commit/e972858ae9d8552bfe5c91f03e8f3fd839dde77f))

## [0.5.0](https://github.com/Vikasa2M/openits-models/compare/v0.4.0...v0.5.0) (2026-09-05)


### Features

* **types:** add the geo-path grouping ([dade446](https://github.com/Vikasa2M/openits-models/commit/dade4466ba2cb0225fdec08514f26b8459bf12c6))
* **work-zone:** add the work-zone types module ([3596d58](https://github.com/Vikasa2M/openits-models/commit/3596d58a0b4075eb0efb20051f2575bdc814c0cf))
* **work-zone:** add the zone-state-changed notification ([bbf510a](https://github.com/Vikasa2M/openits-models/commit/bbf510a916d26c775de123c3dd557c628384d394))
* **work-zone:** report feed health through the common comm-health events ([b7f73a0](https://github.com/Vikasa2M/openits-models/commit/b7f73a0ceb11c0839b6e738c2242a8ae4f23c307))
* **zone-occupancy:** add the zone-occupancy-changed notification ([#26](https://github.com/Vikasa2M/openits-models/issues/26)) ([08d35cd](https://github.com/Vikasa2M/openits-models/commit/08d35cd0329076dd096f99aa2f9ae1c926f462c9))


### Bug Fixes

* **deps:** bump google.golang.org/grpc from 1.82.1 to 1.83.1 ([#25](https://github.com/Vikasa2M/openits-models/issues/25)) ([ea197bd](https://github.com/Vikasa2M/openits-models/commit/ea197bde9a3bb05a410392284d1eef73fb5808ca))
* **deps:** bump google.golang.org/protobuf in the go-dependencies group ([#24](https://github.com/Vikasa2M/openits-models/issues/24)) ([bd9d6b6](https://github.com/Vikasa2M/openits-models/commit/bd9d6b6660991b587f89e450ed43df767b90fe53))
* **work-zone:** apply final whole-branch review fixes ([6341d9d](https://github.com/Vikasa2M/openits-models/commit/6341d9da8f85962d0089d9633a2e42fe4bd233ce))

## [0.4.0](https://github.com/Vikasa2M/openits-models/compare/v0.3.0...v0.4.0) (2026-08-27)


### ⚠ BREAKING CHANGES

* openits-zone-occupancy removes zone-occupancy/configuration/zone/capacity. Consumers needing a stall count take it from the provisioning system that assigned it.
* openits-dms replaces fallback/power-loss with fallback/power-recovery/{short-outage,long-outage} and adds fallback/reset; migrate power-loss -> power-recovery/short-outage. openits-dms replaces environment/ambient-light-lux with environment/ambient-light-level (rescale against the device maximum; do NOT carry the old value across) and environment/ambient-illuminance-lux. Go consumers of enums from shared groupings must move to the new module-scoped type names (e.g. OpenitsSignalControl_SignalController_CabinetPower_State_PowerSource -> OpenitsCabinetPower_PowerSource, OpenitsDms_Sign_Schedule_ScheduleEntry_ Months -> OpenitsSchedule_Month); protobuf field numbers and enum values are unchanged throughout.
* ce-id values change for every event. The published test vector's id changes accordingly, and any implementation that reproduced the previous vector must be updated.

### Features

* add the dms message-changed and sign-status-report notifications ([#21](https://github.com/Vikasa2M/openits-models/issues/21)) ([f979cfe](https://github.com/Vikasa2M/openits-models/commit/f979cfe73bc5a67b953860bbb6ab50eeaaa95cc7))
* drop zone capacity — it is the operator's inventory, not the device's data ([#19](https://github.com/Vikasa2M/openits-models/issues/19)) ([a9ebab8](https://github.com/Vikasa2M/openits-models/commit/a9ebab8b06a9ece5817599e7045f16baacb6f04d))
* emit protobuf `reserved` for tags the field-number lock has retired ([#20](https://github.com/Vikasa2M/openits-models/issues/20)) ([d57f189](https://github.com/Vikasa2M/openits-models/commit/d57f189c3a0ddb6996a80ea9aab850ba7ec6d0b0))
* NTCIP 1203 fidelity pass on DMS, and fix two shared-module defects it exposed ([#17](https://github.com/Vikasa2M/openits-models/issues/17)) ([bc75b8e](https://github.com/Vikasa2M/openits-models/commit/bc75b8e908c8c4027949825714e20047cab9b599))


### Bug Fixes

* make the deterministic ce-id actually restart- and observer-invariant ([edab5f4](https://github.com/Vikasa2M/openits-models/commit/edab5f40379debfb765149bdf29d64aa9cac0a6e))

## [0.3.0](https://github.com/Vikasa2M/openits-models/compare/v0.2.2...v0.3.0) (2026-08-16)


### ⚠ BREAKING CHANGES

* identityref values move namespace, from 'openits-perception-types:object-*' to 'openits-types:object-*'. Proto field tags are unaffected — identityrefs render as proto strings, so no tag moves and no wire layout changes; only the string values do. Generated Go constants rename from OpenitsPerceptionTypes_ObjectClass_* to OpenitsTypes_ObjectClass_*.
* pan-degrees ranges tighten to 0.0..359.9 (360.0 was previously accepted); and generated-name churn in openits.cctv.v1 — the new notification's absolute/velocity containers collide with the ptz/config message names, so the generator renames the config messages to PtzConfigAbsolute/PtzConfigVelocity and the ptz-move-mode enum to OpenitsCctvPtzMoveMode (defining-module naming once two modules reference it). Field numbers, tags, and JSON field names are unchanged, so binary and RFC 7951 wire forms are unaffected; descriptor names and generated Go types move. Pre-1.0.
* signal-controller cabinet-power leaves move one level deeper (cabinet-power/X -> cabinet-power/{config,state}/X) and the on-battery-policy mode enum re-bases around an unspecified=0 member (its wire path is new in the same change). traffic-sensor and perception associations change from config-false to config-true (paths unchanged; semantic change only). Pre-1.0.

### Features

* add the zone-occupancy capability, hoist object-class to the foundation layer ([#13](https://github.com/Vikasa2M/openits-models/issues/13)) ([398b49a](https://github.com/Vikasa2M/openits-models/commit/398b49aeac33f70b78196498d2ce25fcde11b4be))
* carry wire-source provenance on wire-decoded event notifications ([195fac9](https://github.com/Vikasa2M/openits-models/commit/195fac9f850da2f49dad2623e00ea50c077baff0))
* harden signal-control safety and integrity surface ([a0c128e](https://github.com/Vikasa2M/openits-models/commit/a0c128e0df706ad5d7c53d4ea0fd1c81a6444db7))
* service-family corrections + ptz-move-commanded audit event ([342123e](https://github.com/Vikasa2M/openits-models/commit/342123e122127a78cf790030627ffa2b8a33bce3))
* share schedule primitives, make associations provisioned, split cabinet-power policy into config/state ([b1b45e2](https://github.com/Vikasa2M/openits-models/commit/b1b45e218234061a33b098ef42d05a6ace39ebcb))


### Bug Fixes

* collapse duplicate same-date revision statements in the MUTCD deviation modules ([28c7e0a](https://github.com/Vikasa2M/openits-models/commit/28c7e0aaed9ee412b7d48fbcba21fde2c58b5849))
* correct MAP PSID citations and bound TIM duration to J2735 MinutesDuration ([b42ad4e](https://github.com/Vikasa2M/openits-models/commit/b42ad4e0b5d5b3b25599b42d3ce8bb85963a7a73))
* **deps:** bump github.com/openconfig/ygot in the go-dependencies group ([#12](https://github.com/Vikasa2M/openits-models/issues/12)) ([e611930](https://github.com/Vikasa2M/openits-models/commit/e61193031000536b3d21c5bc763a6fe12ccb657c))
* stamp explicit value statements on every remaining enum member ([9ba1efc](https://github.com/Vikasa2M/openits-models/commit/9ba1efc08622e9afa914ba80150c41db61f1a810))
* vendor/platform consistency — Ledstar namespace repatriation, version stamps, plan-id prose ([8437281](https://github.com/Vikasa2M/openits-models/commit/8437281ed9c63926aeeef3344fc15427375d4740))

## [0.2.2](https://github.com/Vikasa2M/openits-models/compare/v0.2.1...v0.2.2) (2026-07-22)


### Bug Fixes

* **deps:** bump golang.org/x/net from 0.53.0 to 0.55.0 ([#7](https://github.com/Vikasa2M/openits-models/issues/7)) ([9d767f9](https://github.com/Vikasa2M/openits-models/commit/9d767f96e16e7847c52a501805fae4cc7179432c))

## [0.2.1](https://github.com/Vikasa2M/openits-models/releases/tag/v0.2.1) (2026-07-21)

Initial public baseline. The repository history was consolidated to a single
root commit ahead of the public launch, and the earlier v0.1.0–v0.2.0 releases
were retired along with it. The model surface (YANG modules, protobuf,
AsyncAPI bindings, schema registry) is unchanged from the final
pre-consolidation state; this release re-establishes the version line.
