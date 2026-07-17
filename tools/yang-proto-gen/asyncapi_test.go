package main

import (
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestEmitAsyncAPI builds the full ce-type catalog and its JSON Schema
// payloads from the real yang/ corpus (mirrors TestBuildCatalog_
// derivesServiceMatrix's setup), emits the AsyncAPI 3.0 document, and
// checks: it parses as YAML with asyncapi: 3.0.0; it has a channel +
// components.messages entry for the known ce-type
// openits.dms.fault-raised.v1; that message's payload embeds the
// fault-raised notification's own JSON Schema (has properties.kind — the
// common-events "kind" identityref every fault-raised notification carries)
// rather than a URL pointer to an out-of-repo schema registry snapshot; and
// two independent emits over the same input are byte-identical.
func TestEmitAsyncAPI(t *testing.T) {
	ms, mods, err := LoadModules(filepath.Join("..", "..", "yang"))
	if err != nil {
		t.Fatal(err)
	}
	cat, err := BuildCatalog(ms, mods)
	if err != nil {
		t.Fatal(err)
	}
	shared := SharedGroupings(mods)
	schemas, err := BuildSchemas(cat, mods, shared)
	if err != nil {
		t.Fatal(err)
	}

	out, err := EmitAsyncAPI(cat, schemas)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(out)

	if !strings.Contains(doc, "asyncapi: 3.0.0") {
		t.Errorf("expected literal \"asyncapi: 3.0.0\" in output, got:\n%s", doc[:min(len(doc), 400)])
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("EmitAsyncAPI output does not parse as YAML: %v", err)
	}
	if v, _ := parsed["asyncapi"].(string); v != "3.0.0" {
		t.Errorf("asyncapi = %v, want 3.0.0", parsed["asyncapi"])
	}

	channels, ok := parsed["channels"].(map[string]any)
	if !ok {
		t.Fatalf("expected top-level channels map, got %T", parsed["channels"])
	}
	if _, ok := channels["openits.dms.fault-raised.v1"]; !ok {
		t.Errorf("missing channel for openits.dms.fault-raised.v1 (have %d channels)", len(channels))
	}

	components, ok := parsed["components"].(map[string]any)
	if !ok {
		t.Fatalf("expected top-level components map, got %T", parsed["components"])
	}
	messages, ok := components["messages"].(map[string]any)
	if !ok {
		t.Fatalf("expected components.messages map, got %T", components["messages"])
	}
	msg, ok := messages["openits.dms.fault-raised.v1"].(map[string]any)
	if !ok {
		t.Fatalf("missing components.messages[openits.dms.fault-raised.v1] (have %d messages)", len(messages))
	}
	payload, ok := msg["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected message payload map, got %T", msg["payload"])
	}
	if got, want := payload["schemaFormat"], "application/schema+json;version=draft-2020-12"; got != want {
		t.Errorf("payload.schemaFormat = %v, want %v", got, want)
	}
	schema, ok := payload["schema"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload.schema map (the embedded JSON Schema), got %T", payload["schema"])
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload.schema.properties map, got %T", schema["properties"])
	}
	if _, ok := props["kind"]; !ok {
		t.Errorf("expected payload.schema.properties.kind (fault-raised's discriminant leaf), got properties: %v", props)
	}

	// Determinism: two independent emits over the same (cat, schemas) input
	// must be byte-identical, mirroring TestGenerate_isDeterministic /
	// TestMarshalSchemaDeterministic_stable.
	out2, err := EmitAsyncAPI(cat, schemas)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(out2) {
		t.Error("EmitAsyncAPI is not deterministic: two emits over the same input produced different bytes")
	}
}



// TestBuildSchemas_coversFullCatalog guards the CLI/test wiring contract:
// every ce-type BuildCatalog returns must resolve to a JSON Schema — a
// missing (SchemaModule, SchemaNotification) pair is a BuildCatalog/
// BuildSchemas invariant break, not something EmitAsyncAPI should have to
// handle by skipping ce-types silently.
func TestBuildSchemas_coversFullCatalog(t *testing.T) {
	ms, mods, err := LoadModules(filepath.Join("..", "..", "yang"))
	if err != nil {
		t.Fatal(err)
	}
	cat, err := BuildCatalog(ms, mods)
	if err != nil {
		t.Fatal(err)
	}
	shared := SharedGroupings(mods)
	schemas, err := BuildSchemas(cat, mods, shared)
	if err != nil {
		t.Fatal(err)
	}
	if len(schemas) != len(cat) {
		t.Errorf("BuildSchemas returned %d schemas for %d ce-types", len(schemas), len(cat))
	}
	for _, c := range cat {
		if _, ok := schemas[c.Type]; !ok {
			t.Errorf("missing schema for ce-type %s", c.Type)
		}
	}
}
