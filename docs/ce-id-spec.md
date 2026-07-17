# Deterministic CloudEvents `ce-id`

OpenITS derives a CloudEvents `ce-id` deterministically from event content so
retries are idempotent at the storage layer (a `ReplacingMergeTree(ce_id,
ce_time)`-style store deduplicates for free). This document is the **contract**;
the reference *implementation* lives in the collector, not in this repo.

## Algorithm

```
digest = SHA-256( ce-source ‖ ce-type ‖ stable-time ‖ payload-bytes )
ce-id  = ULID(timestamp = ce-time-ms, randomness = digest[0:10])
```

- `‖` is byte concatenation with a `0x1f` unit separator between fields.
- `ce-source` and `ce-type` are the CloudEvents attribute values (UTF-8).
- `stable-time` is `ce-occurred-at` when present, else `ce-time`, encoded as
  RFC 3339 with millisecond precision (UTC `Z`).
- `payload-bytes` is the canonical, binary protobuf payload.

## Invariants

- **Restart-invariant:** the id depends only on content, so two publishers (or a
  publisher before and after a restart) that observe the same event produce the
  same id. No durable counter is required.
- **Collision-resistant enough:** 80 bits of content-derived randomness inside
  the ULID within a millisecond window.

## Non-goals

The transport, envelope framing, and the concrete Go implementation are the
collector's concern. This repo owns only the algorithm as part of the wire
contract, so any binding can reproduce it.
