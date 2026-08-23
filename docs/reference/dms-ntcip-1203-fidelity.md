# DMS — NTCIP 1203 fidelity register

How faithfully `openits-dms` represents a real NTCIP 1203 dynamic message
sign, and where it does not.

OpenITS tracks NTCIP 1203 *functionally*, not on the wire
([`05-standards-alignment.md`](../05-standards-alignment.md)) — so
divergence is expected and often deliberate. This register exists to
separate the three kinds of divergence that look alike in a diff:

1. **Deliberate improvements** — the model is intentionally better shaped
   than the standard. Keep, and document the reasoning in the module.
2. **Fidelity defects** — the model claims NTCIP alignment but describes
   something else. A conforming implementer is misled. Fix.
3. **Coverage gaps** — NTCIP carries an axis the model has no home for.
   Decide whether it belongs in the vendor-neutral core.

## Deliberate improvements — keep

| Model | NTCIP 1203 | Why the divergence is right |
|---|---|---|
| `duration-s` + explicit `indefinite` flag | `dmsActivateMessage` duration where 65535 minutes means "forever" | An unset/zero field cannot silently mean *display forever*. The proto wire default (`false` + `0`) is rejected rather than misread. |
| `fallback/power-recovery` bound to power **restore** | `dmsPowerLossMessage` fires on power-loss **detection** | An unpowered sign face cannot display anything. Restore is the operationally meaningful moment. Already documented in-module. |
| `control/state/active` as the face readback | `currentBuffer` memory bank (memoryType 5) | The face is a readback, not a fifth library bank. Modeling it as a bank invites writes to it. |
| Config/state divergence as command status | Separate error scalars per operation | Commanded `central` with actual `local` *is* the "a technician took local control" signal — no extra status leaf needed. |

## Fidelity defects

### CRC width is documented wrong — RESOLVED (was highest consequence)

`openits-dms.yang`, `message-body/crc` and `control/config/active-message/crc`
describe the value as **CRC-32 of the MULTI string**. NTCIP 1203
`dmsMessageCRC` is **CRC-16**.

The `uint32` type accommodates a 16-bit value, so nothing fails validation —
but an implementer following the description computes the wrong function,
and the CRC is load-bearing: it is packed into the 12-byte
`MessageActivationCode`, and a mismatch causes the sign to **refuse the
activation without displaying an error on the face**. The failure presents
as "the command was accepted and nothing happened."

Remediation is description-only and therefore wire-neutral.

### Ambient light was typed in lux; signs do not report lux — RESOLVED

`environment/ambient-light-lux` carried `units "lux"` for a value that is
not an illuminance measurement. NTCIP 1203 §5.8.3 defines
`dmsIllumPhotocellLevelStatus` as a **virtual photocell level**, possibly
fused from several photocells by a *manufacturer-specific* algorithm, on a
scale whose maximum is itself a device object
(`dmsIllumMaxPhotocellLevel`). Raw values are not comparable between makes —
or even between sensor generations from one make: manufacturers ship both
frequency-based and lux-based ambient sensors, and vendor documentation is
explicit that the two require independent recalibration to agree under
identical conditions.

Tellingly, NTCIP 1203 **defines lux in its glossary** (with candela and
lumen) **and never uses it in a single object**.

Resolved by splitting on populability rather than picking one unit:

| Leaf | Populated by | Comparable across a fleet? |
|---|---|---|
| `ambient-light-level` (0–100) | every sign, as `photocellLevel / maxPhotocellLevel` | yes — normalizing against the device's own maximum is what makes it so |
| `ambient-illuminance-lux` | only a calibrated photometric sensor | yes, and directly against EN 12966 |

Lux stays where it is meaningful. It is the SI unit of illuminance and the
unit EN 12966 states its reference ambient condition in (sign luminance
specified under 40 000 lux), so a calibrated reading is checkable against
the European product standard. EN 12966 also explicitly declines to
standardise luminance control *with respect to* ambient light, and
AS 4852.2 mandates automatic control by a light-sensing device without
prescribing a reported unit — so no standard anywhere requires a sign to
report calibrated lux.

`TestDMSControl_PhotocellModeReportsAmbient` enforces the entangled-leaf
rule from [`10-vendor-implementation.md`](../10-vendor-implementation.md): a
sign governing brightness from its photocell must report the level that
governs it.

### `message-memory-type` implied ordinal passthrough — RESOLVED

The typedef is described as "NTCIP 1203 `dmsMessageMemoryType`" while using
its own ordinals (`permanent = 1`; NTCIP uses 2). The renumbering is
correct — OpenITS does not carry NTCIP wire values — but the description
reads as passthrough and will mislead anyone writing a decoder.

