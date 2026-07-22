// check-deviations validates yang/deviations/*.yang modules: each deviation
// must RESOLVE against the base openits YANG modules, and it may only
// TIGHTEN the base contract — never loosen it. See
// docs/06-extension-model.md ("Tier 3 — Deviations").
//
// Two checks run, independently, per deviation module:
//
//  1. Resolution. The deviation module is loaded alongside the base modules
//     via goyang and Process()ed. A deviation statement whose target path
//     doesn't exist in the base tree (typo, wrong prefix, node moved) makes
//     goyang's ApplyDeviate fail, which Process() surfaces as an error. That
//     becomes a Finding with Severity error.
//
//  2. Direction. Every `deviate` block inside every `deviation` statement is
//     classified: `add` only ever adds a new constraint (tightening — a
//     Finding with Severity ok). `not-supported` removes the target node
//     entirely, and `delete` of a `must` removes an existing constraint —
//     both loosen the base contract, so both are Findings with Severity
//     error. `replace` and non-must `delete` can go either way depending on
//     content goyang doesn't interpret (e.g. a `replace` could narrow a
//     range or widen it), so those are Findings with Severity note for a
//     human (TSC review) to judge.
//
// A best-effort third check, run only from main (not from
// ValidateDeviations — see its doc comment on why), drives yanglint in
// Docker to empirically prove tightening: it validates the
// yang/testdata/invalid-<x>-under-<deviation>.json fixture family with the
// named deviation module ALSO loaded, and asserts libyang rejects it. When
// Docker isn't available this degrades to a single informational Finding
// rather than failing the whole tool — the resolve+classify pass above is
// the load-bearing gate.
//
// Wired to `make check-deviations`.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

// Severity classifies a Finding.
type Severity string

const (
	// SeverityError means the tool must exit non-zero: a deviation that
	// doesn't resolve, or one that loosens the base contract.
	SeverityError Severity = "error"
	// SeverityNote flags something a human should look at (e.g. a
	// `replace` deviate, which this tool can't classify as tightening
	// or loosening from the AST alone) but doesn't fail the run.
	SeverityNote Severity = "note"
	// SeverityOK records a confirmed-tightening observation for a clean
	// report; it never fails the run.
	SeverityOK Severity = "ok"
)

// Finding is one observation produced while validating yang/deviations/.
type Finding struct {
	// Deviation is the basename of the yang/deviations/*.yang file the
	// finding is about (e.g. "openits-signal-control-mutcd-strict.yang").
	Deviation string
	// Target is the deviation statement's target path, when known.
	Target   string
	Severity Severity
	Message  string
}

func (f Finding) String() string {
	if f.Target == "" {
		return fmt.Sprintf("[%s] %s: %s", f.Severity, f.Deviation, f.Message)
	}
	return fmt.Sprintf("[%s] %s (%s): %s", f.Severity, f.Deviation, f.Target, f.Message)
}

func main() {
	yangDir := "yang"
	deviationsDir := "yang/deviations"
	if len(os.Args) > 1 {
		yangDir = os.Args[1]
	}
	if len(os.Args) > 2 {
		deviationsDir = os.Args[2]
	}

	findings, err := ValidateDeviations(yangDir, deviationsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "check-deviations: %v\n", err)
		os.Exit(2)
	}

	// Supplementary empirical proof: does base+deviation actually reject
	// what base alone accepts? See RunTighteningProof's doc comment for
	// why this isn't part of ValidateDeviations itself.
	testdataDir := filepath.Join(yangDir, "testdata")
	findings = append(findings, RunTighteningProof(yangDir, deviationsDir, testdataDir)...)

	errCount, noteCount := 0, 0
	for _, f := range findings {
		fmt.Println(f.String())
		switch f.Severity {
		case SeverityError:
			errCount++
		case SeverityNote:
			noteCount++
		}
	}

	if errCount > 0 {
		fmt.Printf("check-deviations: %d error finding(s), %d note(s)\n", errCount, noteCount)
		os.Exit(1)
	}
	fmt.Printf("check-deviations: 0 error findings (%d note(s)) across %d finding(s)\n", noteCount, len(findings))
}

