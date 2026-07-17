package main

import (
	"strings"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
)

// loadFixtureEntry loads a single testdata module and returns its top-level
// container entry by name.
func loadFixtureEntry(t *testing.T, file, container string) *yang.Entry {
	t.Helper()
	ms := yang.NewModules()
	if err := ms.Read("testdata/" + file); err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process %s: %v", file, errs)
	}
	var mod *yang.Entry
	for _, m := range ms.Modules {
		mod = yang.ToEntry(m)
		break
	}
	c := mod.Dir[container]
	if c == nil {
		t.Fatalf("container %q not found in %s", container, file)
	}
	return c
}

func TestLeafrefKeyResolvesToTargetScalar(t *testing.T) {
	root := loadFixtureEntry(t, "configstate-fixture.yang", "signal-controller")
	lock := &FieldLock{Messages: map[string]map[string]int{}}
	pf := &ProtoFile{}
	EmitMessage(root, "SignalController", lock, nil, pf)
	got := pf.Body.String()
	// The phase-id list key is `type leafref { path "../config/phase-id" }`
	// whose target is `leaf phase-id { type uint32; }`. It must render as
	// uint32, not the string fallback.
	if !strings.Contains(got, "uint32 phase_id = 1;") {
		t.Errorf("leafref key not resolved to uint32; got:\n%s", got)
	}
	if strings.Contains(got, "string phase_id") {
		t.Errorf("leafref key wrongly rendered as string; got:\n%s", got)
	}
}

func TestConfigStateNamesAreParentQualified(t *testing.T) {
	root := loadFixtureEntry(t, "configstate-fixture.yang", "signal-controller")
	lock := &FieldLock{Messages: map[string]map[string]int{}}
	pf := &ProtoFile{}
	EmitMessage(root, "SignalController", lock, nil, pf)
	got := pf.Body.String()
	for _, want := range []string{
		"message SignalController {",
		"message SignalControllerConfig {",
		"message SignalControllerState {",
		"message Phase {",
		"message PhaseConfig {",
		"message PhaseState {",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	// No bare Config/State messages anywhere.
	for _, bad := range []string{"message Config {", "message State {"} {
		if strings.Contains(got, bad) {
			t.Errorf("found bare %q — collision not qualified:\n%s", bad, got)
		}
	}
}

// TestConfigStateAllowlistExact guards the opt-in invariant: the allowlist
// contains exactly the modules that have been deliberately restructured to
// the config/state idiom, so no not-yet-converted module silently starts
// emitting config/state. Add a module's route here only alongside actually
// restructuring its YANG.
func TestConfigStateAllowlistExact(t *testing.T) {
	want := map[string]serviceRoute{
		"openits-ess":             {pkg: "openits.ess.v1", file: "openits/ess/v1/state.proto"},
		"openits-ramp-metering":   {pkg: "openits.ramp_metering.v1", file: "openits/ramp_metering/v1/state.proto"},
		"openits-traffic-sensor":  {pkg: "openits.traffic_sensor.v1", file: "openits/traffic_sensor/v1/state.proto"},
		"openits-perception":      {pkg: "openits.perception.v1", file: "openits/perception/v1/state.proto"},
		"openits-dms":             {pkg: "openits.dms.v1", file: "openits/dms/v1/state.proto"},
		"openits-reversible-lane": {pkg: "openits.reversible_lane.v1", file: "openits/reversible_lane/v1/state.proto"},
		"openits-signal-control":  {pkg: "openits.signal_control.v1", file: "openits/signal_control/v1/state.proto"},
		"openits-rsu":             {pkg: "openits.rsu.v1", file: "openits/rsu/v1/state.proto"},
		"openits-cctv":            {pkg: "openits.cctv.v1", file: "openits/cctv/v1/state.proto"},
	}
	if len(configStateRoutes) != len(want) {
		t.Fatalf("configStateRoutes has %d entries, want %d: got %v", len(configStateRoutes), len(want), configStateRoutes)
	}
	for mod, wantRoute := range want {
		gotRoute, ok := configStateRoutes[mod]
		if !ok {
			t.Errorf("configStateRoutes missing expected route for %q", mod)
			continue
		}
		if gotRoute != wantRoute {
			t.Errorf("configStateRoutes[%q] = %+v, want %+v", mod, gotRoute, wantRoute)
		}
	}
}

// TestEmitModuleConfigState drives the config/state walk directly on the
// fixture module (bypassing the empty allowlist) to prove the walk emits the
// full tree with qualified names and resolved key types.
func TestEmitModuleConfigState(t *testing.T) {
	ms := yang.NewModules()
	if err := ms.Read("testdata/configstate-fixture.yang"); err != nil {
		t.Fatalf("read: %v", err)
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process: %v", errs)
	}
	var mod *yang.Entry
	for _, m := range ms.Modules {
		mod = yang.ToEntry(m)
		break
	}
	lock := &FieldLock{Messages: map[string]map[string]int{}}
	pf := &ProtoFile{}
	emitModuleConfigState(mod, pf, lock, nil)
	got := pf.Body.String()
	for _, want := range []string{
		"message SignalController {",
		"message PhaseConfig {",
		"message PhaseState {",
		"uint32 phase_id = 1;",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}
