package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
)

func TestGenerate_producesBuildableProto(t *testing.T) {
	out := t.TempDir()
	lock := filepath.Join(out, "field-numbers.yaml")
	if err := Generate(filepath.Join("..", "..", "yang"), out, lock); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// A known file exists and contains a known message + the package + its
	// own per-service go_package.
	b, err := os.ReadFile(filepath.Join(out, "openits", "common", "v1", "events.proto"))
	if err != nil {
		t.Fatalf("expected openits/common/v1/events.proto: %v", err)
	}
	s := string(b)
	for _, want := range []string{
		"syntax = \"proto3\";",
		"option go_package = \"github.com/Vikasa2M/openits-models/pkg/proto/openits/common/v1;commonv1\";",
		"message FaultRaised {",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("openits/common/v1/events.proto missing %q", want)
		}
	}
	// The lock file was written.
	if _, err := os.Stat(lock); err != nil {
		t.Errorf("field lock not saved: %v", err)
	}
}

// TestGenerate_isDeterministic guards against LoadModules's underlying map
// iteration (ms.Modules, see loader.go) leaking into Generate's output:
// two independent runs over the same real yang/ corpus, into separate
// output directories, must produce byte-identical .proto files and an
// identical field-numbers.yaml — both the module processing order (which
// output file/enum-first-occurrence naming depends on) and the map-derived
// import/message-name sets within each file must be sorted before being
// written.
func TestGenerate_isDeterministic(t *testing.T) {
	yangDir := filepath.Join("..", "..", "yang")
	out1, out2 := t.TempDir(), t.TempDir()
	if err := Generate(yangDir, out1, filepath.Join(out1, "field-numbers.yaml")); err != nil {
		t.Fatalf("Generate (run 1): %v", err)
	}
	if err := Generate(yangDir, out2, filepath.Join(out2, "field-numbers.yaml")); err != nil {
		t.Fatalf("Generate (run 2): %v", err)
	}

	// Per-service output now spans multiple nested directories (one per
	// go_package), not a single flat openits/v1/ — walk out1 recursively and
	// compare every .proto file against its counterpart in out2.
	var relPaths []string
	if err := filepath.WalkDir(filepath.Join(out1, "openits"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".proto") {
			return nil
		}
		rel, err := filepath.Rel(out1, path)
		if err != nil {
			return err
		}
		relPaths = append(relPaths, rel)
		return nil
	}); err != nil {
		t.Fatalf("walk out1: %v", err)
	}
	if len(relPaths) == 0 {
		t.Fatal("expected at least one generated .proto file")
	}
	for _, p := range relPaths {
		b1, err := os.ReadFile(filepath.Join(out1, p))
		if err != nil {
			t.Fatalf("read run1 %s: %v", p, err)
		}
		b2, err := os.ReadFile(filepath.Join(out2, p))
		if err != nil {
			t.Fatalf("read run2 %s: %v", p, err)
		}
		if string(b1) != string(b2) {
			t.Errorf("%s differs between two Generate runs over the same input; output is not deterministic", p)
		}
	}

	l1, err := os.ReadFile(filepath.Join(out1, "field-numbers.yaml"))
	if err != nil {
		t.Fatalf("read run1 lock: %v", err)
	}
	l2, err := os.ReadFile(filepath.Join(out2, "field-numbers.yaml"))
	if err != nil {
		t.Fatalf("read run2 lock: %v", err)
	}
	if string(l1) != string(l2) {
		t.Error("field-numbers.yaml differs between two Generate runs over the same input")
	}
}

// TestGenerate_sharedTypesShared verifies the cross-service packaging
// contract described in the Task 7 brief: the shared wire-source grouping
// (see grouping.go/SharedGroupings) is written exactly once into its own
// openits/v1/types.proto (package openits.types.v1), and every consumer —
// here, common_events.proto — imports that file and references the message
// by its fully-qualified name rather than getting its own inlined copy.
func TestGenerate_sharedTypesShared(t *testing.T) {
	out := t.TempDir()
	lock := filepath.Join(out, "field-numbers.yaml")
	if err := Generate(filepath.Join("..", "..", "yang"), out, lock); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	types, err := os.ReadFile(filepath.Join(out, "openits", "types", "v1", "types.proto"))
	if err != nil {
		t.Fatalf("expected openits/types/v1/types.proto: %v", err)
	}
	ts := string(types)
	for _, want := range []string{
		"syntax = \"proto3\";",
		"package openits.types.v1;",
		"message WireSource {",
	} {
		if !strings.Contains(ts, want) {
			t.Errorf("openits/types/v1/types.proto missing %q:\n%s", want, ts)
		}
	}
	if strings.Count(ts, "message WireSource {") != 1 {
		t.Errorf("expected exactly one WireSource definition in types.proto, got %d", strings.Count(ts, "message WireSource {"))
	}

	common, err := os.ReadFile(filepath.Join(out, "openits", "common", "v1", "events.proto"))
	if err != nil {
		t.Fatalf("expected openits/common/v1/events.proto: %v", err)
	}
	cs := string(common)
	if !strings.Contains(cs, `import "openits/types/v1/types.proto";`) {
		t.Errorf("openits/common/v1/events.proto must import openits/types/v1/types.proto, got:\n%s", cs)
	}
	if !strings.Contains(cs, "openits.types.v1.WireSource source = 100;") {
		t.Errorf("openits/common/v1/events.proto must reference the fully-qualified shared WireSource, got:\n%s", cs)
	}
	if strings.Contains(cs, "message WireSource {") {
		t.Errorf("openits/common/v1/events.proto must not carry its own inlined copy of the shared WireSource message, got:\n%s", cs)
	}
}