// ValidateDeviations loads the base YANG modules under yangDir plus each
// yang/deviations/*.yang module — one at a time, in isolation from its
// siblings — and reports whether each deviation resolves against the base
// and only tightens it. See the package doc comment for the two checks it
// performs.
//
// Isolation matters: each deviation file gets its own fresh *yang.Modules
// loaded with the base tree, so a bad deviation can't cascade errors into
// the evaluation of an unrelated one, and every error Finding is
// unambiguously attributable to the deviation file that caused it.
//
// A non-nil error return means ValidateDeviations itself could not run at
// all (e.g. yangDir doesn't exist, or deviationsDir can't be listed) — not
// that a deviation is bad. Per-deviation problems are Findings, precisely
// so that one broken deviation module doesn't stop the rest from being
// evaluated and reported in the same run.
//
// This function intentionally does not drive yanglint/Docker: it is pure
// goyang static analysis, so it is fast, deterministic, and safe to call
// from unit tests without an external dependency. The Docker-based
// tightening proof lives in RunTighteningProof, called separately by main.
func ValidateDeviations(yangDir, deviationsDir string) ([]Finding, error) {
	devFiles, err := deviationFiles(deviationsDir)
	if err != nil {
		return nil, fmt.Errorf("list deviation files in %s: %w", deviationsDir, err)
	}

	var findings []Finding
	for _, df := range devFiles {
		findings = append(findings, validateOneDeviation(yangDir, df)...)
	}
	return findings, nil
}

// deviationFiles returns the sorted list of *.yang files directly under
// dir. A missing or empty dir is not an error — it just yields no findings
// (a repo with no deviations yet is a legitimate, if uninteresting, state).
func deviationFiles(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []string
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yang") {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)
	return files, nil
}

// validateOneDeviation loads yangDir's base modules plus the single
// devFile into a fresh *yang.Modules, Process()es them, and reports the
// resolution + classification findings for that one deviation module.
func validateOneDeviation(yangDir, devFile string) []Finding {
	name := filepath.Base(devFile)
	ms := yang.NewModules()

	if err := readBaseModules(ms, yangDir); err != nil {
		return []Finding{{
			Deviation: name,
			Severity:  SeverityError,
			Message:   fmt.Sprintf("load base modules from %s: %v", yangDir, err),
		}}
	}
	if err := ms.Read(devFile); err != nil {
		return []Finding{{
			Deviation: name,
			Severity:  SeverityError,
			Message:   fmt.Sprintf("parse: %v", err),
		}}
	}

	if errs := ms.Process(); len(errs) > 0 {
		findings := make([]Finding, 0, len(errs))
		for _, e := range errs {
			findings = append(findings, Finding{
				Deviation: name,
				Severity:  SeverityError,
				Message:   fmt.Sprintf("does not resolve against base: %v", e),
			})
		}
		return findings
	}

	// Resolution succeeded. Walk every `deviation` statement this module
	// contributes and classify its `deviate` blocks. Base modules never
	// carry `deviation` statements themselves, but we still scope the
	// walk to modules whose bare name matches (skipping the
	// "name@revision" alias goyang also files each module under, which
	// points at the very same *Module and would otherwise be visited
	// twice) so this stays correct even if that changed.
	var findings []Finding
	sawDeviation := false
	for modName, m := range ms.Modules {
		if modName != m.Name || m.BelongsTo != nil {
			continue
		}
		if len(m.Deviation) == 0 {
			continue
		}
		sawDeviation = true
		for _, dev := range m.Deviation {
			findings = append(findings, classifyDeviation(name, dev)...)
		}
	}
	if !sawDeviation {
		findings = append(findings, Finding{
			Deviation: name,
			Severity:  SeverityNote,
			Message:   "module resolved but contains no `deviation` statement",
		})
	}
	return findings
}

