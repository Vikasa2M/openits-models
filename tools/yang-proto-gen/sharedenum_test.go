package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
)

// newTestEnumPackage returns a registry whose enums land in a per-package
// types file, plus that file, mirroring what Generate builds per proto
// package.
func newTestEnumPackage(importPath string) (*EnumRegistry, *ProtoFile) {
	typesFile := &ProtoFile{SelfImportPath: importPath}
	r := newEnumRegistry()
	r.Target = typesFile
	r.ImportPath = importPath
	typesFile.ClaimedEnums = r
	return r, typesFile
}

// emitFixtureInto renders a fixture module's notifications into pf.
func emitFixtureInto(t *testing.T, pf *ProtoFile, fixture, modName string) string {
	t.Helper()
	ms := yang.NewModules()
	if err := ms.Read(filepath.Join("testdata", fixture)); err != nil {
		t.Fatalf("read %s: %v", fixture, err)
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process: %v", errs)
	}
	mod, errs := ms.GetModule(modName)
	if len(errs) > 0 {
		t.Fatalf("getmodule: %v", errs)
	}
	lock := &FieldLock{Messages: map[string]map[string]int{}}
	for _, c := range sortedChildren(mod) {
		if c.Kind == yang.NotificationEntry {
			EmitMessage(c, ProtoName(c.Name), lock, nil, pf)
		}
	}
	return pf.Body.String()
}

func emitEnumFixtureInto(t *testing.T, pf *ProtoFile) string {
	t.Helper()
	return emitFixtureInto(t, pf, "enum-fixture.yang", "enum-fixture")
}

// Every enum a service declares lives in that service's types file, whether
// one output file uses it or several. A single always-applied rule beats one
// that triggers on a second reference: the conditional version makes an
// enum's home depend on how many files happen to use it, which is the
// emergent behavior that produced the cross-file rename this mechanism
// replaced. The shared types file is also the shape OpenConfig settled on
// (openconfig-*-types) and the one this emitter already uses for shared
// messages via TypesTarget.
func TestEmitEnum_declaredInPerPackageTypesFile(t *testing.T) {
	reg, typesFile := newTestEnumPackage("openits/demo/v1/types.proto")
	state := &ProtoFile{ClaimedEnums: reg, SelfImportPath: "openits/demo/v1/state.proto"}

	stateBody := emitEnumFixtureInto(t, state)

	if strings.Contains(stateBody, "enum Severity {") {
		t.Errorf("the using file must not declare the enum itself:\n%s", stateBody)
	}
	if n := strings.Count(typesFile.Body.String(), "enum Severity {"); n != 1 {
		t.Errorf("types file declared Severity %d times, want 1:\n%s", n, typesFile.Body.String())
	}
	if !state.Imports["openits/demo/v1/types.proto"] {
		t.Errorf("using file imports = %v, want it to import the types file", state.Imports)
	}
}

// Two files in one package share the single declaration, and neither is
// forced to qualify. A proto type is identified by package plus name, so
// both reference the same bare name.
func TestEmitEnum_twoFilesShareOneDeclaration(t *testing.T) {
	reg, typesFile := newTestEnumPackage("openits/demo/v1/types.proto")
	state := &ProtoFile{ClaimedEnums: reg, SelfImportPath: "openits/demo/v1/state.proto"}
	events := &ProtoFile{ClaimedEnums: reg, SelfImportPath: "openits/demo/v1/events.proto"}

	emitEnumFixtureInto(t, state)
	emitEnumFixtureInto(t, events)

	if n := strings.Count(typesFile.Body.String(), "enum Severity {"); n != 1 {
		t.Errorf("types file declared Severity %d times, want 1", n)
	}
	if strings.Contains(typesFile.Body.String(), "OpenitsEnumFixtureSeverity") {
		t.Errorf("neither file should force qualification:\n%s", typesFile.Body.String())
	}
	for _, f := range []*ProtoFile{state, events} {
		if !f.Imports["openits/demo/v1/types.proto"] {
			t.Errorf("%s imports = %v, want the types file", f.SelfImportPath, f.Imports)
		}
	}
}