// TestGenerate_duplicateMessageNameFails exercises the message-name
// uniqueness guard (carried-over Task 6/7 requirement): two YANG modules
// mapped to the same output file (both start with "openits-common-", per
// pkgFor) each declare a notification named "foo-bar", so both would
// produce "message FooBar {" in common_events.proto. Generate must reject
// this with a clear error naming the colliding YANG origins rather than
// silently emitting a file with two declarations of the same message,
// which protoc rejects outright.
func TestGenerate_duplicateMessageNameFails(t *testing.T) {
	out := t.TempDir()
	lock := filepath.Join(out, "field-numbers.yaml")
	err := Generate(filepath.Join("testdata", "collision-msg"), out, lock)
	if err == nil {
		t.Fatal("expected Generate to fail on a duplicate message name, got nil error")
	}
	for _, want := range []string{"FooBar", "openits-common-collision-a:foo-bar", "openits-common-collision-b:foo-bar"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
	// No output must be written on a validation failure.
	if _, statErr := os.Stat(filepath.Join(out, "openits", "common", "v1", "events.proto")); statErr == nil {
		t.Error("Generate must not write output files when validation fails")
	}
}

// TestGenerate_duplicateFieldTagFails exercises the field-name/tag
// uniqueness guard: a single notification declares both "foo-bar" and
// "foo_bar" leaves, which both convert to the proto field name "foo_bar"
// (see fieldName in emit.go) and so would otherwise be silently assigned
// the same FieldLock tag (FieldLock.Assign dedupes by field *name*).  Real
// openits YANG is kebab-case only, so this can't occur in the actual
// corpus (see TestGenerate_producesBuildableProto passing cleanly over
// yang/) — this fixture proves the guard fires if it ever does.
func TestGenerate_duplicateFieldTagFails(t *testing.T) {
	out := t.TempDir()
	lock := filepath.Join(out, "field-numbers.yaml")
	err := Generate(filepath.Join("testdata", "collision-field"), out, lock)
	if err == nil {
		t.Fatal("expected Generate to fail on a duplicate field tag, got nil error")
	}
	for _, want := range []string{"Event", "duplicate field tag"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}

// TestGenerate_topLevelRPCOmittedFromConfigState guards emitModuleConfigState
// (main.go's self-contained, test-only entry point — Generate itself no
// longer calls it, see its doc comment) against mistaking a top-level `rpc`
// statement for a container root. goyang gives top-level rpc entries Kind ==
// DirectoryEntry — indistinguishable by Kind alone from a real container —
// with their input/output living under Entry.RPC, not Entry.Dir. Without an
// explicit c.RPC != nil skip, the config/state walk would queue a top-level
// rpc as a root too and emit it as an empty top-level message.
//
// This used to run Generate over the real yang/ corpus, keyed on RSU's four
// top-level RPCs (rsu-approve-srm, rsu-broadcast-tim, rsu-cancel-tim,
// rsu-refresh-certificates). Cut B retired all four in favor of
// config-driven surfaces (srm-ssm/decisions) — RSU no longer declares any
// top-level rpc statement — so the guard now runs against its own fixture
// module (testdata/rpc-fixture.yang) instead of depending on some real
// module happening to still have one. See
// TestGenerate_topLevelRPCOmittedFromConfigStateViaGenerate immediately below
// for the equivalent guard in Generate's OWN config/state loop (main.go's
// production code path, exercised by every `make gen`) — that is a
// textually-duplicated but functionally separate copy of this same check,
// and this test does not cover it.
func TestGenerate_topLevelRPCOmittedFromConfigState(t *testing.T) {
	ms := yang.NewModules()
	if err := ms.Read(filepath.Join("testdata", "rpc-fixture.yang")); err != nil {
		t.Fatalf("read: %v", err)
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process: %v", errs)
	}
	var mod *yang.Entry
	for _, m := range ms.Modules {
		mod = yang.ToEntry(m)
		break
	}
	lock := &FieldLock{Messages: map[string]map[string]int{}}
	pf := &ProtoFile{}
	emitModuleConfigState(mod, pf, lock, nil)
	got := pf.Body.String()
	if strings.Contains(got, "message RebootWidget {") {
		t.Errorf("config/state walk must not emit a message for a top-level rpc statement, got:\n%s", got)
	}
	// The real config/state tree must still be present and unaffected.
	if !strings.Contains(got, "message Widget {") {
		t.Errorf("config/state walk missing the real config/state root message, got:\n%s", got)
	}
}

// TestGenerate_topLevelRPCOmittedFromConfigStateViaGenerate drives the SAME
// guard as the test above, but through Generate itself — its own
// config/state loop (main.go, ~line 160), not emitModuleConfigState
// (main.go's self-contained test-only twin, ~line 302, which the test above
// covers). Generate no longer calls emitModuleConfigState (see Generate's doc
// comment): it recomputes the collision set per go_package across every
// pending root instead of per-module, and collects roots into its own
// pending slice. That means Generate's copy of the `if c.RPC != nil {
// continue }` skip is a textually-duplicated but functionally independent
// guard — a regression there would ship into every `make gen` run without
// failing any test unless this exact loop is exercised directly, which is
// what this test is for.
//
// configStateRoutes (pkgmap.go) is the opt-in allowlist controlling which
// modules Generate's config/state pass even looks at; rpc-fixture is a test
// fixture, not a real openits module, so it isn't on that allowlist. This
// test swaps configStateRoutes for a patched copy — the original map plus a
// synthetic route for rpc-fixture — for the duration of the call, then
// restores the original map (same reference, untouched) via defer, so
// TestConfigStateAllowlistExact (which asserts configStateRoutes' exact
// permanent contents) is unaffected regardless of test execution order.
func TestGenerate_topLevelRPCOmittedFromConfigStateViaGenerate(t *testing.T) {
	origRoutes := configStateRoutes
	patched := make(map[string]serviceRoute, len(origRoutes)+1)
	for k, v := range origRoutes {
		patched[k] = v
	}
	patched["rpc-fixture"] = serviceRoute{pkg: "openits.rpcfixture.v1", file: "openits/rpcfixture/v1/state.proto"}
	configStateRoutes = patched
	defer func() { configStateRoutes = origRoutes }()

	// Generate reads every *.yang file directly under its yangDir argument
	// (see LoadModules), not just one named module — copy the existing
	// rpc-fixture.yang fixture into an isolated temp input directory so this
	// run sees only that one module, not the rest of testdata/'s (unrelated,
	// unrouted) fixtures.
	src, err := os.ReadFile(filepath.Join("testdata", "rpc-fixture.yang"))
	if err != nil {
		t.Fatalf("read testdata/rpc-fixture.yang: %v", err)
	}
	yangDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(yangDir, "rpc-fixture.yang"), src, 0o644); err != nil {
		t.Fatalf("write rpc-fixture.yang copy: %v", err)
	}

	out := t.TempDir()
	if err := Generate(yangDir, out, filepath.Join(out, "field-numbers.yaml")); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(out, "openits", "rpcfixture", "v1", "state.proto"))
	if err != nil {
		t.Fatalf("expected openits/rpcfixture/v1/state.proto: %v", err)
	}
	got := string(b)
	if strings.Contains(got, "message RebootWidget {") {
		t.Errorf("Generate's config/state pass must not emit a message for a top-level rpc statement, got:\n%s", got)
	}
	// The real config/state tree must still be present and unaffected.
	if !strings.Contains(got, "message Widget {") {
		t.Errorf("Generate's config/state pass is missing the real config/state root message, got:\n%s", got)
	}
}

