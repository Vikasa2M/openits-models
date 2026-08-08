# Vendor implementation guide

This document is for the engineer who has to make an actual device
say `openits.>` — likely arriving from NTCIP/SNMP land, not the YANG
ecosystem. It answers three questions: what must you implement, what
may you skip, and what do you build it from. It does not repeat
[the data model](data-model.md),
[the extension model](06-extension-model.md), or
[conformance](07-conformance.md); it links into them at the point
each becomes relevant.

Scope: this is a device-side guide. Central/remote aggregation of
OpenITS events is out of scope here — that is a deployment concern
layered on top of what a device emits, not part of implementing the
model. A separate demo repository, `openits-demo-device`
(forthcoming), will walk the roads described below end to end for one
device kind (DMS) with mock hardware; it is not required reading to
follow this guide.

## Your controller is already a Linux box

If your product is a modern ATC cabinet controller, it already runs
Linux — ATC 5201, the industry's own controller hardware standard,
mandates a Linux-capable runtime. OpenITS does not ask you to adopt a
foreign platform; it asks you to run a process (or two) on the gear
you already ship.

Footprint honesty: `libyang` and `sysrepo` are embedded-friendly C
libraries with a long track record on constrained hardware. A Go
daemon built from this repo's generated packages is one static
binary. Nothing described in this guide needs a container
orchestrator, a service mesh, or a cloud control plane. It is a
daemon, or a sidecar process, talking a documented schema.

## The one loop

Every OpenITS implementation runs the same loop, regardless of which
management face it exposes:

```
intent -> validate -> safety gates -> apply -> state mirror -> emit events
```

Intent arrives as a config-tree write (over whichever face you
expose). It is validated against the schema, passed through whatever
safety gates the model encodes as YANG `must` constraints, applied to
the hardware, mirrored back into the state tree as applied truth, and
the transition is emitted as an event.

The YANG is the contract; every transport — NETCONF, RESTCONF, gNMI,
NATS — is a face on this one loop, not a separate model. The
config tree is intent (what was asked for); the state tree is applied
truth (what the hardware is actually doing). Keeping that distinction
honest is most of what conformance checks.

## Two roads: greenfield and brownfield

**Greenfield** — your firmware is YANG-native. The generated Go/ygot
structs under `pkg/` (or a `libyang`-backed tree, if you're not in
Go) *are* your state store; there is no separate internal
representation to keep in sync.

**Brownfield** — you have existing firmware with its own interface
(SNMP, a serial protocol, a proprietary IPC channel) and no interest
in rewriting it. An on-device sidecar process runs alongside your
firmware; a small driver adapts your existing interface into the
OpenITS tree. Zero firmware change. This is the more common path for
gear that predates this model, and it is the one the demo repository
will exercise for DMS.

For a worked, open-source example of the same "adapt an existing
internal store to a modern management face" shape, see the SONiC
pointer under gNMI in the next section — we point at it rather than
re-teach it.

## Management faces

None of these is required by the standard; pick per your market.
OpenITS is transport-neutral by design, so a device can expose one
face or several without changing the model underneath.

- **NETCONF** — the reference server stack is `sysrepo` fronted by
  `Netopeer2`. Because a vendor implementation-deviation module (see
  below) is just another YANG module loaded into the datastore,
  `sysrepo` advertises your deviations in the YANG-library `hello` at
  session start for free — no bespoke deviation-discovery mechanism
  to build.
- **RESTCONF** — sits on the same datastore as NETCONF (`sysrepo` /
  `Netopeer2`, or `clixon` and similar stacks). Pointer only; this
  guide does not re-specify RESTCONF.
- **gNMI** — a natural fit if your state store is already an
  in-process ygot tree: `Set` for intent, on-change `Subscribe` for
  telemetry. Note honestly that gNMI has no notification concept —
  a YANG `notification` statement has no gNMI equivalent, so events
  surface instead as on-change state telemetry (a leaf changes, gNMI
  streams the update), not as a discrete emitted event. For
  translation over an existing internal store — you already have
  firmware state in some proprietary format and want to expose it
  over gNMI without moving that store to ygot — point at SONiC's own
  `sonic-gnmi` server and its `translib` translation layer as the
  worked open-source example of exactly this pattern.
