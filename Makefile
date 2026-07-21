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

.PHONY: all gen check-gen yang-proto-gen proto yang yang-go validate-yang \
	check-revisions check-naming validate-noi check-graduation \
	check-augment-collisions check-deviations check-events-layering proto-lint yang-lint vet fmt tidy build-tools \
	asyncapi asyncapi-check

# Regenerate every generated model artifact from source: YANG -> proto
# (tools/yang-proto-gen), proto -> Go (protoc), YANG -> Go (ygot).
all: gen
gen: yang-proto-gen proto yang-go asyncapi

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

# Protobuf lint via buf (skipped if buf absent).
proto-lint:
	@if command -v buf >/dev/null 2>&1; then buf lint; \
	else echo "buf not installed; skipping proto-lint"; fi

# pyang YANG lint (skipped if pyang absent).
yang-lint:
	@if command -v pyang >/dev/null 2>&1; then \
		pyang --strict --max-line-length=120 -p yang -p yang/ietf \
			yang/*.yang yang/augments/*.yang yang/deviations/*.yang; \
	else echo "pyang not installed; skipping yang-lint"; fi

# --- Go housekeeping ---------------------------------------------------------
vet:
	$(GOCMD) vet ./...
fmt:
	$(GOFMT) -s -w .
tidy:
	$(GOCMD) mod tidy
build-tools:
	$(GOCMD) build ./...

# bindings/nats/asyncapi.yaml is generated in-repo (see the `asyncapi` target
# above) from the YANG-derived ce-type catalog, not copied in from the collector.
