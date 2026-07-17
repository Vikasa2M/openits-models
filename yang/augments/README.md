# OpenITS YANG Augments

This directory holds **augment modules** — vendor or agency contributions that
*add* nodes to OpenITS core modules without modifying them.

See [`docs/06-extension-model.md`](../../docs/06-extension-model.md)
for the full extension model. This README is the contributor quick-reference.

## What is an augment?

A YANG 1.1 module that imports a core OpenITS module and uses `augment`
statements to add new leaves, containers, or lists. Augments NEVER modify
existing core nodes. A consumer that doesn't load an augment ignores the
unknown nodes; protobuf's unknown-field tolerance makes the wire safe.

Identity-only vendor modules (those that only declare `identity` statements
derived from open extension slots in core, with no `augment` statements)
live at top-level `yang/`, not under `yang/augments/`. They are peers to
core modules in placement because the extension mechanism is identityref
derivation rather than tree augmentation. See
`yang/openits-vendor-econolite-signal-control-types.yang` for the
canonical example.

## File naming

`yang/augments/<contributor>-<service>-<feature>.yang`

- `<contributor>` — short identifier of the implementer (vendor short-name,
  agency code, or `example` for the in-tree pedagogical example)
- `<service>` — the core service being augmented (signal-control, dms, ...)
- `<feature>` — short feature description

Examples:

- `siemens-signal-control-vehicle-counts.yang`
- `caltrans-signal-control-corridor-id.yang`
- `econolite-dms-marquee-mode.yang`

## Namespace

`urn:<contributor>:yang:<service>-<feature>`

Vendor namespaces use the vendor's domain or short-name. Agency namespaces
use the agency code. `urn:openits:` is reserved for core modules and
graduated content.

## What an augment must contain

1. `module` statement with the contributor's namespace and prefix
2. `import` of the core module being augmented
3. One or more `augment` statements targeting paths in the core module
4. `revision` statement with date-stamp and brief description
5. `contact` statement naming the maintainer
6. Per-leaf `description` statements

## What an augment must NOT contain

- A `deviation` statement that loosens a core constraint (use
  `yang/deviations/` with TSC review for that)
- An `augment` of another augment (chained augmentation is a maintenance
  nightmare; augment the core directly)
- A YANG 1.1 `notification` statement that lives outside its companion
  notifications module (split following the
  `openits-<service>-notifications` pattern)

## How to submit an augment

1. Author the .yang file in this directory
2. Run `make yang && make validate-yang && make check-revisions`
3. Run `make yang-lint` (pyang strict) — must pass
4. Submit a Notice of Implementation at
   `schema-registry/notices/<your-augment>/<your-org>.yaml` so your
   adoption appears in the graduation tracker
5. Open a PR; the `augment` label requests one operator-aligned reviewer
   per `GOVERNANCE.md`

## Graduation

When ≥ 3 independent organisations submit NoIs against the same augment,
and at least one of them is `implementer_type: operator`, the augment
becomes eligible to be folded into the core module. Run
`make check-graduation` to see eligibility status; the TSC reviews and
votes per `GOVERNANCE.md`.

See [`docs/06-extension-model.md`](../../docs/06-extension-model.md)
for the full graduation rule.
