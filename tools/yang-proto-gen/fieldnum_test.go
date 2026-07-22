package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFieldLock_stabilityAndReserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "field-numbers.yaml")

	l, err := LoadFieldLock(path) // absent -> empty
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	reserved := map[string]int{"kind": reservedKind, "source": reservedSource}
	got := l.Assign("FaultRaised", []string{"kind", "source_device_id", "fault_id", "source"}, reserved)
	if got["kind"] != 99 {
		t.Errorf("kind must be reserved tag 99, got %d", got["kind"])
	}
	if got["source"] != 100 {
		t.Errorf("source must be reserved tag 100, got %d", got["source"])
	}
	if got["source_device_id"] != 1 || got["fault_id"] != 2 {
		t.Errorf("data fields must take 1..N in order: got sdi=%d fid=%d", got["source_device_id"], got["fault_id"])
	}
	if err := l.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Reload and add a NEW field: existing numbers must not move; new appends at 3.
	l2, _ := LoadFieldLock(path)
	got2 := l2.Assign("FaultRaised", []string{"kind", "source_device_id", "fault_id", "severity", "source"}, reserved)
	if got2["source_device_id"] != 1 || got2["fault_id"] != 2 {
		t.Errorf("existing tags moved: sdi=%d fid=%d", got2["source_device_id"], got2["fault_id"])
	}
	if got2["severity"] != 3 {
		t.Errorf("new field must append at 3, got %d", got2["severity"])
	}
	if got2["kind"] != 99 || got2["source"] != 100 {
		t.Errorf("reserved tags changed: kind=%d source=%d", got2["kind"], got2["source"])
	}
	_ = os.Remove(path)
}

func TestFieldLock_persistsAndRetiresTags(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "field-numbers.yaml")

	l, err := LoadFieldLock(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	reserved := map[string]int{"kind": reservedKind, "source": reservedSource}
	// First assignment: kind=99, a=1, b=2, source=100.
	first := l.Assign("M", []string{"kind", "a", "b", "source"}, reserved)
	if first["a"] != 1 || first["b"] != 2 {
		t.Fatalf("initial: a=%d b=%d, want a=1 b=2", first["a"], first["b"])
	}
	if err := l.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Reload, then assign with "b" DROPPED and "c" ADDED.
	// - Persistence: "a" must stay 1.
	// - Retirement: "b"'s tag 2 must NOT be reused; "c" must get 3.
	// A naive (non-persisting) implementation would recompute c=2 here, so
	// c==3 is the assertion that proves persistence + retirement.
	l2, err := LoadFieldLock(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got := l2.Assign("M", []string{"kind", "a", "c", "source"}, reserved)
	if got["a"] != 1 {
		t.Errorf("persistence broken: a=%d, want 1", got["a"])
	}
	if got["c"] != 3 {
		t.Errorf("retirement broken: c=%d, want 3 (b's tag 2 must not be reused)", got["c"])
	}
	if got["kind"] != 99 || got["source"] != 100 {
		t.Errorf("reserved tags changed: kind=%d source=%d", got["kind"], got["source"])
	}
}

// TestFieldLock_unreservedKindSourceGetNormalTags guards the type-aware
// reserve contract fixed at the Assign level: Assign itself has no idea
// whether a field named "kind"/"source" is really the identityref
// kind/WireSource-reference leaf — that's the caller's (EmitMessage's) job
// to decide and report via the reserved map (see reservedFieldTags in
// emit.go). When the caller does NOT mark a "kind" or "source" field as
// reserved (e.g. because it's a plain scalar leaf, not an identityref/
// WireSource-ref), Assign must hand it an ordinary sequential tag exactly
// like any other field — never silently fall back to name-based 99/100.
func TestFieldLock_unreservedKindSourceGetNormalTags(t *testing.T) {
	l := &FieldLock{Messages: map[string]map[string]int{}}
	got := l.Assign("ScalarEvent", []string{"kind", "source", "detail"}, nil)
	if got["kind"] == 99 {
		t.Errorf("unreserved `kind` must not get tag 99, got %d", got["kind"])
	}
	if got["source"] == 100 {
		t.Errorf("unreserved `source` must not get tag 100, got %d", got["source"])
	}
	if got["kind"] != 1 || got["source"] != 2 || got["detail"] != 3 {
		t.Errorf("unreserved fields must take sequential tags 1..N in order: kind=%d source=%d detail=%d",
			got["kind"], got["source"], got["detail"])
	}

	// A mix in the same message: only the field the caller actually marks
	// reserved gets the reserved tag; the other keeps a normal one.
	l2 := &FieldLock{Messages: map[string]map[string]int{}}
	got2 := l2.Assign("MixedEvent", []string{"kind", "source", "detail"}, map[string]int{"kind": reservedKind})
	if got2["kind"] != 99 {
		t.Errorf("caller-marked `kind` must get reserved tag 99, got %d", got2["kind"])
	}
	if got2["source"] == 100 {
		t.Errorf("unmarked `source` must not get tag 100, got %d", got2["source"])
	}
}
