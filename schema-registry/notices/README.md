# OpenITS Notice of Implementation (NoI)

This directory records who is implementing which OpenITS augments.
Each NoI is a small YAML file declaring:

- the augment being implemented
- the implementer's identity and type (operator / vendor / conformance-reference)
- when implementation began
- deployment scale

NoIs are the public, machine-checkable trail that drives the **graduation
rule**: when ≥ 3 independent organisations (with at least one
`implementer_type: operator`) have filed NoIs against the same augment,
the augment becomes eligible to be folded into the core module. See
[`../../docs/plans/yang-extension-model.md`](../../docs/plans/yang-extension-model.md).

## Layout

```
schema-registry/notices/
├── README.md                       (this file)
├── _schema/
│   └── noi-schema.yaml             (NoI YAML schema; CI validates against it)
├── organizations.yaml              (canonical org ids + aliases for independence checks)
└── <augment-name>/                 (one directory per augment)
    └── <implementer>.yaml          (one NoI per implementer)
```

## Filing an NoI

1. Author your augment under `yang/augments/<your-org>-<service>-<feature>.yang`
   (or pick an existing augment to which you're contributing a parallel
   wire-compatible implementation).
2. Implement it in your product or deployment.
3. Author a YAML at
   `schema-registry/notices/<augment-name>/<your-org>.yaml` following the
   schema in `_schema/noi-schema.yaml`.
4. Submit as a PR. CI runs `make validate-noi` to confirm schema
   compliance and `make check-graduation` to refresh the graduation
   tracker.
5. Merge: your implementation is now public record.

## Why submit?

- **Graduation eligibility.** Three NoIs with operator presence make an
  augment eligible to graduate into core; your NoI counts.
- **Vendor visibility.** Operators evaluating products can see which
  vendors have shipped which augments.
- **Adoption signal.** The TSC reviews NoI volume to decide where to
  invest core-module work.

## NoI ≠ conformance certificate

An NoI says "we ship this augment." A conformance certificate (future
work, see `CONFORMANCE.md`) says "we correctly implement OpenITS core
and any declared augments." Conformance is signed and tested; NoI is
just public record.
