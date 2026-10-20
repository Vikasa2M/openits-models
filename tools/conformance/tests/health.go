package tests

import (
	"strings"

	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
)

// The controller-wide operating state (cabinet-output, flash-cause, mode,
// control-source, preempt-active, priority-active, mode-since, monitor)
// lives in signal-controller/operation (config false), distinct from the
// identity mirror in signal-controller/state.

func TestHealth_OperationalStatus(t *T, obs *Observation) {
	op := obs.Device.GetSignalController().GetOperation()
	if op == nil {
		t.Fatalf("operation container missing")
	}
	// ModeSince should be RFC3339; not empty is sufficient at this
	// layer — YANG validation enforces the date-and-time type.
	if op.GetModeSince() == "" {
		t.Errorf("operation/mode-since is unset")
	}
	// Also require that we observed at least one heartbeat on the wire.
	for _, e := range obs.Events {
		if strings.HasSuffix(e.Subject, ".operational-status-report") {
			return
		}
	}
	t.Errorf("no operational-status heartbeat observed during %s window", obs.Window)
}

func TestHealth_NotFlashing(t *T, obs *Observation) {
	op := obs.Device.GetSignalController().GetOperation()
	if op == nil {
		return
	}
	if op.GetCabinetOutput() == yangpkg.OpenitsSignalControlTypes_CabinetOutput_flash {
		t.Errorf("controller is flashing (cause=%s); MMU or programmed flash active", op.GetFlashCause())
	}
}