// The state tree must not depend on the event surface. The YANG layering
// keeps cores and events separate (check-events-layering enforces it), and
// the binding should not invent an edge the model does not have.
func TestEmitEnum_stateNeverImportsEvents(t *testing.T) {
	reg, _ := newTestEnumPackage("openits/demo/v1/types.proto")
	_ = reg
	events := &ProtoFile{ClaimedEnums: reg, SelfImportPath: "openits/demo/v1/events.proto"}
	state := &ProtoFile{ClaimedEnums: reg, SelfImportPath: "openits/demo/v1/state.proto"}

	emitEnumFixtureInto(t, events) // events emits first, as Generate does
	emitEnumFixtureInto(t, state)

	if state.Imports["openits/demo/v1/events.proto"] {
		t.Errorf("state must not import events: %v", state.Imports)
	}
	if events.Imports["openits/demo/v1/state.proto"] {
		t.Errorf("events must not import state: %v", events.Imports)
	}
}

// The types file holding the declaration must not import itself.
func TestEmitEnum_typesFileDoesNotSelfImport(t *testing.T) {
	_, typesFile := newTestEnumPackage("openits/demo/v1/types.proto")

	emitEnumFixtureInto(t, typesFile)

	if typesFile.Imports["openits/demo/v1/types.proto"] {
		t.Errorf("types file self-imported: %v", typesFile.Imports)
	}
	if n := strings.Count(typesFile.Body.String(), "enum Severity {"); n != 1 {
		t.Errorf("types file declared Severity %d times, want 1", n)
	}
}

// Sharing is scoped to one proto package: moving an enum across packages
// would change its fully-qualified name, the exact break this avoids.
func TestEmitEnum_notSharedAcrossPackages(t *testing.T) {
	regA, typesA := newTestEnumPackage("openits/a/v1/types.proto")
	regB, typesB := newTestEnumPackage("openits/b/v1/types.proto")
	a := &ProtoFile{ClaimedEnums: regA, SelfImportPath: "openits/a/v1/state.proto"}
	b := &ProtoFile{ClaimedEnums: regB, SelfImportPath: "openits/b/v1/state.proto"}

	emitEnumFixtureInto(t, a)
	emitEnumFixtureInto(t, b)

	for _, tf := range []*ProtoFile{typesA, typesB} {
		if n := strings.Count(tf.Body.String(), "enum Severity {"); n != 1 {
			t.Errorf("%s declared Severity %d times, want its own copy", tf.SelfImportPath, n)
		}
	}
	if b.Imports["openits/a/v1/types.proto"] {
		t.Errorf("package b must not import across packages: %v", b.Imports)
	}
}

// A genuinely different enum sharing a base name still qualifies: the
// registry keys on the value set, not the name alone.
func TestEmitEnum_differentValueSetStillQualifies(t *testing.T) {
	reg, typesFile := newTestEnumPackage("openits/demo/v1/types.proto")
	first := &ProtoFile{ClaimedEnums: reg, SelfImportPath: "openits/demo/v1/state.proto"}
	second := &ProtoFile{ClaimedEnums: reg, SelfImportPath: "openits/demo/v1/events.proto"}

	emitEnumFixtureInto(t, first)
	emitFixtureInto(t, second, "enum-collision-fixture.yang", "enum-collision-fixture")

	body := typesFile.Body.String()
	if n := strings.Count(body, "enum Severity {"); n != 1 {
		t.Errorf("the first Severity should keep the bare name exactly once:\n%s", body)
	}
	if !strings.Contains(body, "Severity {") || strings.Count(body, "Severity {") < 2 {
		t.Errorf("the disjoint Severity should get its own qualified declaration:\n%s", body)
	}
}