// classifyDeviation reports one Finding per `deviate` block under dev,
// classifying it as tightening (ok), ambiguous (note), or loosening
// (error). See the package doc comment for the rule.
func classifyDeviation(fileName string, dev *yang.Deviation) []Finding {
	target := dev.Name // Deviation.Name is the `deviation "<target-path>"` argument.
	var findings []Finding
	for _, dt := range dev.Deviate {
		switch dt.Name {
		case "add":
			findings = append(findings, Finding{
				Deviation: fileName,
				Target:    target,
				Severity:  SeverityOK,
				Message:   "deviate add: only adds a constraint — tightens the base",
			})
		case "replace":
			findings = append(findings, Finding{
				Deviation: fileName,
				Target:    target,
				Severity:  SeverityNote,
				Message: "deviate replace: can narrow OR widen the replaced property " +
					"(e.g. a range) — not auto-classified as tightening, needs TSC review",
			})
		case "not-supported":
			findings = append(findings, Finding{
				Deviation: fileName,
				Target:    target,
				Severity:  SeverityError,
				Message: "deviate not-supported: removes the target node entirely — " +
					"this loosens the base contract; deviations may only tighten",
			})
		case "delete":
			if len(dt.Must) > 0 {
				exprs := make([]string, 0, len(dt.Must))
				for _, m := range dt.Must {
					exprs = append(exprs, m.Name)
				}
				findings = append(findings, Finding{
					Deviation: fileName,
					Target:    target,
					Severity:  SeverityError,
					Message: fmt.Sprintf(
						"deviate delete removes must constraint(s) [%s] — "+
							"this loosens the base contract; deviations may only tighten",
						strings.Join(exprs, "; ")),
				})
			} else {
				findings = append(findings, Finding{
					Deviation: fileName,
					Target:    target,
					Severity:  SeverityNote,
					Message: "deviate delete (no must involved): deleting a default/" +
						"mandatory/unique/element-count constraint can also loosen the " +
						"base contract — needs TSC review",
				})
			}
		default:
			findings = append(findings, Finding{
				Deviation: fileName,
				Target:    target,
				Severity:  SeverityNote,
				Message:   fmt.Sprintf("unrecognized deviate type %q", dt.Name),
			})
		}
	}
	return findings
}

// readBaseModules reads every .yang file directly under yangDir (and the
// optional yangDir/ietf vendored-import subdirectory) into ms. Mirrors
// tools/yang-proto-gen's LoadModules, minus the Process() call (the caller
// still needs to Read the deviation file before Process()ing).
//
// Deliberately does not read yang/augments or yang/deviations: a broken
// augment, or a sibling deviation, must not be able to fail the resolution
// check for the deviation actually under test.
func readBaseModules(ms *yang.Modules, yangDir string) error {
	var files []string

	rootEnts, err := os.ReadDir(yangDir)
	if err != nil {
		return fmt.Errorf("read yang dir %s: %w", yangDir, err)
	}
	for _, f := range rootEnts {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".yang") {
			files = append(files, filepath.Join(yangDir, f.Name()))
		}
	}
	if ietfEnts, err := os.ReadDir(filepath.Join(yangDir, "ietf")); err == nil {
		for _, f := range ietfEnts {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".yang") {
				files = append(files, filepath.Join(yangDir, "ietf", f.Name()))
			}
		}
	}
	sort.Strings(files)

	for _, f := range files {
		if err := ms.Read(f); err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
	}
	return nil
}

// RunTighteningProof drives yanglint (via Docker) to empirically prove
// tightening for the yang/testdata/invalid-<x>-under-<deviation-suffix>.json
// fixture family: scripts/validate-yang.sh's base-only pass skips this
// family (see its `invalid-*-under-*.json` case), because these fixtures
// are only supposed to fail once the named deviation is ALSO loaded.
//
// For each such fixture, the "<deviation-suffix>" after the last "-under-"
// is matched against yang/deviations/*.yang basenames (a deviation file
// matches if its basename, minus ".yang", ends with the suffix). yanglint
// then runs against every top-level yang/*.yang module plus that one
// deviation file, and the fixture must be REJECTED (non-zero exit).
//
// This is deliberately a separate function from ValidateDeviations (whose
// signature is fixed by the tool's contract and takes no testdata
// directory), and deliberately best-effort: when the `docker` binary isn't
// on PATH, it returns a single informational Finding instead of an error,
// so `go test`/CI without Docker still exercises the load-bearing
// resolve+classify checks in ValidateDeviations.
func RunTighteningProof(yangDir, deviationsDir, testdataDir string) []Finding {
	if _, err := exec.LookPath("docker"); err != nil {
		return []Finding{{
			Severity: SeverityNote,
			Message: "docker not found on PATH; skipping the yanglint tightening proof " +
				"for yang/testdata/invalid-*-under-*.json — run scripts/validate-yang.sh's " +
				"equivalent yanglint invocation manually, or install Docker",
		}}
	}

	fixtures, err := filepath.Glob(filepath.Join(testdataDir, "invalid-*-under-*.json"))
	if err != nil {
		return []Finding{{Severity: SeverityError, Message: fmt.Sprintf("glob %s: %v", testdataDir, err)}}
	}
	sort.Strings(fixtures)
	if len(fixtures) == 0 {
		return []Finding{{
			Severity: SeverityNote,
			Message:  fmt.Sprintf("no %s fixtures found; nothing to empirically prove", filepath.Join(testdataDir, "invalid-*-under-*.json")),
		}}
	}

	baseSchemas, err := filepath.Glob(filepath.Join(yangDir, "*.yang"))
	if err != nil {
		return []Finding{{Severity: SeverityError, Message: fmt.Sprintf("glob %s: %v", yangDir, err)}}
	}
	sort.Strings(baseSchemas)

	repoRoot, err := os.Getwd()
	if err != nil {
		return []Finding{{Severity: SeverityError, Message: fmt.Sprintf("getwd: %v", err)}}
	}

	image := os.Getenv("YANGLINT_IMAGE")
	if image == "" {
		image = "sysrepo/sysrepo-netopeer2:latest"
	}

	var findings []Finding
	for _, fixture := range fixtures {
		findings = append(findings, proveOneFixtureTightens(fixture, yangDir, deviationsDir, baseSchemas, repoRoot, image)...)
	}
	return findings
}