func TestPkgFor(t *testing.T) {
	cases := []struct {
		module  string
		wantPkg string
		wantOK  bool
	}{
		{"openits-common-fault-events", "openits.common.v1", true},
		{"openits-signal-control-events", "openits.signal_control.v1", true},
		{"openits-signal-control", "openits.signal_control.v1", true},
		{"openits-dms-events", "openits.dms.v1", true},
		{"openits-ess-events", "openits.ess.v1", true},
		{"openits-rsu-events", "openits.rsu.v1", true},
		{"openits-ramp-metering-events", "openits.ramp_metering.v1", true},
		{"openits-perception-events", "openits.perception.v1", true},
		{"openits-traffic-sensor-events", "openits.traffic_sensor.v1", true},
		{"openits-reversible-lane-events", "openits.reversible_lane.v1", true},
		{"openits-types", "", false},
		{"openits-nema-common", "", false},
		{"ietf-yang-types", "", false},
	}
	for _, c := range cases {
		pkg, _, ok := pkgFor(c.module)
		if ok != c.wantOK {
			t.Errorf("pkgFor(%q) ok = %v, want %v", c.module, ok, c.wantOK)
			continue
		}
		if ok && pkg != c.wantPkg {
			t.Errorf("pkgFor(%q) pkg = %q, want %q", c.module, pkg, c.wantPkg)
		}
	}
}

