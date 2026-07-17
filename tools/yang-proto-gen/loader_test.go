package main

import (
	"path/filepath"
	"testing"
)

func TestLoadModules_findsNotifications(t *testing.T) {
	// Loads the real repo YANG (../../yang from the tool dir).
	yangDir := filepath.Join("..", "..", "yang")
	ms, mods, err := LoadModules(yangDir)
	if err != nil {
		t.Fatalf("LoadModules: %v", err)
	}
	if ms == nil || len(mods) == 0 {
		t.Fatal("expected modules, got none")
	}
	// openits-common-fault-events must be present and carry 2 notifications
	// (fault-raised, fault-cleared). controller-fault-event moved to
	// openits-signal-control-events since it is signal-control-specific,
	// not cross-service.
	var faultMod *entryModule
	for _, m := range mods {
		if m.Name == "openits-common-fault-events" {
			faultMod = &entryModule{m}
		}
	}
	if faultMod == nil {
		t.Fatal("openits-common-fault-events not loaded")
	}
	n := faultMod.notificationCount()
	if n != 2 {
		t.Fatalf("expected 2 notifications, got %d", n)
	}
}
