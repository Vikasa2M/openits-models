package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openconfig/goyang/pkg/yang"
	"gopkg.in/yaml.v3"
)

// AsyncAPI 3.0 document constants — the parts of the document that aren't
// derived per ce-type. title/version/license/defaultContentType are carried
// over verbatim from the pre-generation asyncapi.yaml (see the P2b-2 Task 2
// brief); messageContentType is the actual wire content type (CloudEvents
// binary mode: the payload bytes are pure protobuf) as it was in that file,
// kept distinct from payloadSchemaFormat, which describes the *schema
// dialect* the payload below is expressed in (JSON Schema draft 2020-12),
// not the wire encoding — AsyncAPI's Multi Format Schema Object exists
// precisely so a message's attached schema can be written in a different
// format than its contentType.
const (
	asyncAPIVersion     = "3.0.0"
	asyncAPITitle       = "OpenITS Event API"
	asyncAPIDocVersion  = "1.0.0"
	asyncAPILicenseName = "Apache-2.0"
	asyncAPILicenseURL  = "https://www.apache.org/licenses/LICENSE-2.0"
	defaultContentType  = "application/cloudevents+protobuf; charset=utf-8"
	messageContentType  = "application/protobuf"
	payloadSchemaFormat = "application/schema+json;version=draft-2020-12"
)

// asyncAPIDescription is built from plain interpreted string literals
// (concatenated, not a single raw string) so the literal backticks around
// "make asyncapi" are just ordinary characters — no raw/interpreted-string
// splicing to get wrong.
const asyncAPIDescription = "Live event API for OpenITS infrastructure (signal control, DMS,\n" +
	"ESS / RWIS, RSU, ramp metering, perception, traffic-sensor, and\n" +
	"reversible-lane). Generated from the YANG-derived ce-type catalog\n" +
	"(tools/yang-proto-gen: BuildCatalog); each message payload is that\n" +
	"notification's own JSON Schema (EmitJSONSchema). Do not edit by hand —\n" +
	"regenerate with `make asyncapi`.\n" +
	"\n" +
	"Envelope is CloudEvents 1.0 in binary mode (CE attributes ride in NATS\n" +
	"headers; payload bytes are pure protobuf on the wire — the JSON Schema\n" +
	"payload below describes that protobuf's structure for tooling, it is not\n" +
	"itself the wire encoding). Transport is NATS with JetStream for durability.\n"

// asyncAPIHeader is the DO-NOT-EDIT provenance comment prepended (outside
// the YAML document proper, as plain "# " comment lines) to every generated
// asyncapi.yaml, mirroring the header the old collector-sourced file
// carried. Kept short enough that "asyncapi: 3.0.0" — always the first
// top-level key in the marshaled document, since "asyncapi" sorts before
// every other top-level key yaml.v3 emits (see EmitAsyncAPI) — lands within
// the file's first 8 lines, matching the P2b-2 Task 2 brief's `head -8 |
// grep "asyncapi: 3.0.0"` sanity check.
const asyncAPIHeader = "# DO NOT EDIT BY HAND. Regenerate with `make asyncapi`.\n" +
	"#\n" +
	"# Source of truth: the YANG-derived ce-type catalog (BuildCatalog) plus\n" +
	"# each notification's own JSON Schema (EmitJSONSchema). `make\n" +
	"# asyncapi-check` fails when this file drifts from a fresh generation.\n" +
	"\n"

