package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
)

// realYangDir points at the repo's real yang/ tree. `go test` runs with the
// package directory as its working directory, so this mirrors the
// convention used by tools/yang-proto-gen's tests.
func realYangDir() string { return filepath.Join("..", "..", "yang") }

func realDeviationsDir() string { return filepath.Join(realYangDir(), "deviations") }

// (a) The shipped mutcd-strict deviation resolves against the real base
// modules and is tightening-only: zero error-severity findings, and at
// least one confirmed-tightening (ok) finding so an empty/no-op result
// doesn't pass this test by accident.
func TestValidateDeviations_shippedMutcdStrict_resolvesAndTightens(t *testing.T) {
	findings, err := ValidateDeviations(realYangDir(), realDeviationsDir())
	if err != nil {
		t.Fatalf("ValidateDeviations: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("expected at least one finding for the shipped mutcd-strict deviation, got none")
	}

	var errs, oks int
	for _, f := range findings {
		t.Logf("%s", f)
		switch f.Severity {
		case SeverityError:
			errs++
		case SeverityOK:
			oks++
		}
	}
	if errs != 0 {
		t.Errorf("expected 0 error findings for the shipped deviation, got %d", errs)
	}
	if oks == 0 {
		t.Error("expected at least one ok (tightening) finding for the shipped deviation's `deviate add`, got none")
	}

	// The one shipped deviation module must actually have been evaluated
	// (not silently skipped because the directory looked empty).
	var sawMutcdStrict bool
	for _, f := range findings {
		if f.Deviation == "openits-signal-control-mutcd-strict.yang" {
			sawMutcdStrict = true
		}
	}
	if !sawMutcdStrict {
		t.Error("expected findings attributed to openits-signal-control-mutcd-strict.yang")
	}
}

// (b) A synthetic deviation that `deviate delete`s the base module's
// yellow-change `must` (removing the 3.0-6.0 s MUTCD range entirely) is a
// loosening violation, not a tightening one. This is the guard's own
// negative test: if classifyDeviation stopped inspecting deviate-delete's
// Must field, this is what would go red.
//
// The synthetic deviation targets the SAME real base tree (yangDir is the
// repo's real yang/) so resolution genuinely succeeds — only the
// direction-of-change classification should fail it. Only the deviation
// file itself lives under the test's temp dir, per the brief.
func TestValidateDeviations_syntheticDeviateDeleteMust_isLoosening(t *testing.T) {
	tmpDeviationsDir := t.TempDir()
	synthetic := `module openits-signal-control-loosen-test {
  yang-version 1.1;
  namespace "urn:test:yang:signal-control-loosen-test";
  prefix openits-sc-loosen-test;

  import openits-signal-control { prefix openits-sc; }

  organization "test";
  contact "test@example.org";
  description
    "Synthetic deviation for check-deviations tests: deletes the base
     yellow-change must, which loosens the standard.";

  revision 2026-07-10 {
    description "Test fixture.";
  }

  deviation "/openits-sc:signal-controller/openits-sc:phases/openits-sc:phase/openits-sc:config/openits-sc:timing/openits-sc:yellow-change" {
    deviate delete {
      must ". >= 3.0 and . <= 6.0";
    }
  }
}
`
	path := filepath.Join(tmpDeviationsDir, "openits-signal-control-loosen-test.yang")
	if err := os.WriteFile(path, []byte(synthetic), 0o644); err != nil {
		t.Fatalf("write synthetic deviation: %v", err)
	}

	findings, err := ValidateDeviations(realYangDir(), tmpDeviationsDir)
	if err != nil {
		t.Fatalf("ValidateDeviations: %v", err)
	}

	var errs []Finding
	for _, f := range findings {
		t.Logf("%s", f)
		if f.Severity == SeverityError {
			errs = append(errs, f)
		}
	}
	if len(errs) == 0 {
		t.Fatal("expected the synthetic `deviate delete { must ...; }` to be flagged as a loosening violation (severity error), got none")
	}
	found := false
	for _, f := range errs {
		if strings.Contains(f.Message, "must") && strings.Contains(strings.ToLower(f.Message), "loosen") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an error finding calling out the deleted must as loosening; got: %v", errs)
	}
}

// A deviation that references a target path which doesn't exist in the
// base tree must surface as an error finding (resolution failure), not
// silently pass.
func TestValidateDeviations_unresolvableTarget_isErrorFinding(t *testing.T) {
	tmpDeviationsDir := t.TempDir()
	synthetic := `module openits-signal-control-bad-target-test {
  yang-version 1.1;
  namespace "urn:test:yang:signal-control-bad-target-test";
  prefix openits-sc-bad-target-test;

  import openits-signal-control { prefix openits-sc; }

  organization "test";
  contact "test@example.org";
  description "Synthetic deviation targeting a node that does not exist.";

  revision 2026-07-10 {
    description "Test fixture.";
  }

  deviation "/openits-sc:signal-controller/openits-sc:phases/openits-sc:phase/openits-sc:timing/openits-sc:does-not-exist" {
    deviate add {
      must ". >= 1";
    }
  }
}
`
	path := filepath.Join(tmpDeviationsDir, "openits-signal-control-bad-target-test.yang")
	if err := os.WriteFile(path, []byte(synthetic), 0o644); err != nil {
		t.Fatalf("write synthetic deviation: %v", err)
	}

	findings, err := ValidateDeviations(realYangDir(), tmpDeviationsDir)
	if err != nil {
		t.Fatalf("ValidateDeviations: %v", err)
	}
	var errs int
	for _, f := range findings {
		t.Logf("%s", f)
		if f.Severity == SeverityError {
			errs++
		}
	}
	if errs == 0 {
		t.Fatal("expected an unresolvable deviation target to produce at least one error finding")
	}
}

// classifyDeviation is the core direction-of-change judgment. Exercise its
// full matrix directly (without needing a real goyang resolve pass) so the
// classification rule itself has fast, isolated coverage.
func TestClassifyDeviation_matrix(t *testing.T) {
	dev := &yang.Deviation{
		Name: "/openits-sc:signal-controller/openits-sc:phases/openits-sc:phase/openits-sc:timing/openits-sc:yellow-change",
		Deviate: []*yang.Deviate{
			{Name: "add"},
			{Name: "replace"},
			{Name: "not-supported"},
			{Name: "delete", Must: []*yang.Must{{Name: ". >= 3.0"}}},
			{Name: "delete"}, // no must — ambiguous, not auto-flagged as error
		},
	}

	findings := classifyDeviation("test.yang", dev)
	if len(findings) != 5 {
		t.Fatalf("expected 5 findings (one per deviate block), got %d: %v", len(findings), findings)
	}

	want := []Severity{SeverityOK, SeverityNote, SeverityError, SeverityError, SeverityNote}
	for i, f := range findings {
		if f.Severity != want[i] {
			t.Errorf("finding %d (%s): got severity %s, want %s", i, f.Message, f.Severity, want[i])
		}
		if f.Target != dev.Name {
			t.Errorf("finding %d: got target %q, want %q", i, f.Target, dev.Name)
		}
	}
}

// A deviations directory that doesn't exist yet (a repo before its first
// deviation lands) is a legitimate empty state, not an error.
func TestValidateDeviations_missingDeviationsDir_isEmptyNotError(t *testing.T) {
	findings, err := ValidateDeviations(realYangDir(), filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("ValidateDeviations: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings for a missing deviations dir, got %v", findings)
	}
}
