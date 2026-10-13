package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompare_NewAndStale(t *testing.T) {
	vs := []Violation{
		{Rule: "units", Module: "openits-dms", Path: "/a"},
		{Rule: "units", Module: "openits-dms", Path: "/b"},
	}
	exc := []Exception{
		{Rule: "units", Module: "openits-dms", Path: "/a", Cut: "dms"},
		{Rule: "units", Module: "openits-dms", Path: "/gone", Cut: "dms"},
	}
	fresh, stale := compare(vs, exc)
	if len(fresh) != 1 || fresh[0].Path != "/b" {
		t.Fatalf("fresh = %v, want only /b", fresh)
	}
	if len(stale) != 1 || stale[0].Path != "/gone" {
		t.Fatalf("stale = %v, want only /gone", stale)
	}
}

func TestReadExceptions_PermanentNeedsReason(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.yaml")
	os.WriteFile(p, []byte("exceptions:\n  - {rule: units, module: m, path: /a, cut: permanent}\n"), 0o644)
	if _, err := readExceptions(p); err == nil || !strings.Contains(err.Error(), "reason") {
		t.Fatalf("err = %v, want a missing-reason error", err)
	}
}

func TestWriteExceptions_PreservesCutAndReason(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.yaml")
	os.WriteFile(p, []byte("exceptions:\n  - {rule: units, module: openits-dms, path: /a, cut: permanent, reason: NTCIP 1203 name}\n"), 0o644)
	vs := []Violation{
		{Rule: "units", Module: "openits-dms", Path: "/a"},
		{Rule: "units", Module: "openits-signal-control-types", Path: "/b"},
	}
	if err := writeExceptions(p, vs); err != nil {
		t.Fatal(err)
	}
	exc, err := readExceptions(p)
	if err != nil {
		t.Fatal(err)
	}
	if exc[0].Cut != "permanent" || exc[0].Reason != "NTCIP 1203 name" {
		t.Fatalf("existing entry lost its cut/reason: %+v", exc[0])
	}
	if exc[1].Cut != "signal-control" {
		t.Fatalf("new entry cut = %q, want signal-control", exc[1].Cut)
	}
}

func TestCutFor(t *testing.T) {
	cases := map[string]string{
		"openits-types":                                 "foundation",
		"openits-common-mode-events":                    "foundation",
		"openits-cabinet-power":                         "foundation",
		"openits-signal-control-types":                  "signal-control",
		"openits-vehicle-detection":                     "signal-control",
		"openits-v2x-radio-types":                       "rsu",
		"openits-scms":                                  "rsu",
		"openits-dms-events":                            "dms",
		"openits-vendor-econolite-signal-control-types": "vendor-examples",
	}
	for mod, want := range cases {
		if got := cutFor(mod); got != want {
			t.Errorf("cutFor(%q) = %q, want %q", mod, got, want)
		}
	}
}
