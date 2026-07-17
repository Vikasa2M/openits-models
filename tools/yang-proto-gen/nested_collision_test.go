package main

import (
	"strings"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
)

func TestNestedListCollisionIsQualified(t *testing.T) {
	ms := yang.NewModules()
	if err := ms.Read("testdata/nested-collision-fixture.yang"); err != nil {
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
	pf := &ProtoFile{ClaimedNames: map[string]bool{}}
	pf.Collisions = collisionSet(mod.Dir["device"])
	EmitMessage(mod.Dir["device"], "Device", lock, nil, pf)
	got := pf.Body.String()
	// A colliding bare name is qualified SYMMETRICALLY: BOTH lists become
	// AlphaSensor/BetaSensor, no bare "Sensor" survives. This is order-
	// independent — neither sibling keeps the bare name — so reordering the
	// two containers in the YANG can never silently rename a proto message.
	if strings.Contains(got, "message Sensor {") {
		t.Errorf("bare 'message Sensor' present — collision not symmetrically qualified:\n%s", got)
	}
	if !strings.Contains(got, "message AlphaSensor {") || !strings.Contains(got, "message BetaSensor {") {
		t.Errorf("expected BOTH AlphaSensor and BetaSensor:\n%s", got)
	}
	// Both parents still reference their own sensor message as a repeated field.
	if !strings.Contains(got, "message Alpha {") || !strings.Contains(got, "message Beta {") {
		t.Errorf("missing parent messages:\n%s", got)
	}
}
