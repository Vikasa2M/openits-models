package tests

import yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"

// TestMonitor_StatusReported checks that a reported cabinet monitor says
// whether it was read. An absent or unknown status would let a monitor
// behind a dead link read as healthy, which is the failure this field
// exists to stop.
func TestMonitor_StatusReported(t *T, obs *Observation) {
	m := obs.Device.GetSignalController().GetOperation().GetMonitor()
	if m == nil {
		return
	}
	switch m.GetStatus() {
	case yangpkg.OpenitsSignalControlTypes_MonitorStatus_UNSET,
		yangpkg.OpenitsSignalControlTypes_MonitorStatus_unknown:
		t.Errorf("operation/monitor is reported without a known status")
	}
}
