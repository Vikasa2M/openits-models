package main

import (
	"encoding/json"
	"sort"

	"github.com/openconfig/goyang/pkg/yang"
)

// EmitJSONSchema returns a complete draft-2020-12 JSON Schema object for one
// notification e, per RFC 7951's JSON encoding rules. Unlike the proto
// backend's EmitMessage (which converts YANG identifiers to
// snake_case/UpperCamelCase for proto's naming conventions), property keys
// here are the YANG names VERBATIM — RFC 7951 kebab-case member names are
// the actual wire encoding, so "occurred-at" must stay "occurred-at", not
// become "occurred_at".
//
// shared is the same grouping-identity->name map SharedGroupings/EmitMessage
// use: a child container whose grouping usage is in shared is emitted once
// into the returned schema's top-level "$defs" and every reference to it
// (including a second occurrence within the same notification, and any
// future notification calling EmitJSONSchema again) is a "$ref" to it,
// mirroring emitSharedMessage's once-per-output-artifact guarantee — except
// here the "artifact" is a single notification's own schema document, so
// each call to EmitJSONSchema gets its own freshly populated $defs rather
// than sharing dedup state across notifications (a JSON Schema document is
// meant to be self-contained/standalone, unlike a .proto file where messages
// accumulate across an entire generation run). Pass a nil/empty map for
// notifications with no shared groupings.
func EmitJSONSchema(e *yang.Entry, shared map[string]string) map[string]any {
	defs := map[string]any{}
	emitted := map[string]bool{}

	schema := jsonSchemaObject(e, shared, defs, emitted)
	schema["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	if len(defs) > 0 {
		schema["$defs"] = defs
	}
	return schema
}

// jsonSchemaObject builds the {type:object, properties, additionalProperties
// [, required]} schema for e's own direct children (leaves, leaf-lists,
// containers, lists, choices) — used both for the top-level notification
// (via EmitJSONSchema) and recursively for every nested container/list-item/
// shared-grouping body.
func jsonSchemaObject(e *yang.Entry, shared map[string]string, defs map[string]any, emitted map[string]bool) map[string]any {
	properties := map[string]any{}
	var required []string

	walkChildren(sortedChildren(e), shared, defs, emitted, properties, &required, false)

	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		sort.Strings(required)
		schema["required"] = required
	}
	return schema
}

// walkChildren walks children (an entry's direct sortedChildren, or a choice
// case's sortedChildren when called recursively from within a choice) into
// properties/required. inChoice marks a call made on behalf of a choice
// case's members: a case's leaves are merged into the *parent* object's
// properties directly (RFC 7951 splices a `case`'s members into the
// enclosing object exactly like `uses` does — there is no separate
// "case wrapper" on the wire), and are never added to required regardless of
// their own YANG mandatory statement, because only one case's members are
// ever present on the wire at a time — the brief's "optional-properties
// matches the wire" rule.
func walkChildren(children []*yang.Entry, shared map[string]string, defs map[string]any, emitted map[string]bool, properties map[string]any, required *[]string, inChoice bool) {
	for _, c := range children {
		if c.Kind == yang.ChoiceEntry {
			for _, cs := range sortedChildren(c) {
				walkChildren(sortedChildren(cs), shared, defs, emitted, properties, required, true)
			}
			continue
		}

		var propSchema map[string]any
		switch {
		case c.IsLeaf():
			propSchema = JSONSchemaType(c.Type)
		case c.IsLeafList():
			propSchema = map[string]any{
				"type":  "array",
				"items": JSONSchemaType(c.Type),
			}
		case c.IsList():
			propSchema = map[string]any{
				"type":  "array",
				"items": jsonSchemaObject(c, shared, defs, emitted),
			}
		case c.IsContainer():
			if grpName, ok := groupingOf(c); ok {
				if sharedMsg, isShared := shared[grpName]; isShared {
					emitSharedDef(c, sharedMsg, shared, defs, emitted)
					propSchema = map[string]any{"$ref": "#/$defs/" + sharedMsg}
					break
				}
			}
			propSchema = jsonSchemaObject(c, shared, defs, emitted)
		default:
			// Kinds with no data-instance representation on the wire
			// (anydata/anyxml are left unhandled, matching the proto
			// backend's scope) contribute no property.
			continue
		}

		properties[c.Name] = propSchema
		if !inChoice && c.Mandatory == yang.TSTrue {
			*required = append(*required, c.Name)
		}
	}
}

// emitSharedDef writes shared-grouping container c's schema (keyed by its
// proto-style message name sharedMsg, e.g. "WireSource") into defs at most
// once per EmitJSONSchema call, mirroring emitSharedMessage's
// emit-once-reference-everywhere contract for the proto backend. Marks
// sharedMsg emitted before recursing (not after), matching
// emitSharedMessage's ordering, so a pathological self-referencing grouping
// cannot recurse forever.
func emitSharedDef(c *yang.Entry, sharedMsg string, shared map[string]string, defs map[string]any, emitted map[string]bool) {
	if emitted[sharedMsg] {
		return
	}
	emitted[sharedMsg] = true
	defs[sharedMsg] = jsonSchemaObject(c, shared, defs, emitted)
}

// MarshalSchemaDeterministic renders m as indented JSON with byte-stable
// output across runs. encoding/json already sorts map[string]any keys
// alphabetically on every Marshal/MarshalIndent call (see the encoding/json
// source: it collects a map's keys and sort.Strings's them before writing),
// so no separate ordered-map/custom-encoder step is needed to make golden
// comparisons deterministic despite Go's randomized map iteration order.
func MarshalSchemaDeterministic(m map[string]any) []byte {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		// m is always built from maps/slices/strings/bools/ints by this
		// package's own emitters, all of which are unconditionally
		// JSON-marshalable — a Marshal error here would mean a caller
		// injected a non-marshalable value into the map, which is a
		// programmer error, not a runtime condition to recover from.
		panic("MarshalSchemaDeterministic: " + err.Error())
	}
	return b
}
