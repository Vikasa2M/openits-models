package main

import (
	"path/filepath"
	"testing"
)

func TestBuildCatalog_derivesServiceMatrix(t *testing.T) {
	ms, mods, err := LoadModules(filepath.Join("..", "..", "yang"))
	if err != nil {
		t.Fatal(err)
	}
	cat, err := BuildCatalog(ms, mods)
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, c := range cat {
		set[c.Type] = true
	}

	// Per-service notification: openits-dms-events:message-activation-failed -> openits.dms.message-activation-failed.v1
	// (sign-fault-raised, sign-fault-cleared, and sign-mode-changed were
	// deleted in favour of the common fault/mode
	// notifications asserted below.)
	if !set["openits.dms.message-activation-failed.v1"] {
		t.Error("missing per-service ce-type openits.dms.message-activation-failed.v1")
	}
	// Common fault-raised fans across EVERY service that has a fault sub-base under fault-event-kind.
	// Post P3c/P3c-2 that is all nine services (each has <svc>-fault-event-kind or a service-based fault-category).
	for _, svc := range []string{"dms", "ess", "rsu", "signal-control", "ramp-metering", "perception", "traffic-sensor", "reversible-lane"} {
		if !set["openits."+svc+".fault-raised.v1"] {
			t.Errorf("common fault-raised should fan to %s: missing openits.%s.fault-raised.v1", svc, svc)
		}
	}
	// mode-changed only fans to services with a mode sub-base under mode-event-kind (NOT every service).
	if !set["openits.dms.mode-changed.v1"] {
		t.Error("dms has a mode sub-base -> expected openits.dms.mode-changed.v1")
	}

	// Bug 1: signal-control's openits-signal-control-events module (cut 3b
	// folded this together from 11 sub-domain modules: phase, detector,
	// coordination, etc.) is a per-service notification-bearing module
	// like any other. Its notifications must route to the signal-control
	// service like any other per-service module.
	if !set["openits.signal-control.phase-state-change.v1"] {
		t.Error("missing openits.signal-control.phase-state-change.v1 (openits-signal-control-events routing)")
	}
	if !set["openits.signal-control.coordination-change.v1"] {
		t.Error("missing openits.signal-control.coordination-change.v1 (openits-signal-control-events routing)")
	}

	// Bug 2: the catalog must have exactly one CeType per Type. Several
	// signal-control ce-types are derivable from more than one source (a
	// deprecated gen-1 per-service notification and either its common-events
	// successor or its non-deprecated sub-domain successor module) — the
	// catalog must dedupe those down to a single entry each.
	uniq := map[string]bool{}
	for _, c := range cat {
		uniq[c.Type] = true
	}
	if len(uniq) != len(cat) {
		t.Errorf("catalog has duplicate Type entries: %d entries but only %d distinct Types", len(cat), len(uniq))
	}
}
