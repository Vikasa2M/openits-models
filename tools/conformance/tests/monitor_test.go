package tests

import (
	"testing"

	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
)

func runMonitor(dev *yangpkg.Device) Result {
	return Run("monitor", TestMonitor_StatusReported, &Observation{Device: dev})
}

func monitorDev(status yangpkg.E_OpenitsSignalControlTypes_MonitorStatus) *yangpkg.Device {
	dev := &yangpkg.Device{}
	m := dev.GetOrCreateSignalController().GetOrCreateOperation().GetOrCreateMonitor()
	m.Status = status
	return dev
}

func TestMonitorStatus_UnsetFails(t *testing.T) {
	if r := runMonitor(monitorDev(yangpkg.OpenitsSignalControlTypes_MonitorStatus_UNSET)); r.Pass {
		t.Fatalf("monitor without a status passed")
	}
}

func TestMonitorStatus_UnknownFails(t *testing.T) {
	if r := runMonitor(monitorDev(yangpkg.OpenitsSignalControlTypes_MonitorStatus_unknown)); r.Pass {
		t.Fatalf("monitor with unknown status passed")
	}
}

func TestMonitorStatus_NotReadPasses(t *testing.T) {
	if r := runMonitor(monitorDev(yangpkg.OpenitsSignalControlTypes_MonitorStatus_not_read)); !r.Pass {
		t.Fatalf("not-read failed: %s", r.Message)
	}
}

func TestMonitorStatus_NoMonitorPasses(t *testing.T) {
	dev := &yangpkg.Device{}
	dev.GetOrCreateSignalController().GetOrCreateOperation()
	if r := runMonitor(dev); !r.Pass {
		t.Fatalf("absent monitor failed: %s", r.Message)
	}
}
