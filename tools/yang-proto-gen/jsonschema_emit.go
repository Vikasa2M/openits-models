package main

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

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
	applyRefines(e, properties, &required)

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

// applyRefines applies the `refine` substatements of e's own `uses`
// statements to the object schema walkChildren built. goyang merges a
// grouping's children into e.Dir but leaves each `refine` on the UsesStmt
// without touching the merged child entries (entry.go applies mandatory /
// min-elements only when they are declared directly on a node or carried by a
// `deviate`), so a notification that tightens a shared grouping's member —
// zone-occupancy-changed's `refine presence { mandatory true; }`, work-zone's
// `refine point { min-elements 1; }` — would otherwise publish a JSON schema
// looser than its YANG. Only refines that target a direct child (no '/' in
// the path) apply at this level; a deeper target sits below an optionality
// boundary this object does not own, exactly the rule walkChildren follows.
func applyRefines(e *yang.Entry, properties map[string]any, required *[]string) {
	for _, u := range usesOf(e) {
		if u == nil {
			continue
		}
		for _, r := range u.Refine {
			if r == nil || strings.Contains(r.Name, "/") {
				continue
			}
			prop, ok := properties[r.Name].(map[string]any)
			if !ok {
				continue
			}
			if r.Mandatory != nil {
				*required = without(*required, r.Name)
				if r.Mandatory.Name == "true" {
					*required = append(*required, r.Name)
				}
			}
			if r.MinElements != nil && prop["type"] == "array" {
				if n, err := strconv.Atoi(r.MinElements.Name); err == nil && n > 0 {
					prop["minItems"] = n
				}
			}
		}
	}
}

// usesOf returns the `uses` statements declared directly on e's own YANG
// node. Entry.Uses would carry the same list, but goyang populates it only
// under ParseOptions.StoreUses, which this generator (and any caller building
// its own yang.Modules) does not set; the AST node behind the entry always
// has them. Every node kind that can carry `uses` and that this emitter
// builds an object schema for is covered; a kind not listed simply has no
// refines to apply.
func usesOf(e *yang.Entry) []*yang.Uses {
	switch n := e.Node.(type) {
	case *yang.Notification:
		return n.Uses
	case *yang.Container:
		return n.Uses
	case *yang.List:
		return n.Uses
	case *yang.Case:
		return n.Uses
	case *yang.Grouping:
		return n.Uses
	case *yang.Augment:
		return n.Uses
	case *yang.Input:
		return n.Uses
	case *yang.Output:
		return n.Uses
	}
	return nil
}

// without returns ss with every occurrence of s removed.
func without(ss []string, s string) []string {
	out := ss[:0:0]
	for _, v := range ss {
		if v != s {
			out = append(out, v)
		}
	}
	return out
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