// proveOneFixtureTightens matches one invalid-<x>-under-<suffix>.json
// fixture to its deviation module and runs the yanglint rejection proof.
func proveOneFixtureTightens(fixture, yangDir, deviationsDir string, baseSchemas []string, repoRoot, image string) []Finding {
	base := strings.TrimSuffix(filepath.Base(fixture), ".json")
	idx := strings.LastIndex(base, "-under-")
	if idx == -1 {
		// Glob pattern guarantees this doesn't happen, but stay defensive.
		return []Finding{{Severity: SeverityError, Message: fmt.Sprintf("fixture %s doesn't match invalid-<x>-under-<deviation>.json", fixture)}}
	}
	suffix := base[idx+len("-under-"):]

	devFile, err := matchDeviationFile(deviationsDir, suffix)
	if err != nil {
		return []Finding{{Severity: SeverityError, Message: fmt.Sprintf("fixture %s: %v", filepath.Base(fixture), err)}}
	}

	args := []string{"run", "--rm", "-v", repoRoot + ":/w", "-w", "/w", image,
		"yanglint", "-f", "json", "-p", yangDir, "-p", filepath.Join(yangDir, "ietf")}
	args = append(args, baseSchemas...)
	args = append(args, devFile, fixture)

	cmd := exec.Command("docker", args...)
	out, runErr := cmd.CombinedOutput()

	fixtureName := filepath.Base(fixture)
	devName := filepath.Base(devFile)
	if runErr == nil {
		// yanglint accepted the data — the deviation did NOT reject what
		// this fixture claims it should. That's a failed tightening
		// proof, not a healthy deviation.
		return []Finding{{
			Deviation: devName,
			Target:    fixtureName,
			Severity:  SeverityError,
			Message: fmt.Sprintf(
				"tightening proof failed: yanglint ACCEPTED %s under base+%s "+
					"(expected rejection). yanglint output: %s",
				fixtureName, devName, strings.TrimSpace(string(out))),
		}}
	}
	return []Finding{{
		Deviation: devName,
		Target:    fixtureName,
		Severity:  SeverityOK,
		Message: fmt.Sprintf(
			"tightening proof: yanglint correctly REJECTED %s under base+%s",
			fixtureName, devName),
	}}
}

// matchDeviationFile finds the yang/deviations/*.yang file whose basename
// (minus ".yang") ends with suffix, e.g. suffix "mutcd-strict" matches
// "openits-signal-control-mutcd-strict.yang".
func matchDeviationFile(deviationsDir, suffix string) (string, error) {
	devFiles, err := deviationFiles(deviationsDir)
	if err != nil {
		return "", fmt.Errorf("list deviation files in %s: %w", deviationsDir, err)
	}
	var matches []string
	for _, df := range devFiles {
		if strings.HasSuffix(strings.TrimSuffix(filepath.Base(df), ".yang"), suffix) {
			matches = append(matches, df)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no deviation module under %s matches suffix %q", deviationsDir, suffix)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous: %d deviation modules under %s match suffix %q: %s", len(matches), deviationsDir, suffix, strings.Join(matches, ", "))
	}
}
