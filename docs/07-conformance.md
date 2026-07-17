# 07 — Conformance

Conformance is what makes "OpenITS-compatible" a contract clause
rather than a marketing claim. This document describes what
conformance means, how to claim it, and how operators verify
vendor claims.

## What conformance means

An implementation conforms to OpenITS for a given service when:

1. **It emits events on the canonical subject hierarchy.** Every
   message uses the seven-token shape
   `openits.{region}.{agency}.{agency-unit}.{service}.{controller-id}.{event}`
   with valid tokens (validated against the agency registry).

2. **Headers carry the CloudEvents envelope.** Every message has
   the required CloudEvents headers (`ce-specversion`, `ce-type`,
   `ce-source`, `ce-id`, `ce-time`, `ce-dataschema`,
   `ce-datacontenttype`) populated correctly.

3. **Payloads parse against the schema-registry snapshot.** The
   `ce-dataschema` URL points at an immutable snapshot directory
   that always carries `schema.yang` — the YANG source of truth the
   message body is validated against. Every notification-bearing
   snapshot additionally ships a generated `schema.proto` and
   `schema.json`; the guarantee conformance relies on is the YANG
   match, not a specific generated file's presence.

4. **Per-service shape rules hold.** Each service has additional
   assertions: signal-control's `OperationalStatus` emits at the
   configured cadence; RSU's TIM-broadcast events follow the
   declared lifecycle; traffic-sensor interval reports arrive within
   their configured data interval; and so on.

5. **Idempotent retries.** A retried publish produces the same
   `ce-id`, allowing consumers to deduplicate without
   coordination.

6. **The implementation's NoIs are filed for any augments it
   ships.** If you ship `vendor-x-signal-control-feature-y`, you
   file a Notice of Implementation against that augment. This
   creates the public record that drives graduation.

## Profiles

A conformance claim names a **profile** that scopes what was
verified:

| Profile | Means |
|---------|-------|
| `core` | The implementer correctly emits and consumes every event in the named service's core module. |
| `core-plus-augments=<list>` | Core, plus the named augments. Each augment listed adds its own assertions to the suite. |
| `core-plus-deviations=<list>` | Core, plus the named deviation rules (e.g., `mutcd-strict` for tightened MUTCD timing). |
| `complete` | Core + all declared augments + all declared deviations. |

A vendor that claims `core` for the signal-control service is
saying "every signal-control core event flows through our
implementation correctly, on the right subject, with the right
envelope, with a parseable payload." That's the floor. Augment
and deviation profiles layer on top.

## How to claim conformance

The conformance kit lives in `tools/conformance/` in the
reference repository. To claim conformance:

1. **Stand up your implementation against a test NATS endpoint.**
   The kit subscribes to `openits.>` on your endpoint and
   listens for events for a configurable window.

2. **Run the conformance kit:**

   ```
   go run ./tools/conformance \
     -driver mock \
     -kind asc \
     -window 60s
   ```

   (`mock` is the in-tree test driver; replace with your driver
   when running against your real implementation. `-kind` selects
   the device suite — `asc` (signal-control), `rsu`, `dms`, `ess`,
   or `ramp-metering`. The standalone binary release is upcoming
   follow-up work; today the kit runs in-tree.)

3. **Inspect the report.** The kit prints a `PASS`/`FAIL` line per
   test case followed by a summary count (`N passed, M failed`) to
   stdout. Iterate until it passes the profile you're claiming. (A
   machine-readable JSON report is planned follow-up work; today the
   output is the human-readable text summary.)

4. **Publish the report.** Capture the run output and host it at a
   stable URL. Consumers and the TSC review it.

5. **Publish the claim.** Once a public conformance board lands,
   open a PR adding a row that names the implementer, service,
   profile, kit version, report URL, and date. Until then, link
   the report from your release notes or product page.

The TSC reviews submitted claims. Conformance claims unsupported by
the cited report are rejected. Disputed claims are moved to the
disputes table with a brief explanation; the implementer can
withdraw and resubmit.

## How to demand conformance in an RFP

Sample contract language for transportation agencies writing
procurement RFPs:

