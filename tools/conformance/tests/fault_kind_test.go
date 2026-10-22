package tests

import (
	"strings"
	"testing"

	commonv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/common/v1"
)

func faultEv(service, kind string) EventEnvelope {
	return EventEnvelope{
		Subject: "openits.us-xx.agency.d01." + service + ".dev-1.fault-raised",
		CEType:  "openits." + service + ".fault-raised.v1",
		CEID:    service + kind,
		Data:    &commonv1.FaultRaised{Kind: kind, FaultId: "f-1"},
	}
}

func runKind(events ...EventEnvelope) Result {
	return Run("kind", TestFault_KindMatchesService, &Observation{Events: events})
}

func TestFaultKind_OwnServicePasses(t *testing.T) {
	if r := runKind(faultEv("dms", "openits-dms-types:dms-fault-lamp")); !r.Pass {
		t.Fatalf("own-service kind failed: %s", r.Message)
	}
}

func TestFaultKind_PlatformFormPasses(t *testing.T) {
	if r := runKind(faultEv("signal-control", "openits-signal-control-types:sc-fault-cabinet-on-battery")); !r.Pass {
		t.Fatalf("signal-control form of a cabinet-power fault failed: %s", r.Message)
	}
}

// Vendor modules are not generator inputs, so their identities are absent
// from the generated enums; the check skips a module it does not know.
func TestFaultKind_VendorDerivedPasses(t *testing.T) {
	if r := runKind(faultEv("signal-control", "openits-vendor-econolite-signal-control-types:econolite-vendor-alarm-1")); !r.Pass {
		t.Fatalf("vendor identity derived from the signal-control base failed: %s", r.Message)
	}
}

func TestFaultKind_OtherServiceFails(t *testing.T) {
	r := runKind(faultEv("signal-control", "openits-dms-types:dms-fault-lamp"))
	if r.Pass || !strings.Contains(r.Message, "signal-control") {
		t.Fatalf("cross-service kind not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

func TestFaultKind_BarePlatformIdentityFails(t *testing.T) {
	r := runKind(faultEv("signal-control", "openits-cabinet-power:cabinet-power-on-battery"))
	if r.Pass {
		t.Fatalf("bare cabinet-power identity accepted under a service subject")
	}
}

func TestFaultKind_ClearedOtherServiceFails(t *testing.T) {
	ev := faultEv("signal-control", "openits-dms-types:dms-fault-lamp")
	ev.Subject = strings.TrimSuffix(ev.Subject, "fault-raised") + "fault-cleared"
	ev.Data = &commonv1.FaultCleared{Kind: "openits-dms-types:dms-fault-lamp", FaultId: "f-1"}
	if r := runKind(ev); r.Pass {
		t.Fatalf("fault-cleared with another service's kind accepted")
	}
}

func TestFaultKind_EmptyKindFails(t *testing.T) {
	r := runKind(faultEv("dms", ""))
	if r.Pass || !strings.Contains(r.Message, "no kind") {
		t.Fatalf("empty kind not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

func TestFaultKind_NoModulePrefixFails(t *testing.T) {
	r := runKind(faultEv("dms", "dms-fault-lamp"))
	if r.Pass || !strings.Contains(r.Message, "module prefix") {
		t.Fatalf("unprefixed kind not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

func TestFaultKind_MisspelledCoreModuleFails(t *testing.T) {
	r := runKind(faultEv("signal-control", "openits-signal-control-type:sc-fault-power"))
	if r.Pass || !strings.Contains(r.Message, "unknown module") {
		t.Fatalf("misspelled core module not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

func TestFaultKind_NonFaultEventSkipped(t *testing.T) {
	ev := faultEv("dms", "openits-signal-control-types:sc-fault-power")
	ev.Subject = strings.TrimSuffix(ev.Subject, "fault-raised") + "mode-changed"
	if r := runKind(ev); !r.Pass {
		t.Fatalf("non-fault event not skipped: %s", r.Message)
	}
}

func TestFaultKind_UnknownServiceSkipped(t *testing.T) {
	if r := runKind(faultEv("widget", "openits-dms-types:dms-fault-lamp")); !r.Pass {
		t.Fatalf("unknown service not skipped: %s", r.Message)
	}
}

func TestFaultKind_ReversibleLaneLeafPasses(t *testing.T) {
	for _, kind := range []string{
		"openits-reversible-lane-types:reversible-lane-fault-plc-fault",
		"openits-reversible-lane-types:reversible-lane-fault-plc-configuration",
		"openits-reversible-lane-types:reversible-lane-fault-cabinet-door-open",
		"openits-reversible-lane-types:reversible-lane-fault-gate-invalid-inputs",
	} {
		if r := runKind(faultEv("reversible-lane", kind)); !r.Pass {
			t.Fatalf("reversible-lane leaf %s failed: %s", kind, r.Message)
		}
	}
}

func TestFaultKind_ReversibleLaneBareCabinetPowerFails(t *testing.T) {
	r := runKind(faultEv("reversible-lane", "openits-cabinet-power:cabinet-power-door-open"))
	if r.Pass {
		t.Fatalf("bare cabinet-power identity accepted under reversible-lane")
	}
}

func TestFaultKind_ReversibleLaneBareServiceBaseFails(t *testing.T) {
	r := runKind(faultEv("reversible-lane", "openits-reversible-lane-types:reversible-lane-fault-event-kind"))
	if r.Pass {
		t.Fatalf("bare reversible-lane-fault-event-kind accepted under reversible-lane")
	}
}
