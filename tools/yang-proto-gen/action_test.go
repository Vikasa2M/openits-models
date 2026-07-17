package main

import (
	"strings"
	"testing"
)

func TestActionEmitsRequestResponse(t *testing.T) {
	root := loadFixtureEntry(t, "action-fixture.yang", "device")
	lock := &FieldLock{Messages: map[string]map[string]int{}}
	pf := &ProtoFile{}
	EmitMessage(root, "Device", lock, nil, pf)
	got := pf.Body.String()

	for _, want := range []string{
		"message RebootRequest {",
		"uint32 delay_seconds = 1;",
		"string reason = 2;",
		"message RebootResponse {",
		"bool accepted = 1;",
		"uint32 estimated_downtime_seconds = 2;",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	// The action must NOT appear as a field of the enclosing Device message.
	deviceBlock := got[strings.Index(got, "message Device {"):]
	deviceBlock = deviceBlock[:strings.Index(deviceBlock, "}")]
	if strings.Contains(deviceBlock, "reboot") {
		t.Errorf("action leaked as a Device field:\n%s", deviceBlock)
	}
}