### `illumination-control` covers three of five NTCIP modes — WON'T FIX

`dmsIllumControl` defines photocell, timer, manual, manualDirect and
manualIndexed. The model has the first three, and should keep only those.

manualDirect and manualIndexed are not an operational distinction about how
brightness is governed — they describe how the command is *addressed on the
wire* (a raw light-output level vs an index into the controller's step
table). The model already normalizes that away: `brightness-setpoint` is a
percentage, and a driver converts. Adding them would widen a clean,
meaningful enum to mirror a MIB.

This is the general rule for this register: a gap is only worth closing when
the missing thing is operationally meaningful in the model's own terms.
OpenITS is not a YANG transcription of NTCIP 1203 — where the standard
encodes a protocol detail, the model is right to omit it.

## Coverage gaps

Ordered by operational consequence.

### Message source mode had no structured home — RESOLVED

`control/state/active/source` was `type string`, and an operator console
could not answer "did the TMC put that there, or did a fallback?" without
parsing prose that no schema constrained.

The instructive part is what the fix turned out **not** to be. The obvious
move — mirror `dmsMsgSourceMode` as a nine-value axis — is wrong, because
that object conflates two orthogonal axes that OpenITS already separates
(see `openits-common-mode-events`: `control-source` is *who holds
authority*, `mode-change-trigger` is *why it changed*):

| `dmsMsgSourceMode` | Axis | Home |
|---|---|---|
| local, central, timebasedScheduler, otherCom1, otherCom2 | authority | `control/state/control-mode` — **already modeled** |
| commLoss | cause | `openits-types:trigger-comm-loss` — already shared |
| powerRecovery, powerLoss, endDuration | cause | added as DMS-scoped trigger identities |

So the authority half was never missing, and cloning the NTCIP object
would have created a fourth axis overlapping an existing one. The actual
gap was three identities and one leaf:
`control/state/active/activation-trigger`.

**A decoder must split `dmsMsgSourceMode` across two leaves**, never map it
to either one alone.

The three new identities are scoped to `openits-dms-types` rather than
`openits-types`: identityrefs serialize module-qualified so there is no
collision, and a power-recovery cause does not yet belong on every
service's shared surface. Promotion later is additive.

`comm-loss-active` / `power-loss-active` are retained as a narrow boolean
projection of the new leaf, for consumers that only need the predicate.

The `source` string is retained and re-scoped in its description to the
**actor** axis — a third distinct thing. `openits-types:command-provenance`
already exists for that and names DMS message activation as an intended
adopter, with adoption deferred as cross-service follow-up work.

### Power-recovery fallbacks were collapsed into one — RESOLVED

`fallback/power-loss` was a single container whose own description said it
fired on power *restore* — the right semantics under a misleading name.

Replaced by `fallback/power-recovery` with `short-outage` / `long-outage`
and a `short-outage-threshold-s`, plus a sibling `fallback/reset` for a
non-power controller restart. The split is kept because the two cases
warrant different copy — a momentary dropout should restore something
innocuous, while a sustained outage may mean the sign was dark through a
changed traffic situation — not merely because NTCIP splits them.

Field controllers commonly hold *stale* pointers in these objects, aimed at
library rows whose content has since been rewritten. `TestDMSFallback_ReferencesResolve`
now checks that every fallback naming a slot names one that exists and is
valid.

### `comm-loss-timeout-s` cannot express "never"

The leaf defaults to 300 s with no documented zero semantics. NTCIP
`dmsTimeCommLoss = 0` means *never revert* — the last message stays on the
face indefinitely after comms drop. That value currently has nowhere to go,
and the distinction is a safety-relevant policy choice, not a detail.

### No graphics inventory — RESOLVED

`state/capabilities` carried a `font` list but nothing for graphics, while
MULTI `[gN]` references them. Added `capabilities/graphic`, mirroring the
font list. `TestDMSMessages_GraphicsReferencedExist` checks that every
graphic a displayed message references is in the advertised inventory.

The asymmetry is visible inside the repo: `openits-dms-events` already
defines `error-type = graphic-not-found`. The failure mode is modeled;
the inventory whose absence causes it is not. A central system can
pre-validate font references and cannot pre-validate graphic references.

### No power-source axis on DMS — RESOLVED

`openits-dms` had no power leaf anywhere, and `openits-cabinet-power` — whose
own description named DMS as an intended consumer — had a `power-source` enum
(on-line / on-battery / bypass / off) with no solar or generator member.

