# arcit-coverage

Walks the openits YANG modules, collects every `arc-it-flow` extension
annotation, and diffs the result against a curated ARC-IT inventory.
Emits a Markdown report suitable for FHWA / ITS JPO alignment review.

## Usage

```
go run ./tools/arcit-coverage \
  -inventory tools/arcit-coverage/arcit_inventory.json \
  -yang-dir  yang \
  > coverage.md
```

Flags:

| Flag | Default | Purpose |
|------|---------|---------|
| `-inventory` | `arcit_inventory.json` | Path to inventory JSON |
| `-yang-dir`  | `../../yang` | Directory containing `*.yang` files |

## Inventory

`arcit_inventory.json` holds the ARC-IT service packages we claim
coverage on, each with the canonical flow names from arc-it.net. Flow
names must match the annotation argument on the YANG node exactly —
they are compared case- and whitespace-sensitive (after trimming).

Extend the inventory by adding new service-package objects and flow
entries. Flows present in YANG but missing from the inventory appear
under **Orphan Annotations** in the report.

## Output

```
# ARC-IT Coverage Report — OpenITS

## Service Package TI01 (Traffic Signal Control)

Coverage: 7 / 8 flows (87%)

| Flow | Annotated | Node |
|------|-----------|------|
| TMC -> Roadway Signal Controller : signal control plan | ✅ | /openits-signal-control/signal-controller/phases |
...
```

## How annotations are recognized

In YANG modules, annotate any schema node with:

```yang
openits-types:arc-it-flow
  "TMC -> Roadway Signal Controller : signal control plan";
```

The extension is defined once in `openits-types.yang`. The scanner
accepts any statement whose keyword is `arc-it-flow` (or a
prefix-qualified form ending in `:arc-it-flow`), so module authors can
import `openits-types` under any local prefix.
