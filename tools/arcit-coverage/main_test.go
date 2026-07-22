package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestScanYANG_RepoModules(t *testing.T) {
	anns, modules, err := scanYANG("../../yang")
	if err != nil {
		t.Fatalf("scanYANG: %v", err)
	}
	if len(modules) == 0 {
		t.Fatal("no modules loaded")
	}
	if len(anns) == 0 {
		t.Fatal("no arc-it-flow annotations found; expected ≥1 in openits-signal-control")
	}
	wantFlow := "TMC -> Roadway Signal Controller : signal control plan"
	var found bool
	for _, a := range anns {
		if a.Flow == wantFlow {
			found = true
			if !strings.HasPrefix(a.Path, "/openits-signal-control") {
				t.Errorf("flow %q annotated on unexpected path %q", wantFlow, a.Path)
			}
		}
	}
	if !found {
		t.Errorf("missing expected annotation %q", wantFlow)
	}
}

func TestWriteReport_ShapeAndCoverage(t *testing.T) {
	inv, err := loadInventory("arcit_inventory.json")
	if err != nil {
		t.Fatalf("loadInventory: %v", err)
	}
	anns, modules, err := scanYANG("../../yang")
	if err != nil {
		t.Fatalf("scanYANG: %v", err)
	}
	var buf bytes.Buffer
	if err := writeReport(&buf, inv, anns, modules); err != nil {
		t.Fatalf("writeReport: %v", err)
	}
	got := buf.String()
	for _, needle := range []string{
		"# ARC-IT Coverage Report — OpenITS",
		"## Service Package TI01 (Traffic Signal Control)",
		"Coverage: 7 / 8 flows",
		"| Flow | Annotated | Node |",
		"TMC -> Roadway Signal Controller : signal control plan",
	} {
		if !strings.Contains(got, needle) {
			t.Errorf("report missing %q\n-- got --\n%s", needle, got)
		}
	}
}