// BuildSchemas resolves, for every ce-type in cat, the JSON Schema of the
// YANG notification it derives from — cat[i].SchemaModule /
// cat[i].SchemaNotification identify that notification (see CeType in
// catalog.go) — keyed by ce-type Type. shared is SharedGroupings's output,
// passed through to EmitJSONSchema unchanged (each ce-type's schema is
// still a fully self-contained document per EmitJSONSchema's own
// once-per-call $defs contract; BuildSchemas doesn't share $defs across
// ce-types).
//
// Returns an error if a ce-type's (SchemaModule, SchemaNotification) pair
// isn't found in mods: that would mean BuildCatalog produced a CeType whose
// schema source doesn't actually exist, a BuildCatalog/BuildSchemas
// invariant mismatch that must fail loudly rather than silently drop a
// ce-type's payload.
func BuildSchemas(cat []CeType, mods []*yang.Entry, shared map[string]string) (map[string]map[string]any, error) {
	byModule := make(map[string]*yang.Entry, len(mods))
	for _, m := range mods {
		byModule[m.Name] = m
	}

	schemas := make(map[string]map[string]any, len(cat))
	for _, c := range cat {
		mod, ok := byModule[c.SchemaModule]
		if !ok {
			return nil, fmt.Errorf("build schemas: ce-type %s: module %q not found", c.Type, c.SchemaModule)
		}
		notif, ok := mod.Dir[c.SchemaNotification]
		if !ok || notif.Kind != yang.NotificationEntry {
			return nil, fmt.Errorf("build schemas: ce-type %s: notification %q not found in module %q", c.Type, c.SchemaNotification, c.SchemaModule)
		}
		schemas[c.Type] = EmitJSONSchema(notif, shared)
	}
	return schemas, nil
}

// EmitAsyncAPI renders a deterministic AsyncAPI 3.0.0 YAML document from cat
// (BuildCatalog's output) and schemas (BuildSchemas's output, keyed by
// ce-type Type): one channel + one send/receive operation pair + one
// components.messages entry per ce-type, with the message payload set to
// the ce-type's own embedded JSON Schema rather than a URL pointer to an
// out-of-repo schema-registry snapshot (the property the old
// collector-sourced asyncapi.yaml lacked — see docs/asyncapi-delta.md).
//
// Determinism: every value built here is a plain Go map/slice, and
// yaml.v3's Marshal sorts every map's keys before encoding (see
// gopkg.in/yaml.v3@v3.0.1/encode.go's keyList/sort.Sort and sorter.go's
// Less) — so two independent calls over equal (cat, schemas) inputs produce
// byte-identical output regardless of Go's randomized map iteration order,
// mirroring MarshalSchemaDeterministic's contract for the JSON Schema
// backend (jsonschema_emit.go).
func EmitAsyncAPI(cat []CeType, schemas map[string]map[string]any) ([]byte, error) {
	channels := make(map[string]any, len(cat))
	operations := make(map[string]any, 2*len(cat))
	messages := make(map[string]any, len(cat))

	for _, c := range cat {
		schema, ok := schemas[c.Type]
		if !ok {
			return nil, fmt.Errorf("emit asyncapi: no JSON Schema for ce-type %s", c.Type)
		}

		channels[c.Type] = map[string]any{
			"address": c.Type,
			"title":   c.Service + " — " + c.Event,
			"messages": map[string]any{
				c.Type: map[string]any{"$ref": "#/components/messages/" + c.Type},
			},
		}

		channelRef := map[string]any{"$ref": "#/channels/" + c.Type}
		messageRefs := []any{
			map[string]any{"$ref": "#/channels/" + c.Type + "/messages/" + c.Type},
		}
		operations["publish_"+c.Type] = map[string]any{
			"action":   "send",
			"channel":  channelRef,
			"summary":  "Publish " + c.Type,
			"messages": messageRefs,
		}
		operations["consume_"+c.Type] = map[string]any{
			"action":   "receive",
			"channel":  channelRef,
			"summary":  "Consume " + c.Type,
			"messages": messageRefs,
		}

		messages[c.Type] = map[string]any{
			"name":        c.Type,
			"title":       c.Type,
			"summary":     c.Service + "." + c.Event,
			"contentType": messageContentType,
			"headers":     ceHeaders(c.Type),
			"payload": map[string]any{
				"schemaFormat": payloadSchemaFormat,
				"schema":       schema,
			},
		}
	}

	doc := map[string]any{
		"asyncapi": asyncAPIVersion,
		"info": map[string]any{
			"title":       asyncAPITitle,
			"version":     asyncAPIDocVersion,
			"description": asyncAPIDescription,
			"license": map[string]any{
				"name": asyncAPILicenseName,
				"url":  asyncAPILicenseURL,
			},
		},
		"defaultContentType": defaultContentType,
		"channels":           channels,
		"operations":         operations,
		"components": map[string]any{
			"messages": messages,
		},
	}

	body, err := yaml.Marshal(doc)
	if err != nil {
		// doc is built entirely from maps/slices/strings/consts plus the
		// EmitJSONSchema-produced schema maps (themselves unconditionally
		// marshalable — see MarshalSchemaDeterministic's identical
		// reasoning in jsonschema_emit.go). A Marshal error here would
		// mean a caller passed a non-marshalable value in schemas, a
		// programmer error surfaced as a normal error since (unlike
		// MarshalSchemaDeterministic) EmitAsyncAPI's signature already
		// returns one.
		return nil, fmt.Errorf("emit asyncapi: marshal: %w", err)
	}

	return append([]byte(asyncAPIHeader), body...), nil
}