- **NATS** — the reference event binding. Every event lands on the
  seven-token subject
  `openits.{region}.{agency}.{agency-unit}.{service}.{controller-id}.{event}`
  (each token lowercase alphanumeric-and-hyphen) and carries a
  CloudEvents envelope whose `ce-type` follows
  `openits.<service>.<event>.v<major>` (e.g.
  `openits.dms.message-activation-failed.v1`); see "Claiming
  conformance" below for the full envelope and how a claim against it
  is verified. A symmetric command channel — intent flowing in over
  NATS, not just events flowing out — is experimental and demo-only
  today; there is no binding specification for it yet (see "Unpaved
  roads" below).

Field reality: a lot of field-deployed devices sit behind cellular
NAT with no reachable inbound address. That pushes hard toward
transports that work outbound-only. NATS's model — the device dials
out to a broker and stays connected — survives that constraint for
free. NETCONF, RESTCONF, and gNMI's server-listens model needs a
tunnel, a reverse-connect wrapper, or a management VPN to work the
same way over the same link. Pick faces knowing which one your
fleet's actual network topology can reach.

## Consuming a release

Pin a tagged release — a git tag `vX.Y.Z` of this repository, per
[Versioning & releasing](versioning.md) — or a specific
`schema-registry/<module>/<revision>/` snapshot. Never track `main`;
it moves under active development and carries no compatibility
promise between commits.

Build against the generated artifacts, not by re-parsing YANG
yourself:

- Go consumers: `go get github.com/Vikasa2M/openits-models@vX.Y.Z`
  and import `pkg/yang/openits` (the ygot `Device` struct — one
  field per device family, e.g. `Device.Sign` for DMS) or
  `pkg/proto/openits/<service>/v1` (the generated Protobuf package).
- Non-Go consumers: the tagged release attaches a curated bundle
  (`openits-models-vX.Y.Z.tar.gz` / `.zip`) containing `yang/`,
  `api/proto/`, `schema-registry/`, and `bindings/` without the
  Go-generated `pkg/` tree — use that rather than cloning the
  working repository.
- Either way, `schema-registry/` is the durable reference: every
  snapshot carries `schema.yang`, the source of truth a payload is
  validated against, alongside generated `schema.proto` / `schema.json`
  where applicable.

## Declaring what you don't implement

Real devices don't implement every leaf the model documents. OpenITS
gives you a machine-readable way to say so: a **vendor implementation
deviation** module, written in your own namespace and shipped with
your product — never merged into this repository.

```yang
module example-dms-deviations {
  yang-version 1.1;
  namespace "urn:example:yang:example-dms-deviations";
  prefix exdms;

  import openits-dms { prefix dms; }

  organization "Example Signs (a fictional vendor)";
  description
    "Implementation deviations for the Example Signs DMS product line.
     Declares model surface this product does not implement.  This
     module is the vendor's artifact: it lives with the product, not
     in the OpenITS repository.";

  revision 2026-07-22 { description "Initial declaration."; }

  deviation "/dms:sign/dms:environment/dms:humidity-percent" {
    deviate not-supported;
  }
  deviation "/dms:sign/dms:environment/dms:door-open" {
    deviate not-supported;
  }
  deviation "/dms:sign/dms:environment/dms:sign-face-temperature-c" {
    deviate not-supported;
  }
}
```

A few rules govern this lane:

- **It lives in your tree, your namespace.** The module above is
  `urn:example:yang:example-dms-deviations` — a namespace the
  fictional "Example Signs" vendor owns, not `urn:openits:yang:...`.
  It is never submitted upstream and never merged into this repo.
- **It is a different animal from the in-repo agency deviations**
  documented in [Tier 3 of the extension model](06-extension-model.md).
  Those live under `yang/deviations/`, stay in the `openits`
  namespace, and may only *tighten* the standard (narrower ranges,
  added `must` rules). A vendor implementation-deviation module does
  the opposite: it declares surface the product does *not* implement.
  The two mechanisms share the `deviation` statement and nothing
  else.
- **`deviate not-supported` only.** You are declaring gaps in your
  implementation, not redefining what a leaf means. Don't reach for
  `deviate replace` or `deviate add` here.
- **Import the base module by the release's revision**, and validate
  the pair before you ship:

  ```
  yanglint yang/openits-dms.yang your-deviations.yang
  ```

  (run from a checkout of the release you're implementing against;
  adjust the base module name for your device family.)
- **A deviation never touches the wire.** The generated Protobuf and
  JSON artifacts are produced from the un-deviated core YANG; there
  is no "deviated" variant of `pkg/` or `schema-registry/`. Your
  deviation module doesn't change what a leaf looks like on the wire
  — it just means the leaf is never populated, because your hardware
  has nothing to put there.
- **Ship the file with your product and cite it in your conformance
  claim** (see "Claiming conformance" below) — the deviation module
  is what turns "we don't support that sensor" from a line in a data
  sheet into a machine-checkable fact.
- **Watch for entangled leaves.** Deviating one leaf can have
  knock-on meaning for another. The worked example: `openits-dms`
  models `ambient-light-lux` (an environment sensor) and
  `illumination-control` (a config leaf selecting `photocell` /
  `timer` / `manual` brightness governance) as related but distinct
  surfaces. If your sign has no photocell, deviating
  `ambient-light-lux` alone is not enough — a consumer reading
  `illumination-control: photocell` on a device that can never report
  ambient light has been told something false. Deviate the sensor
  *and* constrain the config leaf's usable enum values (or default it
  away from `photocell`), and say so in the module's description.
  Don't deviate a leaf in isolation without checking what else in the
  tree assumes it exists.

