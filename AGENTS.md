# Agent guide — openits-models

This repository is the OpenITS data-model layer: YANG models for ITS field
devices, with generated protobuf, Go bindings, JSON Schema registry
snapshots, and an AsyncAPI binding — all checked in and drift-gated by CI.
`docs/` is the authority on conventions; start with `docs/data-model.md`
and `docs/04-design-decisions.md`.

## Golden rules

1. **Never hand-edit generated files** (`api/proto/`, `pkg/`,
   `bindings/nats/asyncapi.yaml`, `schema-registry/`, `field-numbers.yaml`).
   Change the YANG (or the generator) and run `make gen`.
2. **Model change = YANG edit + new `revision` statement + regen + fixtures,
   committed together.** Pipeline order: edit YANG → `make gen` →
   `scripts/update-schema-registry.sh` → gates → one commit.
3. **Wire compatibility is sacred**: never change an existing enum member's
   `value` or reuse a proto tag; give new enum members explicit values above
   the max; identity additions are non-breaking.
4. **Every new `must` constraint needs a `valid-*` AND `invalid-*` fixture**
   in `yang/testdata/`.
5. **Conventional Commits drive releases**: `feat:`/`fix:` on the model
   surface cut versions; `chore:`/`ci:`/`docs:` don't.
6. This is a **public standard**: no issue-tracker IDs, personal names, or
   internal URLs in models, fixtures, or docs.

## Task playbooks

Detailed step-by-step workflows live in `.claude/skills/` as plain
markdown. Read the matching one BEFORE starting:

| Task | Read |
|---|---|
| Change/extend an existing YANG module | `.claude/skills/extending-a-model/SKILL.md` |
| A CI gate failed / interpret `make` gate output | `.claude/skills/model-ci-gates/SKILL.md` |
| Add or change a notification/event | `.claude/skills/adding-an-event/SKILL.md` |
| Add a whole new device service family | `.claude/skills/adding-a-service/SKILL.md` |
| Review (or self-review) a model PR | `.claude/skills/model-pr-review/SKILL.md` |

## Quick commands

- Full local gate: `make all` (mirrors CI); individual gates:
  `make check-gen validate-yang yang-lint check-revisions check-naming`
- Conformance: `go run ./tools/conformance -driver mock -kind <kind>`
  (kinds: asc, rsu, dms, ess, ramp-metering, traffic-sensor,
  reversible-lane, perception, cctv)
- Scaffold a new service: `go run ./tools/openits-new-service -h`
