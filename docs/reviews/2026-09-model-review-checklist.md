# Model review checklist (2026-09-07, tree at `ea197bd`)

Working list from the full-tree review. Check items off as they land.

**How to read it**

- Tags: `[B]` blocker, `[M]` major, `[m]` minor, `[N]` nit, `[gap]` a domain gap a practitioner will notice (judgment call, not a defect).
- Waves: `W1` wire-breaking or type-changing, do before the 1.0 tag; `W2` additive taxonomy and fidelity; `W3` gates and fixtures; `W4` citations and docs. "pre-1.0" means the change is breaking once 1.0 ships.
- Line numbers are against `ea197bd` and drift as you edit.
- Every YANG edit needs a new `revision`, `make gen`, `scripts/update-schema-registry.sh`, and a valid plus invalid fixture for any new or changed `must`, in one commit. British spellings in descriptions are listed per module so they can ride the next real revision of that module.
- Sections are ordered by module family because that is the unit a revision bumps. The ledger at the top is the cross-cutting "start here" list; each row links to the section where the work is.

---

## Start here: the ledger

Thirteen items verified directly against the tree, the generated catalog, a yanglint run, or the standard's own site.

- [ ] 1. Nine V2X capability notifications live in `openits-rsu-events` and publish under the `rsu` token. → [RSU](#3-rsu-device-profile) and [V2X](#4-v2x-capability-modules)
- [ ] 2. `comm-health-event` fans out to only signal-control and work-zone. → [Common events](#9-cctv-and-the-common-event-modules)
- [ ] 3. `cctv-control-mode` has no `operational-mode` base; CCTV `mode-changed` can never validate. → [CCTV](#9-cctv-and-the-common-event-modules)
- [ ] 4. ESS has no observation-report notification. → [ESS](#6-ess-and-traffic-sensor)
- [ ] 5. PSID has no defined integer encoding. → [V2X](#4-v2x-capability-modules)
- [ ] 6. RSU signing `must` covers SPaT and MAP only. → [RSU](#3-rsu-device-profile)
- [ ] 7. The SPaT-requires-UTC `must` is proven by no fixture (its fixture fails on JSON encoding). → [Fixtures](#fixtures-and-gates)
- [ ] 8. DMS `error-type` is a closed seven-member enum. → [DMS](#5-dynamic-message-signs)
- [ ] 9. `flash-cause` cannot name a conflict-monitor flash; monitor taxonomy is `mmu-*`. → [Signal-control types](#2-signal-control-types-events-and-platform-bricks)
- [ ] 10. Detector `delay` is whole seconds; NTCIP 1202 is tenths. → [Signal-control core](#1-signal-control-core)
- [ ] 11. Ramp advance-warning-sign interlock rejects MUTCD-conformant meters. → [Ramp](#7-ramp-metering-and-reversible-lane)
- [ ] 12. ARC-IT package ids wrong in four of seven families. → [Docs and citations](#docs-and-citations)
- [ ] 13. Docs claim the MUTCD yellow bound lives in `nema-common`; it lives in the deviation. → [Docs and citations](#docs-and-citations)

---

## Generator and binding

- [x] `[B]` W1 **Proto field presence.** `tools/yang-proto-gen/emit.go` never read `mandatory`, so every scalar and enum leaf emitted bare and proto3 implicit presence made absent and zero the same bytes — `occupancy-count`'s "absent, which is not the same as zero" was unrepresentable, and an absent `fault-entry/severity` decoded as `info`. `EmitMessage` now labels a leaf `optional` unless it is mandatory (resolving `refine mandatory true` off the `uses` statement the way `jsonschema_emit.go` already did) or its proto type is message-typed. 1,663 fields gained presence; 261 stay bare, led by the mandatory `kind`/`sequence`/`source-device-id` trio. Verified: `buf lint` and `buf breaking` (WIRE_JSON, against main) both green, 240/240 conformance checks across all nine kinds, ce-id vectors reproduce, and `asyncapi.yaml`, `schema-registry/`, and `field-numbers.yaml` are byte-identical.
- [x] `[M]` W4 ce-id spec: "cleared to their zero values" was ambiguous once `observed-by` gained explicit presence (assigning `""` serializes, clearing does not). Reworded as normative, and the encoding-visible-schema-change caveat recorded under Invariants.
- [x] `[M]` W1 **Done 2026-09-08.** Written as `docs/migration/v0.6.0.md` and linked from `docs/versioning.md`, which now states the convention that a wire-breaking release owes consumers one collected migration note. Covers the pointer break, all seven identity conversions with member-for-member tables, DMS activation errors, the scalar retypes, and the enum un-prefixing. Original: **Consumer note for the release.** The change is source-breaking for Go consumers: scalar fields become pointers. Read paths through `GetX()` are unaffected (the accessor keeps its value-returning signature); struct literals and direct field assignment need `proto.String`/`proto.Uint32`/`.Enum()`. Five conformance mocks were updated in-tree as the worked example. Size the release as a breaking minor on the v0.x line and say this in the changelog.
- [ ] `[m]` W2 Consider whether enum leaves should additionally get an `_UNSPECIFIED = 0` sentinel now that presence covers absence; several enums put a real member at zero (`FAULT_SEVERITY_INFO`, `POWER_SOURCE_ON_LINE`, `VIDEO_CODEC_H264`). Presence makes this less urgent but not moot for consumers that ignore presence.

- [x] `[M]` W3 **Done 2026-09-07. Generated enum names were not stable against reference count or file order.** `emitEnum` qualified a name only when it was already claimed, so referencing an existing `-types` typedef from a second output file renamed the enum in the FIRST file, carrying its VALUE identifiers with it — and protojson serializes enum values by name. Fixed by declaring each enum once per proto package and having the other file import it: a proto type is identified by package plus name, not by the declaring file, so the declaration's location is invisible to consumers. `ProtoFile.ClaimedNames` became an `EnumRegistry` keyed on (base name, value-set signature), so genuinely different enums sharing a base name still qualify. Five tests in `sharedenum_test.go` cover sharing, the import, package isolation, and the disjoint-value-set case. **This was not hypothetical: 13 enums across 6 services were already sitting in the collided state**, and the fix un-stutters them (`OpenitsCctvPtzMoveMode` back to `PtzMoveMode`), which is 36 field references and a third `buf.yaml` allowlist entry. Refined the same day after review: rather than 'first declarer wins' — which put the declaration in whichever sibling asked first and gave 7 services a `state.proto` -> `events.proto` import the YANG layer forbids — each service's enums now live in a generated per-service `types.proto` in the same proto package, which both state and events import. Same package means no name, wire or Go symbol changed, so the relocation itself needed no allowlist entry; the 36 un-stutter renames are inherent to fixing the collision either way. This follows the `TypesTarget` precedent already in the emitter for shared messages, and OpenConfig's `openconfig-*-types` shape. `docs/reference/yang-reference-conventions.md` corrected: a named typedef is stable against the use-site path but was never automatically stable across output files.
- [ ] `[M]` W3 **Generator gap found while doing the above.** The proto emitter tombstones retired *field* tags via `field-numbers.yaml`, but nothing reserves retired *enum values*: deleting an enum member emits an unreserved deletion that `buf breaking` catches only because it compares against main, and a later member could inherit the number. Perception's retired `zone-function` member is deprecated in place as the workaround. Give the lock an enum-value section, or make the emitter emit `reserved` for members a revision drops.
- [x] `[M]` W1 **Done 2026-09-08 — resolved by removing the mechanism, not the entries.** `ignore_only` is gone from `buf.yaml` entirely, with a comment explaining why: an entry disables the rule for a whole file, and it goes inert on merge, so its removal is a follow-up PR that is easy to forget and invisible when forgotten. Intended breaks now show as a red `buf breaking` on the PR that makes them and are merged on maintainer authority against the migration note. Original: **Empty the allowlist before release.** Two scoped `FIELD_WIRE_JSON_COMPATIBLE_TYPE` entries were added for the two intended retypes (signal-control detector `delay`, DMS `error-type`). Both are genuine wire breaks with no non-breaking spelling. Remove the entries once the release carrying them ships; the list should normally be empty on main.

---

## 0. Foundation (`openits-types`)

- [ ] `[M]` W1 `device-identity-config` carries `maintained-by`, `install-date`, `owner`. All three fail the interpretation test in `04-design-decisions.md` (device never acts on them, not needed to interpret a reading). Remove, or record the exception beside the capacity decision.
- [x] `[M]` W1 **Done 2026-09-08.** `time-source` hoisted to `openits-types` as an identity unioning both vocabularies (gnss, gnss-pps, ptp, ntp, cellular, line-frequency, manual, local). RSU's closed enum removed, signal-control's local identity retired, both retyped. Original: one `time-source` identity base here; retype RSU `configured-time-source` and `diagnostics/time-source` (closed enum, `openits-rsu-types.yang:113-126`) and signal-control `time-source` (identity, `openits-signal-control-types.yang:1300-1313`) to it. Converge RSU `time-valid`/`holdover` with signal-control `sync-status`.
- [ ] `[m]` W1 `typedef celsius` (decimal64, fraction-digits 1) here; five modules define their own (`openits-ess.yang:178-188`, `openits-traffic-sensor.yang:594-606`, `openits-perception.yang:507,525`, `openits-device-diagnostics.yang:58,90`).
- [ ] `[m]` W1 Collapse `openits-perception-types:zone-id` (:123) and `openits-zone-occupancy-types:occupancy-zone-id` (:50), both exact copies of `device-id`, onto one shared id typedef; align `work-zone-id` (length 1..128, no pattern) and `ess-types:sensor-id` (unconstrained).
- [x] `[m]` W1 **Done 2026-09-08.** Consolidated **four** representations into one foundation axis: `travel-direction` with an intermediate `carriageway-direction` base. `linear-reference/direction` typed to it, reversible-lane's enum removed, work-zone's own identity set retired one day after being added. The `measure` imperial-unit question is still open. Original: `linear-reference/direction` is free text (`'NB','SB'`); make it an enum or identity. Decide the `measure` unit policy: `miles` is the only imperial unit in the tree.
- [ ] `[m]` W1 Add `grouping geo-polygon` (`list vertex { key vertex-index; min-elements 3; uses geo-point; }`, `uint16` key) and use it from perception `configuration/zone` (`openits-perception.yang:340-353`), which today keys on `uint8` while `geo-path` keys on `uint16`.
- [ ] `[m]` W2 `wire-source`: add `case onvif { leaf topic; leaf source-token; }`; add `'ntcip-1205'` and `'onvif'` to the decoder examples; add NTCIP 1205 to the `ntcip-oid` reference. Consider a `vendor-api` case for perception incident events.
- [ ] `[m]` W2 `units "degrees"` on `geo-point/latitude` and `longitude` (:626-627); every composer inherits it.
- [ ] `[m]` W1 Counter doctrine sweep while cheap: 22 running totals are plain `uint32/uint64` (see [mechanical audit §15](#10-mechanical-audit-whole-tree)); only two `yang:counter64` exist and one of them (`openits-signal-control.yang:951 measurement/volume`) is a per-period value.
- [ ] `[m]` W1 One `-seconds` versus `-s` suffix rule family-wide (signal-control mixes both; ramp uses `-s`; diagnostics `-seconds`). Recommend no suffix, rely on `units`, matching `nema-common`.
- [ ] `[m]` W1 One units-string style: `vehicles-per-hour` (ramp-types:122) vs `vehicles/hour` (traffic-sensor-types:290); `veh/km` (:362) vs spelled-out everywhere else; slash forms (`km/h`, `kilobits/second`, `frames/second`, `counts/minute`) vs hyphen forms (`meters-per-second`, `millimeters-per-hour`, `points-per-second`). 32 distinct strings today.
- [ ] `[m]` `fault-severity` ORDINAL CAVEAT says the order "inverts the X.733 warning<minor convention", but the values are warning=1, minor=2, which is the X.733 order. Rewrite the caveat to say what it means (peer tiers, do not rank across the pair).
- [ ] `[m]` `associated-devices` description says it lives in a "state subtree" (:764-770); perception composes it config-true (`openits-perception.yang:258-266`). Fix the description or the placement.
- [ ] `[N]` Vendor product names in a core description: `openits-types.yang:958-959` lists Econolite ASC3, McCain MaxTime, Trafficware ATC, Yunex. Consider neutral wording.
- [ ] `[N]` British spellings to ride the next revision: `kilometre` (:223, :364), `serialise` (:609).
- [ ] `[N]` `command-provenance` is "DEFINED here; per-service ADOPTION is follow-up" (:740-754). Adoption items are listed under DMS, CCTV, ramp, reversible-lane, and mode-changed below.

---

## 1. Signal-control core

Modules: `openits-signal-control`, `openits-nema-common`, `deviations/openits-signal-control-mutcd`, `deviations/openits-signal-control-mutcd-strict`, `openits-cabinet-power`, `openits-schedule`.

### Blockers and major

- [x] `[B]` W1 pre-1.0 **Done 2026-09-07.** Detector `delay` retyped to `decimal64 fraction-digits 1`, range 0.0-255.0, reference narrowed to §5.3.2.5. Fixture now encodes it as the RFC 7951 decimal string `"3.0"`. This is the one change that trips `buf breaking` (decimal64 renders as a JSON string); a scoped `ignore_only` for `FIELD_WIRE_JSON_COMPATIBLE_TYPE` on that one file was added to `buf.yaml` and must be removed once the release carrying it ships. Original finding: `delay` was `uint16` seconds. NTCIP 1202 v03 §5.3.2.5 `vehicleDetectorDelay` is tenths (0–255.0 s). Retype to `decimal64 { fraction-digits 1; range "0.0 .. 255.0"; }` and cite the object section. Sibling `extend` (:885) is already tenths.
- [ ] `[M]` W1 pre-1.0 `simultaneous-gap` (:787) has inverted polarity and default versus NTCIP phaseOptions bit 11 ("Simultaneous Gap Disable"; NEMA default is simultaneous gap-out enabled). Rename to `simultaneous-gap-disable { default "false"; }`, cite §5.2.2.21 bit 11.
- [ ] `[M]` W1 pre-1.0 `split-mode` (:1290-1306) is a different member set than NTCIP `splitMode` (§5.5.9.4: other, none, minimumVehicleRecall, maximumVehicleRecall, pedestrianRecall, maximumVehicleAndPedestrianRecall, phaseOmitted, nonActuated). Add `max-vehicle-and-ped-recall` and `non-actuated`; drop `coordinated-fixed`/`coordinated-floating` (coordination is `coordinated-phases` :1215-1218; fixed/floating is `force-off-mode` :1238-1245, cite `coordForceMode` §5.5.4). Document that NTCIP `coordCorrectionMode`/`coordForceMode`/`coordMaximumMode` are unit-level.
- [ ] `[M]` W2 `max-green-2` (:737-746) description conflates `phaseMaximum2` (§5.2.2.7) with `phaseDynamicMaxLimit`/`Step` (§5.2.2.18-19). Correct the description; add `coordination/timing-plan/maximum-mode` (enum maximum-1/maximum-2/maximum-3/max-inhibit, §5.5.3); add optional `max-green-3` and a `dynamic-max { limit; step }` container so Indiana 52-54 are interpretable.
- [ ] `[M]` W1 pre-1.0 `startup/flash-phases` (:653-673) has no NTCIP meaning. Replace with per-phase `options/startup-state` (enum per §5.2.2.20 `phaseStartup`) and `options/automatic-flash-entry` / `automatic-flash-exit` booleans (phaseOptions bits 1-2). Mark `all-red-duration-seconds` as engineering practice, not NTCIP.
- [ ] `[M]` W1 pre-1.0 Preemption: drop `priority-order` (:1534; NTCIP §5.7.2.2 orders by preempt number); state on `preemptor-id` that lower number is higher priority; re-express the rail-supremacy `must` (:1500) against `preemptor-id`; replace `flash-dwell-seconds` (:1568-1573) with `flash-dwell { type boolean; }` (bit 3); add `no-override-next` (bit 2) if equal priority is needed.
- [ ] `[M]` W3 Seven deviation `must`s have no invalid fixture: `mutcd.yang:102` (walk 0 or ≥4), `:134` (ped-clear ≥3), `:154` (overlap red-clear ≤6); `mutcd-strict.yang:69` (yellow ≤6), `:81` (red-clear ≤6), `:104` (overlap yellow 4–6), `:114` (overlap red-clear ≤6). Add each. Add one `valid-us-timing-under-signal-control-mutcd.json` and teach `check-deviations` to require a positive fixture.
- [ ] `[M]` W2 Schedule precedence is unspecified when two `schedule-entry` rows match one date (:1454-1466; same shape `openits-dms.yang:742-752`; grouping `openits-schedule.yang:97-118`). State the tie-break once in `openits-schedule:calendar-selector` (cite NTCIP 1201) and make `day-plan` mandatory in all three composers.

### Minor

- [ ] `[m]` W2 `split-seconds` (:1255-1262) is optional but its `must` fails on absence with a misleading message. Make it `mandatory true`.
- [ ] `[m]` W4 Deviation error messages call MUTCD Guidance a mandate (`mutcd.yang:82-86`; §4F.17 para 13 is "should"). Reword to "enforces MUTCD 4F.17 para 13 Guidance as a hard bound". Re-justify `ped-clear >= 3` (:134-138) as a jurisdiction floor or drop it; §4I.06 sets no minimum for the ped change interval itself. Verify the buffer-interval figure (2 s vs 3 s) against the published 11th-edition PDF.
- [ ] `[m]` W2 `operation` (:1654-1696) lacks the `control-source` axis the foundation reserved for it (ramp has it). Add config-false `control-source` identityref plus `stop-time-active` and `manual-control-enabled` booleans (Indiana 178/180).
- [ ] `[m]` W1 pre-1.0 `signal-operation-on-battery` (:439-447) uses `full-operation` as value 0, so a proto3 unset field decodes as "full operation on battery". Re-base with `unspecified = 0` or make `not-on-battery` the zero value.
- [x] `[m]` W1 pre-1.0 **Done 2026-09-07.** Retyped to `uint32`. Note: JSON-visible even though buf treats uint64 and uint32 as compatible, since RFC 7951 encodes 64-bit as a string and 32-bit as a number. Original finding: it was `yang:counter64` but resets every collection period. Retype `uint32`, `units "vehicles"`.
- [ ] `[m]` W2 Overlap: `fya` container may only appear on an FYA overlap (:1033) but an FYA overlap without one is valid; add `must "not(derived-from-or-self(type,'openits-sc-types:overlap-fya')) or fya"`. Consider `min-elements 1` on `included-phases` (:1014-1017) for non-pedestrian types.
- [ ] `[m]` W1 `-seconds` suffix inconsistent inside the module (:659,668,766,964,1285,1535,1556,1567,1568,1577,1615-1617 suffixed; :737,746,802,878,884,1204,1210,1538 unsuffixed). Apply the family rule from section 0.
- [ ] `[m]` W1 pre-1.0 `observe-dst` boolean (:1404-1412) loses NTCIP 1201 `globalDaylightSaving` US-vs-European rule variants. Identityref `dst-rule` (none/us/eu/other) or drop the boolean and rely on the IANA timezone.

### Nits

- [ ] `[N]` W4 About forty references read `NTCIP 1202:2019 §5 (...)` with no object section (e.g. :745,755,779,795,864). Cite `§5.2.2.7 phaseMaximum2`, `§5.5.9.4 splitMode`, `§5.2.2.30 phasePedAdvanceWalkTime` (the LPI leaf's NTCIP object is uncited today), etc.
- [ ] `[N]` pyang lint: 17 `must` statements out of canonical order (:694-1521); ~60 enum members without description (:497-585, 1220-1330, 1442, 1476, 1636); `openits-schedule.yang:64-80` enum descriptions.
- [ ] `[N]` Stale: `openits-cabinet-power.yang:289-293` comment describes a removed policy grouping; `phase-runtime-state` carries per-leaf `config false` (:596-626) contrary to RFC 8407 §4.13 and the fix already applied to `device-hardware`; :1298 "omits the phase this plan" missing "in"; :490 `channel-flash-state` lacks NTCIP `channelFlash` bit 3 "alternate half hertz".
- [ ] `[N]` `nema-common` leaf-level self-value musts (:127 `>= 1.0`, :154 `> 0.0`, :171 `>= 0.0`) are type-level constraints dressed as musts; a `range` on the `decimal64` expresses them. Same for `openits-ramp-metering.yang:589,624` and `openits-reversible-lane.yang:237`. Not dead code; tidy when touching the file.

### Domain gaps

- [ ] `[gap]` Per-phase call/hold/omit state (`phaseStatusGroupVehCalls/PedCalls/PhaseOns`) so the twin can mirror Indiana 41-49.
- [ ] `[gap]` Detector roles and locking: `call`/`queue`/`added-initial`/`passage`/`red-lock`/`yellow-lock` options (§5.3 `vehicleDetectorOptions`), `switch-phase`, `queue-limit`; phase-level `non-lock-detector-memory` (phaseOptions bit 5). `mode presence/pulse` is a card setting; `fail-action` has no NTCIP object (verify vendor-specific).
- [ ] `[gap]` Volume-density half-ported: `cars-before-reduction` (§5.2.2.14), `reduce-by`, `guaranteed-passage` (bit 12), `conditional-service` (bit 14); `call-to-nonactuated` collapses NTCIP's two CNA groups (bits 3-4).
- [ ] `[gap]` Preemption entry/track/exit detail: `enter-yellow-change`/`enter-red-clear`, `track-yellow-change`/`track-red-clear`, `dwell-ped`, `cycling-phases`, `exit-type`, `link`, `advance-preemption-time`, `gate-down` input; `track-clearance/green-seconds` range 1..65535 vs NTCIP 0..255.
- [ ] `[gap]` Coordination: `pattern` distinct from plan when `sequence-id` absent; `cycle-length` 1..65535 vs `patternCycleTime` 0..255; `offset` unbounded vs 0..255.
- [ ] `[gap]` MMU/cabinet: `mmu/fault-type` single-valued though monitors latch several; no per-channel live output (`channelStatusGroup`); no cabinet-type discriminator (TS-1/TS-2/ATC/ITS).
- [ ] `[gap]` Pedestrian: no buffer-interval leaf (11th ed. §4I.06 para 4 makes it a Standard), no `phaseYellowandRedChangeTimeBeforeEndPedClear`, no `ped-delay-time` (NTCIP LPI offset is `phasePedAdvanceWalkTime + phasePedDelayTime`).
- [ ] `[gap]` Timebase auxiliary/special-function outputs (`timebaseAscAuxillaryFunction`/`SpecialFunction`).

---

## 2. Signal-control types, events, and platform bricks

Modules: `openits-signal-control-types`, `openits-signal-control-events`, `openits-vendor-econolite-signal-control-types`, `openits-vehicle-detection`, `openits-device-diagnostics`, `augments/example-signal-control-vehicle-counts`.

### Blockers and major

- [x] `[B]` W1 pre-1.0 **Done 2026-09-07.** Added `flash-fault-monitor` (unitFlashStatus faultMonitor(5): TS1 conflict monitor, ITS-cabinet CMU) and `flash-other` (other(1)); removed `flash-exit` (a mode transition, not a cause); every remaining member now states its NTCIP mapping. Original finding: `flash-cause` could not represent `faultMonitor(5)` or `other(1)`; `flash-programmed-tod` and `flash-exit` map to no wire value. Add `flash-fault-monitor` (CMU-212/2212, TS1 conflict monitor) and `flash-other`; describe `flash-programmed-tod` as the refinement of `automatic(3)` when TOD is the source; drop `flash-exit` (report exit as `mode-changed`). Cite `NTCIP 1202:2019 §5 (unitFlashStatus)` on the base.
- [x] `[M]` W1 pre-1.0 **Done 2026-09-07.** Renamed to `monitor-fault-type` with `monitor-*` members; added `monitor-field-check`, `monitor-port1-fail`, `monitor-diagnostic`, `monitor-watchdog`; dropped `mmu-none`. Core `operation/mmu` renamed to `operation/monitor` (old field tombstoned as `reserved 4; reserved "mmu";`), fixture renamed to `valid-signal-controller-monitor-flash-latched.json`. `conflict-monitor` keeps its name: that readback genuinely is the MMU/CMU program card. Original finding: `mmu-fault-type` was TS2-MMU-specific in name and content. Rename base to `monitor-fault-type`, members to `monitor-conflict`, `monitor-red-fail`, etc.; add `monitor-field-check`, `monitor-port1-fail`, `monitor-diagnostic`, `monitor-watchdog`; drop `mmu-none` (absence under config-false means no active fault); rename core `state/mmu` (`openits-signal-control.yang:1675-1690`) to `monitor`.
- [x] `[M]` W2 **Done 2026-09-07.** The four HR fault identities re-based onto `controller-fault-event-kind`, and the notification's `kind` narrowed to it. Wire-neutral: they still derive from `sc-fault-event-kind` through it. `invalid-controller-fault-event-inventory-kind.json` proves an inventory category is now rejected. Original finding: it was an orphan (:861-868): `fault-unit-flash-status`, `fault-unit-alarm-group-1`, `fault-power-failure`, `fault-vendor-alarm` (:841-859) derive from `sc-fault-event-kind` directly, so the documented filter returns nothing. Rebase the four (and the Econolite slots' parent) onto `controller-fault-event-kind`; set `controller-fault-event/kind { base controller-fault-event-kind; }` (`openits-signal-control-events.yang:552-559`); add `invalid-controller-fault-event-state-category-kind.json`.
- [ ] `[M]` W1 pre-1.0 Indiana 173 is a state report modeled as a fault with mandatory `raised` and `severity` (events :543-585); scheduled TOD flash becomes a fleet-wide fault. Route 173 through `mode-changed` (current=`mode-flash`, `trigger`, `wire-source`), add a `flash-cause` leaf there or on a signal-control-derived identity, deprecate `fault-unit-flash-status`, keep `controller-fault-event` for 174/182/184/185.
- [x] `[M]` W1 pre-1.0 **Done 2026-09-07.** Replaced with typed unit-bearing leaves (`pattern`, `cycle-length-s`, `offset-s`, `split-s`, `cycle-state`, each with a `previous-` counterpart) plus `phase-number`, renamed from `split-number` and now populated for Indiana 50-54 as well as 134-149. New `coordination-cycle-state` typedef in `-types` carries the eight HR values; the core's polled five-member enum is unchanged and documented as the persistent subset. Fixture `valid-coordination-split-change.json` added. Original finding: it carried unit-less `int32` `new-value`/`previous-value` and cannot carry the phase for Indiana 50-54. Replace with typed leaves: `pattern` (`plan-id`), `cycle-length-s`, `offset-s`, `split-s`, `phase-number` (rename of `split-number`), and `cycle-state` typed by a `coordination-cycle-state` typedef hoisted into `-types` (the core enum at `openits-signal-control.yang:1323-1331` reuses it).
- [x] `[M]` W1 pre-1.0 **Done 2026-09-07.** `wire-source` removed from both TSAM notifications. Original finding: they were synthesized but claimed provenance (events :460-539) are synthesized by the adaptive system; remove `wire-source` from both.
- [x] `[M]` W2 **Done 2026-09-07.** Added `phase-barrier-termination` (31), `overlap-fya-begin-permissive` / `-end-permissive` (32/33), and `controller-clock-updated` (181, which also gives `sc-comm-health-event-kind` its first derived identity). Codes 55/56 and 175 and the `unit-alarm-status-1` bits remain. Original finding: unmapped ATSPM codes, add `phase-barrier-termination` (31), `overlap-fya-begin-permissive`/`-end-permissive` (32/33, under `sc-overlap-event-kind`), `ext-io-advance-warning-sign` (55/56, on/off in payload), `controller-clock-updated` (181, under `sc-comm-health-event-kind`), `fault-alarm-group-state-change` (175); add `unit-alarm-status-1 { type bits }` to `controller-fault-event` for 174. Verify whether 183 exists in the 2020 table.
- [x] `[M]` W2 **Done 2026-09-07.** Added `vehicle-detector-fault-no-activity`, `-max-presence`, `-erratic-count`, `-communications` under `sc-detector-event-kind`, matching `vehicleDetectorReportedAlarms`. Original finding: detector fault identities stopped at Indiana 84-88. Add `vehicle-detector-fault-no-activity`, `-max-presence`, `-erratic-count`, `-communications` under `sc-detector-event-kind` citing `vehicleDetectorReportedAlarms`; mirror for ped.
- [ ] `[M]` W1 pre-1.0 `openits-vehicle-detection` is BSM analytics under a neutral capability name (:18-29, 93-104, 111-113, 144-151, 160-168, 220-248): `count-basis` defined by temporary-ID rotation, `penetration-estimate-pct`, four fixed class leaves though `openits-types:object-class` now exists. Either rename to `openits-v2x-bsm-analytics`, or restructure as neutral `counts`/`speed-metrics`/`list class { key class; leaf class { identityref object-class } leaf pct }` with a `bsm-basis` presence container. Single composer today (`openits-rsu.yang:719`).
- [x] `[M]` W1 pre-1.0 **Done 2026-09-07.** Both removed; `detectors/detector/config/name` added to the core as the place the label belongs. Fixture also corrected: it carried Indiana code 132 (a cycle-length change) on a `vehicle-detector-on` event, now 82 with the param matching the channel. Original finding: they were collector-injected inventory strings (events :138-139) are collector-injected inventory strings on the highest-rate event. Drop them; add `leaf name` to `detectors/detector/config` (core :850-928 has none). Keep `phase-served`.

### Minor

- [ ] `[m]` W2 Kind bases: `detector-report/kind` → `identityref { base sc-detector-event-kind; base openits-types:report-event-kind; }` (multi-base intersection); give `detector-transition` an `sc-detector-transition-kind` sub-base and move 81-88 under it; `operational-status-report` and `unmapped-event` use the service root (events :153-159, 598-604, 640-649). Add `invalid-detector-transition-report-kind.json`.
- [x] `[m]` W1 pre-1.0 **Done 2026-09-08.** Converted on both surfaces together, as required. One identity with two altitudes: `cycle-state-persistent` for the samplable states, the three instantaneous HR markers under the root. The polled readback narrows by type, which retired two prose caveats — the marker exclusion, and the warning that absent was indistinguishable from `free` at proto enum value 0. Original: **`cycle-state` was an enum on both surfaces and should probably be an identity on neither or both.** Added `coordination-cycle-state` (8 HR values) to `openits-signal-control-types` on 2026-09-07 as an enum, mirroring the core's existing 5-value polled `cycle-state`. It sits close to the extensibility line: free and in-step are closed, but transition method is vendor territory (shortway and adaptive transitions exist beyond add/subtract/dwell), and the closest analogue in the same module, `controller-mode`, is already an identity. Do NOT convert only one side — an enum on the polled readback and an identity on the event for one axis is worse than either used consistently. Convert both together or leave both.

- [x] `[m]` W1 pre-1.0 **Done 2026-09-08.** Now an identity (`priority-transit`/`-freight`/`-emergency`/`-other`), matching `preemption-type` beside it; the `none` member is gone, since absence of a request is an absent leaf. Original: `priority-type` was a closed enum with `other` and `none` on an axis that mirrors an identity. Make it `identity priority-type` (transit/freight/emergency/other); drop `none` (meaningless on `strategy/config/priority-class`).
- [ ] `[m]` W1 pre-1.0 `mode-priority` (:1064-1065) contradicts the module's own "priority biases, does not override" semantics (NTCIP 1211 runs inside coordination). Drop it; document `mode-manual` as manual interval advance with `control-source = control-manual`; add a `reference` to `unitControlStatus`/`unitAlarmStatus1` on `controller-mode` saying which bits produce which mode.
- [ ] `[m]` W1 `sc-comm-health-event-kind` (:379-386) has no derived identities and no user. Put `controller-clock-updated` (181) under it, or delete pre-1.0.
- [ ] `[m]` W4 `detector-type` (:1211-1254) cites `vehicleDetectorTable` as defining a technology axis; it does not. Reword as OpenITS-original; consider `detector-thermal`, `detector-lidar`, `detector-infrared`.
- [ ] `[m]` W4 `tsp-event/priority-class` (events :434-437) "0..7 per NTCIP 1211": verify against the MIB (1211 class objects are 1..10; J2735 `RequestImportanceLevel` is 0..14). Cite the exact object or drop the range.
- [ ] `[m]` W1 pre-1.0 Name drift for one concept: `detector-channel` (:116), `channel` (:137), `detector-id` (:175); `tsp-number` bare `uint8` (:425) vs the `preempt-slot` typedef. One name, a `tsp-slot` typedef.
- [x] `[m]` W1 pre-1.0 **Done 2026-09-08.** Leaf renamed `source` -> `trigger` and retyped onto `openits-types:mode-change-trigger`; `unspecified` becomes an absent leaf. The rename tombstoned cleanly, so it cost no allowlist entry. Original: it duplicated the `mode-change-trigger` axis as a closed enum; retype to `identityref { base openits-types:mode-change-trigger; }`. Add `uses wire-source` to `plan-applied` and `detector-report` (:144-196, 243-289) and state the polled-vs-computed basis.
- [ ] `[m]` W1 pre-1.0 `operational-status-report` (events :607-629): drop `flash-active` (restates `mode`) and `uptime-seconds` (duplicates `system-runtime`); add `control-source`. Drop `phase-state-change/hold-active` and `call-registered` (:91-92), state restated as booleans an HR row cannot populate.
- [ ] `[m]` W1 pre-1.0 `openits-device-diagnostics` carries Unix-host shapes: `load-1min/5min/15min` (:91-93), PIDs in `process-list` (:114-135), severity-bucketed `log-statistics` (:137-161). Move the load leaves into their own `unix-load` grouping; add `units "celsius"` to `threshold-high-c`/`threshold-low-c`; state in the module description which groupings are universal.
- [ ] `[m]` W3 Fixtures: `valid-detector-transition.json` has kind `vehicle-detector-on` with `indiana-code: 132, indiana-param: 300` (a cycle-length change); use 82 / channel 12. `valid-controller-fault-event.json` has no `source` block though it is an HR-decoded 173.

### Nits

- [ ] `[N]` `openits-signal-control-types.yang:17-22` module description still says the module is only the event-kind hierarchy. `openits-signal-control-events.yang:27` `openits-version "0.1.0"` after a BREAKING revision (:43-47). `unmapped-event` lacks `arc-it-flow` (:634) and its `source` is semantically mandatory. `preemption-cleared/kind` says "(exit-interval, force-off)" while the body says only exit triggers it (:355 vs :337-350). `preemption-activated/type` is mandatory but not derivable from an Indiana 102 row (:318-323). Vendor module `organization "Econolite Group (vendor)"` while the description says Econolite has not ratified it (:13-14, 28-32). pyang: enum members without descriptions at types :982-985, 1025-1028; events :444-448, 508-514. Example augment uses `count-1hour` where vehicle-detection uses `vehicles-1hr`, and carries no `openits-version`.

### Domain gaps

- [ ] `[gap]` No typed event for controller clock updates (181).
- [ ] `[gap]` No FYA begin/end-permissive (32/33) or barrier termination (31).
- [ ] `[gap]` Stuck-on / no-activity / erratic detector faults cannot be named as events.
- [ ] `[gap]` No conflict-monitor flash cause for non-MMU cabinets.
- [ ] `[gap]` No coordination-fault identity (`unitAlarmStatus1` coordFault/coordFail).
- [ ] `[gap]` `operational-status-report` cannot say central vs local backup.
- [ ] `[gap]` `coordination-change` cannot say seconds vs percent or which phase.
- [ ] `[gap]` Preemption `source-id` is free-form; no rail-crossing / EVP unit typing.

---

## 3. RSU device profile

Modules: `openits-rsu`, `openits-rsu-types`, `openits-rsu-events`, `deviations/openits-rsu-us-band-plan`.

### Blockers and major

- [ ] `[B]` W1 pre-1.0 Re-home the capability notifications: `rsu-srm-received`, `rsu-srm-status-change`, `rsu-tim-loaded/-cleared/-broadcast/-broadcast-state-changed`, `rsu-broadcast-sample` → `openits-v2x-messaging-events`; `rsu-channel-fault` → `openits-v2x-radio-events`; `rsu-certificate-expiring`, `rsu-security-event` → `openits-scms-events` (plus a new `openits-scms-types` for `certificate-status`, `certificate-type`, `enrollment-status`, `geographic-region-type`, `rsu-security-event-type`/`sec-*`). Move the `*-kind` identities (`openits-rsu-types.yang:223-293`) to the matching `-types` modules. Leave `rsu-gps-status-change` (or move to a platform GNSS events module). Or: record in `docs/08-capability-architecture.md` an explicit exception and why. Update `docs/05-standards-alignment.md:41-53`.
- [ ] `[B]` W2 Signing `must` (`openits-rsu.yang:282-289`) covers SPaT/MAP only. Extend the antecedent to TIM, PSM, RTCM, SSM enables (same `permit-unsigned-broadcast` escape hatch); add `invalid-rsu-tim-unsigned.json`, `-ssm-unsigned.json`, and a MAP-only unsigned fixture.
- [ ] `[M]` W1 Hoist ~330 lines of inline content: `gnss-status` (:370-387, 576-630), `clock` (:320-366), `cellular-backhaul` (:631-718) into `openits-device-diagnostics`; radio PHY (:465-508) into `openits-v2x-radio:radios`; `spat-sync` (:510-566) into `messages/spat/state`. Expand groupings at the current paths where possible so proto/Go are unaffected.
- [ ] `[M]` W2 Every notification's `kind` is `base rsu-event-kind` or `rsu-fault-event-kind` (events :196, 246, 286, 318, 345, 375, 424, 478, 530, 572, 611). Add an intermediate base per notification in `-types`, re-base each leaf identity under it (wire-neutral), narrow each `kind`, add one `invalid-rsu-*-bad-kind.json` per notification.
- [ ] `[M]` W1 pre-1.0 Time-source convergence (see section 0).
- [ ] `[M]` W1 pre-1.0 SRM events (events :206-237, :256-277): add `intersection { region; id }` (share the messaging-types pair as a grouping) to both events and `active-requests`; add `basic-vehicle-role` identities to `openits-v2x-messaging-types` and retype `vehicle-class`; rename `approach` → `inbound-lane`; add `msg-count`; write real descriptions for the six stub leaves.
- [ ] `[M]` W1 pre-1.0 TIM events cannot join the state entry: event `msg-id` is J2735 `packetID`, state list is keyed by operator `id` with no packet leaf (events :434-441, 452-456, 488-491, 582-589; `openits-v2x-messaging.yang:604-612`). Carry both `id` and `packet-id` on the events (or add `packet-id` readback to the state entry); retype event `duration-seconds` to minutes to match `duration-minutes`.
- [ ] `[M]` W1 pre-1.0 `rsu-broadcast-sample` (events :601-659) is an aggregate; remove `uses wire-source`.
- [ ] `[M]` W1 pre-1.0 `rsu-channel-fault` (events :314-339) is a gen-1 fault notification: no `fault-id`, no `severity`, no cleared counterpart, no channel on the inventory row (`openits-rsu.yang:743-765`). Add mandatory `fault-id` + `severity` and `uses channel-ref` on the inventory entry, or deprecate in favor of `fault-raised` with `rsu-channel-fault-kind`.
- [ ] `[M]` W2 Fault/mode vocabulary (types :165-178, 184-216): add `rsu-fault-time-sync`, `rsu-fault-spat-source`, `rsu-fault-secure-storage`, `rsu-fault-file-integrity`, `rsu-fault-access`, `rsu-fault-watchdog`, `mode-other`, `sec-scms-connection-restored`; document certificate renewal/expiry as `fault-cleared`/`fault-raised` on `rsu-fault-certificate`. `halted-stale` (`openits-v2x-messaging.yang:448-454`) has no event.
- [ ] `[M]` W1 pre-1.0 `store-forward` (:391-456) cites NTCIP 1218 for a behavior 1218 does not define (its three are `rsuMsgRepeat`, `rsuIFM`, `rsuReceivedMsg`). Cite the object or drop the attribution; if no second implementation has it, move to an augment. If kept: `max-age-seconds`, `oldest-message-age-seconds`, `storage-used-pct`, replace `messages-forwarded-today` with a windowed counter.
- [ ] `[M]` W1 pre-1.0 `spat-sync` `min-yellow-violation`, `max-green-exceeded`, `phase-gap-errors` (:516-520, 551-565) are judgments against limits the RSU does not hold; drop or move to an augment. Move `asc-poll-interval-ms` to `messages/spat/config` with a state mirror.
- [ ] `[M]` W1 pre-1.0 `rsu-certificate-expiring/certificate-type` (events :300-303) is a free string because `certificate-type` sits in the `openits-scms` core. Create `openits-scms-types`, retype, add `status` identityref; keep `days-until-expiry` only if documented as the device-reported threshold value (state is `int32`, event is `uint16`).
- [ ] `[M]` W2 US band-plan deviation (`openits-rsu-us-band-plan.yang:23-29`) narrows `dsrc-channel-number`, which exists only for DSRC/dual-mode; a C-V2X radio at 5.860 GHz validates under the "US band plan". Rebuild around a technology-neutral `center-frequency-mhz` + `bandwidth-mhz` (see V2X) with `deviate add must` on the 5.895–5.925 GHz edge for every `radio-tech`; keep the DSRC narrowing via a `dsrc-channel` refinement rather than bare `uint8`; add a `valid-*-under-*` positive fixture; cite 47 CFR Part 90 Subpart M / Part 95 Subpart L with section numbers and the FCC docket (20-164; verify the 2024 Second R&O number).
- [ ] `[M]` W3 Malformed fixtures: `invalid-rsu-spat-enabled-local-time.json` encodes `intersection` as an object (yanglint rejects the encoding before the `must`; verified). `invalid-rsu-tim-broadcast-priority-out-of-range.json` carries the removed `broadcast-at` leaf and no event header (it does fail on the range in notif mode). `invalid-rsu-tim-window.json` exercises a `must` that no longer exists and fails only on `duration-minutes: -5`. Fix the first, rebuild the second from `valid-rsu-tim-broadcast.json`, rename or drop the third.

### Minor

- [ ] `[m]` W1 pre-1.0 `operating` breaks the mirror idiom (:320-366, :620-624): `mode` mirrored as `active-mode`; `configured-time-source` has no mirror. One `operating-config` grouping used in both; rename `active-mode` → `mode`; move `diagnostics/time-source` under `operating/state`.
- [ ] `[m]` W1 Duplicated inline enums in notifications (events :540-556 `prior`/`current`; :493-517 `reason`) → `typedef tim-broadcast-state`, `typedef tim-clear-reason` in `-types`.
- [ ] `[m]` W1 pre-1.0 `gps-fix-status`, `gps-status`, `rsu-gps-status-change` name GNSS concepts (types :99-111; rsu :576-585; events :341-369). Rename `gnss-*`; event `satellites` (used) vs state `satellites-visible` (in view) → `satellites-used`. Do it with the events move since the notification rename changes its ce-type.
- [ ] `[m]` W1 pre-1.0 `backhaul-cellular/signal-quality` (:648-678) is derived from `rsrp-dbm`; `data-usage-bytes` is billing inventory. Drop both (or document bars as modem-reported).
- [ ] `[m]` W2 `rsu-broadcast-sample/broadcast` key `msg-id` is undefined for SPaT/MAP rows (events :626-636). Key on `msg-type` + optional `msg-id`, or state the per-type key rule.
- [ ] `[m]` W4 Citation format: `"NTCIP 1218:2020 (rsuMode)."`, `"IEEE 1609.2-2022"` do not match the conventions (types :96,155,169,201,226-285,302; events :52,73; rsu :90,244-246); J2735 references lack the ASN.1 path. ARC-IT object naming: "TMC" (events :243,342,603) vs "Traffic Management Center" (:417,471,524); physical object is "Connected Vehicle Roadside Equipment".

### Nits

- [ ] `[N]` 14 revisions lack `reference` (rsu :118,167,217,224,231; types :44,55; events :75,85,107,143,167,175,180). British spellings: rsu :112 "behaviours", :395; types :159 "Modelled". `sample-window-s` (events :621) vs family `-seconds`.

### Domain gaps

- [ ] `[gap]` NTCIP 1218 `rsuMsgRepeat` as a PSID-keyed store-and-repeat table (start/stop, channel, interval, priority, payload); only TIM has a home.
- [ ] `[gap]` `rsuReceivedMsg` forwarding modeled for BSM only; PSM and SRM cannot be forwarded.
- [ ] `[gap]` `rsuInterfaceLog` absent.
- [ ] `[gap]` `rsuGnssMaxDeviation` / clock deviation tolerance / clock-source timeout thresholds the RSU acts on.
- [ ] `[gap]` C-V2X: LTE-V2X vs NR-V2X, resource pool, ARFCN, sidelink RSRP, band-47 identifiers.
- [ ] `[gap]` IEEE 1609.3 WSA/WRA (`rsuWsaConfig`, `rsuWraConfig`) not modeled.
- [ ] `[gap]` SSM fidelity: `srm-request-status` lacks processing, watchOtherTraffic, maxPresence, reserviceLocked.
- [ ] `[gap]` `evp-auto-grant` defaults `true` (`openits-v2x-messaging.yang:1028-1030`); a DOT would expect opt-in.

---

## 4. V2X capability modules

Modules: `openits-v2x-messaging`, `openits-v2x-messaging-types`, `openits-v2x-radio`, `openits-v2x-radio-types`, `openits-scms`.

### Blockers and major

- [ ] `[B]` W1 pre-1.0 `typedef psid` in `openits-v2x-messaging-types` with a stated integer form (recommend the raw p-encoded octets as an unsigned big-endian integer, matching the fixtures and NTCIP 1218's octet string), `reference "IEEE 1609.12 (PSID allocations); IEEE 1609.3 Annex (p-encoding)."`, state the SPaT/MAP/TIM/BSM/PSM/RTCM values in that form. Retype all thirteen `psid` leaves (`openits-v2x-messaging.yang:402,415-422,495-504,522,623-630,762-768,871-878,929-932,969-972,988-995,1208`; `openits-scms.yang:532-537`). Existing node references cite 1609.3 for allocations that live in 1609.12.
- [ ] `[B]` W1 pre-1.0 Capability events module(s) — see RSU item 1. None of the three capabilities has an events module today.
- [ ] `[M]` W1 pre-1.0 Radio is DSRC-shaped (`openits-v2x-radio.yang:198-205, 230-239`; types :50-64, :164). Add technology-neutral `center-frequency-mhz` (uint16, `units "MHz"`) and `bandwidth-mhz` (enum 10|20) to `channel/config` with state mirrors; keep `dsrc-channel-number` as the DSRC convenience; add a `pc5` container (`when radio-cv2x`) with `sync-source` (gnss/enb/ue) and `resource-pool-id`; drop `radio-dual-mode` from the per-channel axis (keep for `reported-radio-tech`); simplify the `must` at :219-229 to `radio-dsrc` only.
- [ ] `[M]` W1 pre-1.0 MAP per-intersection attributes are still single leaves after the multi-intersection change: `geometry-version` (:485), `lane-count`, `approach-count` (:556-565). Move `revision` (0..127, cite `IntersectionGeometry.revision`, `MsgCount`) into `map/config/intersection` and mirror plus lane/approach counts into `map/state/intersection`. State in `intersection-ref` (:314-326) that a MAP with no `RoadRegulatorID` is keyed as `region 0`.
- [ ] `[M]` W1 pre-1.0 SRM active request (:1096-1163): `uses intersection-ref` (hoist to `-types`); `identity vehicle-role` (J2735 BasicVehicleRole) and retype `vehicle-class` (:1145); append `processing`, `watch-other-traffic`, `max-presence`, `reservice-locked` to `srm-request-status` (values 5..8); rename `approach` (:1117) → `inbound-lane` with `reference "SAE J2735 (SignalRequest.inBoundLane)"`; document `srm-preemption-request` (types :71-95) as role-derived, not a J2735 request type.
- [ ] `[M]` W1 pre-1.0 Remove derived BSM/PSM analytics: `unique-vehicles-1min`, `unique-vehicles-1hr`, `average-speed` (:831-845), `pedestrians-detected-1min/1hr` (:889-898). Unique-vehicle counts are unmeasurable under pseudonym rotation. Keep `received-count`, `forwarded-count`, `invalid-count`, `last-received`.
- [ ] `[M]` W1 pre-1.0 TIM region and content (:604-742): replace `region` internals (`uses geo-location` + `radius` + `direction` + `direction-tolerance`, :688-709) with `uses geo-point` + `choice shape { case circle { radius-m } case path { uses geo-path; lane-width-m } }`; retype `direction` to a 16-flag `bits heading-slice` (`reference "SAE J2735 (HeadingSlice)"`); add `frame-type` (advisory/roadSignage/commercialSignage) and `content-type` (advisory/workZone/genericSign/speedLimit/exitService) enums in `-types`; add per-message `broadcast-interval` (ms, `reference "NTCIP 1218 (rsuMsgRepeatTxInterval)"`). `geo-location` brings a second `heading` leaf beside `direction` today.
- [ ] `[M]` W1 pre-1.0 Config/state idiom on the list surfaces: `channel` key `channel-id` and mandatory `radio-tech` sit outside `config` (radio :194-215) and `state/radio-tech` (:298) mirrors a leaf that is not in `config`; `tx-power` (:250) vs `tx-power-dbm` (:311); `certificate` is a config-false list with a nested `container state { config false; }` (scms :475-491); `tim/active/message` key is not a leafref into `../config/id` (messaging :604-615). Apply the idiom; re-point the `must`/`when` XPaths.

### Minor

- [ ] `[m]` W1 pre-1.0 Counters: every monotonic counter is plain `uint64` (radio :304-309, 317-319; messaging :433, 546, 715, 821-829, 884, 904, 938, 978, 1066-1090, 1210-1217, 1228-1238; scms :387-396, 445-453, 457-464). Sweep to `yang:counter64`; widen `tx-errors`/`rx-errors` from `uint32`; delete `radio/state/security-failures` (:319) in favor of `message-errors/security/verify-failures` (:1229).
- [ ] `[m]` W1 pre-1.0 `channel-fault-none` / `radio-fault-none` (radio-types :86, :96) encode absence as a value. Remove; absence means no fault.
- [ ] `[m]` W3 Fixtures: `invalid-spat-broadcast-interval-too-high/-too-low.json`, `invalid-map-broadcast-interval-out-of-range.json` also encode `intersection` as an object (they fail on the range first, so they do prove the range; still fix the encoding). Add `valid-rsu-dsrc-alternating.json` so the radio `must` at :219 has a positive case.
- [ ] `[m]` W2 Missing constraints (each with valid/invalid fixtures): `misbehavior-reporting/enabled` ⇒ `authority-url`; `auto-renewal` ⇒ `scms-url` (scms :281-305); `grant-authority = controller-prs` ⇒ `asc-device`, and `forward-to-asc = false` contradicts `controller-prs` (messaging :1008-1027); `auto-approve-tsp` "applies only when rsu-local" → `when "../grant-authority = 'rsu-local'"` and drop its default (:1046-1054).
- [ ] `[m]` W1 pre-1.0 Derived SCMS leaves: `days-to-app-cert-expiry`, `days-until-expiry`, `crl-overdue` (scms :350-357, 377-381, 419-425, 516-521). Keep the timestamps; drop the day counts and the boolean, or keep exactly one form family-wide.
- [ ] `[m]` W4 `tx-power-dbm` regulatory prose (radio :139-153) does not match the 47 CFR 90.377 class table (A 0/23 dBm EIRP, B 10/23, C 20/33, D 28.8/33, 44.8 EIRP public-safety RSU; 2024 C-V2X rules use a flat EIRP/PSD limit). Cite the section, correct the prose, consider `range "-40 .. 28.8"` or state that 30 admits a non-US setpoint. Unverified; check before editing.
- [ ] `[m]` W4 References and vocabulary: add `reference "SAE J2735 (DSRCmsgID N)"` per message identity (18 MAP, 19 SPaT, 20 BSM, 28 RTCM, 29 SRM, 30 SSM, 31 TIM, 32 PSM) and `msg-eva` (22), `msg-ica` (23), `msg-rsa` (27), `msg-csr` (21), `msg-pvd` (26), `msg-sdsm` (41, J3224 / J2735-2023) (messaging-types :61-69); verify "SAE J2945/1 §6.3.8" (radio :279); `channel-ref/channel-id` unbounded string vs `length "1..32"` key (radio-types :72-77); SPaT 100..1000 / MAP 500..5000 ms ranges cite nothing (:356-364, 471-479; the norms come from the USDOT RSU spec / J2945 practice); `channel-busy-ratio` (:310) as `percentage-precise`; `renewal-days-before-expiry` needs `units "days"` (scms :265-270); `ssp` pattern admits odd hex length, use `([0-9a-fA-F]{2})*` (:538-541); `identified-region-id` flattens the 1609.2 CHOICE (:560-566); `radius-m` uint32 vs 1609.2 `CircularRegion.radius` Uint16 (:575-579).

### Nits

- [ ] `[N]` Canonical order: `must` after `description` at messaging :336, 339, 751, radio :219; `when` after `type` at radio :232. British spellings: scms :72, 331, 553, 570; messaging :594; radio :132. Enum `uper-1609dot2` (messaging :250) breaks the sibling naming; rename pre-1.0 or not at all.

### Domain gaps

- [ ] `[gap]` No neutral RF description: center frequency, bandwidth, PC5 sync source and resource pool, 802.11p data rate, `rsuRadioMacAddress`.
- [ ] `[gap]` No IEEE 1609.3 WSA / NTCIP 1218 `rsuWsaServiceTable`.
- [ ] `[gap]` Message-type vocabulary stops at 2016: no EVA, ICA, RSA, CSR, PVD, SDSM; no `DSRCmsgID` binding.
- [ ] `[gap]` TIM: circle-only regions, no `frameType`/content class, no `sspTimRights`, no per-message repeat interval or channel/PSID pairing.
- [ ] `[gap]` SRM/SSM: no intersection reference, no `BasicVehicleRole`, no in/outbound lane, no `RequestID`+`msgCount` pair.
- [ ] `[gap]` SCMS: `security-enabled` conflates signing with verification; single `scms-url` where 1609.2.1 has RA, ECA, MA, CRL-store endpoints; no `cert-not-yet-valid`; no "certificate currently used to sign"; no bootstrap/DCM state.
- [ ] `[gap]` BSM `filter-by-region`/`region-radius` (:803-815) and PSM `alert-asc` (:865-870) read as one product's knobs; no NTCIP 1218 or J2945 basis cited. Ask the contributor which unit they came from.
- [ ] `[gap]` Cross-capability safety musts live in the profile (`openits-rsu.yang:268-289`) and are lost by any second composer of `messaging`. Say in doc 08 where such invariants belong.

---

## 5. Dynamic message signs

Modules: `openits-dms`, `openits-dms-types`, `openits-dms-events`, `deviations/openits-dms-mutcd`, `augments/ledstar-dms-travel-time`, `docs/reference/dms-ntcip-1203-fidelity.md`.

### Blockers and major

- [x] `[B]` W1 pre-1.0 **Done 2026-09-07.** `error-type` retyped to `identityref { base openits-dms-types:activation-error; }` with two sub-bases: `activation-error-content` (dmsMultiSyntaxError, fix the message) and `activation-error-control` (dmsActivateMsgError, check the sign), plus hardware and other under the root. Twenty-five leaf identities; the seven retired enum members have a stated one-to-one migration. New fixtures: `valid-dms-activation-failed-priority.json` (the refusal the old enum could not express) and `invalid-dms-activation-error-wrong-base.json` (proves the base restriction fires). Second entry in the `buf.yaml` breaking allowlist. Original finding: it was a closed seven-member enum; `dmsMultiSyntaxError` has 15 members and `dmsActivateMsgError` (priority, messageCRC, messageStatus, memoryType, messageNumber, underValidation, localMode, centralMode, centralOverrideMode) is not represented. Replace with `identityref { base openits-dms-types:activation-error; }` with leaf identities for every operationally distinct member; keep the existing seven as identities. Correct the crc description (`openits-dms.yang:842-845`): the sign reports a CRC mismatch as `dmsActivateMsgError = messageCRC(8)`; it is silent on the face only.
- [ ] `[M]` W1 pre-1.0 `active-message` (:790-826): make it a `presence` container so the guard becomes `must "indefinite = 'true' or duration-s"` and `memory-type` can be `mandatory true`; add a reference-completeness `must` (`not(memory-type) or memory-type = 'blank' or slot-number`) to every config-side `message-reference` user (:410-420, 435-493, 732-739); add `invalid-dms-activation-no-slot.json` and `invalid-dms-fallback-no-slot.json`; decide whether `memory-type = blank` with no duration is legal.
- [ ] `[M]` W1 pre-1.0 Delete `request-self-test` (:778-789); the conventions doc names run-self-test as an operational RPC. Keep `diagnostics/last-self-test`.
- [ ] `[M]` W2 `comm-loss-timeout-s` (:437) cannot express "never revert" (NTCIP `dmsTimeCommLoss = 0`). Add `never-revert { type boolean; default false; }` with a `must` on the `comm-loss` container, or range the timeout `1..` and document 0 as illegal. Fixtures either way. Close the register's open row.
- [ ] `[M]` W2 `openits-dms-mutcd` (:11-41): cite `MUTCD 2009 §2L.05 ¶06 (Guidance)` for the 2 s floor and label it agency policy; add `deviate add must` on `/sign/messages/slot/config/multi-string` for ≤2 pages (¶04 Standard) and no flash/moving-text tags (¶05 Standard) with citation-grade error messages and `invalid-dms-three-pages-under-dms-mutcd.json` / `invalid-dms-flash-tag-under-dms-mutcd.json`; optionally `default-page-off-time-s <= 0.3` (Guidance); state that per-page `[pt]` overrides are not policed. Verify the 11th-edition wording (2009 text was fetched; 11th could not be).
- [ ] `[M]` W2 August notifications: add `base openits-types:report-event-kind;` to `dms-sign-status-report` (`openits-dms-types.yang:220-223`); add `identity dms-face-change-event-kind { base dms-event-kind; }`, rebase `dms-message-changed` under it, constrain `message-changed/kind` (events :253-256), add `invalid-message-changed-bad-kind.json`; hoist `message-body` + `message-reference` into `openits-dms-types` as `face-message` and `uses` it from both `control/state/active` and `message-changed` (events :237-330); add a `TestDMSEvent_ChangedMirrorsLiveState` conformance check.
- [ ] `[M]` W1 pre-1.0 `messages/slot` (:661-707): `uses message-body` in `slot/state` so a consumer can read the MULTI the sign actually holds; decide whether permanent-bank rows are state-only (separate config-false list, or `when "memory-type != 'permanent'"` on config); add `must "memory-type != 'blank'"` on `slot/config` (the typedef at types :134-156 mixes reference-target and storage-bank semantics).
- [ ] `[M]` W1 pre-1.0 Two `door-open` leaves on one sign (`environment/door-open` :1055; `cabinet-power/state/door-open` `openits-cabinet-power.yang:283`). Drop the environment one or document the split. `diagnostics/controller-uptime-s` (:1133-1137) duplicates `device-diagnostics:uptime-seconds`; compose `openits-device-diagnostics:system-runtime` and delete it.
- [~] `[M]` W1 pre-1.0 **`technology` done 2026-09-08** as the `display-technology` identity, adding e-paper and other; `unknown` becomes an absent leaf. The `sign-type` portable axis is still open. Original: `sign-type` lacks the portable axis (NTCIP `dmsSignType` values 129-134); `technology` (:542-552) is a closed enum on an extensible axis (e-paper exists). Add `portable { type boolean; }` (or a `mounting` enum) to capabilities; add `blank-out`/`drum` sign-type members with explicit values above 3; retype `technology` to `identityref { base display-technology; }`.

### Minor

- [ ] `[m]` W1 pre-1.0 Geometry split: `sign-width-pixels`/`sign-height-pixels`/`technology` at `state` root (:531-532) vs face/pitch/character leaves in `state/capabilities` (:561-578). Move the three into `capabilities`; split `pixel-pitch-mm` into horizontal/vertical (NTCIP `vmsHorizontalPitch`/`vmsVerticalPitch`).
- [ ] `[m]` W1 pre-1.0 `humidity-percent` (:1051-1054; events :408-411) is whole-number while ESS uses `percentage-precise`, whose description names humidity as its use case. Retype both.
- [ ] `[m]` W4 `docs/reference/dms-ntcip-1203-fidelity.md:123` says `timebasedScheduler` lands in `control/state/control-mode`; `dms-control-mode` (types :389-394) has no scheduler value. Correct: map to `activation-trigger = trigger-schedule` with `control-mode` unchanged.
- [ ] `[m]` W2 `dms-mode-event-kind` (types :191-203) covers display-state and control-authority transitions under one kind. Add `dms-display-state-changed` and `dms-control-mode-changed` leaf identities.
- [ ] `[m]` W2 LEDSTAR augment (`ledstar-dms-travel-time.yang:91-177`): config-true and config-false leaves interleaved in one list entry (:153-176) → `config`/`state` containers; `source` (:159-165) free string → identity; `units` after `must` (:126, :156) breaks canonical order. At graduation: hoist route/destination/current-minutes/updated-at/stale-after-s/displayed into a core `travel-time` capability with an open `travel-time-source` base; leave templating/rounding in the augment. The concept is doc-06 rule 3 (universal concept, vendor value set), not rule 2 as its text claims.
- [ ] `[m]` W1 pre-1.0 `crc` (:391-399) is `uint32` "for proto-tag stability"; ygot maps every unsigned width ≤32 to proto `uint32`, so `uint16` is proto-neutral. Retype or drop the sentence.
- [ ] `[m]` W2 Adopt `command-provenance` for `owner`/`source` strings (:909-919).

### Nits

- [ ] `[N]` Canonical order: `must` after `description` at :725, :802. Enum members without descriptions: :519-526; types :136-140. Revisions without `reference`: dms :212, 219; events :28, 82, 130, 136, 147, 153; types :33, 96. British spellings: "colour" :593-597, types :235-236; "honoured" :322, types :389; "modelled" :718, types :300; "materialises" :796. ARC-IT object "ITS Roadway Equipment" (events :334) vs "Roadway Equipment" elsewhere in the slice.

### Domain gaps

- [ ] `[gap]` Portable CMS: no portable flag / trailer class.
- [ ] `[gap]` Brightness table (`dmsIllumNumBrightLevels` / `dmsIllumBrightnessValues`); `illumination-control` has no `other`.
- [ ] `[gap]` Pixel/lamp failure location (`dmsPixelFailureTable` x, y, stuck-on/off); counts only today.
- [ ] `[gap]` Message defaults (`dmsDefaultFont`, justification, flash on/off, fore/background RGB).
- [ ] `[gap]` Temperature envelope: ambient, housing min/max, `dmsTempCritical*` (open in the register).
- [ ] `[gap]` Policy limits on message content (max pages this agency permits, flash permitted) so the MUTCD deviation has something to bind to.
- [ ] `[gap]` `[f]` field values (time, temperature, speed) have no state home.

---

## 6. ESS and traffic-sensor

Modules: `openits-ess`, `openits-ess-types`, `openits-ess-events`, `openits-traffic-sensor`, `openits-traffic-sensor-types`, `openits-traffic-sensor-events`, `openits-vendor-trafficvision-traffic-sensor-types`, `augments/trafficvision-traffic-sensor-camera`.

### Blockers and major

- [ ] `[B]` W1 pre-1.0 ESS has no observation-report notification (`openits-ess-events.yang` has only `weather-alert` :106 and `sensor-recalibrated` :171). Hoist all ESS typedefs (`celsius`, `precipitation-type`, `visibility-situation`, `pavement-condition`, `bearing-degrees`, ...) and the observation containers' leaves (`openits-ess.yang:405-653`) into groupings in `openits-ess-types` (`atmospheric-observation`, `precipitation-observation`, `visibility-observation`, `pavement-observation`, `radiation-observation`); add `identity ess-observation-report { base ess-event-kind; base openits-types:report-event-kind; }` and the notification; core containers `uses` the same groupings; add `valid-ess-observation-report.json`.
- [ ] `[M]` W1 pre-1.0 `weather-alert` (events :244-307) references thresholds that exist nowhere in `openits-ess`. Recommended: add `station/thresholds/threshold` config list keyed by `threshold-id` (`quantity` identityref into a new `ess-observed-quantity` base, `comparator`, `value`, `hysteresis`, `sensor-id`), documented as device-enforced; make `threshold-id` and `sensor-id` mandatory on the event; drop `unit` (implied by `quantity`). Alternative: delete the notification. Do not ship the unanchored form.
- [ ] `[M]` W4 FHWA Scheme F contract is wrong (`openits-traffic-sensor.yang:381-394, 437-451`; types :561-568): Scheme F is defined by axle count and spacing, so axle-count bounds cannot reproduce it any more than length can. Under `scheme-fhwa-13` define `class-id` = FHWA class 1..13; make `min/max-axle-count` and `min/max-length-m` descriptive-only for axle schemes; delete "only when the bins carry axle-count bounds". Add a valid fixture with axle bounds (none exists).
- [ ] `[M]` W1 pre-1.0 `queue-state-changed/zone-id` (events :171-175) is optional `string`; `queues/queue-zone` has no `measured-at` (ts :454-463, 557-575). Add `typedef queue-zone-id` in `-types`, use it for config `zone-id` and the notification (`mandatory true`); add `measured-at` to the state list.
- [ ] `[M]` W2 Queue change trigger undefined (types :456-461; events :151-176): narrow the notification's base to `ts-queue-state-changed` (wire-compatible), add `ts-queue-formed`, `ts-queue-cleared`, `ts-queue-extent-changed` under it, state that duration ticks do not emit, add `invalid-queue-state-changed-report-kind.json`.
- [ ] `[M]` W1 pre-1.0 Pavement `state` (ess :568-626) lacks the `quality` flag; `atmospheric` (:405-477) bundles 2-4 instruments under one `sensor-id`/`quality`. Hoist `quality` to `typedef observation-quality` in `-types`; add it to pavement; split `atmospheric` into `air-temperature`, `humidity`, `pressure`, `wind` each with `uses observation-metadata`; consider `list temperature-sensor` keyed by sensor-id with `height-m` (1204 `essTemperatureSensorTable`).
- [ ] `[M]` W1 pre-1.0 `diagnostics/sensor/state/type` (ess :678-679) is a free-form string. Add `identity ess-sensor-type` in `-types` (air-temperature, humidity, pressure, wind, precipitation, visibility, pavement, subsurface, solar-radiation, snapshot-camera, water-level) and retype.
- [ ] `[M]` W1 pre-1.0 `openits-vehicle-detection` is not composed by traffic-sensor and should not be as-is (see section 2). Do not compose it; the universal per-lane surface is `lane-interval-data`.

### Minor

- [ ] `[m]` W4 NTCIP 1204 citation drift: `visibility-situation` (ess :48-58, 224-244) claims 1204 members that are not (mist-haze, sand-storm, sleet, heavy-fog, heavy-rain, heavy-snow) and misses patchy-fog, vehicle-spray, sun-glare, swarm-of-insects (append as 13-16; verify against the v04 MIB); `precipitation-type` (:190-210) cites `essPrecipSituation` but the richer set is WMO present-weather, reword as "extends"; `essSnowDepth` (:519) → cite `essRoadwaySnowDepth` and add `adjacent-snow-depth-mm`; `essMobileFriction` (:591-598) is a mobile-ESS object cited for a fixed puck; subsurface (:619-625) folds one probe into each pavement entry and drops `essSubSurfaceMoisture` → add `subsurface-moisture-percent`.
- [ ] `[m]` W4 Wind direction: `wind-direction-deg` says "0 and 360 both denote north" (:462-469) but `bearing-degrees` is 0.0..359.9 (:166-176) and a fixture proves 360 is rejected; module header (:36) says 0..360; `intensity` (:491-492) omits `violent`. Fix the descriptions.
- [ ] `[m]` W1 Platform duplication: `celsius` (see section 0); `uptime-s` (ts :594-606) vs `uptime-seconds`; neither ESS nor traffic-sensor composes `openits-device-diagnostics` or `openits-cabinet-power`; ESS has no uptime/door/battery/line-voltage though 1204 carries a power/door group.
- [ ] `[m]` W1 pre-1.0 `flow-rate-vph` (types :280-295) self-describes as derivable; drop. Keep `density` but describe it as the vendor's own estimate.
- [ ] `[m]` W2 Queue zone has no geometry (ts :454-482); `downstream-reference` is prose. Add `container downstream-point { uses geo-point; }` (or `geo-path`); drop `carriageway` (derivable via `lane-reference`).
- [ ] `[m]` W2 Vendor identities: `inactive-disabled-by-user` (vendor :303-308) bases on `inactive-reason` rather than core `inactive-disabled` (types :542-546), so the fleet filter misses it; `inactive-low-view-quality` (:309-314) loses the preset half of the compound source value. Re-base; document the mapping.
- [ ] `[m]` W2 ESS sensor heights (:377-402) are config-only but device-reported in 1204; add a `state` mirror via grouping.

### Nits

- [ ] `[N]` pyang: 50 enum members without description (ess :192-274); revisions without `reference` (ess :98; ess-events :83, 89, 98; ts :115, 125; ts-events :36, 58, 84, 97; augment :62, 72). British spellings: "modelled" (ts-types :328), "travelling" (:355). `data-quality` typedef sits under the "Reusable groupings" banner with broken indentation (ts-types :181-202), description duplicated at the leaf (:296-302). `calibration/status default "unknown"` on a config-false leaf (ts :641). `lane` is really a detection zone (ts :486-489). Bin contiguity/non-overlap is prose-only (ts :398-405); say so in the description.

### Domain gaps

- [ ] `[gap]` Per-vehicle records (timestamp, lane, speed, length, class) for ATSPM, speed studies, WIM reconciliation.
- [ ] `[gap]` Speed-bin histograms per interval.
- [ ] `[gap]` Detector-health thresholds the device enforces (NEMA TS-2 no-activity, max-presence, erratic-count timers) are missing from config while the fault identities exist (ts-types :503-520).
- [ ] `[gap]` ESS station category and site description (`essNtcipCategory`, `essNtcipSiteDescription`), snapshot camera table, mobile ESS.
- [ ] `[gap]` ESS water-level (flood) and air-quality groups; roadway vs adjacent snow depth.
- [ ] `[gap]` Multiple temperature sensors at heights; subsurface moisture.
- [ ] `[gap]` Zones vs lanes: stop-bar and advance zone on one lane, or a zone spanning lanes, cannot be expressed with a `lane` list keyed 1..32.

---

## 7. Ramp metering and reversible lane

Modules: `openits-ramp-metering`, `openits-ramp-metering-types`, `openits-ramp-metering-events`, `openits-reversible-lane`, `openits-reversible-lane-types`, `openits-reversible-lane-events`.

### Blockers and major

- [ ] `[B]` W1 pre-1.0 AWS interlock (`openits-ramp-metering.yang:401-408`) applies to every metering mode; MUTCD 11th ed. §4P.02 makes the W3-8 sign Guidance and only for part-time meters. Give `advance-warning-sign` a config-side `installed` boolean (or `presence`), condition the `must` on it, move the "metering requires a beacon" rule to a jurisdiction deviation, `when "../installed = 'true'"` on `aws-fault-action` (:531-540).
- [ ] `[M]` W4 ARC-IT: "TI03 (Traffic Metering)" → TM05 everywhere (:34-35, 92, 124, 139, 231-232; types :53, 61); annotate config with `TMC -> ITS Roadway Equipment : traffic metering control`, state with `ITS Roadway Equipment -> TMC : traffic metering status`, detector readings with `traffic detector data` (:424-425, 570-571, 810-811, 904). Reversible-lane: use TM16's `reversible lane control` / `reversible lane status` rather than `lane management control/status` (`openits-reversible-lane.yang:258-259, 278-279, 342-343`).
- [ ] `[M]` W4 MUTCD "Chapter 4M" → 4T (lane-use control signals) and 4P (ramp signals) including the user-facing error messages (`openits-reversible-lane.yang:50, 65, 109, 122, 200, 489, 493`; types :50, 84, 126); "NTCIP 1207:2017" does not exist → `NTCIP 1207:2014 §<section>` (`openits-ramp-metering-types.yang:110, 179`; bare "NTCIP 1207 v02" / "MUTCD" at ramp :92, 124, 138).
- [ ] `[M]` W1 pre-1.0 Delete `headway-s` (:597-609, self-described as derived) and the consistency `must` at :694-712; rewrite feasibility (:726-739) against the rate: `(min-green + yellow-change + red-clear) * release-rate-vph <= 3600 * vehicles-per-green * count(../../../lanes/lane[not(config/bypass = 'true')])`. Add `control/state/current-cycle-s` if an observed cycle is wanted.
- [ ] `[M]` W1 Composed `yellow-change > 0` (`openits-nema-common.yang:152-168`) forbids the two-section ramp face MUTCD §4P.02 permits, and the feasibility `must` skips when yellow is omitted (:727). Relax nema-common to `>= 0.0` with the description stating 0 = two-section face (the signal-control deviation already imposes 3.0-6.0), or give ramp its own `ramp-release-timing` grouping; drop `not(yellow-change)` from the feasibility `must`.
- [ ] `[M]` W2 `active-plan-id` (:269-272) is defined once in `meter-control-config` with `require-instance false`, so config intent may name a plan that does not exist. Declare it strict in `control/config` and `require-instance false` in `control/state`.
- [ ] `[M]` W1 pre-1.0 Reversible-lane has two commanded surfaces with no stated authority (`control/config` :268-338 vs per-lane `lcs-direction-a/b` :482-510). Add `control/config/lcs-control-mode { facility-sequenced | per-lane-manual }` (default facility-sequenced), state that per-lane config is honored only in `per-lane-manual`, add the guarding `must`; or drop per-lane config and keep per-lane state.
- [ ] `[M]` W2 Reversible-lane has no mode family, `control-source`, or mode-event sub-base (types :135-150; `last-command-source` :359-366 is a free string). Add `reversible-lane-mode-event-kind` (dual-based on `mode-event-kind`), a `facility-mode` family (`mode-automatic`, `mode-manual-local`, `mode-maintenance` dual-based on `openits-types:mode-maintenance`, `mode-failed` on `mode-failed-generic`), `control/config|state/mode`, `control/state/control-source`.

### Minor

- [ ] `[m]` W2 `lcs-indication` (types :116-129) omits MUTCD §4T.02's white two-way and one-way left-turn arrows. Append values 6 and 7; extend the :487/:491 musts so a white arrow toward one direction requires red-x opposite.
- [ ] `[m]` W1 pre-1.0 Notification `kind` constrained to the service root (rm-events :131-139, 155-163, 189-197; rl-events :122-131, 169-177, 213-221). Add `rm-rate-event-kind` / `rm-queue-override-event-kind` / `rl-facility-state-event-kind` sub-bases (second base on the existing leaf identities, wire-neutral), narrow each `kind`, add an invalid fixture per notification. `lane-state-changed` should share a grouping with `control/state`.
- [ ] `[m]` W1 pre-1.0 Identity prefixes inconsistent within each `-types` module (`ramp-meter-*`, `rm-*`, `mode-*`, `metering-algorithm-*` at rm-types :156-289; `reversible-lane-*`, `rl-*`, bare `sweep-confirmed` at rl-types :135-230). Pick one per module.
- [ ] `[m]` W1 `release-coordination` enum written twice (:816-822, :827-833) → `typedef release-coordination` in `-types` and a `lanes-config` grouping used by both containers.
- [ ] `[m]` W2 Units and types: `count-vph` (:363), `min/max-flow-vph` (:796-797) no units; `queue-vehicles` (types :145-150) no units; `signal-head-faults` "since reset" as uint16 (:934-937) → `yang:counter32`; `controller-uptime-s` uint32 (:925) vs `device-diagnostics` uint64 → compose `system-runtime`. Reversible-lane has no diagnostics or comm-health container despite a `plc-communication` fault kind.
- [ ] `[m]` W2 Missing constraints: `when "../bypass = 'true'"` on `bypass-operation` (:316-325); band `max-occupancy-pct > min-occupancy-pct` and `plan-id` presence (:790-799); mainline-detector `role = 'advance' or role = 'merge'` (:903-915); reversible `config` must `direction-a and direction-b` (rl :229-255); `target-direction` `when`-guarded on `target-state = 'open'` (:298-307); lane config `gated` boolean so `gate-state` absence is interpretable (:527-530).
- [ ] `[m]` W3 Fixture gaps: `invalid-rm-headway-inconsistent.json` (the only candidate today has zero lanes so `count()` short-circuits the must); `invalid-reversible-lane-fyx-b-without-red-x-a.json` (no `flashing-yellow-x` case; :491 only tested jointly); a `valid-lcs-conflict-detected` variant with a `plc-register` source block (the module's only wire-source case is untested); one ramp-rooted `invalid-rm-min-green-zero.json` (the composed nema-common musts have signal-control fixtures only).

### Nits

- [ ] `[N]` Descriptions: role description omits `hov-bypass`, `advance` (:357); `aws-fault-while-metering` labeled "Derived" (:556-559) though the device acts on it; mainline `list detector` has no description (:909, pyang error); "prior/current strings" (types :176-178) are identityrefs; `queue-override-activated` has `threshold-vehicles` but no `threshold-pct` for an occupancy trigger, `queue-override-cleared` carries no trigger axis (rm-events :166-179); `segment-id`/`lane-id` untyped `string` vs core `length "1..64"` (rl-events :224-235). Canonical-order errors at rm :402, 676, 687, 705, 734, 755, 847 and rl :487, 491.

### Domain gaps

- [ ] `[gap]` No ramp performance/interval report (no `report-event-kind` derivative): metered volume per lane, queue-detector occupancy, mainline occupancy, override duration.
- [ ] `[gap]` NTCIP 1207 dependency groups: one meter-wide `release-coordination` cannot express mixed alternate/independent lanes or per-group timing.
- [ ] `[gap]` Wrong-way and intrusion detection events (TM16 includes them); today only interlock preconditions (rl-types :211-212).
- [ ] `[gap]` Reversible vs fixed lanes flag; `transition/phase` (:400-417) has no `aborted`/`failed`; `lane-flow-state` cannot distinguish operator-closed from conflict-driven fail-safe closed.
- [ ] `[gap]` Ramp fault catalog: no `ramp-meter-fault-advance-warning-sign`, no conflict-monitor category for dual-lane heads (rm-types :187-209).
- [ ] `[gap]` Comm-loss fallback policy for ramp (DMS has `message-fallback`); reversible-lane has no comm-health surface.
- [ ] `[gap]` Traffic-responsive binding: which mainline detectors feed the band table and over what smoothing interval; queue thresholds do not name the queue detector.

---

## 8. Perception, zone-occupancy, work-zone

Modules: `openits-perception`, `openits-perception-types`, `openits-perception-events`, `openits-zone-occupancy`, `openits-zone-occupancy-types`, `openits-zone-occupancy-events`, `openits-work-zone-types`, `openits-work-zone-events`, `openits-vendor-trafficvision-perception-types`, `augments/trafficvision-perception-incident-media`.

### Major

- [x] `[M]` W1 pre-1.0 **Done 2026-09-07.** Replaced with the cardinal WZDx v4 set (northbound/southbound/eastbound/westbound, inner-loop/outer-loop), keeping `wz-direction-both` and adding `wz-direction-unknown` so an unreported direction is not read as 'both'. `valid-work-zone-cardinal-direction.json` added. Original finding: the identities were relative to a linear reference the event never carries. Replace with cardinal identities (`wz-direction-northbound`, `-southbound`, `-eastbound`, `-westbound`, `-inner-loop`, `-outer-loop`), keep `-both`, add `-unknown`. Optionally add an optional `linear-reference`-shaped container (route, measure-start, measure-end).
- [ ] `[M]` W2 Work-zone is events-only with no resync story (events :29-38, 49-67; types :19-23). Add a re-report clause to `zone-state-changed` mirroring `openits-zone-occupancy-events.yang:190-193` (feed MUST re-emit every active zone on reconnect); either add a feed-hosted `config false` `work-zones/work-zone` list composing a `work-zone-state` grouping shared with the notification, or record in doc 04 why this phenomenon alone has no twin; reword the header so it forbids a device profile for the road event, not for future WZDx field devices.
- [x] `[M]` W2 **Done 2026-09-07.** Added `worker-presence` (a presence container: `workers-present`, `method` identity, `last-confirmed-at`), `reduced-speed-limit-kmh`, `work-zone-type` (static/moving/planned-moving-area), `start-verified`/`end-verified`, and the eight missing `lane-impact` members. `valid-work-zone-workers-present.json` added. `event-type` and `road-names` remain. Original scope: `worker-presence` container (`workers-present`, `method` identity, `last-confirmed-at`), `reduced-speed-limit-kmh` (`openits-types:speed-kmh`), `event-type` identity (work-zone/detour/restriction), `work-zone-type` identity (static/moving/planned-moving), `road-name` leaf-list, `start-verified`/`end-verified` booleans, the missing `lane-impact` values (all-lanes-open, merge-left/right, shift-left/right, split, flagging, temporary-traffic-signal, unknown); consider a `lane` list (`lane-order`, `status` identity).
- [x] `[M]` W1 pre-1.0 **Done 2026-09-07.** Removed `occupancy-count` and `presence` from `zones/zone` (tags tombstoned); `average-speed-kmh` stays, since throughput is perception's own function. `zone-function`'s `presence` member is `status deprecated` rather than deleted, because the generator tombstones retired field tags but has no mechanism for retired enum values, so deleting it would have left value 2 unreserved for a future member to inherit. Original finding: perception still carried `occupancy-count` and `presence` and `zone-function = presence` (:211-222) after zone-occupancy took ownership of presence (:683-685 composes it). Remove them (keep `average-speed-kmh`), drop `presence` from `zone-function`, point the container description at the capability; document that an operator configures the same id in both lists if a perception zone is also an occupancy region.
- [x] `[M]` W2 **Done 2026-09-07.** `disposition` and `reviewed-by` added to `zone-incident-updated` once the enum-sharing fix below unblocked them (`valid-zone-incident-reviewed.json`). `clear-reason` added to `zone-incident-cleared` (four identities) and `first-observed` to `zone-incident-detected`, so detection latency is computable. Fixtures added for both. Original scope: add `disposition` (and `reviewed-by`) to `zone-incident-updated` (events :233-238); add `clear-reason` identity to `zone-incident-cleared` (:241-265: `pcp-clear-no-longer-observed`, `-operator-false-alarm`, `-zone-removed`, `-sensor-fault`); add `first-observed` to `zone-incident-detected` and state that `occurred-at` is the alarm time.
- [x] `[M]` W2 **Done 2026-09-07.** Added `incident-collision`, `incident-stopped-vehicle-shoulder`, `incident-smoke-fire`, `incident-slow-vehicle`, `incident-animal-in-roadway`, `incident-other`. `valid-zone-incident-collision.json` added. Re-basing the TrafficVision vendor identities remains. Original scope: add `incident-collision`, `incident-stopped-vehicle-shoulder`, `incident-smoke-fire`, `incident-slow-vehicle`, `incident-animal-in-roadway`, `incident-low-visibility`, `incident-speeding`, `incident-other`; graduate or re-base the TrafficVision identities (vendor :73-90; `incident-generic` papers over the missing core catch-all).

### Minor

- [x] `[m]` W1 pre-1.0 **Done 2026-09-07.** `temporary-signal`'s `default "false"` removed, so absent now means the feed did not report one rather than an assertion that no temporary signal is present. Most upstream feeds carry no such field, so the default made every one of them make that claim.
- [x] `[m]` W1 pre-1.0 **Done 2026-09-08.** Now the `zone-function` identity (`zf-count`/`-speed`/`-incident`/`-wrong-way`) in `-types`, with the wrong-way `must` rewritten to `derived-from-or-self`. This also retired the deprecated `presence` enum member: with the enumeration gone there is no value space left to protect. Original: `zone-function` was a closed enum on a vendor-extensible axis with no member descriptions. Convert to an identity base with `zf-*` members in `-types`, or at minimum add descriptions.
- [ ] `[m]` W1 `geo-polygon` grouping (see section 0) and use it for perception `vertex` (:340-353).
- [ ] `[m]` W2 Perception incident notifications omit `wire-source` though the incident `type` mirrors a device-reported field (events :151-216; vendor reference :69-70). Add `uses wire-source` to detected/updated/cleared; consider a `vendor-api` tag case.
- [ ] `[m]` W3 The registry JSON schema (`schema-registry/openits-work-zone-events/2026-09-02/schema.json:30-48`) drops `min-elements` while the AsyncAPI payload carries `minItems: 1` (`bindings/nats/asyncapi.yaml:7660-7676`); the YANG description (events :111-115) says neither projects it. Make the registry emitter project `min-elements`, then fix the description. Generator fix.
- [ ] `[m]` W3 Fixture gaps: `invalid-perception-zone-two-vertices.json` (yanglint enforces `min-elements`); `valid-perception-wrongway-zone.json` (the wrong-way `must`'s satisfied branch is never exercised); `valid-zone-occupancy-changed-occupied.json` (no fixture carries `zoc-zone-occupied`); complete the event header in `invalid-zone-incident-type-wrong-base.json` so it isolates the identity-base failure.
- [ ] `[m]` W2 `wz-comm-health-event-kind` (wz-types :88-103) has no leaf identities, so the fleet filter its description advertises never matches (same shape as `sc-comm-health-event-kind`). Add `wz-comm-lost` etc. dual-based on the common leaves, or reword to say the sub-base is generator metadata and attribution is by subject.
- [ ] `[m]` W4 Doc drift: `docs/09-coverage-scope.md:37-38` (work-zone "Planned"); `docs/data-model.md:33-34` module table omits zone-occupancy and work-zone; `docs/05-standards-alignment.md:23` parking "no OpenITS service yet" (zone-occupancy covers the NTCIP 1208 occupancy half).

### Nits

- [ ] `[N]` Canonical order: `must` after `key`/`description` at perception :288 (pyang error). `zone-incident-detected/type` description omits congestion / slowed-traffic (events :182). `data-interval-s` floor 10 s (perception :274-277) vs `interval-duration-s` floor 1 s (events :297-300).
- [ ] `[N]` ARC-IT flow tags look borrowed (unverified): object tracks tagged "traffic images" (perception :360-361); parking/bay occupancy tagged "traffic flow" (`openits-zone-occupancy.yang:159-160`; events :66-67, 170-171) rather than a PM01 parking-availability flow.
- [ ] `[N]` W1 pre-1.0 Three unrelated things are called `zone-id`; `openits.work-zone.zone-state-changed.v1` is disambiguated only by the service token (Go `ZoneStateChanged`). Rename to `work-zone-changed` now.

### Domain gaps

- [ ] `[gap]` No lane binding on perception zones (lane number, lane count, approach); cannot join to traffic-sensor lanes or say lane vs shoulder.
- [ ] `[gap]` Work-zone worker presence and reduced speed limit (covered above).
- [ ] `[gap]` TPIMS truck parking has no region kind (bay/row/lot/curb-segment/lane-segment); consider `region-kind` identity and `sensing-manual` for verified counts.
- [ ] `[gap]` `object-class` and traffic-sensor classification bins have no crosswalk; `object-truck` is one bucket (FHWA 5-7 vs 8-13).
- [ ] `[gap]` No AID detection-performance surface: alarm-vs-onset time, per-zone alarms raised / confirmed / false in `zone-interval-report`.
- [ ] `[gap]` Work-zone `occurred-at` is ambiguous between effect time and publish time (events :63-65); WZDx separates `update_date` from `start_date`.
- [ ] `[gap]` Portable-device location is neither solved nor scoped (doc 09 :59-67): the geo-path is the zone's, not the device's.

---

## 9. CCTV and the common event modules

Modules: `openits-cctv`, `openits-cctv-types`, `openits-cctv-events`, `openits-common-fault-events`, `openits-common-mode-events`, `openits-common-comm-health-events`.

### Blockers and major

- [ ] `[B]` W2 Add `base openits-types:operational-mode;` to `cctv-control-mode` (`openits-cctv-types.yang:80-82`; DMS precedent `openits-dms-types.yang:338`); add `valid-cctv-mode-changed.json`; make the CCTV mock emit `mode-changed` (`tools/conformance/mock_cctv.go:164-216`).
- [ ] `[B]` W2 `comm-health-event` fans out to 2 of 10 services (`tools/yang-proto-gen/catalog.go:104-111`; sub-bases only at `openits-signal-control-types.yang:380` and `openits-work-zone-types.yang:89`). Preferred: fan `openits-common-comm-health-events` to every service root unconditionally (its six kinds are service-neutral). Alternative: add `<svc>-comm-health-event-kind` dual-bases to the eight other `-types` modules. Correct the module description (:17-28) to state the actual rule.
- [ ] `[M]` W2 `comm-lost` is observer-synthesized but nothing says so (`openits-common-comm-health-events.yang:119-131, 220`; `openits-types.yang:831-841`). State in `comm-lost`/`comm-restored`: emitted by the observing poller, `observed-by` MUST be present, `occurred-at` is the observer's clock. Update `valid-comm-health-event.json` (no fixture in the tree sets `observed-by`); add a conformance assertion; consider `when "not(derived-from-or-self(../kind,'...:comm-lost'))"` on `source`.
- [ ] `[M]` W2 `comm-attempt-window` (:95-100, 165-196) has no window; `percent-loss` is derived. Add `window-seconds` (`when` the comm-attempt-window kind); drop `percent-loss` pre-1.0 or document it as populated only when the counters are not.
- [ ] `[M]` W1 pre-1.0 PTZ `absolute` and `velocity` are sibling presence containers (`openits-cctv.yang:176-225`); a config with both validates. Make `choice move { case absolute; case velocity; }` in `ptz/config` and in `ptz-move-commanded` (events :106-107, 126-172); drop `move-mode` on the notification or restrict it to a two-member typedef (`ptz-move-mode` admits preset/tour/idle, which the description forbids).
- [ ] `[M]` W2 Continuous moves have no device-side timeout. Add `ptz/config/continuous-move-timeout-ms` (uint16, `units "milliseconds"`, `reference "NTCIP 1205 cctvTimeout"`) outside the presence container, with a readback in `ptz/capabilities`.
- [ ] `[M]` W1 pre-1.0 Presets (:265-310) are operator-written coordinates; devices store presets as "store current position" (`presetStorePosition`/`presetGotoPosition`; ONVIF `SetPreset`). Make `name` the only config; move coordinates to per-entry `state` (absent when the device cannot read back); add `presets/store { leafref ../preset/preset-id }` as the store command using the request-token pattern below.
- [ ] `[M]` W2 Capabilities (:148-165, 517-544) omit PTZ limits and equipment availability. Add `pan-min/max-degrees`, `tilt-min/max-degrees`, `zoom-ratio-max`, `home-preset`, `has-wiper/washer/heater/blower`, `supports-relative-move`, `focus-controllable`, `iris-controllable` (`reference "NTCIP 1205 cctvRange"`); cite `rangeTrueNorthOffset` on `pan-reference-offset-deg`.
- [ ] `[M]` W2 ONVIF provenance case in `wire-source` (see section 0).
- [ ] `[M]` W1 pre-1.0 Control ownership (:590-606; types :74-101; events :210-247): drop the NTCIP 1205 attribution (1205 has no ownership/priority/lockout objects; unverified but confident); describe `lockout-denied` as the interim form of the future `command-rejected`; adopt `command-provenance` (`actor-id`/`actor-class`) for `requested-by`, `commanded-by`, `recalled-by`, `current-holder`.

### Minor

- [ ] `[m]` W1 pre-1.0 `washer` (:523-535) is device-auto-cleared config. Replace with `washer-cycle-requested-at { type yang:date-and-time; }` (idempotent under replay; the device acts once per distinct value); same pattern for preset `store`.
- [ ] `[m]` W1 pre-1.0 Pan/tilt `fraction-digits 1` (:183-191, 233-247, 278-287; events :137-146) is coarse for preset repeatability at 30x; `pan-reference-offset-deg` is whole-degree `heading-degrees` (:125-132). Use `fraction-digits 2` everywhere; make the offset a matching decimal64.
- [ ] `[m]` W1 pre-1.0 `operational-status = degraded` (:100-106; types :203-216) is computed from stream health and faults. Drop `degraded` or state it is poller-computed and not reconcilable against the fault list. Conformance requires the leaf (`tools/conformance/tests/cctv.go:159-165`).
- [ ] `[m]` W4 Tours (:21-23, 312-367): module description lists them under NTCIP 1205, which defines no tour objects; cite `ONVIF PTZ Service Specification (PresetTour)`; drop `ordered-by user` (double ordering with the `sequence` key); consider per-stop `speed-percent`.
- [ ] `[m]` W2 Add `relative` move mode (types :218-227) with a delta case; add `environment/config/power` boolean with `power-on` readback.
- [ ] `[m]` W2 Compose `openits-device-diagnostics` on `camera`; document that `comm-health-event/link-id` references the device's `comm-link-state` entries; `enclosure-temp-c` duplicates what diagnostics would carry.
- [ ] `[m]` W1 pre-1.0 `presets/recall` (:293-299) and `tours/run` (:345-350) are bare command leaves beside a `state` container; wrap in `container config`.
- [ ] `[m]` W3 `mode-changed` prior/current are not tied to one mode space (`openits-common-mode-events.yang:90-118`): document "prior and current MUST share an immediate base; a family with several mode spaces derives one mode-event-kind per space" (DMS has two under one kind). Fix the DMS and ramp mocks, which emit bare strings (`tools/conformance/mock_driver.go:625-627, 1241-1244`) and add a harness check that both values are module-qualified and derive from `operational-mode`.

### Nits

- [ ] `[N]` `openits-cctv.yang:29-30` says arbitration "lands in a follow-up revision"; it shipped (:75-82). `stream/state/uri` should be `inet:uri`; `frame-rate` lacks the unit suffix its siblings carry. `fault-raised/severity` is mandatory while `fault-entry/severity` is optional (`openits-types.yang:883-886`). `privacy-masks` cites "NTCIP 1205 / camera privacy zones" (:422-428); 1205's zone table is an on-screen label, not a blanking mask (unverified). British spellings: "honoured" :91, 373, 386, 394, 622, 634; types :91; "favour" in events revisions.
- [ ] `[N]` W4 `docs/data-model.md:36` says gen-1 per-service fault/mode notifications "are deprecated"; they were deleted (`openits-ess-events.yang:75-79`, `openits-dms-events.yang:120-127`, `openits-ramp-metering-events.yang:86`; `catalog.go:97-99`). One gen-1-shaped survivor: `rsu-channel-fault` (section 3).

### Domain gaps

- [ ] `[gap]` A camera that stops answering has no `comm-lost` subject (covered by the fan-out item).
- [ ] `[gap]` No speed on absolute goto (1205 position speed byte; ONVIF `Speed`), no home position.
- [ ] `[gap]` Camera power, on-screen label/ID overlay (`cctvLabel`), day/night (IR-cut) mode, backlight compensation.
- [ ] `[gap]` ONVIF is invisible: no provenance case, no reference, no acknowledgment that tours, normalized zoom, and paused tours are ONVIF shape.
- [ ] `[gap]` Stream description lacks a profile/token and multicast address.

---

## 10. Mechanical audit (whole tree)

Evidence-driven items not already listed above.

- [ ] `[M]` W3 Eight `must`s have no invalid fixture: `openits-ramp-metering.yang:705`; `openits-signal-control-mutcd.yang:102, 134, 154`; `openits-signal-control-mutcd-strict.yang:69, 81, 104, 114`. (Also listed under sections 1 and 7.)
- [ ] `[m]` W3 Three `must`s have no valid fixture exercising the guarded node: `openits-perception.yang:288` (wrong-way), `openits-traffic-sensor.yang:446` (axle-count), `openits-v2x-radio.yang:223` (alternating).
- [ ] `[m]` W3 `yang/testdata/README.md` lists `invalid-yellow-below-mutcd.json`, `invalid-min-green-below-four.json`, `invalid-red-clear-below-one.json`, none of which exist; its table covers 8 of 209 fixtures; it claims every `-under-` fixture is base-valid but only `valid-yellow-3.5-base.json` is a base-valid twin and no gate proves the rest. Rewrite; add twins or make `check-deviations` prove base-alone acceptance.
- [ ] `[m]` W1 pre-1.0 Retype four string leaves: `openits-rsu-events.yang:300 certificate-type`; `openits-v2x-messaging.yang:1157 denial-reason` (no denial vocabulary exists); `openits-ess.yang:333, 340 sensor-id` → `openits-ess-types:sensor-id` (already used by ess-events); `openits-traffic-sensor.yang:460 zone-id` → a typedef.
- [ ] `[m]` W1 Eleven typedefs live in core modules rather than their `-types` companion and are therefore unreachable from events modules: `openits-signal-control.yang:426, 439, 480, 489, 497, 512, 526, 535, 546, 563, 573`; also `openits-dms.yang:299 pixel-count`; `openits-ess.yang:166-270` (seven; ess-types has one); `openits-ramp-metering.yang:239`; `openits-reversible-lane.yang:207`; `openits-perception.yang:211`; `openits-v2x-messaging.yang:196-297` (six); `openits-v2x-radio.yang:125, 139`. Hoist the ones an events module will need.
- [ ] `[m]` W2 Add `units` to 24 leaves: `openits-cctv.yang:476, 480` (pixels); `openits-device-diagnostics.yang:76, 77` (bytes), `:98-100, 106-108, 129` (kilobytes), `:130` (seconds); `openits-ramp-metering.yang:796, 797` (vehicles per hour); `openits-rsu.yang:516, 546` (milliseconds); `openits-rsu-events.yang:228` and `openits-v2x-messaging.yang:1122` (seconds), `:1215` (messages per second); `openits-signal-control-events.yang:438` (seconds); `openits-types.yang:626, 627` (degrees); `openits-v2x-radio.yang:306, 307` (bytes).
- [ ] `[m]` W1 pre-1.0 Counter sweep, 22 leaves: `openits-cabinet-power.yang:214`; `openits-device-diagnostics.yang:59, 76, 77, 78, 131, 141`; `openits-rsu.yang:526, 531, 536`; `openits-scms.yang:387, 392, 445, 457, 461`; `openits-v2x-messaging.yang:1081, 1086, 1210, 1211`; `openits-v2x-radio.yang:306, 307, 317`. Retype to `yang:counter64` or document reset-on-restart.
- [ ] `[m]` W1 `release-coordination` inline enum duplicated (ramp :816-830); seven other grouping-local inline enums are LATENT if a second composer appears: `trafficvision-traffic-sensor-camera.yang:184`; `openits-dms.yang:332, 402`; `openits-ess.yang:314, 344`; `openits-ramp-metering.yang:289, 318`. Decide once.
- [ ] `[m]` W2 Describe the five undescribed nodes (`openits-ramp-metering.yang:909`; `openits-signal-control.yang:1100, 1106, 1434, 1440`) and the 178 undescribed enum members (signal-control 63, ess 42, reversible-lane-types 15, reversible-lane 13, signal-control-events 12, signal-control-types 8, dms 8, schedule 7, perception 5, dms-types 5). Fix the 35 canonical-order warnings (`must`/`units`/`when` after `description`). `pyang --lint` currently exits 1 on the ramp-metering error.
- [ ] `[m]` W1 pre-1.0 Two `operational-status` typedefs with different members (`openits-cctv-types.yang:203` vs `openits-traffic-sensor-types.yang:153`); `bearing-degrees` (ess :166) shadows `heading-degrees` with different precision. Resolve before they freeze.
- [ ] `[m]` W1 Duplicate identity local names across modules: `mode-manual` (ramp-types :255, sc-types :1065), `mode-off` (dms-types :320, ramp-types :239, sc-types :1066), `mode-unknown` (dms-types :316, ramp-types :235). Harmless in YANG; confusing in identityref strings and Go. `mode-continuous/-alternating/-immediate` (radio :169-171) use the `mode-` prefix but derive from `channel-mode`.
- [ ] `[N]` W4 123 `revision` statements lack `reference` (accepted deviation from RFC 8407 §4.8; note only). 53 of 57 modules lack a module-level `reference` (present only in cabinet-power, cctv, schedule, rsu-us-band-plan). `openits-signal-control-events` is at `openits-version 0.1.0` with two revisions while its core is 0.16.0.
- [ ] `[N]` `openits-nema-common.yang:65` revision text keeps the literal "TODO(WG)". Vendor product strings in fixtures (`Econolite Cobalt`, `Siemens m60-ramp`, `Vanguard VF-2320`, `Falcon K`); a public standard may prefer neutral placeholders.
- [ ] `[N]` W3 `buf.yaml` `except:` block (line 18) does not list `ENUM_ZERO_VALUE_SUFFIX`, yet generated enums assign 0 to real members with no `_UNSPECIFIED` sentinel; how `proto-lint` passes is unverified. Check.
- [ ] `[N]` Twenty config-true leafrefs state `require-instance true` explicitly while 23 rely on the default (`openits-dms.yang:749`; `openits-signal-control.yang:665, 867, 895, ...`). Style only.

---

## Fixtures and gates

Cross-cutting test-integrity work (W3). Individual fixture items are also listed under their module sections.

- [ ] `scripts/validate-yang.sh` accepts any yanglint rejection as "invalid fixture behaved". Add a per-fixture expected rejection (data path and failing statement; a header comment or sidecar map) and assert it against yanglint's error line. This is what would have caught ledger #7.
- [ ] Add a `valid-*-under-*` fixture family and teach `check-deviations` to require one positive fixture per deviation.
- [ ] Taxonomy-completeness gate: per service, tabulate which cross-service bases have a dual-based sub-identity, which `operational-mode` family exists, which notifications the catalog publishes for it, and whether each published notification is populatable (ledger #3 is the template). Seed from mechanical audit §6.
- [ ] Kind-narrowing gate: every notification's `kind` base must be a sub-base whose derived identities are exactly that notification's kinds, with a bad-kind invalid fixture. Zone-occupancy is the fixture shape.
- [ ] Run `pyang --lint` in CI (or at least canonical order and enum descriptions) once the current 337 findings are cleared.
- [ ] Add the three interpretation-test questions from `docs/04-design-decisions.md` to `.claude/skills/model-pr-review/SKILL.md` as a per-leaf check.
- [ ] Fix `invalid-rsu-spat-enabled-local-time.json` (list encoded as object; verified by yanglint) and the three interval fixtures with the same encoding; rebuild `invalid-rsu-tim-broadcast-priority-out-of-range.json`; rename or drop `invalid-rsu-tim-window.json`.
- [ ] Notification-mode yanglint enforces neither `must` nor `mandatory` (`scripts/validate-yang.sh:138-150`); `check-notif-mandatory.py` covers mandatory only. Note this in the conventions doc or find a yanglint mode that enforces both.

---

## Docs and citations

W4 unless noted.

- [x] `[B]` **Done 2026-09-08.** Corrected in four places across both docs: the shared grouping carries engineering floors only, the jurisdiction bounds live in `yang/deviations/`, and the split is now explained (a US signal-timing bound is not a fact about a ramp meter, and both compose the grouping). Original: `docs/data-model.md` and `docs/04-design-decisions.md` said the MUTCD 3.0–6.0 s yellow bound lives in `openits-nema-common:phase-timing`. It lives only in `deviations/openits-signal-control-mutcd.yang:80-88`; the core requires only `> 0`. Rewrite both passages (the modeling is right; ramp meters share the grouping).
- [ ] `[M]` ARC-IT service-package ids, verified against arc-it.net: signal control cites "SU01 (Traffic Signal Control)" and "TI01" → TM03 (Traffic Signal Control); ramp metering "TI03 (Traffic Metering)" → TM05; DMS "TI06 (Traffic Information Dissemination)" → TM06; ESS "MC01 (Environmental Monitoring)" → WX01 (Weather Data Collection). TM01, TM08, TM16 are correct. Fix in the module headers and revision references, `docs/09-coverage-scope.md`, `docs/reference/yang-reference-conventions.md` (the DMS example), and every `arc-it-flow` annotation that names a package. Then re-run `tools/arcit-coverage`.
- [ ] `[m]` `docs/data-model.md`: module table (:33-34) omits zone-occupancy, work-zone, cctv, cabinet-power, schedule, and the V2X capabilities; ":36" says gen-1 notifications are deprecated (deleted); the `openits-version` paragraph says backfilling is open work (every core module is stamped; only the example augment and four deviations are not).
- [ ] `[m]` `docs/09-coverage-scope.md` lists work-zone as Planned; `docs/05-standards-alignment.md` describes the TIM/SRM/certificate events as `rsu` service events (:41-53) and says parking has no service (:23).
- [ ] `[m]` `schema-registry/index.json` and the catalog builder file the V2X capability modules and vehicle-detection under "foundation" while zone-occupancy is a "service". Decide the categorization rule (a capability with events is a service?) before V2X gains an events module and flips.
- [ ] `[m]` `docs/reference/dms-ntcip-1203-fidelity.md:123` `timebasedScheduler` row (section 5).
- [ ] `[m]` `yang/testdata/README.md` (section 10).
- [ ] `[m]` `docs/08-capability-architecture.md`: state where cross-capability safety invariants belong (profile `must` vs conformance harness) and either record the RSU events exception or the events-module plan.
- [ ] `[m]` `docs/reference/yang-reference-conventions.md`: add the "prior and current share an immediate base" rule for `mode-changed`; note that notification-mode yanglint does not enforce `must`.
- [ ] `[N]` Citation format sweep to the conventions' canonical forms (`NTCIP <doc>:<year> §<section>`, `IEEE <std>:<year> §`, J2735 with ASN.1 path) across rsu, v2x, signal-control, ramp, reversible-lane, ess.
- [ ] `[N]` British spellings in `yang/` (24 hits, 8 tokens), batched per module above; `docs/` prose is outside the check-revisions constraint and can be swept any time.
