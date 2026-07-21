package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// buildRealIndex loads the real yang/ corpus + schema-registry/ tree and
// returns the assembled Index, failing the test on any error. Mirrors
// TestEmitAsyncAPI's real-corpus setup.
func buildRealIndex(t *testing.T) *Index {
	t.Helper()
	ms, mods, err := LoadModules(filepath.Join("..", "..", "yang"))
	if err != nil {
		t.Fatal(err)
	}
	cat, err := BuildCatalog(ms, mods)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := BuildIndex(mods, cat, filepath.Join("..", "..", "schema-registry"))
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

// TestBuildIndex_content asserts the neutral self-index carries the real
// service/module/registry facts a consumer relies on: all nine services with
// their core-module identity, a known service's ce-types and normative
// references, the foundation module list, and the registry snapshot map — with
// the index never indexing itself.
func TestBuildIndex_content(t *testing.T) {
	idx := buildRealIndex(t)

	if idx.Standard != "OpenITS" {
		t.Errorf("standard = %q, want OpenITS", idx.Standard)
	}
	if idx.IndexVersion != indexFormatVersion {
		t.Errorf("indexVersion = %q, want %q", idx.IndexVersion, indexFormatVersion)
	}

	// All nine services from the catalog service map are present, sorted by slug.
	wantSlugs := []string{"cctv", "dms", "ess", "perception", "ramp-metering", "reversible-lane", "rsu", "signal-control", "traffic-sensor"}
	if len(idx.Services) != len(wantSlugs) {
		t.Fatalf("got %d services, want %d", len(idx.Services), len(wantSlugs))
	}
	for i, s := range idx.Services {
		if s.Slug != wantSlugs[i] {
			t.Errorf("services[%d].Slug = %q, want %q (services must be sorted by slug)", i, s.Slug, wantSlugs[i])
		}
	}

	// The dms service descriptor: identity from the core module, ce-types
	// from the catalog, normative references from module + revision `reference`.
	dms := findService(t, idx, "dms")
	if dms.Namespace != "urn:openits:yang:dms" {
		t.Errorf("dms namespace = %q, want urn:openits:yang:dms", dms.Namespace)
	}
	if dms.Description == "" {
		t.Error("dms description is empty; want the core module's description")
	}
	if !contains(dms.Events, "openits.dms.fault-raised.v1") {
		t.Errorf("dms events missing openits.dms.fault-raised.v1; got %v", dms.Events)
	}
	if !contains(dms.Events, "openits.dms.message-activation-failed.v1") {
		t.Errorf("dms events missing openits.dms.message-activation-failed.v1; got %v", dms.Events)
	}
	if !contains(dms.RefStd, "NTCIP 1203 v03") {
		t.Errorf("dms refStd missing \"NTCIP 1203 v03\"; got %v", dms.RefStd)
	}
	// A ";" inside parentheses must NOT split a citation (paren-aware split).
	if !contains(dms.RefStd, "NTCIP 1203:2010 (DMS functional reference; fault classes)") {
		t.Errorf("dms refStd should keep a parenthesized \";\" intact as one citation; got %v", dms.RefStd)
	}
	// No broken fragment (a citation that is just a dangling close-paren tail).
	for _, r := range dms.RefStd {
		if r == "fault classes)" || r == "no wire compatibility)" {
			t.Errorf("dms refStd contains a broken fragment %q — paren-aware split regressed", r)
		}
	}

	// Per-module breakdown: dms composes core + -types + -events.
	wantMods := []string{"openits-dms", "openits-dms-events", "openits-dms-types"}
	var gotMods []string
	for _, m := range dms.Modules {
		gotMods = append(gotMods, m.Name)
	}
	for _, w := range wantMods {
		if !contains(gotMods, w) {
			t.Errorf("dms modules missing %q; got %v", w, gotMods)
		}
	}
	// Service-level revisions rollup is the union of composing modules' revisions.
	if !contains(dms.Revisions, "2026-07-21") {
		t.Errorf("dms revisions rollup missing 2026-07-21; got %v", dms.Revisions)
	}

	// Foundation carries the shared layer and never a service-scoped module.
	fdn := foundationNames(idx)
	for _, w := range []string{"openits-types", "openits-nema-common", "openits-common-fault-events"} {
		if !contains(fdn, w) {
			t.Errorf("foundation missing %q; got %v", w, fdn)
		}
	}
	if contains(fdn, "openits-dms") {
		t.Error("foundation must not contain the service-scoped module openits-dms")
	}

	// Registry snapshot map: keyed by module, then revision, then paths
	// relative to schema-registry/. The index must never index itself.
	if _, ok := idx.Registry["index.json"]; ok {
		t.Error("registry must not contain an entry for index.json (self-indexing)")
	}
	if _, ok := idx.Registry["notices"]; ok {
		t.Error("registry must exclude the notices/ tree")
	}
	dmsReg, ok := idx.Registry["openits-dms"]
	if !ok {
		t.Fatalf("registry missing openits-dms; have %d modules", len(idx.Registry))
	}
	files, ok := dmsReg["2026-07-21"]
	if !ok {
		t.Fatalf("registry openits-dms missing revision 2026-07-21; have %v", regRevKeys(dmsReg))
	}
	if !contains(files, "openits-dms/2026-07-21/schema.yang") {
		t.Errorf("registry openits-dms/2026-07-21 missing schema.yang path; got %v", files)
	}
}

// TestMarshalIndex_deterministic guards the byte-stability contract check-gen
// relies on: two independent marshals over the same corpus are byte-identical.
func TestMarshalIndex_deterministic(t *testing.T) {
	a := buildRealIndex(t)
	b := buildRealIndex(t)
	ba, err := MarshalIndex(a)
	if err != nil {
		t.Fatal(err)
	}
	bb, err := MarshalIndex(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(ba) != string(bb) {
		t.Error("MarshalIndex is not deterministic: two builds over the same input produced different bytes")
	}
	// And it must be valid JSON with the documented top-level shape.
	var parsed map[string]any
	if err := json.Unmarshal(ba, &parsed); err != nil {
		t.Fatalf("index.json does not parse as JSON: %v", err)
	}
	for _, k := range []string{"standard", "indexVersion", "services", "foundation", "registry"} {
		if _, ok := parsed[k]; !ok {
			t.Errorf("index.json missing top-level key %q", k)
		}
	}
}

func TestModuleServiceSlug(t *testing.T) {
	cases := []struct {
		name     string
		wantSlug string
		wantOK   bool
	}{
		{"openits-dms", "dms", true},
		{"openits-dms-types", "dms", true},
		{"openits-dms-events", "dms", true},
		{"openits-signal-control", "signal-control", true},
		{"openits-signal-control-events", "signal-control", true},
		{"openits-ramp-metering-types", "ramp-metering", true},
		// Foundation / shared — no single service.
		{"openits-common-fault-events", "", false},
		{"openits-types", "", false},
		{"openits-nema-common", "", false},
		{"openits-v2x-messaging", "", false},
		{"openits-vendor-econolite-signal-control-types", "", false},
	}
	for _, c := range cases {
		got, ok := moduleServiceSlug(c.name)
		if got != c.wantSlug || ok != c.wantOK {
			t.Errorf("moduleServiceSlug(%q) = (%q, %v), want (%q, %v)", c.name, got, ok, c.wantSlug, c.wantOK)
		}
	}
}

func TestNormalizeRefs(t *testing.T) {
	cases := []struct {
		raw  string
		want []string
	}{
		{"NTCIP 1203 v03; MUTCD.", []string{"NTCIP 1203 v03", "MUTCD"}},
		{"NTCIP 1203:2010 (DMS functional reference; fault classes).", []string{"NTCIP 1203:2010 (DMS functional reference; fault classes)"}},
		{"NTCIP 1207 (ramp metering); MUTCD 11th ed. 4F.17.", []string{"NTCIP 1207 (ramp metering)", "MUTCD 11th ed. 4F.17"}},
		// Whitespace/newline collapse.
		{"RFC 7950\n   section 11.", []string{"RFC 7950 section 11"}},
		{"", nil},
	}
	for _, c := range cases {
		got := normalizeRefs(c.raw)
		if !equalStrings(got, c.want) {
			t.Errorf("normalizeRefs(%q) = %v, want %v", c.raw, got, c.want)
		}
	}
}

// --- helpers ---------------------------------------------------------------

func findService(t *testing.T, idx *Index, slug string) ServiceIndex {
	t.Helper()
	for _, s := range idx.Services {
		if s.Slug == slug {
			return s
		}
	}
	t.Fatalf("service %q not found in index", slug)
	return ServiceIndex{}
}

func foundationNames(idx *Index) []string {
	var out []string
	for _, m := range idx.Foundation {
		out = append(out, m.Name)
	}
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func regRevKeys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
