---
name: adding-a-service
description: The end-to-end workflow for adding a NEW device service family (a new types/core/events YANG module trio, proto package, conformance kind) to this repo. Use this whenever a task introduces support for a new device class or subsystem — a new NTCIP/ITS device type, sensor family, or field equipment category — rather than extending an existing module. The scaffold tool handles the boilerplate but there are several registration points it does not cover, and missing one produces confusing generator/CI behavior.
---

# Adding a new service family

A "service" is a device family with its own module trio, proto package,
and conformance kind. Study one recent complete service (e.g. the CCTV
trio: `openits-cctv-types.yang`, `openits-cctv.yang`,
`openits-cctv-events.yang`) as the reference shape before starting.
`docs/09-coverage-scope.md` describes what belongs in scope;
`docs/08-capability-architecture.md` describes the capability layout.

## 1. Scaffold

```
go run ./tools/openits-new-service \
  -service <hyphen-name> \
  -description "<one sentence>" \
  -events <comma-separated-domain-events> \
  -reference "<standard, e.g. NTCIP 1205>" \
  -dry-run          # inspect first, then run without -dry-run
```

Do NOT list fault/mode events in `-events` — those flow through the
common fault/mode event modules via `kind` identities (see the generated
events module's header comment and the `adding-an-event` skill).

## 2. Registration points the scaffold does NOT cover

Missing any of these is the classic new-service failure mode:

1. **Generator routing** — add the service to `configStateRoutes` in
   `tools/yang-proto-gen/pkgmap.go` BEFORE running `make gen`, or the
   generator silently emits no config/state proto for it.
2. **Schema list** — `SCHEMAS` array in `scripts/validate-yang.sh`.
3. **Generation lists** — module list and `-exclude_modules` in
   `scripts/yang-gen.sh`.
4. **Registry list** — `MODULES` in `scripts/update-schema-registry.sh`.
5. **Conformance kind** — register the kind + per-kind checks under
   `tools/conformance/` (follow an existing `tests/<service>.go`), give
   the mock driver device data for it.
6. **CI matrix** — add the kind to the `conformance` job matrix in
   `.github/workflows/ci.yml`.

## 3. Model the device neutrally

The core module models the DEVICE CLASS, not one vendor's controller:
neutral leaf names, referenced standards in `reference` statements,
vendor specifics via the extension model (`docs/06-extension-model.md`,
augments under `yang/augments/`). Reuse platform groupings where they
exist (device identity, diagnostics, cabinet-power) instead of redefining
per-service copies. Follow `extending-a-model` for every placement/idiom
decision inside the modules.

## 4. Naming collisions to avoid

- An inline enum leaf with the same name as a sibling/top-level container
  collides in generated proto (both map to the same message/enum name) —
  the generator's collision check misses this case; rename the leaf.
- Module names feed proto package routing by PREFIX — pick the service
  name so it doesn't prefix-collide with an existing service.

## 5. Verify

Full pipeline from `extending-a-model`, plus:

```
go run ./tools/conformance -driver mock -kind <new-kind>
make check-graduation        # coverage/maturity gate
```

The PR should land the trio + generated artifacts + fixtures + conformance
checks + CI matrix entry as one reviewable unit, commit type `feat:`.
