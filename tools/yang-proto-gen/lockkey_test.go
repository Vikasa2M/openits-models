package main

import (
	"path/filepath"
	"testing"
)

// TestLockKeyIsServicePackageScoped is the regression guard for the
// field-number-lock conflation defect: two different service modules that each
// declare a same-named container (here control/config, which both render as a
// proto message named "ControlConfig") must NOT share a single field-number
// lock bucket. Keyed only by the bare message name, both services' fields land
// in one "ControlConfig" bucket and get co-mingled tags — a wire-safety hazard
// the moment a same-named leaf is added to both. The lock must instead be keyed
// by the service's proto package, so each service's ControlConfig gets its own
// independent tag space.
func TestLockKeyIsServicePackageScoped(t *testing.T) {
	controlA := loadFixtureEntry(t, "svc-collision-a.yang", "control")
	controlB := loadFixtureEntry(t, "svc-collision-b.yang", "control")

	// One shared lock, exactly as Generate uses across every service.
	lock := &FieldLock{Messages: map[string]map[string]int{}}

	pfA := &ProtoFile{LockPrefix: "openits.ramp_metering.v1"}
	EmitMessage(controlA, "Control", lock, nil, pfA)
	pfB := &ProtoFile{LockPrefix: "openits.reversible_lane.v1"}
	EmitMessage(controlB, "Control", lock, nil, pfB)

	// The bare, unqualified bucket must not exist: that is the conflated bucket.
	if _, ok := lock.Messages["ControlConfig"]; ok {
		t.Errorf("bare 'ControlConfig' lock bucket present — two services conflated into one tag space")
	}

	keyA := "openits.ramp_metering.v1.ControlConfig"
	keyB := "openits.reversible_lane.v1.ControlConfig"
	bucketA, okA := lock.Messages[keyA]
	bucketB, okB := lock.Messages[keyB]
	if !okA || !okB {
		t.Fatalf("expected package-scoped buckets %q and %q; got keys %v", keyA, keyB, keysOf(lock.Messages))
	}

	// Each bucket holds ONLY its own service's fields.
	if _, leaked := bucketA["b_only"]; leaked {
		t.Errorf("%s leaked svc-b field b_only: %v", keyA, bucketA)
	}
	if _, leaked := bucketB["a_only"]; leaked {
		t.Errorf("%s leaked svc-a field a_only: %v", keyB, bucketB)
	}

	// Non-conflation is observable in the tags: svc-b's first field takes tag 1
	// in its own namespace. In the conflated (bare-key) world it would append
	// after svc-a's two fields and land at tag 3.
	if bucketB["b_only"] != 1 {
		t.Errorf("b_only = %d, want 1 (svc-b's ControlConfig must not be pushed past svc-a's fields): %v", bucketB["b_only"], bucketB)
	}
}

// keysOf returns the sorted-order-agnostic key set of a lock's message map, for
// diagnostics.
func keysOf(m map[string]map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestGenerate_lockKeysArePackageQualified drives the real corpus and asserts
// the same invariant end-to-end: no bare cross-service message bucket survives,
// and the three services that each declare a control/config get three distinct,
// package-qualified ControlConfig buckets with disjoint fields.
func TestGenerate_lockKeysArePackageQualified(t *testing.T) {
	out := t.TempDir()
	lockPath := filepath.Join(out, "field-numbers.yaml")
	if err := Generate(filepath.Join("..", "..", "yang"), out, lockPath); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	l, err := LoadFieldLock(lockPath)
	if err != nil {
		t.Fatalf("load lock: %v", err)
	}

	// A bare "ControlConfig" bucket is the conflation symptom: three services
	// (dms, ramp_metering, reversible_lane) each declare control/config.
	if _, ok := l.Messages["ControlConfig"]; ok {
		t.Errorf("bare 'ControlConfig' bucket present — services still conflated")
	}

	rm := l.Messages["openits.ramp_metering.v1.ControlConfig"]
	rl := l.Messages["openits.reversible_lane.v1.ControlConfig"]
	if rm == nil || rl == nil {
		t.Fatalf("expected package-qualified ControlConfig buckets for ramp_metering and reversible_lane")
	}
	// reversible_lane's target_state must not appear in ramp_metering's bucket,
	// and ramp_metering's active_plan_id must not appear in reversible_lane's.
	if _, leaked := rm["target_state"]; leaked {
		t.Errorf("ramp_metering ControlConfig leaked reversible_lane field target_state: %v", rm)
	}
	if _, leaked := rl["active_plan_id"]; leaked {
		t.Errorf("reversible_lane ControlConfig leaked ramp_metering field active_plan_id: %v", rl)
	}
}
