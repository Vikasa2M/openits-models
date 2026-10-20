package tests

import (
	"strings"

	signalcontrolv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/signal_control/v1"
)

const cabinetOutputFlash = signalcontrolv1.CabinetOutput_CABINET_OUTPUT_FLASH

// TestEvents_CabinetOutputChanged checks each cabinet-output-changed
// event: it reports a transition (previous differs from current when
// both are set) and carries flash-cause only while flashing. yanglint
// does not enforce notification constraints, so they are enforced here.
func TestEvents_CabinetOutputChanged(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".cabinet-output-changed") {
			continue
		}
		ev, ok := e.Data.(*signalcontrolv1.CabinetOutputChanged)
		if !ok {
			t.Errorf("cabinet-output-changed on %s has payload %T", e.Subject, e.Data)
			continue
		}
		if ev.PreviousCabinetOutput != nil && ev.GetPreviousCabinetOutput() == ev.GetCabinetOutput() {
			t.Errorf("cabinet-output-changed on %s: previous and current are both %v", e.Subject, ev.GetCabinetOutput())
		}
		if ev.FlashCause != nil && ev.GetCabinetOutput() != cabinetOutputFlash {
			t.Errorf("cabinet-output-changed on %s carries flash-cause %q while cabinet-output is %v", e.Subject, ev.GetFlashCause(), ev.GetCabinetOutput())
		}
	}
}

// TestEvents_StatusReportFlashCause checks each operational-status-report:
// flash-cause is present only while cabinet-output is flash. yanglint does
// not enforce notification constraints, so the sibling of the
// cabinet-output-changed check is enforced here.
func TestEvents_StatusReportFlashCause(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".operational-status-report") {
			continue
		}
		ev, ok := e.Data.(*signalcontrolv1.OperationalStatusReport)
		if !ok {
			t.Errorf("operational-status-report on %s has payload %T", e.Subject, e.Data)
			continue
		}
		if ev.FlashCause != nil && ev.GetCabinetOutput() != cabinetOutputFlash {
			t.Errorf("operational-status-report on %s carries flash-cause %q while cabinet-output is %v", e.Subject, ev.GetFlashCause(), ev.GetCabinetOutput())
		}
	}
}
