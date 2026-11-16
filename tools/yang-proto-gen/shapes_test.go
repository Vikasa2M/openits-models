package main

import (
	"reflect"
	"strings"
	"testing"
)

// The index reads schema.shape's header back, so ShapeFile's root lines and
// shapeRoots must agree, and a body line that happens to look like a root
// line must not be read as one.
func TestShapeRoots_ReadsHeaderOnly(t *testing.T) {
	shape := `// Wire shape of m.
//
// root openits.x.v1.A sha256:aa
// root openits.x.v1.B sha256:bb

message openits.x.v1.A {
  // root openits.x.v1.C sha256:cc
}
`
	got := shapeRoots([]byte(shape))
	want := map[string]string{"openits.x.v1.A": "sha256:aa", "openits.x.v1.B": "sha256:bb"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shapeRoots = %v, want %v", got, want)
	}
	if shapeRoots([]byte("message a {\n}\n")) != nil {
		t.Fatal("shape without root lines should yield nil")
	}
}

// Every live notification and every config/state root of an allowlisted
// module is a shape root, under the package Generate routes it to.
func TestModuleRootNames_MatchesGenerateRouting(t *testing.T) {
	_, mods, err := LoadModules("../../yang")
	if err != nil {
		t.Fatal(err)
	}
	names := moduleRootNames(mods)
	for _, want := range []string{
		"openits.zone_occupancy.v1.ZoneOccupancyChanged",
		"openits.common.v1.FaultRaised",
	} {
		found := false
		for _, ns := range names {
			for _, n := range ns {
				found = found || string(n) == want
			}
		}
		if !found {
			t.Errorf("no module lists root %s", want)
		}
	}
	for m, ns := range names {
		if strings.HasSuffix(m, "-types") {
			t.Errorf("types module %s should have no roots, got %v", m, ns)
		}
	}
	if len(names["openits-dms"]) == 0 {
		t.Error("openits-dms is on configStateRoutes and should have state roots")
	}
}
