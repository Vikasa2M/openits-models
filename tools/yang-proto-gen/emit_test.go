package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
)

func readGolden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return string(b)
}

func TestEmitMessage_leavesContainers(t *testing.T) {
	ms := yang.NewModules()
	for _, f := range []string{
		filepath.Join("testdata", "leaf-fixture.yang"),
		filepath.Join("..", "..", "yang", "ietf", "ietf-yang-types.yang"),
	} {
		if err := ms.Read(f); err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process: %v", errs)
	}
	mod, errs := ms.GetModule("leaf-fixture")
	if len(errs) > 0 {
		t.Fatalf("getmodule: %v", errs)
	}
	lock := &FieldLock{Messages: map[string]map[string]int{}}
	var pf ProtoFile
	for _, c := range sortedChildren(mod) {
		if c.Kind == yang.NotificationEntry {
			EmitMessage(c, ProtoName(c.Name), lock, nil, &pf)
		}
	}
	got := strings.TrimSpace(pf.Body.String())
	want := strings.TrimSpace(readGolden(t, "leaf-fixture.golden"))
	if got != want {
		t.Errorf("proto mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestEmitMessage_choice(t *testing.T) {
	ms := yang.NewModules()
	if err := ms.Read(filepath.Join("testdata", "choice-fixture.yang")); err != nil {
		t.Fatalf("read choice-fixture.yang: %v", err)
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process: %v", errs)
	}
	mod, errs := ms.GetModule("choice-fixture")
	if len(errs) > 0 {
		t.Fatalf("getmodule: %v", errs)
	}
	lock := &FieldLock{Messages: map[string]map[string]int{}}
	var pf ProtoFile
	for _, c := range sortedChildren(mod) {
		if c.Kind == yang.NotificationEntry {
			EmitMessage(c, ProtoName(c.Name), lock, nil, &pf)
		}
	}
	got := strings.TrimSpace(pf.Body.String())
	want := strings.TrimSpace(readGolden(t, "choice-fixture.golden"))
	if got != want {
		t.Errorf("proto mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestEmitMessage_choiceMultiLeafCase(t *testing.T) {
	ms := yang.NewModules()
	if err := ms.Read(filepath.Join("testdata", "choice-multi-fixture.yang")); err != nil {
		t.Fatalf("read choice-multi-fixture.yang: %v", err)
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process: %v", errs)
	}
	mod, errs := ms.GetModule("choice-multi-fixture")
	if len(errs) > 0 {
		t.Fatalf("getmodule: %v", errs)
	}
	lock := &FieldLock{Messages: map[string]map[string]int{}}
	var pf ProtoFile
	for _, c := range sortedChildren(mod) {
		if c.Kind == yang.NotificationEntry {
			EmitMessage(c, ProtoName(c.Name), lock, nil, &pf)
		}
	}
	got := strings.TrimSpace(pf.Body.String())
	want := strings.TrimSpace(readGolden(t, "choice-multi-fixture.golden"))
	if got != want {
		t.Errorf("proto mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestEmitMessage_enum(t *testing.T) {
	ms := yang.NewModules()
	if err := ms.Read(filepath.Join("testdata", "enum-fixture.yang")); err != nil {
		t.Fatalf("read enum-fixture.yang: %v", err)
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process: %v", errs)
	}
	mod, errs := ms.GetModule("enum-fixture")
	if len(errs) > 0 {
		t.Fatalf("getmodule: %v", errs)
	}
	lock := &FieldLock{Messages: map[string]map[string]int{}}
	var pf ProtoFile
	for _, c := range sortedChildren(mod) {
		if c.Kind == yang.NotificationEntry {
			EmitMessage(c, ProtoName(c.Name), lock, nil, &pf)
		}
	}
	got := strings.TrimSpace(pf.Body.String())
	want := strings.TrimSpace(readGolden(t, "enum-fixture.golden"))
	if got != want {
		t.Errorf("proto mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestEmitEnum_unspecifiedMemberNoDuplicate guards the fix for a real
// protoc-rejected bug: a YANG enum whose first declared member is literally
// named `unspecified` (e.g. openits-signal-control-events'
// change-kind leaf) gets YANG value 0 for that member. Before the fix,
// emitEnum always prepended a synthetic `<PREFIX>_UNSPECIFIED = 0` sentinel
// regardless of the enum's own contents, colliding with the proto value
// name the real `unspecified` member maps to and producing two
// `CHANGE_KIND_UNSPECIFIED` entries in the same enum — which protoc rejects
// with "CHANGE_KIND_UNSPECIFIED is already defined". The fix must emit the
// real member as the proto zero value and skip the synthetic entirely.
func TestEmitEnum_unspecifiedMemberNoDuplicate(t *testing.T) {
	ms := yang.NewModules()
	if err := ms.Read(filepath.Join("testdata", "enum-unspecified-fixture.yang")); err != nil {
		t.Fatalf("read enum-unspecified-fixture.yang: %v", err)
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process: %v", errs)
	}
	mod, errs := ms.GetModule("enum-unspecified-fixture")
	if len(errs) > 0 {
		t.Fatalf("getmodule: %v", errs)
	}
	lock := &FieldLock{Messages: map[string]map[string]int{}}
	var pf ProtoFile
	for _, c := range sortedChildren(mod) {
		if c.Kind == yang.NotificationEntry {
			EmitMessage(c, ProtoName(c.Name), lock, nil, &pf)
		}
	}
	body := pf.Body.String()

	if n := strings.Count(body, "CHANGE_KIND_UNSPECIFIED"); n != 1 {
		t.Fatalf("expected exactly one CHANGE_KIND_UNSPECIFIED entry, got %d:\n%s", n, body)
	}
	if !strings.Contains(body, "CHANGE_KIND_UNSPECIFIED = 0;") {
		t.Errorf("expected CHANGE_KIND_UNSPECIFIED to be the zero value, got:\n%s", body)
	}
	for _, want := range []string{"CHANGE_KIND_PATTERN = 1;", "CHANGE_KIND_CYCLE_LENGTH = 2;"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q, got:\n%s", want, body)
		}
	}
}

// emitModuleNotifications loads the real repo YANG (../../yang from the tool
// dir), computes shared groupings across the whole module set, and emits
// every notification in module modName into one ProtoFile — exercising the
// shared-grouping reference path exactly as a real multi-notification module
// would.
func emitModuleNotifications(t *testing.T, modName string) string {
	t.Helper()
	yangDir := filepath.Join("..", "..", "yang")
	_, mods, err := LoadModules(yangDir)
	if err != nil {
		t.Fatalf("LoadModules: %v", err)
	}
	shared := SharedGroupings(mods)

	var mod *yang.Entry
	for _, m := range mods {
		if m.Name == modName {
			mod = m
		}
	}
	if mod == nil {
		t.Fatalf("module %s not loaded", modName)
	}

	lock := &FieldLock{Messages: map[string]map[string]int{}}
	var pf ProtoFile
	for _, c := range sortedChildren(mod) {
		if c.Kind == yang.NotificationEntry {
			EmitMessage(c, ProtoName(c.Name), lock, shared, &pf)
		}
	}
	return pf.Body.String()
}

func TestEmit_wireSourceSharedOnce(t *testing.T) {
	// openits-common-fault-events hosts 2 notifications (fault-raised,
	// fault-cleared) since controller-fault-event moved to
	// openits-signal-control-events.
	body := emitModuleNotifications(t, "openits-common-fault-events")
	if strings.Count(body, "message WireSource {") != 1 {
		t.Fatalf("expected exactly one WireSource message, got %d:\n%s",
			strings.Count(body, "message WireSource {"), body)
	}
	if strings.Count(body, "WireSource source = 100;") < 2 {
		t.Errorf("each notification should reference WireSource source = 100")
	}
}

// messageBlockText returns the full text of the top-level "message NAME {
// ... }" block in body, including its closing brace line, for tests that
// need to inspect one message's fields without a plain substring match
// leaking into a sibling message (mirrors extractMessageBlocks in main.go,
// which returns only the tags — this keeps the text too). A message's own
// block always ends at the first line that is exactly "}" with no leading
// whitespace; a `oneof` block's closing brace is always indented, so it
// never terminates the scan early.
func messageBlockText(t *testing.T, body, name string) string {
	t.Helper()
	lines := strings.Split(body, "\n")
	header := "message " + name + " {"
	for i, line := range lines {
		if line != header {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			if lines[j] == "}" {
				return strings.Join(lines[i:j+1], "\n")
			}
		}
	}
	t.Fatalf("message %s not found in:\n%s", name, body)
	return ""
}

// TestEmit_reservedTagsAreTypeAware_fixture guards the type-aware reserved
// tag fix (fieldnum.go's Assign + emit.go's reservedFieldTags): tag 100 is
// reserved only for the message-typed field the `wire-source` grouping
// produces, and tag 99 only for an identityref `kind` leaf — never merely
// because a field happens to be *named* source/kind. Before the fix,
// FieldLock.Assign reserved tag 100 for ANY field named "source" (99 for
// "kind") purely by name.
//
// The fixture (testdata/reserved-tag-fixture.yang) exercises all three
// shapes in one isolated module: scalar-source-event's kind/source are
// ordinary string leaves; wire-source-event-a/b `uses` a container-shaped
// "wire-source" grouping (mirroring the real openits-types:wire-source
// shape) at two sites so SharedGroupings treats it as shared exactly like
// the real corpus; identityref-kind-event's kind is a genuine identityref.
func TestEmit_reservedTagsAreTypeAware_fixture(t *testing.T) {
	ms := yang.NewModules()
	if err := ms.Read(filepath.Join("testdata", "reserved-tag-fixture.yang")); err != nil {
		t.Fatalf("read reserved-tag-fixture.yang: %v", err)
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process: %v", errs)
	}
	mod, errs := ms.GetModule("reserved-tag-fixture")
	if len(errs) > 0 {
		t.Fatalf("getmodule: %v", errs)
	}

	shared := SharedGroupings([]*yang.Entry{mod})
	if _, ok := shared["wire-source"]; !ok {
		t.Fatalf("fixture setup broken: wire-source not detected as shared, got: %v", shared)
	}

	lock := &FieldLock{Messages: map[string]map[string]int{}}
	var pf ProtoFile
	for _, c := range sortedChildren(mod) {
		if c.Kind == yang.NotificationEntry {
			EmitMessage(c, ProtoName(c.Name), lock, shared, &pf)
		}
	}
	body := pf.Body.String()

	scalar := messageBlockText(t, body, "ScalarSourceEvent")
	if strings.Contains(scalar, "source = 100;") {
		t.Errorf("scalar `source` leaf must not get reserved tag 100, got:\n%s", scalar)
	}
	if !strings.Contains(scalar, "string source = ") {
		t.Errorf("expected a plain string source field, got:\n%s", scalar)
	}
	if strings.Contains(scalar, "kind = 99;") {
		t.Errorf("scalar `kind` leaf must not get reserved tag 99, got:\n%s", scalar)
	}
	if !strings.Contains(scalar, "string kind = ") {
		t.Errorf("expected a plain string kind field, got:\n%s", scalar)
	}

	for _, msg := range []string{"WireSourceEventA", "WireSourceEventB"} {
		block := messageBlockText(t, body, msg)
		if !strings.Contains(block, "WireSource source = 100;") {
			t.Errorf("%s's WireSource-ref `source` must keep reserved tag 100, got:\n%s", msg, block)
		}
	}

	idKind := messageBlockText(t, body, "IdentityrefKindEvent")
	if !strings.Contains(idKind, "kind = 99;") {
		t.Errorf("identityref `kind` leaf must keep reserved tag 99, got:\n%s", idKind)
	}
}

// TestEmit_reservedTagsAreTypeAware_realCorpus is the real-corpus companion
// to TestEmit_reservedTagsAreTypeAware_fixture: openits-rsu-events'
// rsu-security-event and rsu-tim-loaded notifications each declare a plain
// `leaf source { type string; }` (see yang/openits-rsu-events.yang)
// that must NOT collide with tag 100, while openits-common-fault-events'
// fault-raised notification's `source` is the real WireSource
// shared-message reference (via `uses openits-types:wire-source`) and must
// keep it.
func TestEmit_reservedTagsAreTypeAware_realCorpus(t *testing.T) {
	rsuBody := emitModuleNotifications(t, "openits-rsu-events")
	for _, msg := range []string{"RsuSecurityEvent", "RsuTimLoaded"} {
		block := messageBlockText(t, rsuBody, msg)
		if strings.Contains(block, "source = 100;") {
			t.Errorf("%s's scalar `source` leaf must not get reserved tag 100, got:\n%s", msg, block)
		}
		if !strings.Contains(block, "string source = ") {
			t.Errorf("%s must still declare a plain string source field, got:\n%s", msg, block)
		}
	}

	faultBody := emitModuleNotifications(t, "openits-common-fault-events")
	block := messageBlockText(t, faultBody, "FaultRaised")
	if !strings.Contains(block, "WireSource source = 100;") {
		t.Errorf("FaultRaised's WireSource-ref `source` must keep reserved tag 100, got:\n%s", block)
	}
}

// enumBlock returns the text of the "enum <name> { ... }" block in body,
// from its opening brace to its closing "}", for tests that need to check
// one enum's values without a plain substring match accidentally matching
// inside a different enum's longer, qualified value names (e.g. "STATE_X"
// is a substring of "ENUM_COLLISION_B_STATE_X").
func enumBlock(t *testing.T, body, name string) string {
	t.Helper()
	start := strings.Index(body, "enum "+name+" {")
	if start == -1 {
		t.Fatalf("enum %s not found in:\n%s", name, body)
	}
	end := strings.Index(body[start:], "}\n")
	if end == -1 {
		t.Fatalf("enum %s block not terminated in:\n%s", name, body)
	}
	return body[start : start+end]
}

// findEntry descends mod.Dir following path segments (YANG identifiers),
// for tests that need to reach a specific nested container in the real
// yang/ tree rather than iterating a module's top-level notifications.
func findEntry(t *testing.T, mod *yang.Entry, path ...string) *yang.Entry {
	t.Helper()
	e := mod
	for _, p := range path {
		next, ok := e.Dir[p]
		if !ok {
			t.Fatalf("entry %q not found under %s", p, e.Path())
		}
		e = next
	}
	return e
}

// TestEmit_flatGroupingInlined guards the container-vs-flat grouping
// semantics: openits-types:geo-location is a flat grouping (bare leaves:
// latitude, longitude, elevation, heading), `uses`-d inside
// openits-perception's perception-sensor/identity container. RFC 7950
// splices a `uses`d flat grouping's leaves directly into the parent's data
// tree, so the proto emitted for Identity must match — latitude/longitude
// as direct fields of Identity — with no separate GeoLocation message,
// regardless of how many other sites also use geo-location (there are
// several; see TestSharedGroupings_onlyContainerGroupings).
func TestEmit_flatGroupingInlined(t *testing.T) {
	yangDir := filepath.Join("..", "..", "yang")
	_, mods, err := LoadModules(yangDir)
	if err != nil {
		t.Fatalf("LoadModules: %v", err)
	}
	shared := SharedGroupings(mods)

	var perception *yang.Entry
	for _, m := range mods {
		if m.Name == "openits-perception" {
			perception = m
		}
	}
	if perception == nil {
		t.Fatalf("module openits-perception not loaded")
	}
	// perception-sensor/config carries the sensor-identity-config grouping,
	// which uses the flat geo-location grouping — the subject of this test.
	config := findEntry(t, perception, "perception-sensor", "config")

	lock := &FieldLock{Messages: map[string]map[string]int{}}
	var pf ProtoFile
	EmitMessage(config, "Config", lock, shared, &pf)
	body := pf.Body.String()

	// latitude/longitude are decimal64 in YANG, which this emitter maps to
	// proto "string" per RFC 7951 (decimal64 is a JSON string on the
	// wire) — see ProtoScalar. The scalar type isn't what this test is
	// about; what matters is that they appear as direct fields of
	// Identity, not nested under a shared/nested GeoLocation message.
	for _, want := range []string{"string latitude = ", "string longitude = "} {
		if !strings.Contains(body, want) {
			t.Errorf("expected geo-location leaf %q inlined as a direct field of Config, got:\n%s", want, body)
		}
	}
	if strings.Contains(body, "message GeoLocation") {
		t.Errorf("geo-location is a flat grouping and must never be emitted as its own message:\n%s", body)
	}
}

// TestEmit_enumNoCrossModuleCollision guards the enum registry against
// keying solely by the bare YANG identifier: two unrelated modules each
// declare an inline `enumeration` on a same-named leaf ("state") with
// different, partially-overlapping value sets (both include "active"),
// mirroring the real prior/current collision between
// openits-rsu-events and openits-signal-control-events.
// Both enums must emit in full, with distinct type names and distinct
// value identifiers — no overwriting/merging.
func TestEmit_enumNoCrossModuleCollision(t *testing.T) {
	ms := yang.NewModules()
	for _, f := range []string{
		filepath.Join("testdata", "enum-collision-a.yang"),
		filepath.Join("testdata", "enum-collision-b.yang"),
	} {
		if err := ms.Read(f); err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process: %v", errs)
	}

	modA, errs := ms.GetModule("enum-collision-a")
	if len(errs) > 0 {
		t.Fatalf("getmodule a: %v", errs)
	}
	modB, errs := ms.GetModule("enum-collision-b")
	if len(errs) > 0 {
		t.Fatalf("getmodule b: %v", errs)
	}

	lock := &FieldLock{Messages: map[string]map[string]int{}}
	var pf ProtoFile
	for _, c := range sortedChildren(modA) {
		if c.Kind == yang.NotificationEntry {
			EmitMessage(c, ProtoName(c.Name), lock, nil, &pf)
		}
	}
	for _, c := range sortedChildren(modB) {
		if c.Kind == yang.NotificationEntry {
			EmitMessage(c, ProtoName(c.Name), lock, nil, &pf)
		}
	}
	body := pf.Body.String()

	if strings.Count(body, "enum State {") != 1 {
		t.Fatalf("expected exactly one bare `enum State {` (first-seen variant keeps the bare name), got %d:\n%s",
			strings.Count(body, "enum State {"), body)
	}
	if !strings.Contains(body, "enum EnumCollisionBState {") {
		t.Fatalf("expected module-b's colliding State enum to emit under a distinct, module-qualified name, got:\n%s", body)
	}

	// Isolate each enum's own block before checking its values, since
	// e.g. "STATE_ADVISORY" is a substring of the qualified
	// "ENUM_COLLISION_B_STATE_ADVISORY" — an unscoped Contains check on
	// the whole body would pass even if the values leaked across enums.
	blockA := enumBlock(t, body, "State")
	for _, want := range []string{"STATE_IDLE", "STATE_ACTIVE", "STATE_FAULT"} {
		if !strings.Contains(blockA, want) {
			t.Errorf("expected %q in module-a's bare State enum, got block:\n%s", want, blockA)
		}
	}
	for _, notWant := range []string{"DISABLED", "ADVISORY", "SUSPENDED"} {
		if strings.Contains(blockA, notWant) {
			t.Errorf("module-b's enum value %q leaked into module-a's bare State enum, got block:\n%s", notWant, blockA)
		}
	}

	// module-b's State is a different enum (disabled/advisory/active/
	// suspended) and must not have been silently dropped/overwritten: it
	// gets its own module-qualified name and value prefix.
	blockB := enumBlock(t, body, "EnumCollisionBState")
	for _, want := range []string{
		"ENUM_COLLISION_B_STATE_DISABLED",
		"ENUM_COLLISION_B_STATE_ADVISORY",
		"ENUM_COLLISION_B_STATE_ACTIVE",
		"ENUM_COLLISION_B_STATE_SUSPENDED",
	} {
		if !strings.Contains(blockB, want) {
			t.Errorf("expected %q in module-b's qualified State enum, got block:\n%s", want, blockB)
		}
	}
	if strings.Contains(blockB, "FAULT") || strings.Contains(blockB, "IDLE") {
		t.Errorf("module-a's enum values leaked into module-b's qualified State enum, got block:\n%s", blockB)
	}

	// Field references must point at the right enum for their own message.
	if !strings.Contains(body, "State state = ") {
		t.Errorf("expected event-a's state field to reference the bare State enum, got:\n%s", body)
	}
	if !strings.Contains(body, "EnumCollisionBState state = ") {
		t.Errorf("expected event-b's state field to reference the qualified EnumCollisionBState enum, got:\n%s", body)
	}
}