// ceHeaders builds the generic CloudEvents-attribute header schema every
// generated message carries, with ce-type pinned to this message's own
// ce-type via `const`. Deliberately generic (no per-service ce-source URN
// pattern, no per-module ce-dataschema URL): BuildCatalog's CeType doesn't
// carry the per-notification revision-dated schema-registry path the old
// hand-maintained asyncapi.yaml's ce-dataschema constants encoded, and
// inventing one here would be exactly the kind of undocumented judgment
// call the brief's derivation rules don't authorize.
func ceHeaders(ceType string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ce-specversion":     map[string]any{"type": "string", "const": "1.0"},
			"ce-type":            map[string]any{"type": "string", "const": ceType},
			"ce-source":          map[string]any{"type": "string", "description": "CloudEvents source URI identifying the emitting device or service instance."},
			"ce-id":              map[string]any{"type": "string", "description": "Event identifier, unique within the source."},
			"ce-time":            map[string]any{"type": "string", "format": "date-time"},
			"ce-datacontenttype": map[string]any{"type": "string", "const": messageContentType},
			"traceparent":        map[string]any{"type": "string", "description": "W3C Trace Context. Optional; recommended."},
		},
	}
}

// GenerateAsyncAPI loads every YANG module under yangDir, derives the
// ce-type catalog (BuildCatalog) and each ce-type's JSON Schema payload
// (BuildSchemas/EmitJSONSchema), and writes the resulting AsyncAPI 3.0
// document to outPath. Mirrors Generate's load-derive-write shape (see
// main.go) but produces a single file rather than a tree of .proto files —
// this is the function the -asyncapi CLI flag calls.
func GenerateAsyncAPI(yangDir, outPath string) error {
	ms, mods, err := LoadModules(yangDir)
	if err != nil {
		return fmt.Errorf("generate asyncapi: load yang modules from %s: %w", yangDir, err)
	}

	cat, err := BuildCatalog(ms, mods)
	if err != nil {
		return fmt.Errorf("generate asyncapi: build catalog: %w", err)
	}

	shared := SharedGroupings(mods)
	schemas, err := BuildSchemas(cat, mods, shared)
	if err != nil {
		return fmt.Errorf("generate asyncapi: %w", err)
	}

	doc, err := EmitAsyncAPI(cat, schemas)
	if err != nil {
		return fmt.Errorf("generate asyncapi: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("generate asyncapi: mkdir %s: %w", filepath.Dir(outPath), err)
	}
	if err := os.WriteFile(outPath, doc, 0o644); err != nil {
		return fmt.Errorf("generate asyncapi: write %s: %w", outPath, err)
	}
	return nil
}
