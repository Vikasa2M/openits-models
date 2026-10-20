package tests

import yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"

// TestMonitor_StatusReported checks that a reported cabinet monitor says
// whether it was read. An unset status would let a monitor behind a dead
// link read as healthy, which is the failure this field exists to stop.
// An explicit unknown is an honest answer and passes.
func TestMonitor_StatusReported(t *T, obs *Observation) {
	m := obs.Device.GetSignalController().GetOperation().GetMonitor()
	if m == nil {
		return
	}
	if m.GetStatus() == yangpkg.OpenitsSignalControlTypes_MonitorStatus_UNSET {
		t.Errorf("operation/monitor is reported without a status")
	}
}
