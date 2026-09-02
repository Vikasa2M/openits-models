# openits-models — data-model layer (YANG, protobuf, AsyncAPI, schema-registry).
# Extracted from the Vikasa monorepo. Regeneration tooling for the model
# artifacts under pkg/ lives here; the collector consumes this as a Go module.

GOCMD := go
GOFMT := gofmt

# Protobuf
PROTO_DIR       := api/proto
PROTO_OUT       := pkg/proto/openits/v1
FIELD_LOCK      := field-numbers.yaml
# YANG output directories
YANG_GO_OUT := pkg/yang

.PHONY: all ci gen check-gen yang-proto-gen proto yang yang-go validate-yang \
	check-revisions check-naming check-enum-values check-inline-enums validate-noi check-graduation \
	check-augment-collisions check-deviations check-events-layering proto-lint proto-breaking yang-lint \
	vet test conformance fmt tidy build-tools \
	asyncapi asyncapi-check catalog catalog-check

# Regenerate every generated model artifact from source: YANG -> proto
# (tools/yang-proto-gen), proto -> Go (protoc), YANG -> Go (ygot).
#
# NOTE: `all` regenerates and gates nothing. For the full local gate, use
# `make ci`.
all: gen
gen: yang-proto-gen proto yang-go asyncapi catalog

# The full local gate: every check CI runs, in one command.
#
# CI runs these as separate jobs so they parallelize and report
# independently, which means this list and the job list in
# .github/workflows/ci.yml are two copies of one fact. Add a gate to both,
# or a check that runs in CI will pass silently here.
#
# That is not hypothetical — it already cost a round trip. The first version
# of this target covered only the four YANG-shaped jobs, so a branch that
# added a service to tools/yang-proto-gen broke `go test ./...` and still
# came back green from every local gate; CI caught it. Hence the mapping
# below: each of CI's seven jobs, and what covers it here.
#
#   ci.yml job        covered by
#   ----------------  ------------------------------------------------
#   go-test           build-tools, vet, test
#   check-gen         check-gen
#   buf               proto-lint, proto-breaking
#   yang-checks       check-revisions ... check-graduation
#   validate-yang     validate-yang
#   yang-lint         yang-lint
#   conformance       conformance
#
# check-gen comes first: it regenerates and fails on drift, so every gate
# after it reads artifacts that match the YANG rather than whatever was
# last committed. conformance comes last because it is the slowest.
#
# Run this on a COMMITTED tree. check-gen's drift test is
# `git diff --exit-code` over the whole worktree, not just generated
# paths, so uncommitted source edits fail it too — which reads as a gate
# failure when it is only unstaged work.
#
# The buf targets announce and skip when buf is not installed, rather than
# failing. That is a visible line in the output, not a silent pass, but it
# does mean a green `make ci` on a machine without buf has not run them.
ci: check-gen build-tools vet test \
	validate-yang yang-lint check-revisions check-naming \
	check-enum-values check-inline-enums check-deviations \
	check-augment-collisions check-events-layering check-ce-id-vectors \
	validate-noi check-graduation \
	proto-lint proto-breaking conformance

# Fail if regenerating drifts from what's committed — the freshness gate.
check-gen: gen
	git diff --exit-code

# --- Code generation ---------------------------------------------------------
# YANG -> proto. Writes api/proto/openits/v1/*.proto (event payloads +
# shared types.proto) and the field-number lock; command.proto/device.proto
# are hand-curated and untouched by this step.
yang-proto-gen:
	$(GOCMD) run ./tools/yang-proto-gen -yang yang -out $(PROTO_DIR) -lock $(FIELD_LOCK)

# proto -> Go (protoc + protoc-gen-go, pinned — see scripts/proto-gen.sh).
proto:
	./scripts/proto-gen.sh

# YANG -> Go (ygot fakeroot structs). Requires the pinned ygot generator.
yang: yang-go
yang-go:
	./scripts/yang-gen.sh go

# YANG -> AsyncAPI 3.0. Derives the ce-type catalog (tools/yang-proto-gen:
# BuildCatalog) and embeds each ce-type's JSON Schema (EmitJSONSchema) as its
# message payload. Writes bindings/nats/asyncapi.yaml — the AsyncAPI document
# belongs to the NATS reference profile (see bindings/nats/README.md), not the
# transport-neutral model layer.
asyncapi:
	$(GOCMD) run ./tools/yang-proto-gen -asyncapi -yang yang -out bindings/nats

# Fail if regenerating asyncapi.yaml drifts from what's committed.
asyncapi-check: asyncapi
	git diff --exit-code -- bindings/nats/asyncapi.yaml

