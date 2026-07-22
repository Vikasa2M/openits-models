package main

import (
	"reflect"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
)

func TestJSONSchemaType(t *testing.T) {
	cases := []struct {
		name string
		typ  *yang.YangType
		want map[string]any
	}{
		{"string", &yang.YangType{Kind: yang.Ystring}, map[string]any{"type": "string"}},
		{"bool", &yang.YangType{Kind: yang.Ybool}, map[string]any{"type": "boolean"}},
		{"int32", &yang.YangType{Kind: yang.Yint32}, map[string]any{"type": "integer"}},
		{"int64-is-string", &yang.YangType{Kind: yang.Yint64}, map[string]any{"type": "string"}},
		{"uint64-is-string", &yang.YangType{Kind: yang.Yuint64}, map[string]any{"type": "string"}},
		{"decimal64-is-string", &yang.YangType{Kind: yang.Ydecimal64}, map[string]any{"type": "string"}},
		{"identityref-is-string", &yang.YangType{Kind: yang.Yidentityref}, map[string]any{"type": "string"}},
		{"date-and-time", &yang.YangType{Kind: yang.Ystring, Name: "date-and-time"}, map[string]any{"type": "string", "format": "date-time"}},
		{
			"string-pattern-anchored",
			&yang.YangType{Kind: yang.Ystring, Pattern: []string{"[a-zA-Z0-9_-]+"}},
			map[string]any{"type": "string", "pattern": "^([a-zA-Z0-9_-]+)$"},
		},
		{
			"string-multi-pattern-anchored-allof",
			&yang.YangType{Kind: yang.Ystring, Pattern: []string{"[a-zA-Z0-9_-]+", "[a-z].*"}},
			map[string]any{"type": "string", "allOf": []map[string]any{
				{"pattern": "^([a-zA-Z0-9_-]+)$"},
				{"pattern": "^([a-z].*)$"},
			}},
		},
	}
	for _, c := range cases {
		got := JSONSchemaType(c.typ)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