Resolved by composing the shared `cabinet-power` groupings into
`sign/cabinet-power` (the pattern `openits-signal-control` already used) and
appending `solar` / `generator`. Composing beat adding a DMS-local enum: an
off-grid sign's state of charge and runtime-remaining are what answer
"will the face be lit tonight", and `power-source: solar` alone does not.
`TestDMSPower_OffGridReportsCharge` enforces that.

Composing it also exposed a latent generated-name bug in that shared module;
see the inline-enumeration rule in
[`yang-reference-conventions.md`](yang-reference-conventions.md).

### Fault categories covered six of fifteen error bits — RESOLVED

`dms-fault-event-kind` has leaf identities for lamp, pixel, controller,
environment, power and communication. NTCIP `shortErrorStatus` is a
fifteen-bit field.

Temperature, humidity, door and climate-control fold into
`dms-fault-environment` defensibly. **Photocell** and **message** (the
sign's own complaint about its displayed content) did not fold anywhere
honest, and were added as `dms-fault-photocell` / `dms-fault-message`.

The remaining uncovered bits are deliberately not modeled: they are either
covered by `dms-fault-environment` or describe hardware classes this model
does not carry (drum). Coverage of a bitmask is not itself a goal.

### No validate-time failure event — RESOLVED

`message-activation-failed` covered activation only, while a message can
fail validation long before anything tries to activate it — a distinct
operator situation (the library write did not take) with no event home.

Resolved with a `phase` leaf (activate / validate) on the existing
notification rather than a second one: the payload is identical for both,
and the phase is an orthogonal axis, not a parallel classification of the
`kind` hierarchy. `activate` is value 0, so the proto default preserves the
event's historical meaning.

### Minor — no leaf

- Controller watchdog failure count (a restart-history signal).
- Critical high/low temperature thresholds (`dmsTempCritical*`).
- ~~Pixel pitch in mm~~ — added as `capabilities/pixel-pitch-mm`;
  `TestDMSCapabilities_PitchMatchesGeometry` now checks face width / pitch
  against the advertised pixel width.

## Driver-layer concerns — deliberately not modeled

These are real, and they are decode problems. Modeling them would encode
one controller's defects into a vendor-neutral standard.

| Concern | Correct handling |
|---|---|
| Sentinel `-128` on temperature objects | Decode to **leaf absent**, never to a value. A sentinel is not a measurement. |
| `2 = none` on error scalars | NTCIP uses 2, not 0, for "no error". Do not alarm on 2. |
| Capability objects returning malformed data | Leave `supported-multi-tags` / `max-pages` **absent** rather than guessing. Absent means unknown; a guess is a fabrication. |
| Optional tables with `NumRows = 0` | Gate row reads on the count. Reading row 1 of an empty table returns an error that looks like agent failure. |
| Objects absent for the product class | Lamp, drum, fuel and engine objects are absent on an LED matrix sign. Treat as "not this product", not as a fault. |
| Vendor-private subtrees | No schema, no model home. |
| Advertised pixel width contradicting the nameplate | Recover geometry from `sign-face-width-mm` ÷ `pixel-pitch-mm`. Both leaves now exist, so the correction is expressible in the model and checked by conformance rather than carried as implicit arithmetic in every driver. |

## Remediation status

| Tier | Scope | Wire impact | Status |
|---|---|---|---|
| 1 | CRC width wording; memory-type ordinal clarification; missing fault identities; `illumination-control` manual sub-modes | Wire-neutral + additive | Applied |
| 2 | `activation-trigger` leaf + three DMS-scoped trigger identities | Additive (one new field tag) | Applied |
| 3 | Graphics inventory; `pixel-pitch-mm`; `cabinet-power` composition; validate/activate `phase` leaf | Additive | Applied |
| 3b | Fallback reshape: `power-loss` → `power-recovery`/{`short-outage`,`long-outage`} + threshold, plus `reset` | Breaking (YANG path) | Applied |
| 3c | `cabinet-power` inline enums hoisted to named typedefs | Breaking (Go bindings only) | Applied |
| — | `command-provenance` adoption (replaces the `source` string) | Breaking | Open — cross-service, not DMS-only |
| — | Conformance checks for the new DMS surface | — | Applied (5 checks, mutation-tested) |
| 4 | `ambient-light-lux` split into `ambient-light-level` + `ambient-illuminance-lux` | Breaking (leaf replaced) | Applied |
| — | `comm-loss-timeout-s` cannot express "never revert" | Additive | **Open** |
| — | Watchdog failure count; critical-temperature thresholds | Additive | Open (low value) |

The `command-provenance` row is deliberately not folded into Tier 2. It is
breaking, and the grouping names five intended adopters across services;
converting DMS alone would fragment a planned cross-service change. Per
`.claude/skills/extending-a-model`, a breaking reshape does not ride along
with an additive change.
