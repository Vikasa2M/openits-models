package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// TestJSONSchema_validatesRealInstance is the JSON-Schema analogue of the
// proto backend's yanglint gate: TestEmitJSONSchema_fixture (jsonschema_emit_test.go)
// only proves EmitJSONSchema's output is internally self-consistent against a
// hand-derived golden. This test proves something a golden comparison
// cannot: that the schema Generate actually writes for
// openits-common-fault-events:fault-raised is RFC 7951-*correct*, by
// running it through a real, independent JSON Schema validator
// (santhosh-tekuri/jsonschema, pinned at v5.3.1 — see go.mod) against a real
// wire fixture, and confirming the validator both accepts the valid fixture
// and rejects a type-violating mutation of it. A schema that was internally
// consistent but wrong (e.g. typed severity as "number") would still pass a
// golden-file comparison; it would not pass this test.
//
// Wrapped-vs-inner decision (documented, not ambiguous): RFC 7951 wraps a
// notification's payload in a single "<module>:<notification-name>"
// top-level key — see yang/testdata/valid-cross-service-fault-reaches-orphan.json,
// whose entire document is {"openits-common-fault-events:fault-raised": {...}}.
// EmitJSONSchema (jsonschema_emit.go) never adds that outer wrapper: its
// schema describes only the notification's own body, built from e's direct
// children (jsonSchemaObject). So this test validates the *inner* payload
// object — fixture["openits-common-fault-events:fault-raised"] — against
// the generated schema, not the outer wrapped document. That mirrors how a
// real consumer would use it: dispatch on the RFC 7951 wrapper key first
// (routing to "which schema"), then validate the unwrapped payload against
// that schema, the same way a protobuf consumer validates a decoded
// FaultRaised message body without its envelope.
func TestJSONSchema_validatesRealInstance(t *testing.T) {
	out := t.TempDir()
	lock := filepath.Join(out, "field-numbers.yaml")
	if err := Generate(filepath.Join("..", "..", "yang"), out, lock); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Locate the schema Generate emitted for the fault-raised notification
	// declared in openits-common-fault-events.yang, without hardcoding
	// emitJSONSchemas' internal directory layout — only the module- and
	// notification-qualified filename convention it documents.
	const wantSuffix = "openits-common-fault-events.fault-raised.schema.json"
	var schemaPath string
	if err := filepath.WalkDir(out, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, wantSuffix) {
			schemaPath = path
		}
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", out, err)
	}
	if schemaPath == "" {
		t.Fatalf("Generate did not emit a schema file ending in %q under %s", wantSuffix, out)
	}

	sch, err := jsonschema.Compile(schemaPath)
	if err != nil {
		t.Fatalf("compile %s: %v", schemaPath, err)
	}

	fixturePath := filepath.Join("..", "..", "yang", "testdata", "valid-cross-service-fault-reaches-orphan.json")
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixturePath, err)
	}
	var wrapped map[string]any
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", fixturePath, err)
	}
	const wrapperKey = "openits-common-fault-events:fault-raised"
	inst, ok := wrapped[wrapperKey].(map[string]any)
	if !ok {
		t.Fatalf("fixture %s missing %q payload object, got: %v", fixturePath, wrapperKey, wrapped)
	}

	if err := sch.Validate(inst); err != nil {
		t.Errorf("expected the real RFC 7951 fixture to validate against the generated fault-raised schema, got: %v", err)
	}

	// Mutate a copy: severity is schema-typed "string" (RFC 7951 maps a YANG
	// enumeration to a plain JSON string — see JSONSchemaType's yang.Yenum
	// case), so replacing it with a JSON number must fail validation. This
	// is the case a self-consistency check alone cannot catch: a wrong type
	// mapping would still produce an internally-consistent schema, but would
	// fail to reject wire data that violates the real RFC 7951 encoding.
	mutated := make(map[string]any, len(inst))
	for k, v := range inst {
		mutated[k] = v
	}
	if _, ok := mutated["severity"]; !ok {
		t.Fatalf("fixture setup broken: no severity field to mutate, got: %v", inst)
	}
	mutated["severity"] = 42.0

	if err := sch.Validate(mutated); err == nil {
		t.Error("expected validation to fail for severity mutated from a string to a number, got nil error")
	}
}

// TestEmitJSONSchemas_prunesOrphanedSchemas proves emitJSONSchemas deletes
// stale schema.json files left behind by a notification that no longer
// exists in the live YANG corpus — without this, `make gen` is append-only
// for schema.json: it writes one file per live notification but never
// removes one for a notification that was deleted or deprecated out, so a
// stale schema describing deleted API surface lingers on disk indefinitely.
// (This is exactly what happened when 6 per-service fault/mode
// notifications were deleted: 6 orphaned schema.json files were left
// behind.) A stray
// schema.json is planted in the real openits/common/v1 output directory
// (openits-common-fault-events routes there, per pkgmap.go's serviceRoutes)
// before Generate runs, named so it cannot correspond to any live
// notification; it must be gone afterward, while a schema.json for a real
// live notification in the same directory must still be present.
func TestEmitJSONSchemas_prunesOrphanedSchemas(t *testing.T) {
	out := t.TempDir()
	lock := filepath.Join(out, "field-numbers.yaml")

	dir := filepath.Join(out, "openits", "common", "v1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	stalePath := filepath.Join(dir, "openits-x.stale-notif.schema.json")
	if err := os.WriteFile(stalePath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write stale fixture: %v", err)
	}

	if err := Generate(filepath.Join("..", "..", "yang"), out, lock); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Errorf("expected orphaned schema %s to be pruned, stat err = %v", stalePath, err)
	}

	livePath := filepath.Join(dir, "openits-common-fault-events.fault-raised.schema.json")
	if _, err := os.Stat(livePath); err != nil {
		t.Errorf("expected live notification schema %s to survive pruning: %v", livePath, err)
	}
}
