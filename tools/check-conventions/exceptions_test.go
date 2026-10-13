package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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
		"openits-types":                                     "foundation",
		"openits-common-mode-events":                        "foundation",
		"openits-cabinet-power":                             "foundation",
		"openits-signal-control-types":                      "signal-control",
		"openits-vehicle-detection":                         "rsu",
		"openits-v2x-radio-types":                           "rsu",
		"openits-scms":                                      "rsu",
		"openits-dms-events":                                "dms",
		"openits-traffic-sensor-types":                      "sensing",
		"openits-perception":                                "sensing",
		"openits-zone-occupancy-events":                     "sensing",
		"openits-vendor-econolite-signal-control-types":     "signal-control",
		"openits-vendor-trafficvision-perception-types":     "sensing",
		"openits-vendor-trafficvision-traffic-sensor-types": "sensing",
		"openits-brand-new-service":                         "",
	}
	for mod, want := range cases {
		if got := cutFor(mod); got != want {
			t.Errorf("cutFor(%q) = %q, want %q", mod, got, want)
		}
	}
}

func TestWriteExceptions_UnmappedModuleFails(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.yaml")
	err := writeExceptions(p, []Violation{{Rule: "units", Module: "openits-brand-new-service", Path: "/a"}})
	if err == nil || !strings.Contains(err.Error(), "openits-brand-new-service") {
		t.Fatalf("err = %v, want an error naming the module", err)
	}
}

func TestReadExceptions_Duplicate(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.yaml")
	os.WriteFile(p, []byte("exceptions:\n  - {rule: units, module: m, path: /a, cut: dms}\n  - {rule: units, module: m, path: /a, cut: dms}\n"), 0o644)
	if _, err := readExceptions(p); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("err = %v, want a duplicate error", err)
	}
}

func TestCheckGrowth(t *testing.T) {
	base := []Exception{
		{Rule: "units", Module: "openits-dms", Path: "/a", Cut: "dms"},
		{Rule: "units", Module: "openits-dms", Path: "/b", Cut: "dms"},
	}
	shrunk := base[:1]
	if err := checkGrowth(shrunk, base); err != nil {
		t.Fatalf("shrinking: err = %v, want nil", err)
	}
	if err := checkGrowth(base, base); err != nil {
		t.Fatalf("unchanged: err = %v, want nil", err)
	}
	grown := append([]Exception{{Rule: "counters", Module: "openits-ess", Path: "/c", Cut: "ess"}}, base...)
	err := checkGrowth(grown, base)
	if err == nil || !strings.Contains(err.Error(), "exceptions file grew: counters openits-ess /c is not in the base") {
		t.Fatalf("growing: err = %v, want a growth error naming counters openits-ess /c", err)
	}
}

func TestRun_BaseGrowthFails(t *testing.T) {
	dir := t.TempDir()
	cur := filepath.Join(dir, "cur.yaml")
	base := filepath.Join(dir, "base.yaml")
	// Baseline the fixture's current violations under a stand-in cut.
	mods, err := loadModules("testdata/schema")
	if err != nil {
		t.Fatal(err)
	}
	vs := checkSchema(mods)
	refs, err := ruleReferences("testdata/schema")
	if err != nil {
		t.Fatal(err)
	}
	var f exceptionsFile
	for _, v := range append(vs, refs...) {
		f.Exceptions = append(f.Exceptions, Exception{Rule: v.Rule, Module: v.Module, Path: v.Path, Cut: "foundation"})
	}
	b, err := yaml.Marshal(&f)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(cur, b, 0o644)
	if err := run("testdata/schema", cur, cur, false, io.Discard); err != nil {
		t.Fatalf("base equal to current: err = %v, want nil", err)
	}
	os.WriteFile(base, []byte("exceptions: []\n"), 0o644)
	err = run("testdata/schema", cur, base, false, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "exceptions file grew") {
		t.Fatalf("empty base: err = %v, want a growth error", err)
	}
	err = run("testdata/schema", cur, filepath.Join(dir, "missing.yaml"), false, io.Discard)
	if err == nil {
		t.Fatal("missing base file: err = nil, want an error")
	}
}
