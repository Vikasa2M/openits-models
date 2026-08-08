# Deterministic CloudEvents `ce-id`

OpenITS derives a CloudEvents `ce-id` deterministically from event content so
retries are idempotent at the storage layer (a `ReplacingMergeTree(ce_id,
ce_time)`-style store deduplicates for free). This document is the **contract**;
the reference *implementation* lives in the collector, not in this repo.

## Algorithm

```
identity = event payload with the producer-assigned leaves cleared (below)
digest   = SHA-256( ce-source ‖ ce-type ‖ stable-time ‖ identity-bytes )
ce-id    = ULID(timestamp = stable-time-ms, randomness = digest[0:10])
```

- `‖` is byte concatenation with a `0x1f` unit separator **between** fields —
  three separators, none trailing.
- `ce-source` and `ce-type` are the CloudEvents attribute values (UTF-8).
- `stable-time` is `occurred-at` when present, else `ce-time`, encoded as
  RFC 3339 with millisecond precision (UTC `Z`). It is the **only** time that
  feeds the id — both the digest and the ULID timestamp. `ce-time` is when a
  publisher *observed* the event and MUST NOT influence the id; using it would
  give two observers of one event two different ids, which is the exact
  outcome this construction exists to prevent.
- `identity-bytes` is the canonical, binary protobuf payload with the
  producer-assigned leaves cleared to their zero values before encoding:
  - **`sequence`** — a per-producer counter that resets on restart and differs
    between redundant producers. It is transport bookkeeping for gap
    detection, not event identity.
  - **`observed-by`** — names the observer, not the occurrence.

  Every other leaf participates. Clear them on a *copy*: the wire payload
  still carries both values.

## Invariants

- **Restart-invariant:** the id is a pure function of the event's identity, so
  a producer that observes the same event before and after a restart emits the
  same id. No durable counter is required.
- **Observer-invariant:** two producers watching one device — a redundant
  pair, or a device that reports an event a collector also infers — agree on
  the id without coordinating.
- **Collision-resistant enough:** 80 bits of content-derived randomness inside
  the ULID within a millisecond window.

### Producer obligation

Because `sequence` is excluded, two *distinct* events that differ **only** by
`sequence` collapse to one id. For state-change notifications this cannot
arise: `occurred-at` plus the changed values already distinguish them. It can
arise for device-reported log events where several rows share one timestamp
tick and the device's own counter is the only discriminator.

A producer emitting such events MUST carry a device-supplied distinguishing
value in a leaf that participates in the digest. Relying on `sequence` to
separate them is not conformant — it would make two real events indistinguishable
to any consumer deduplicating on `ce-id`.

## Test vectors

Both use the payload `openits.dms.v1.MessageActivationFailed{reason:
"validation"}`, whose deterministic proto3 encoding is
`1a0a76616c69646174696f6e`, and the `ce-type`
`openits.dms.message-activation-failed.v1`. Any implementation must reproduce
both exactly.

**Vector 1 — `occurred-at` equals `ce-time`.**

| Field | Value |
|---|---|
| `ce-source` | `urn:openits:sign:us-xx:example-agency:d01:demo-sign-1` |
| `occurred-at` / `stable-time` | `2026-07-22T12:00:00.000Z` |
| `ce-time` | `2026-07-22T12:00:00.000Z` |
| **`ce-id`** | `01KY4V4VG09C9D44NNQCDWKJCN` |

**Vector 2 — backfill, where `occurred-at` and `ce-time` diverge.** This is
the vector that pins the ULID timestamp to `stable-time`; vector 1 cannot,
because the two times coincide there and both readings produce the same id.

| Field | Value |
|---|---|
| `ce-source` | `urn:openits:sign:us-xx:example-agency:d01:demo-sign-1` |
| `occurred-at` / `stable-time` | `2026-07-22T11:59:00.000Z` |
| `ce-time` | `2026-07-22T12:05:00.000Z` |
| **`ce-id`** | `01KY4V30X08F697N951X9F30AS` |

An implementation that sources the ULID timestamp from `ce-time` produces
`01KY4VE0F08F697N951X9F30AS` here — identical randomness, different timestamp
prefix. That is the failure this vector exists to catch.

## Non-goals

The transport, envelope framing, and the concrete implementation are the
producer's concern. This repo owns only the algorithm as part of the wire
contract, so any binding can reproduce it.
