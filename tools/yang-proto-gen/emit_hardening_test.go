package main

import (
	"strings"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
)

// parseFixtureSrc parses src as an in-memory YANG module (no testdata file
// needed) and returns its top-level module Entry. Modeled on the
// testdata-file loading helpers in nested_collision_test.go /
// leafref_choice_test.go / configstate_test.go, but sourced from a literal
// string via yang.Modules.Parse so these hardening tests don't need any new
// fixture files.
func parseFixtureSrc(t *testing.T, src, name string) *yang.Entry {
	t.Helper()
	ms := yang.NewModules()
	if err := ms.Parse(src, name+".yang"); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process %s: %v", name, errs)
	}
	mod, errs := ms.GetModule(name)
	if len(errs) > 0 {
		t.Fatalf("getmodule %s: %v", name, errs)
	}
	return mod
}

// TestCollisionSet_SeesThroughChoiceCase guards collisionSetForRoots's walk
// against the gap where a nested-message name that only occurs inside a
// choice/case is invisible to the collision pre-pass. `device/alpha/sensor`
// and `device/source(choice)/beta(case)/sensor` are both plain containers
// named "sensor" — a real collision, since both would emit as a bare
// "message Sensor {" today — but before the walk descends through
// choice/case, only alpha's is ever counted (the walk stops at the choice
// node: it is neither IsList() nor IsContainer()), so the collision is
// missed and neither occurrence gets parent-qualified.
func TestCollisionSet_SeesThroughChoiceCase(t *testing.T) {
	const src = `
module choice-collision-fixture {
  yang-version 1.1;
  namespace "urn:openits:test:choicecollision";
  prefix ccf;

  container device {
    container alpha {
      container sensor {
        leaf a { type uint32; }
      }
    }
    choice source {
      case beta {
        container sensor {
          leaf b { type uint32; }
        }
      }
    }
  }
}
`
	mod := parseFixtureSrc(t, src, "choice-collision-fixture")
	device := mod.Dir["device"]
	if device == nil {
		t.Fatalf("fixture setup broken: device container not found")
	}
	cs := collisionSet(device)
	if !cs["Sensor"] {
		t.Errorf("collisionSet(device) = %v, want \"Sensor\" present — the choice/case-nested "+
			"sensor container must be counted alongside alpha's, not skipped", cs)
	}
}

// TestResolveLeafref_CyclicPathTerminates guards resolveLeafref/leafFieldType
// against a cyclic leafref chain. `a`'s leafref path points at `b`, and `b`'s
// points right back at `a` — goyang's Modules.Process does not itself reject
// this (RFC 7950 forbids it, and yanglint's upstream validator rejects it
// today, but that check is not performed at this layer), so nothing stops
// this generator's own resolveLeafref from receiving it.
//
// resolveLeafrefDepth is the depth-guarded core resolveLeafref delegates to
// (see emit.go): calling it directly here at depth 0, capped at
// maxLeafrefDepth hops, proves the cap itself actually terminates a cycle
// rather than merely trusting resolveLeafref's wrapper not to regress. Before
// the cap exists, resolveLeafrefDepth is not declared at all, so this test
// file fails TO COMPILE — the acceptable RED form here, since actually
// exercising the pre-cap code path (unbounded a<->b mutual recursion via
// resolveLeafref/leafFieldType) would crash the whole test binary with a
// stack overflow (a fatal, unrecoverable Go runtime error) instead of
// failing this one test.
func TestResolveLeafref_CyclicPathTerminates(t *testing.T) {
	const src = `
module cyclic-leafref-fixture {
  yang-version 1.1;
  namespace "urn:openits:test:cyclicleafref";
  prefix clf;

  container root {
    leaf a {
      type leafref {
        path "../b";
      }
    }
    leaf b {
      type leafref {
        path "../a";
      }
    }
  }
}
`
	mod := parseFixtureSrc(t, src, "cyclic-leafref-fixture")
	root := mod.Dir["root"]
	if root == nil {
		t.Fatalf("fixture setup broken: root container not found")
	}
	a, b := root.Dir["a"], root.Dir["b"]
	if a == nil || b == nil {
		t.Fatalf("fixture setup broken: a=%v b=%v", a, b)
	}

	if got := resolveLeafrefDepth(a, 0); got != nil {
		t.Errorf("resolveLeafrefDepth(a, 0) on cyclic a<->b chain = %v, want nil (depth cap should terminate the cycle)", got)
	}

	var nested strings.Builder
	var pf ProtoFile
	if got := leafFieldType(a, &nested, &pf); got != "string" {
		t.Errorf("leafFieldType(a) on cyclic leafref chain = %q, want %q (historical unresolvable fallback)", got, "string")
	}
}

// TestSharedGroupingSkip_ConfigStateContainers guards EmitMessage against
// routing a config/state container through the shared-message shortcut just
// because it happens to be sourced from a registered (>=2-site,
// container-shaped) shared grouping. `shared-settings` is `uses`-d at two
// sites (device-a, device-b) with a container-shaped body named "config", so
// SharedGroupings registers it as shared. Without the config/state guard,
// EmitMessage's container branch takes the groupingOf/shared shortcut for
// EITHER site's "config" child, emitting ONE top-level "SharedSettings"
// message referenced by both — collapsing what must be two independent
// per-service config surfaces (DeviceAConfig, DeviceBConfig) into one shared
// shape, which is wrong even though nothing in the current corpus actually
// triggers it yet.
func TestSharedGroupingSkip_ConfigStateContainers(t *testing.T) {
	const src = `
module shared-config-fixture {
  yang-version 1.1;
  namespace "urn:openits:test:sharedconfig";
  prefix scf;

  grouping shared-settings {
    container config {
      leaf x { type string; }
    }
  }

  container root {
    container device-a {
      uses shared-settings;
    }
    container device-b {
      uses shared-settings;
    }
  }
}
`
	mod := parseFixtureSrc(t, src, "shared-config-fixture")
	shared := SharedGroupings([]*yang.Entry{mod})
	if _, ok := shared["shared-settings"]; !ok {
		t.Fatalf("fixture setup broken: shared-settings not detected as shared, got: %v", shared)
	}

	root := mod.Dir["root"]
	if root == nil {
		t.Fatalf("fixture setup broken: root container not found")
	}
	lock := &FieldLock{Messages: map[string]map[string]int{}}
	pf := &ProtoFile{ClaimedNames: map[string]bool{}}
	pf.Collisions = collisionSet(root)
	EmitMessage(root, "Root", lock, shared, pf)
	got := pf.Body.String()

	if strings.Contains(got, "message SharedSettings {") {
		t.Errorf("config container sourced from a shared grouping was routed through the "+
			"shared-message shortcut instead of per-parent config/state nesting:\n%s", got)
	}
	if !strings.Contains(got, "message DeviceAConfig {") || !strings.Contains(got, "message DeviceBConfig {") {
		t.Errorf("expected per-parent DeviceAConfig/DeviceBConfig nested messages, got:\n%s", got)
	}
}
