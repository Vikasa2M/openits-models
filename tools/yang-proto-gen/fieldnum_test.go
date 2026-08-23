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

// A dropped field's tag stays in the lock so it is never reused; Retired is
// what surfaces it to the emitter so the .proto can say `reserved`. Without
// that, the lock's guarantee is invisible to protoc, to `buf breaking`, and
// to anyone reading the generated proto.
func TestFieldLock_retiredSurfacesDroppedFields(t *testing.T) {
	l := &FieldLock{Messages: map[string]map[string]int{}}
	first := l.Assign("M", []string{"a", "b", "c"}, nil)

	// Nothing dropped yet.
	if names, tags := l.Retired("M", first); len(names) != 0 || len(tags) != 0 {
		t.Fatalf("no fields dropped, want no retired; got names=%v tags=%v", names, tags)
	}

	// Drop "a" and "b"; keep "c".
	live := l.Assign("M", []string{"c"}, nil)
	names, tags := l.Retired("M", live)
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("retired names = %v, want [a b] ordered by tag", names)
	}
	if len(tags) != 2 || tags[0] != first["a"] || tags[1] != first["b"] {
		t.Errorf("retired tags = %v, want [%d %d]", tags, first["a"], first["b"])
	}
}

// A field that comes back reclaims its original tag rather than staying
// retired — so reserving its NAME is safe. If it did not, reserving the name
// on removal would permanently block a leaf from ever returning.
func TestFieldLock_returningFieldReclaimsTagAndStopsBeingRetired(t *testing.T) {
	l := &FieldLock{Messages: map[string]map[string]int{}}
	first := l.Assign("M", []string{"a", "b"}, nil)
	wantB := first["b"]

	// Drop "b" — now retired.
	live := l.Assign("M", []string{"a"}, nil)
	if names, _ := l.Retired("M", live); len(names) != 1 || names[0] != "b" {
		t.Fatalf("after drop, retired = %v, want [b]", names)
	}

	// Bring "b" back alongside a genuinely new field.
	live = l.Assign("M", []string{"a", "b", "z"}, nil)
	if live["b"] != wantB {
		t.Errorf("returning field b got tag %d, want its original %d", live["b"], wantB)
	}
	if names, _ := l.Retired("M", live); len(names) != 0 {
		t.Errorf("b returned, so nothing should be retired; got %v", names)
	}
	if live["z"] == wantB {
		t.Errorf("new field z stole b's tag %d", wantB)
	}
}

func TestEmitReserved_rendersNumbersAndNames(t *testing.T) {
	if got := emitReserved(nil, nil); got != "" {
		t.Errorf("no retired fields should emit nothing, got %q", got)
	}
	got := emitReserved([]string{"capacity", "old_name"}, []int{3, 7})
	want := "  reserved 3, 7;\n  reserved \"capacity\", \"old_name\";\n"
	if got != want {
		t.Errorf("emitReserved =\n%q\nwant\n%q", got, want)
	}
}