# YANG + schema-registry -> schema-registry/index.json. Emits the neutral
# machine-readable self-index of the standard (services, foundation modules,
# ce-types, revisions, normative references, and the registry snapshot map)
# that any consumer — the open-its.org website, vendors, tooling — reads with
# zero knowledge of any particular consumer. Scans schema-registry/ for the
# snapshot map and writes index.json beside it. Generated, never hand-edited.
catalog:
	$(GOCMD) run ./tools/yang-proto-gen -catalog -yang yang -out schema-registry

# Fail if regenerating index.json drifts from what's committed.
catalog-check: catalog
	git diff --exit-code -- schema-registry/index.json

# --- Validation / lint -------------------------------------------------------
# Validate golden YANG instance data against modules (yanglint in Docker).
validate-yang:
	./scripts/validate-yang.sh

# Fail if a module's content changed without a revision bump.
check-revisions:
	./scripts/check-revisions.sh

# Reject legacy ce-types / URN / subject forms.
check-naming:
	./scripts/check-naming.sh

# Every first-party enum member must carry an explicit value statement
# (implicit positional numbering is a silent wire-break hazard).
check-enum-values:
	python3 scripts/check-enum-values.py

# No grouping composed across module lines may carry an inline enumeration:
# ygot names such a type after a USE-SITE path, so adding a second composer
# silently renames it out from under the first (wire-neutral, but it breaks
# Go consumers of an unrelated module).
check-inline-enums:
	python3 scripts/check-inline-enums.py

# Validate NoI YAML under schema-registry/notices/.
validate-noi:
	$(GOCMD) run ./tools/noi-validator schema-registry/notices

# Per-augment NoI / graduation report.
check-graduation:
	$(GOCMD) run ./tools/check-graduation

# Warn on augments targeting the same YANG path.
check-augment-collisions:
	$(GOCMD) run ./tools/check-augment-collisions yang/augments

# Validate that yang/deviations/* resolve against their base and only tighten.
check-deviations:
	$(GOCMD) run ./tools/check-deviations yang yang/deviations

# Enforce that yang/*-events.yang modules import only openits-types /
# ietf-yang-types / openits-nema-common / a *-types module — never a
# service core or another events module.
check-events-layering:
	$(GOCMD) run ./tools/check-events-layering yang

# Re-derive every ce-id test vector published in docs/ce-id-spec.md from
# the algorithm that document specifies. The spec is normative for the wire
# contract, so its vectors have to be arithmetic, not prose.
check-ce-id-vectors:
	$(GOCMD) run ./tools/check-ce-id-vectors

# Protobuf lint via buf (skipped if buf absent).
proto-lint:
	@if command -v buf >/dev/null 2>&1; then buf lint; \
	else echo "buf not installed; skipping proto-lint"; fi

# Protobuf breaking-change check against main, the second half of CI's `buf`
# job. CI resolves main to a concrete SHA and fetches it; locally the main
# branch in this repo is the comparison point, so the answer is only as
# current as your last fetch. A no-op when run on main itself.
proto-breaking:
	@if command -v buf >/dev/null 2>&1; then \
		buf breaking --against '.git#branch=main'; \
	else echo "buf not installed; skipping proto-breaking"; fi

# pyang YANG lint (skipped if pyang absent).
yang-lint:
	@if command -v pyang >/dev/null 2>&1; then \
		pyang --strict --max-line-length=120 -p yang -p yang/ietf \
			yang/*.yang yang/augments/*.yang yang/deviations/*.yang; \
	else echo "pyang not installed; skipping yang-lint"; fi

# Conformance harness against the built-in mock device. CI runs one matrix
# job per kind; locally they run in sequence. This list must match the
# matrix in .github/workflows/ci.yml.
CONFORMANCE_KINDS := asc rsu dms ess ramp-metering traffic-sensor \
	reversible-lane perception cctv

conformance:
	@set -e; for kind in $(CONFORMANCE_KINDS); do \
		echo "== conformance: $$kind"; \
		$(GOCMD) run ./tools/conformance -driver mock -kind "$$kind"; \
	done

# --- Go housekeeping ---------------------------------------------------------
vet:
	$(GOCMD) vet ./...

# Unit tests for every tool and generated package — CI's `go-test` job runs
# this alongside build-tools and vet. The tools under tools/ carry real
# assertions about the model (the service catalog is pinned in one of them),
# so this is a model gate, not just Go housekeeping.
test:
	$(GOCMD) test ./...
fmt:
	$(GOFMT) -s -w .
tidy:
	$(GOCMD) mod tidy
build-tools:
	$(GOCMD) build ./...

# bindings/nats/asyncapi.yaml is generated in-repo (see the `asyncapi` target
# above) from the YANG-derived ce-type catalog, not copied in from the collector.
