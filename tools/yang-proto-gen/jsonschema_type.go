package main

import "github.com/openconfig/goyang/pkg/yang"

// JSONSchemaType returns the JSON Schema fragment for a leaf's type per RFC 7951.
// It handles YANG types and constraints, returning the appropriate JSON Schema map.
func JSONSchemaType(t *yang.YangType) map[string]any {
	// date-and-time is modeled as a string typedef; detect by typedef name.
	if t.Name == "date-and-time" {
		return map[string]any{"type": "string", "format": "date-time"}
	}

	switch t.Kind {
	case yang.Yint64, yang.Yuint64, yang.Ydecimal64, yang.Yidentityref:
		// RFC 7951: int64/uint64/decimal64/identityref map to strings in JSON
		return map[string]any{"type": "string"}
	case yang.Ybool:
		return map[string]any{"type": "boolean"}
	case yang.Yint8, yang.Yint16, yang.Yint32, yang.Yuint8, yang.Yuint16, yang.Yuint32:
		result := map[string]any{"type": "integer"}
		// Add minimum/maximum from Range when present
		if len(t.Range) > 0 {
			minRange := t.Range[0]
			maxRange := t.Range[len(t.Range)-1]
			minVal, _ := minRange.Min.Int()
			maxVal, _ := maxRange.Max.Int()
			result["minimum"] = minVal
			result["maximum"] = maxVal
		}
		return result
	case yang.Yenum:
		result := map[string]any{"type": "string"}
		if t.Enum != nil {
			result["enum"] = t.Enum.Names()
		}
		return result
	case yang.Ystring:
		result := map[string]any{"type": "string"}
		// Add pattern/minLength/maxLength from YANG constraints when present.
		//
		// YANG patterns are implicitly anchored: per RFC 7950 §9.4.5, a
		// "pattern" restriction requires the *entire* string to match, as if
		// the expression were wrapped in ^(...)$. JSON Schema / ECMA-262
		// "pattern" is an unanchored substring search, so each pattern must
		// be explicitly anchored here to preserve YANG's full-match
		// semantics; otherwise the generated schema would accept values
		// (e.g. "bad value!" against "[a-zA-Z0-9_-]+") that YANG rejects.
		//
		// When a YANG type has multiple pattern statements, a value must
		// match ALL of them (YANG conjunction), so anchor each individually
		// and combine them with "allOf" rather than merging into one regex.
		switch len(t.Pattern) {
		case 0:
			// no pattern constraint
		case 1:
			result["pattern"] = "^(" + t.Pattern[0] + ")$"
		default:
			allOf := make([]map[string]any, 0, len(t.Pattern))
			for _, pat := range t.Pattern {
				allOf = append(allOf, map[string]any{"pattern": "^(" + pat + ")$"})
			}
			result["allOf"] = allOf
		}
		if len(t.Length) > 0 {
			minLen := t.Length[0]
			maxLen := t.Length[len(t.Length)-1]
			minVal, _ := minLen.Min.Int()
			maxVal, _ := maxLen.Max.Int()
			if minVal > 0 {
				result["minLength"] = minVal
			}
			if maxVal > 0 {
				result["maxLength"] = maxVal
			}
		}
		return result
	default:
		return map[string]any{"type": "string"}
	}
}
