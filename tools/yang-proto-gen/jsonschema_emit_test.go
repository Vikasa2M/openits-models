package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
)

// TestEmitJSONSchema_fixture mirrors TestEmitMessage_leavesContainers: load
// the fixture, EmitJSONSchema the one notification, and compare the
// deterministic marshal against a hand-derived golden. Locks the RFC 7951
// property-key rules (kebab-case, not snake_case), the leaf/container walk,
// numeric range mapping, and mandatory -> required.
func TestEmitJSONSchema_fixture(t *testing.T) {
	ms := yang.NewModules()
	for _, f := range []string{
		filepath.Join("testdata", "jsonschema-fixture.yang"),
		filepath.Join("..", "..", "yang", "ietf", "ietf-yang-types.yang"),
	} {
		if err := ms.Read(f); err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process: %v", errs)
	}
	mod, errs := ms.GetModule("jsonschema-fixture")
	if len(errs) > 0 {
		t.Fatalf("getmodule: %v", errs)
	}

	var notif *yang.Entry
	for _, c := range sortedChildren(mod) {
		if c.Kind == yang.NotificationEntry {
			notif = c
		}
	}
	if notif == nil {
		t.Fatalf("no notification found in jsonschema-fixture")
	}

	schema := EmitJSONSchema(notif, nil)
	got := strings.TrimSpace(string(MarshalSchemaDeterministic(schema)))
	want := strings.TrimSpace(readGolden(t, "jsonschema-fixture.golden"))
	if got != want {
		t.Errorf("schema mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestMarshalSchemaDeterministic_stable guards byte-stability across repeated
// calls on the same logical map: encoding/json sorts map[string]any keys
// alphabetically on every Marshal, so two calls on equivalent-but-freshly-
// built maps (as if produced by independent EmitJSONSchema runs) must
// produce identical bytes, not just deep-equal structures.
func TestMarshalSchemaDeterministic_stable(t *testing.T) {
	build := func() map[string]any {
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"zebra": map[string]any{"type": "string"},
				"alpha": map[string]any{"type": "integer"},
				"mid":   map[string]any{"type": "boolean"},
			},
		}
	}
	a := MarshalSchemaDeterministic(build())
	b := MarshalSchemaDeterministic(build())
	if string(a) != string(b) {
		t.Errorf("expected identical bytes across independent builds, got:\n%s\n---\n%s", a, b)
	}
}

// TestEmitJSONSchema_sharedGrouping guards the $defs/$ref path: a container
// child whose grouping identity is in the shared map is emitted once into
// top-level $defs and referenced everywhere else via $ref, instead of being
// inlined per use (mirrors TestEmit_wireSourceSharedOnce's proto
// counterpart). Reuses reserved-tag-fixture.yang, whose wire-source grouping
// is uses'd at 2 sites (wire-source-event-a/b) specifically so
// SharedGroupings treats it as shared.
func TestEmitJSONSchema_sharedGrouping(t *testing.T) {
	ms := yang.NewModules()
	if err := ms.Read(filepath.Join("testdata", "reserved-tag-fixture.yang")); err != nil {
		t.Fatalf("read reserved-tag-fixture.yang: %v", err)
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process: %v", errs)
	}
	mod, errs := ms.GetModule("reserved-tag-fixture")
	if len(errs) > 0 {
		t.Fatalf("getmodule: %v", errs)
	}

	shared := SharedGroupings([]*yang.Entry{mod})
	if _, ok := shared["wire-source"]; !ok {
		t.Fatalf("fixture setup broken: wire-source not detected as shared, got: %v", shared)
	}

	var a, b *yang.Entry
	for _, c := range sortedChildren(mod) {
		switch c.Name {
		case "wire-source-event-a":
			a = c
		case "wire-source-event-b":
			b = c
		}
	}
	if a == nil || b == nil {
		t.Fatalf("fixture missing wire-source-event-a/b notifications")
	}

	schemaA := EmitJSONSchema(a, shared)
	schemaB := EmitJSONSchema(b, shared)

	defsA, ok := schemaA["$defs"].(map[string]any)
	if !ok {
		t.Fatalf("expected $defs in schema A, got: %v", schemaA)
	}
	if _, ok := defsA["WireSource"]; !ok {
		t.Errorf("expected $defs.WireSource in schema A, got: %v", defsA)
	}

	propsA, ok := schemaA["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties in schema A, got: %v", schemaA)
	}
	sourceRef, ok := propsA["source"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties.source in schema A, got: %v", propsA)
	}
	if sourceRef["$ref"] != "#/$defs/WireSource" {
		t.Errorf("expected properties.source to be a $ref to WireSource, got: %v", sourceRef)
	}

	defsB, ok := schemaB["$defs"].(map[string]any)
	if !ok {
		t.Fatalf("expected $defs in schema B, got: %v", schemaB)
	}
	if _, ok := defsB["WireSource"]; !ok {
		t.Errorf("expected $defs.WireSource in schema B too (each notification's own schema is self-contained), got: %v", defsB)
	}
}

// TestEmitJSONSchema_choiceOptionalProperties guards the choice/case rule: a
// choice's case leaves are merged directly into the parent object's
// properties (no nested oneof-shaped wrapper, matching the RFC 7951 wire
// encoding where only one case's leaves are present at a time) and are never
// added to "required", even though nothing in the fixture marks them
// mandatory — the case leaves are inherently optional at the schema level
// since only one case's members appear on the wire for any given instance.
func TestEmitJSONSchema_choiceOptionalProperties(t *testing.T) {
	ms := yang.NewModules()
	if err := ms.Read(filepath.Join("testdata", "choice-multi-fixture.yang")); err != nil {
		t.Fatalf("read choice-multi-fixture.yang: %v", err)
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process: %v", errs)
	}
	mod, errs := ms.GetModule("choice-multi-fixture")
	if len(errs) > 0 {
		t.Fatalf("getmodule: %v", errs)
	}

	var notif *yang.Entry
	for _, c := range sortedChildren(mod) {
		if c.Kind == yang.NotificationEntry {
			notif = c
		}
	}
	if notif == nil {
		t.Fatalf("no notification found in choice-multi-fixture")
	}

	schema := EmitJSONSchema(notif, nil)
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties, got: %v", schema)
	}
	for _, want := range []string{"decoder", "a-code", "a-note", "b-oid"} {
		if _, ok := props[want]; !ok {
			t.Errorf("expected property %q merged in from choice cases, got: %v", want, props)
		}
	}
	if _, ok := schema["required"]; ok {
		t.Errorf("no leaf in the fixture is mandatory, so no top-level required should be present, got: %v", schema["required"])
	}
}