// TestGenerate_badYangDirSurfacesError guards against silently swallowing a
// bad -yang path. Before the fix, LoadModules treated a failed os.ReadDir on
// yangDir itself exactly like the optional yangDir/ietf subdirectory not
// existing — silently skip it — so a nonexistent -yang path produced zero
// modules, zero errors, and Generate quietly wrote zero output files
// instead of failing. LoadModules now only tolerates a missing ietf/
// subdirectory; a missing/unreadable yangDir itself is a hard error.
func TestGenerate_badYangDirSurfacesError(t *testing.T) {
	out := t.TempDir()
	lock := filepath.Join(out, "field-numbers.yaml")

	nonexistent := filepath.Join(out, "does-not-exist")
	if err := Generate(nonexistent, out, lock); err == nil {
		t.Error("expected Generate to fail for a nonexistent -yang directory, got nil error")
	}

	badDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(badDir, "not-yang.yang"), []byte("this is not valid yang { { {"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := Generate(badDir, out, lock); err == nil {
		t.Error("expected Generate to return an error for an invalid YANG file, got nil")
	}
}

// findCrossFileMessageCollisions must flag a bare message name declared in two
// different output files that share a go_package (Go go_package flatten +
// field-number-lock conflation), stay silent when every name is unique to
// one file, and — the whole point of per-service packaging — stay silent
// when the colliding name's two files route to DIFFERENT go_packages.
func TestFindCrossFileMessageCollisions(t *testing.T) {
	clash := map[string]string{
		"a.proto": "message Foo {\n  string x = 1;\n}\n",
		"b.proto": "message Foo {\n  string y = 1;\n}\n",
		"c.proto": "message Bar {\n  string z = 1;\n}\n",
	}
	clashPkg := map[string]string{
		"a.proto": "openits.svc.v1",
		"b.proto": "openits.svc.v1",
		"c.proto": "openits.svc.v1",
	}
	got := findCrossFileMessageCollisions(clash, clashPkg)
	if len(got) != 1 || !strings.Contains(got[0], "Foo") {
		t.Fatalf("expected a single Foo collision, got %v", got)
	}
	if !strings.Contains(got[0], "a.proto") || !strings.Contains(got[0], "b.proto") {
		t.Errorf("collision should name both files: %v", got)
	}

	// A message nested inside another (different file, same go_package)
	// still collides by bare name, because the field-number lock keys by
	// bare name.
	nested := map[string]string{
		"types.proto": "message Wrap {\n  message Inner {\n    string a = 1;\n  }\n}\n",
		"svc.proto":   "message Inner {\n  string b = 1;\n}\n",
	}
	nestedPkg := map[string]string{
		"types.proto": "openits.types.v1",
		"svc.proto":   "openits.types.v1",
	}
	if got := findCrossFileMessageCollisions(nested, nestedPkg); len(got) != 1 || !strings.Contains(got[0], "Inner") {
		t.Errorf("expected an Inner collision across nested+top-level, got %v", got)
	}

	unique := map[string]string{
		"a.proto": "message Foo {\n  string x = 1;\n}\n",
		"b.proto": "message Bar {\n  string y = 1;\n}\n",
	}
	uniquePkg := map[string]string{
		"a.proto": "openits.svc.v1",
		"b.proto": "openits.svc.v1",
	}
	if got := findCrossFileMessageCollisions(unique, uniquePkg); len(got) != 0 {
		t.Errorf("expected no collisions for unique names, got %v", got)
	}

	// The crux of per-service packaging: the SAME bare name in two files
	// that route to DIFFERENT go_packages (two distinct services, e.g. one
	// declaring "Detector" in signal-control's package and the other in
	// ramp-metering's) must NOT be flagged.
	crossService := map[string]string{
		"signal_control/v1/events.proto": "message Detector {\n  string x = 1;\n}\n",
		"ramp_metering/v1/state.proto":   "message Detector {\n  string y = 1;\n}\n",
	}
	crossServicePkg := map[string]string{
		"signal_control/v1/events.proto": "openits.signal_control.v1",
		"ramp_metering/v1/state.proto":   "openits.ramp_metering.v1",
	}
	if got := findCrossFileMessageCollisions(crossService, crossServicePkg); len(got) != 0 {
		t.Errorf("expected no collision for the same bare name in two different go_packages, got %v", got)
	}
}
