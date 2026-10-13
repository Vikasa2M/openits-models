package main

import (
	"sort"
	"testing"
)

func violationsFor(t *testing.T, rule string) []string {
	t.Helper()
	mods, err := loadModules("testdata/schema")
	if err != nil {
		t.Fatalf("loadModules: %v", err)
	}
	var got []string
	for _, v := range checkSchema(mods) {
		if v.Rule == rule {
			got = append(got, v.Path)
		}
	}
	sort.Strings(got)
	return got
}

func assertPaths(t *testing.T, rule string, want ...string) {
	t.Helper()
	got := violationsFor(t, rule)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("%s: got %v, want %v", rule, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s: got %v, want %v", rule, got, want)
		}
	}
}

const root = "/openits-test-conventions"

func TestBeforeAfter(t *testing.T) {
	assertPaths(t, "before-after",
		root+"/bad-change/prior",
		root+"/bad-change/previous-rate",
	)
}

func TestUnknownZero(t *testing.T) {
	assertPaths(t, "unknown-zero",
		root+"/dev/state/no-unknown",
		root+"/dev/state/unknown-not-zero",
		"typedef shared-status",
	)
}

func TestUnknownZero_TypedefReportedOncePerModule(t *testing.T) {
	n := 0
	for _, p := range violationsFor(t, "unknown-zero") {
		if p == "typedef shared-status" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("typedef shared-status reported %d times, want 1", n)
	}
}

func TestUnits(t *testing.T) {
	assertPaths(t, "units",
		root+"/dev/state/delay-seconds",
		root+"/dev/state/timeout",
		root+"/dev/state/height-m",
		root+"/dev/state/bad-match-ms",
		root+"/dev/state/ratio-percent",
	)
}

func TestCounters(t *testing.T) {
	assertPaths(t, "counters", root+"/dev/state/crc-errors", root+"/dev/state/rx-total")
}

// TestKindBase also covers shared-fault, whose kind is typed to an
// identity openits-types defines directly on device-event-kind: that is a
// cross-service base, not a service root, so it is not reported.
func TestKindBase(t *testing.T) {
	assertPaths(t, "kind-base", root+"/bad-change/kind")
}

func TestUnknownZero_TypedefReportedAgainstDefiningModule(t *testing.T) {
	mods, err := loadModules("testdata/schema")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, v := range checkSchema(mods) {
		if v.Rule == "unknown-zero" && v.Path == "typedef shared-status" {
			n++
			if v.Module != "openits-test-conventions" {
				t.Errorf("typedef violation Module = %q, want the defining module openits-test-conventions", v.Module)
			}
		}
	}
	if n != 1 {
		t.Fatalf("typedef shared-status reported %d times across modules, want 1", n)
	}
}
