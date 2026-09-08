package main

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
)

// emitPresenceFixture renders presence-fixture.yang's single notification and
// returns the proto body. The fixture carries one leaf of each shape the
// presence rule has to discriminate: a directly-mandatory leaf, a plain
// optional leaf, a date-and-time leaf (message-typed on the wire), a
// leaf-list, and a grouping member tightened by `refine mandatory true`.
func emitPresenceFixture(t *testing.T) string {
	t.Helper()
	ms := yang.NewModules()
	for _, f := range []string{
		filepath.Join("testdata", "presence-fixture.yang"),
		filepath.Join("..", "..", "yang", "ietf", "ietf-yang-types.yang"),
	} {
		if err := ms.Read(f); err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
	}
	if errs := ms.Process(); len(errs) > 0 {
		t.Fatalf("process: %v", errs)
	}
	mod, errs := ms.GetModule("presence-fixture")
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
	return pf.Body.String()
}

// fieldLine returns the emitted line declaring proto field name, without its
// leading indentation, or "" when no such field was emitted.
func fieldLine(t *testing.T, body, name string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^\s*(.*\b` + regexp.QuoteMeta(name) + ` = \d+;)\s*$`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// A leaf that is not mandatory can legitimately be absent, and proto3's
// implicit presence cannot say so: absent and zero are the same bytes. The
// emitter must label it `optional` so the binding carries what the model
// already encodes.
func TestEmitMessage_nonMandatoryLeafGetsOptional(t *testing.T) {
	body := emitPresenceFixture(t)

	got := fieldLine(t, body, "note")
	if want := "optional string note"; !strings.HasPrefix(got, want) {
		t.Errorf("non-mandatory leaf: got %q, want it to start with %q", got, want)
	}
}

// A leaf the model declares mandatory is always on the wire, so implicit
// presence is unambiguous for it and `optional` would be noise.
func TestEmitMessage_mandatoryLeafStaysBare(t *testing.T) {
	body := emitPresenceFixture(t)

	got := fieldLine(t, body, "kind")
	if strings.HasPrefix(got, "optional ") {
		t.Errorf("mandatory leaf: got %q, want no optional label", got)
	}
}

// `refine mandatory true` is left on the uses statement by goyang rather than
// applied to the merged child entry, so an emitter that reads only
// Entry.Mandatory would label zone-occupancy-changed's refined `presence`
// optional and lose the guarantee the notification tightened it to give.
func TestEmitMessage_refinedMandatoryLeafStaysBare(t *testing.T) {
	body := emitPresenceFixture(t)

	got := fieldLine(t, body, "presence")
	if strings.HasPrefix(got, "optional ") {
		t.Errorf("refined-mandatory grouping member: got %q, want no optional label", got)
	}
}

// A grouping member nobody refined stays optional, so the refine lookup must
// not mark every member of a refined `uses` mandatory.
func TestEmitMessage_unrefinedGroupingMemberGetsOptional(t *testing.T) {
	body := emitPresenceFixture(t)

	got := fieldLine(t, body, "tally")
	if want := "optional uint32 tally"; !strings.HasPrefix(got, want) {
		t.Errorf("unrefined grouping member: got %q, want it to start with %q", got, want)
	}
}

// Message-typed fields already carry explicit presence in proto3, and
// repeated fields have no presence to add; both must stay bare.
func TestEmitMessage_messageAndRepeatedFieldsStayBare(t *testing.T) {
	body := emitPresenceFixture(t)

	if got := fieldLine(t, body, "occurred_at"); strings.HasPrefix(got, "optional ") {
		t.Errorf("timestamp field: got %q, want no optional label", got)
	}
	if got := fieldLine(t, body, "tag"); !strings.HasPrefix(got, "repeated ") {
		t.Errorf("leaf-list field: got %q, want it to stay repeated", got)
	}
}
