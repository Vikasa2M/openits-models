package tests

import (
	"testing"

	signalcontrolv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/signal_control/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const cabinetSubject = "openits.us-xx.agency.d01.signal-control.dev-1.cabinet-output-changed"

func cabinetEv(data protoreflect.ProtoMessage) EventEnvelope {
	return EventEnvelope{Subject: cabinetSubject, CEType: "openits.signal-control.cabinet-output-changed.v1", Data: data}
}

func runCabinet(ev EventEnvelope) Result {
	return Run("cabinet", TestEvents_CabinetOutputChanged, &Observation{Events: []EventEnvelope{ev}})
}

func TestCabinetOutput_NormalToFlashWithCausePasses(t *testing.T) {
	r := runCabinet(cabinetEv(&signalcontrolv1.CabinetOutputChanged{
		PreviousCabinetOutput: signalcontrolv1.CabinetOutput_CABINET_OUTPUT_NORMAL.Enum(),
		CabinetOutput:         signalcontrolv1.CabinetOutput_CABINET_OUTPUT_FLASH,
		FlashCause:            proto.String("openits-signal-control-types:flash-monitor"),
	}))
	if !r.Pass {
		t.Fatalf("valid transition failed: %s", r.Message)
	}
}

func TestCabinetOutput_NoTransitionFails(t *testing.T) {
	r := runCabinet(cabinetEv(&signalcontrolv1.CabinetOutputChanged{
		PreviousCabinetOutput: signalcontrolv1.CabinetOutput_CABINET_OUTPUT_FLASH.Enum(),
		CabinetOutput:         signalcontrolv1.CabinetOutput_CABINET_OUTPUT_FLASH,
	}))
	if r.Pass {
		t.Fatal("flash->flash passed")
	}
}

func TestCabinetOutput_CauseWhileNotFlashingFails(t *testing.T) {
	r := runCabinet(cabinetEv(&signalcontrolv1.CabinetOutputChanged{
		CabinetOutput: signalcontrolv1.CabinetOutput_CABINET_OUTPUT_NORMAL,
		FlashCause:    proto.String("openits-signal-control-types:flash-monitor"),
	}))
	if r.Pass {
		t.Fatal("flash-cause with normal output passed")
	}
}

func TestCabinetOutput_NoPreviousDarkPasses(t *testing.T) {
	r := runCabinet(cabinetEv(&signalcontrolv1.CabinetOutputChanged{
		CabinetOutput: signalcontrolv1.CabinetOutput_CABINET_OUTPUT_DARK,
	}))
	if !r.Pass {
		t.Fatalf("dark without previous failed: %s", r.Message)
	}
}

func TestCabinetOutput_WrongPayloadFails(t *testing.T) {
	r := runCabinet(cabinetEv(&signalcontrolv1.PreemptionActivated{}))
	if r.Pass {
		t.Fatal("wrong payload type passed")
	}
}

const statusSubject = "openits.us-xx.agency.d01.signal-control.dev-1.operational-status-report"

func runStatusFlash(r *signalcontrolv1.OperationalStatusReport) Result {
	ev := EventEnvelope{Subject: statusSubject, CEType: "openits.signal-control.operational-status-report.v1", Data: r}
	return Run("status", TestEvents_StatusReportFlashCause, &Observation{Events: []EventEnvelope{ev}})
}

func TestStatusReportFlashCause_FlashWithCausePasses(t *testing.T) {
	r := runStatusFlash(&signalcontrolv1.OperationalStatusReport{
		CabinetOutput: signalcontrolv1.CabinetOutput_CABINET_OUTPUT_FLASH.Enum(),
		FlashCause:    proto.String("openits-signal-control-types:flash-monitor"),
	})
	if !r.Pass {
		t.Fatalf("flash with cause failed: %s", r.Message)
	}
}

func TestStatusReportFlashCause_NormalWithCauseFails(t *testing.T) {
	r := runStatusFlash(&signalcontrolv1.OperationalStatusReport{
		CabinetOutput: signalcontrolv1.CabinetOutput_CABINET_OUTPUT_NORMAL.Enum(),
		FlashCause:    proto.String("openits-signal-control-types:flash-monitor"),
	})
	if r.Pass {
		t.Fatal("normal output with flash-cause passed")
	}
}

func TestStatusReportFlashCause_NormalWithoutCausePasses(t *testing.T) {
	r := runStatusFlash(&signalcontrolv1.OperationalStatusReport{
		CabinetOutput: signalcontrolv1.CabinetOutput_CABINET_OUTPUT_NORMAL.Enum(),
	})
	if !r.Pass {
		t.Fatalf("normal without cause failed: %s", r.Message)
	}
}
