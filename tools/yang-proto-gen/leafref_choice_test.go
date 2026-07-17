package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
)

// TestResolveLeafref_insideChoiceCase guards leafref type resolution when the
// leafref leaf is declared inside a choice/case. A leafref path is written
// against the data tree, where choice/case are transparent (RFC 7950 §6.5,
// §9.9); goyang's Entry tree keeps choice/case as real nodes, so a naive
// schema-tree walk under-counts every "../" by the choice/case nesting depth
// and falls through to the string fallback. The channel/source choice in
// openits-signal-control (channelTable) is the first choice-of-leafref in the
// model set and surfaced this: `phase` (leafref -> phase-number:uint16) was
// emitted as proto `string` instead of `uint32`.
func TestResolveLeafref_insideChoiceCase(t *testing.T) {
	ms := yang.NewModules()
	if err := ms.Read(filepath.Join("testdata", "leafref-choice-fixture.yang")); err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process: %v", errs)
	}
	mod, errs := ms.GetModule("leafref-choice-fixture")
	if len(errs) > 0 {
		t.Fatalf("getmodule: %v", errs)
	}

	source := mod.Dir["root"].Dir["channel"].Dir["source"]
	if source == nil || !source.IsChoice() {
		t.Fatalf("fixture setup broken: channel/source is not a choice: %+v", source)
	}
	itemRef := source.Dir["by-item"].Dir["item-ref"]
	labelRef := source.Dir["by-label"].Dir["label-ref"]
	if itemRef == nil || labelRef == nil {
		t.Fatalf("fixture setup broken: case leaves not found (item-ref=%v label-ref=%v)", itemRef, labelRef)
	}

	var nested strings.Builder
	var pf ProtoFile
	if got := leafFieldType(itemRef, &nested, &pf); got != "uint32" {
		t.Errorf("item-ref (leafref -> uint16 inside choice/case): got %q, want uint32", got)
	}
	if got := leafFieldType(labelRef, &nested, &pf); got != "string" {
		t.Errorf("label-ref (leafref -> string inside choice/case): got %q, want string", got)
	}
}