> The Vendor's product SHALL conform to OpenITS core for the
> {service} service at revision {YYYY-MM-DD} or later. The Vendor
> SHALL provide a conformance report produced by the OpenITS
> conformance kit version {X.Y.Z} or later, dated within 90 days
> of contract execution.
>
> The Vendor SHALL re-run conformance after any update to the
> OpenITS service revision is published, and provide an updated
> report within 60 days.
>
> Optionally: The Vendor SHALL conform to the following
> deviations: [list]. The Vendor SHALL ship augment
> {augment-name} version {revision}.

This language is reusable across vendors because the conformance
target is a public artefact. There's no per-vendor negotiation
of what conformance means.

## How to verify a vendor-supplied report

When a vendor claims conformance and gives you a report:

1. **Note the kit version** the report cites.
2. **Inspect the test results.** Every failed test case is a
   non-conformance. Some failures are catastrophic (subject
   format wrong); others are partial (one event type
   misimplemented).
3. **Check the report against your own kit run.** Optionally,
   subscribe to the vendor's test NATS endpoint with your own
   conformance kit and compare. If your run passes where theirs
   passed, the claim is real. If your run fails where theirs
   passed, the report is suspect — escalate to the TSC dispute
   process.
4. **Verify the report integrity.** Today's reports are the kit's
   plain-text output — reviewable, not tamper-evident. The
   standalone signed-binary kit release (upcoming) adds
   machine-readable JSON and ED25519-signed reports verifiable
   against a public key in the schema registry; we expect
   kit-signed reports to become the procurement-grade standard
   once the binary release ships.

## What the kit checks today

The conformance kit's current checks include:

- **Subject hierarchy.** Every published event lives on the
  seven-token shape with valid tokens.
- **CloudEvents envelope.** `ce-type` matches the configured
  service; `ce-source` is a valid `urn:openits:controller:`
  URN; `ce-id` is a deterministic ULID; `ce-dataschema` points
  at a snapshotted revision.
- **Per-event payload.** Body parses cleanly against the
  schema-registry copy at the declared revision (Protobuf, where
  that snapshot ships a `schema.proto`; validated against the
  snapshot's `schema.yang` otherwise).
- **Per-service shape rules.** See
  `tools/conformance/tests/<service>.go` for the assertion list
  per service.

## What the kit does NOT check yet

A few things on the deferred list, captured for transparency:

- **Wire-level idempotency of retries.** Planned. Currently
  validated by inspection of the deterministic-`ce-id` policy.
- **`ce-id` determinism across cold-restart.** Planned.
- **Multi-leaf fan-in correctness at central.** Planned.
- **Augment-namespace events alongside core.** Lands when the
  first augment is deployed against the kit.
- **Signed report production.** Lands with the standalone
  binary release.

These are explicit follow-up; their absence does not invalidate
the conformance model. Today's reports are reviewable; the
binary release adds tamper-evidence.

## Ongoing conformance vs one-time claim

Conformance is not a one-time gate. The TSC's working assumption:

- A claim should be **dated within 90 days** of when it's cited.
- A claim should be **re-run after a service revision update**
  and refreshed within 60 days.
- A public conformance board (planned) will track the claim date
  so reviewers can see freshness at a glance.

A vendor that cites a 2-year-old conformance report is asking
the operator to take it on faith that nothing has drifted in two
years. The TSC's published expectation is that operators
treat stale claims with the same skepticism they treat unaudited
financials.

## Disputes

When a vendor publicly claims conformance but their deployment
doesn't behave conformantly, an operator can:

1. **Open an issue at the OpenITS repository.** Include the
   vendor's report and the operator's re-run of the kit against
   the deployment.
2. **The TSC reviews.** If the claim is unsupported, the
   implementer is added to a public dispute log on the conformance
   board.
3. **The implementer can withdraw and resubmit.** Withdrawing a
   claim is encouraged when a bug is found; it's a normal part
   of the lifecycle, not a black mark.

The dispute mechanism's existence matters more than its
frequency. Most claims will be honest; the dispute log exists for
the rare case where they're not, and the existence of the
mechanism keeps the rare case rare.

## Status today

The conformance status board is empty as of writing. The
machinery is in place; the first claim is whoever's first.

**Conformance is not just for vendors.** A research group, an
agency, or a hobbyist who builds an OpenITS-conformant emitter
can run the kit and publish a report. The bar is the same. The
TSC reviews on shape, not on commercial weight.

If your organisation runs OpenITS — emitting or consuming — and
you can pass the kit, please consider filing. The empty board
becomes a populated one, and the project's credibility grows
from public evidence.
