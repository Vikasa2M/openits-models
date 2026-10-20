package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testYang = "testdata/yang"

// testAllow allowlists the fam-c kind, whose values are not
// operational-mode identities.
var testAllow = map[string]string{
	"test-fam-c:c-mode-event-kind": "test: values are not operational-mode identities",
}

func load(t *testing.T, extra map[string]string) *Schema {
	t.Helper()
	dirs := []string{testYang}
	if len(extra) > 0 {
		d := t.TempDir()
		for name, body := range extra {
			if err := os.WriteFile(filepath.Join(d, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		dirs = append(dirs, d)
	}
	s, err := LoadSchema(dirs...)
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	return s
}

func extraModule(body string) map[string]string {
	return map[string]string{"test-extra.yang": `module test-extra {
  yang-version 1.1;
  namespace "urn:test:extra";
  prefix fx;
  import openits-types { prefix ot; }
  import test-fam-a { prefix fa; }
` + body + "\n}\n"}
}

func wantViolation(t *testing.T, vs []string, substr string) {
	t.Helper()
	for _, v := range vs {
		if strings.Contains(v, substr) {
			return
		}
	}
	t.Fatalf("no violation containing %q in %q", substr, vs)
}

// ---- rule (a): every mode kind names one operational-mode base ----

func TestKinds_GoodSchemaPasses(t *testing.T) {
	if vs := load(t, nil).CheckKinds(testAllow); len(vs) != 0 {
		t.Fatalf("unexpected violations: %q", vs)
	}
}

func TestKinds_MissingBaseFails(t *testing.T) {
	vs := load(t, extraModule(`  identity x-mode-event-kind { base ot:mode-event-kind; }`)).CheckKinds(testAllow)
	wantViolation(t, vs, "test-extra:x-mode-event-kind has no openits-types:mode-value-base")
}

func TestKinds_NotAllowlistedFails(t *testing.T) {
	vs := load(t, nil).CheckKinds(nil)
	wantViolation(t, vs, "test-fam-c:c-mode-event-kind has no openits-types:mode-value-base")
}

func TestKinds_DerivedSubKindNeedsItsOwnBase(t *testing.T) {
	vs := load(t, extraModule(`  identity x-sub-kind { base fa:a-mode-event-kind; }`)).CheckKinds(testAllow)
	wantViolation(t, vs, "test-extra:x-sub-kind has no openits-types:mode-value-base")
}

func TestKinds_TwoBasesFail(t *testing.T) {
	vs := load(t, extraModule(`  identity x-mode-event-kind {
    base ot:mode-event-kind;
    ot:mode-value-base "fa:a-mode";
    ot:mode-value-base "fa:a-mode";
  }`)).CheckKinds(testAllow)
	wantViolation(t, vs, "has 2 openits-types:mode-value-base")
}

func TestKinds_BaseNotOperationalModeFails(t *testing.T) {
	vs := load(t, extraModule(`  identity x-values;
  identity x-mode-event-kind {
    base ot:mode-event-kind;
    ot:mode-value-base "x-values";
  }`)).CheckKinds(testAllow)
	wantViolation(t, vs, "not derived from openits-types:operational-mode")
}

func TestKinds_BaseIsOperationalModeItselfFails(t *testing.T) {
	vs := load(t, extraModule(`  identity x-mode-event-kind {
    base ot:mode-event-kind;
    ot:mode-value-base "ot:operational-mode";
  }`)).CheckKinds(testAllow)
	wantViolation(t, vs, "not derived from openits-types:operational-mode")
}

func TestKinds_BaseInOtherFamilyFails(t *testing.T) {
	vs := load(t, extraModule(`  identity x-mode-event-kind {
    base ot:mode-event-kind;
    ot:mode-value-base "fa:a-mode";
  }`)).CheckKinds(testAllow)
	wantViolation(t, vs, "test-extra:x-mode-event-kind: openits-types:mode-value-base test-fam-a:a-mode is not defined in the kind's own module")
}

func TestKinds_UnresolvedBaseFails(t *testing.T) {
	vs := load(t, extraModule(`  identity x-mode-event-kind {
    base ot:mode-event-kind;
    ot:mode-value-base "zz:nowhere";
  }`)).CheckKinds(testAllow)
	wantViolation(t, vs, `"zz:nowhere" does not resolve`)
}

func TestKinds_MisplacedExtensionFails(t *testing.T) {
	vs := load(t, extraModule(`  identity x-other {
    ot:mode-value-base "fa:a-mode";
  }`)).CheckKinds(testAllow)
	wantViolation(t, vs, "test-extra:x-other is not a mode-event-kind")
}

func TestKinds_StaleAllowlistFails(t *testing.T) {
	allow := map[string]string{
		"test-fam-c:c-mode-event-kind": "x",
		"test-fam-a:a-mode-event-kind": "x",
		"test-fam-z:gone":              "x",
	}
	vs := load(t, nil).CheckKinds(allow)
	wantViolation(t, vs, "test-fam-a:a-mode-event-kind is allowlisted but declares")
	wantViolation(t, vs, "test-fam-z:gone is allowlisted but is not a mode-event-kind")
}

// ---- rule (b): fixture values derive from their kind's base ----

func modeFixture(t *testing.T, dir, name, kind, prev, mode string) string {
	t.Helper()
	body := `{"openits-common-mode-events:mode-changed": {"kind": "` + kind + `"`
	if prev != "" {
		body += `, "previous-mode": "` + prev + `"`
	}
	if mode != "" {
		body += `, "mode": "` + mode + `"`
	}
	body += `}}`
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func checkOne(t *testing.T, kind, prev, mode string) []string {
	t.Helper()
	s := load(t, nil)
	p := modeFixture(t, t.TempDir(), "valid-x.json", kind, prev, mode)
	applies, vs, err := s.CheckFixture(p)
	if err != nil {
		t.Fatal(err)
	}
	if !applies {
		t.Fatal("mode-changed fixture not recognised")
	}
	return vs
}

func TestFixture_OwnFamilyPasses(t *testing.T) {
	if vs := checkOne(t, "test-fam-a:a-mode-event-kind", "test-fam-a:a-off", "test-fam-a:a-on"); len(vs) != 0 {
		t.Fatalf("own-family values rejected: %q", vs)
	}
}

func TestFixture_UnprefixedBaseArgumentResolves(t *testing.T) {
	if vs := checkOne(t, "test-fam-b:b-mode-event-kind", "", "test-fam-b:b-on"); len(vs) != 0 {
		t.Fatalf("own-family values rejected: %q", vs)
	}
}

func TestFixture_DualBasedFamilyUnknownPasses(t *testing.T) {
	if vs := checkOne(t, "test-fam-a:a-mode-event-kind", "test-fam-a:a-unknown", "test-fam-a:a-on"); len(vs) != 0 {
		t.Fatalf("family unknown (dual-based on the canonical) rejected: %q", vs)
	}
}

func TestFixture_CrossFamilyModeFails(t *testing.T) {
	vs := checkOne(t, "test-fam-a:a-mode-event-kind", "", "test-fam-b:b-on")
	wantViolation(t, vs, "mode test-fam-b:b-on does not derive from test-fam-a:a-mode")
}

func TestFixture_CrossFamilyPreviousModeFails(t *testing.T) {
	vs := checkOne(t, "test-fam-a:a-mode-event-kind", "test-fam-b:b-on", "test-fam-a:a-on")
	wantViolation(t, vs, "previous-mode test-fam-b:b-on does not derive from test-fam-a:a-mode")
}

func TestFixture_CanonicalUnknownOutsideFamilyFails(t *testing.T) {
	vs := checkOne(t, "test-fam-a:a-mode-event-kind", "", "openits-types:mode-unknown")
	wantViolation(t, vs, "mode openits-types:mode-unknown does not derive from test-fam-a:a-mode")
}

func TestFixture_AbstractBaseAsValueFails(t *testing.T) {
	vs := checkOne(t, "test-fam-a:a-mode-event-kind", "test-fam-a:a-mode", "test-fam-a:a-on")
	wantViolation(t, vs, "previous-mode test-fam-a:a-mode is the abstract")
	vs = checkOne(t, "test-fam-a:a-mode-event-kind", "", "test-fam-a:a-mode")
	wantViolation(t, vs, "mode test-fam-a:a-mode is the abstract")
}

func TestFixture_UnknownValueFails(t *testing.T) {
	wantViolation(t, checkOne(t, "test-fam-a:a-mode-event-kind", "", "test-fam-a:a-nope"), "unknown identity")
}

func TestFixture_UnprefixedValueFails(t *testing.T) {
	wantViolation(t, checkOne(t, "test-fam-a:a-mode-event-kind", "", "a-on"), "module prefix")
}

func TestFixture_MissingModeFails(t *testing.T) {
	wantViolation(t, checkOne(t, "test-fam-a:a-mode-event-kind", "", ""), "no mode")
}

func TestFixture_AbstractKindFails(t *testing.T) {
	wantViolation(t, checkOne(t, "openits-types:mode-event-kind", "", "test-fam-a:a-on"), "names no openits-types:mode-value-base")
}

func TestFixture_AllowlistedKindFails(t *testing.T) {
	wantViolation(t, checkOne(t, "test-fam-c:c-mode-event-kind", "", "test-fam-a:a-on"), "names no openits-types:mode-value-base")
}

func TestFixture_OtherNotificationSkipped(t *testing.T) {
	p := filepath.Join(t.TempDir(), "valid-other.json")
	os.WriteFile(p, []byte(`{"x:fault-raised": {"kind": "test-fam-b:b-on"}}`), 0o644)
	applies, vs, err := load(t, nil).CheckFixture(p)
	if err != nil || applies || len(vs) != 0 {
		t.Fatalf("non-mode-changed fixture: applies=%v vs=%q err=%v", applies, vs, err)
	}
}

// ---- run: rules (b) over valid-*, (c) over the invalid directory ----

func runDirs(t *testing.T, fixtures, invalid string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := run([]string{testYang}, fixtures, invalid, testAllow, &out)
	return out.String(), err
}

func TestRun_GoodTreePasses(t *testing.T) {
	fx, inv := t.TempDir(), t.TempDir()
	modeFixture(t, fx, "valid-a.json", "test-fam-a:a-mode-event-kind", "test-fam-a:a-off", "test-fam-a:a-on")
	modeFixture(t, fx, "invalid-a.json", "test-fam-a:a-mode-event-kind", "", "test-fam-b:b-on") // not scanned by (b)
	modeFixture(t, inv, "invalid-mode-changed-cross.json", "test-fam-a:a-mode-event-kind", "", "test-fam-b:b-on")
	out, err := runDirs(t, fx, inv)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out, "OK") {
		t.Fatalf("no OK line: %q", out)
	}
}

func TestRun_BadValidFixtureFails(t *testing.T) {
	fx, inv := t.TempDir(), t.TempDir()
	modeFixture(t, fx, "valid-a.json", "test-fam-a:a-mode-event-kind", "", "test-fam-b:b-on")
	modeFixture(t, inv, "invalid-mode-changed-cross.json", "test-fam-a:a-mode-event-kind", "", "test-fam-b:b-on")
	if _, err := runDirs(t, fx, inv); err == nil || !strings.Contains(err.Error(), "valid-a.json") {
		t.Fatalf("cross-family valid fixture not reported: %v", err)
	}
}

func TestRun_InvalidFixtureThatPassesFails(t *testing.T) {
	fx, inv := t.TempDir(), t.TempDir()
	modeFixture(t, inv, "invalid-mode-changed-own.json", "test-fam-a:a-mode-event-kind", "", "test-fam-a:a-on")
	if _, err := runDirs(t, fx, inv); err == nil || !strings.Contains(err.Error(), "invalid-mode-changed-own.json passes") {
		t.Fatalf("passing invalid fixture not reported: %v", err)
	}
}

func TestRun_InvalidFixtureNotModeChangedFails(t *testing.T) {
	fx, inv := t.TempDir(), t.TempDir()
	p := filepath.Join(inv, "invalid-other.json")
	if err := os.WriteFile(p, []byte(`{"x:fault-raised": {"kind": "test-fam-b:b-on"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runDirs(t, fx, inv); err == nil || !strings.Contains(err.Error(), "invalid-other.json is not a mode-changed fixture") {
		t.Fatalf("non-mode-changed invalid fixture not reported: %v", err)
	}
}

func TestRun_EmptyInvalidDirFails(t *testing.T) {
	if _, err := runDirs(t, t.TempDir(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "no invalid-mode-changed-*.json") {
		t.Fatalf("empty invalid directory not reported: %v", err)
	}
}

func TestRun_BadKindSchemaFails(t *testing.T) {
	fx, inv := t.TempDir(), t.TempDir()
	modeFixture(t, inv, "invalid-mode-changed-cross.json", "test-fam-a:a-mode-event-kind", "", "test-fam-b:b-on")
	var out bytes.Buffer
	if err := run([]string{testYang}, fx, inv, nil, &out); err == nil || !strings.Contains(err.Error(), "c-mode-event-kind") {
		t.Fatalf("un-allowlisted kind without a base not reported: %v", err)
	}
}