## The floor: what you must not deviate

Some surface is not yours to deviate away, regardless of what your
hardware can or can't do. This floor is **advisory** — nothing in the
toolchain enforces it mechanically — and it is enforced instead by
TSC review of conformance claims: a deviation that touches the floor
is grounds to reject or dispute the claim.

- **The event header and deterministic event identity.** Every event
  you emit needs the header fields and content-derived identity the
  model defines, so consumers can deduplicate retries: the identity is
  a SHA-256 digest over `ce-source`, `ce-type`, stable-time, and the
  payload with the producer-assigned leaves cleared, folded into a
  ULID stamped with that same stable-time. See the
  [deterministic `ce-id` spec](ce-id-spec.md) for the normative
  algorithm — don't reimplement it from this summary.
- **Device identity.** What device is this, and where — the
  identity surface every service composes.
- **State-mirror truthfulness.** The state tree must reflect reality.
  A device that can't sense something should omit the leaf (or
  deviate it); it must never report a fixed or fabricated value.
- **The fault inventory.** If your hardware can fail in a way the
  model has a fault identity for, that fault must surface.
- **Safety-gating surfaces.** For DMS specifically: message-activation
  validation (the checks a candidate message passes before it goes
  live) and fallback behavior (what the sign does when it can't
  safely display anything). These exist because a wrong or
  ungated message on a roadway sign is a safety incident, not a bug
  report.

None of this means every leaf is mandatory. Leaves the model already
documents as technology-specific — the flip-disk `lamps-*` counters
in `openits-dms`, for example, which only apply to flip-disk/hybrid
signs — may simply be omitted on hardware where they don't apply, no
deviation module required. The floor is about honesty and safety, not
about implementing every optional sensor.

## Claiming conformance

Conformance claims, profiles, and the kit are specified in
[Conformance](07-conformance.md); this section covers only how a
deviation module interacts with a claim.

Name your deviation module in your conformance claim alongside the
profile — the same way a `core-plus-deviations=<list>` profile names
in-repo agency deviations. Today, the conformance kit does not read
deviation files: it doesn't know your product declared
`humidity-percent` unsupported, so a kit run against your device will
report a failure for that leaf like any other gap. Until kit
awareness of deviation files lands (a named future improvement, not
yet built), the honest path is to annotate your published kit-run
report by hand: for each failure that corresponds to a leaf your
declared deviation module marks `not-supported`, note that in the
report next to the failing test, citing the deviation module. A
reviewer can then check your annotation against the module you
shipped, rather than take your word for it.

## Unpaved roads

Named here as an invitation, not a commitment — none of the
following exists yet:

- **A binding-neutral Tier 1 conformance harness.** Today's kit
  verifies Tier 1 and Tier 2 together against a NATS endpoint. A
  deployment claiming Tier 1 only, over some other transport, has no
  in-tree harness yet — it validates the same model rules against its
  own transport by hand.
- **A normative NTCIP-to-OpenITS mapping, per service.** This guide
  points at the underlying NTCIP standards informally; a rigorous,
  field-by-field mapping document for each service does not exist
  yet.
- **A NATS command-channel binding chapter.** The experimental
  symmetric command channel mentioned under "Management faces" above
  has no binding specification — today it is demo-only and
  explicitly labeled experimental wherever it appears.
