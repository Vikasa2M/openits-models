package main

import (
	"path/filepath"
	"testing"
)

func TestSharedGroupings(t *testing.T) {
	// Loads the real repo YANG (../../yang from the tool dir).
	yangDir := filepath.Join("..", "..", "yang")
	_, mods, err := LoadModules(yangDir)
	if err != nil {
		t.Fatalf("LoadModules: %v", err)
	}

	shared := SharedGroupings(mods)

	msg, ok := shared["wire-source"]
	if !ok {
		t.Fatalf("expected wire-source to be detected as shared (used across multiple notification modules), got: %v", shared)
	}
	if msg != "WireSource" {
		t.Errorf("expected wire-source to map to message name %q, got %q", "WireSource", msg)
	}
}

// TestSharedGroupings_onlyContainerGroupings guards the two bugs fixed in
// SharedGroupings: (1) it used to count once per grouping-body *member*
// instead of once per `uses` *site*, so a flat grouping's leaves inflated
// its count far past its real number of usage sites; (2) even with correct
// counting, a flat grouping used at >=2 sites (geo-location, phase-timing)
// must still never be reported as shared — RFC 7950 `uses` splices a flat
// grouping's leaves directly into the parent tree, so the proto must
// inline them too. Only wire-source (container-shaped: `grouping
// wire-source { container source {...} }`) qualifies.
func TestSharedGroupings_onlyContainerGroupings(t *testing.T) {
	yangDir := filepath.Join("..", "..", "yang")
	_, mods, err := LoadModules(yangDir)
	if err != nil {
		t.Fatalf("LoadModules: %v", err)
	}

	shared := SharedGroupings(mods)

	if msg, ok := shared["wire-source"]; !ok || msg != "WireSource" {
		t.Errorf("expected wire-source -> WireSource in shared groupings, got: %v", shared)
	}

	// geo-location (4 sites) and phase-timing (2 sites) are flat groupings
	// used at >=2 sites each — exactly the case the container-shape filter
	// exists for. system-info and comm-link-state are flat groupings used
	// at exactly 1 real site each; the old per-member counting bug
	// inflated their count to ~9 (one per leaf) and wrongly reported them
	// as shared too.
	for _, flat := range []string{"geo-location", "phase-timing", "system-info", "comm-link-state"} {
		if msg, ok := shared[flat]; ok {
			t.Errorf("flat grouping %q must not be reported as shared (inlining is correct), got mapped to %q; full map: %v", flat, msg, shared)
		}
	}
}
