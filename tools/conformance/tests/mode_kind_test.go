package tests

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	commonv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/common/v1"
)

func modeEv(service, kind, prev, mode string) EventEnvelope {
	mc := &commonv1.ModeChanged{Kind: kind, Mode: mode}
	if prev != "" {
		mc.PreviousMode = proto.String(prev)
	}
	return EventEnvelope{
		Subject: "openits.us-xx.agency.d01." + service + ".dev-1.mode-changed",
		CEType:  "openits." + service + ".mode-changed.v1",
		CEID:    service + kind + mode,
		Data:    mc,
	}
}

func runMode(events ...EventEnvelope) Result {
	return Run("mode", TestMode_ValueMatchesKind, &Observation{Events: events})
}

func TestModeKind_OwnFamilyPasses(t *testing.T) {
	cases := []EventEnvelope{
		modeEv("signal-control", "openits-signal-control-types:sc-mode-event-kind",
			"openits-signal-control-types:mode-free", "openits-signal-control-types:mode-coordinated"),
		modeEv("dms", "openits-dms-types:dms-mode-event-kind", "openits-dms-types:mode-blank", "openits-dms-types:mode-normal"),
		modeEv("ramp-metering", "openits-ramp-metering-types:ramp-meter-mode-event-kind",
			"openits-ramp-metering-types:mode-on-standby", "openits-ramp-metering-types:mode-active"),
		modeEv("rsu", "openits-rsu-types:rsu-mode-event-kind", "openits-rsu-types:mode-standby", "openits-rsu-types:mode-operate"),
	}
	for _, ev := range cases {
		if r := runMode(ev); !r.Pass {
			t.Errorf("own-family mode-changed failed: %s", r.Message)
		}
	}
}

// The DMS unknown dual-bases on the canonical openits-types:mode-unknown;
// it stays a sign-mode value, so it is accepted under the DMS kind.
func TestModeKind_DualBasedUnknownPassesForDMS(t *testing.T) {
	if r := runMode(modeEv("dms", "openits-dms-types:dms-mode-event-kind", "openits-dms-types:mode-unknown", "openits-dms-types:mode-normal")); !r.Pass {
		t.Fatalf("DMS mode-unknown rejected: %s", r.Message)
	}
}

func TestModeKind_CrossFamilyModeFails(t *testing.T) {
	r := runMode(modeEv("signal-control", "openits-signal-control-types:sc-mode-event-kind",
		"openits-signal-control-types:mode-free", "openits-dms-types:mode-blank"))
	if r.Pass || !strings.Contains(r.Message, "openits-dms-types:mode-blank") {
		t.Fatalf("cross-family mode not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

func TestModeKind_CrossFamilyPreviousModeFails(t *testing.T) {
	r := runMode(modeEv("signal-control", "openits-signal-control-types:sc-mode-event-kind",
		"openits-dms-types:mode-blank", "openits-signal-control-types:mode-free"))
	if r.Pass || !strings.Contains(r.Message, "previous-mode") {
		t.Fatalf("cross-family previous-mode not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

// The canonical identity itself does not derive from any family base; a
// family reports its own unknown (dual-based on the canonical) instead.
func TestModeKind_CanonicalUnknownOutsideFamilyFails(t *testing.T) {
	r := runMode(modeEv("signal-control", "openits-signal-control-types:sc-mode-event-kind", "", "openits-types:mode-unknown"))
	if r.Pass {
		t.Fatalf("canonical mode-unknown accepted under the signal-control kind")
	}
}

func TestModeKind_KindFromOtherServiceFails(t *testing.T) {
	r := runMode(modeEv("signal-control", "openits-dms-types:dms-mode-event-kind", "", "openits-dms-types:mode-normal"))
	if r.Pass || !strings.Contains(r.Message, "not the signal-control mode kind") {
		t.Fatalf("kind from another service not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

func TestModeKind_AbstractKindFails(t *testing.T) {
	if r := runMode(modeEv("dms", "openits-types:mode-event-kind", "", "openits-dms-types:mode-normal")); r.Pass {
		t.Fatalf("abstract mode-event-kind accepted")
	}
}

func TestModeKind_BareValueFails(t *testing.T) {
	r := runMode(modeEv("dms", "openits-dms-types:dms-mode-event-kind", "blank", "openits-dms-types:mode-normal"))
	if r.Pass || !strings.Contains(r.Message, "module prefix") {
		t.Fatalf("bare value not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

func TestModeKind_EmptyModeFails(t *testing.T) {
	r := runMode(modeEv("dms", "openits-dms-types:dms-mode-event-kind", "", ""))
	if r.Pass || !strings.Contains(r.Message, "no mode") {
		t.Fatalf("empty mode not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

func TestModeKind_EmptyKindFails(t *testing.T) {
	r := runMode(modeEv("dms", "", "", "openits-dms-types:mode-normal"))
	if r.Pass || !strings.Contains(r.Message, "no kind") {
		t.Fatalf("empty kind not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

func TestModeKind_MisspelledValueModuleFails(t *testing.T) {
	r := runMode(modeEv("dms", "openits-dms-types:dms-mode-event-kind", "", "openits-dms-type:mode-normal"))
	if r.Pass || !strings.Contains(r.Message, "unknown module") {
		t.Fatalf("misspelled value module not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

// Vendor modules are not generator inputs; a vendor-derived value is
// skipped, as TestFault_KindMatchesService skips vendor kinds.
func TestModeKind_VendorValueSkipped(t *testing.T) {
	if r := runMode(modeEv("signal-control", "openits-signal-control-types:sc-mode-event-kind", "",
		"openits-vendor-acme-signal-control-types:acme-mode-x")); !r.Pass {
		t.Fatalf("vendor value not skipped: %s", r.Message)
	}
}

func TestModeKind_VendorKindSkipped(t *testing.T) {
	if r := runMode(modeEv("signal-control", "openits-vendor-acme-signal-control-types:acme-mode-kind", "",
		"openits-dms-types:mode-normal")); !r.Pass {
		t.Fatalf("vendor kind not skipped: %s", r.Message)
	}
}

// CCTV mode values are not operational-mode identities yet; the kind is
// checked against the subject, the values are not.
func TestModeKind_CCTVValuesSkipped(t *testing.T) {
	if r := runMode(modeEv("cctv", "openits-cctv-types:cctv-mode-event-kind", "", "openits-cctv-types:cctv-control-local")); !r.Pass {
		t.Fatalf("CCTV values not skipped: %s", r.Message)
	}
}

func TestModeKind_UnknownServiceSkipped(t *testing.T) {
	if r := runMode(modeEv("widget", "openits-dms-types:dms-mode-event-kind", "", "openits-signal-control-types:mode-free")); !r.Pass {
		t.Fatalf("unknown service not skipped: %s", r.Message)
	}
}

func TestModeKind_OtherEventSkipped(t *testing.T) {
	ev := faultEv("dms", "openits-signal-control-types:sc-fault-power")
	if r := runMode(ev); !r.Pass {
		t.Fatalf("non-mode event not checked as mode-changed: %s", r.Message)
	}
}

// Every family enum named by the table exists in the generated schema.
func TestModeKind_TableEnumsExist(t *testing.T) {
	for svc, f := range modeFamilyByService {
		if f.enum == "" {
			continue
		}
		if len(enumMembers(f.enum)) == 0 {
			t.Errorf("%s: enum %s missing from the generated schema", svc, f.enum)
		}
	}
}
