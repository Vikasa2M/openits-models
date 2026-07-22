package tests

import "strings"

// The controller-wide operational rollup (mode, flash-active, flash-cause,
// MMU, last-mode-change) lives in signal-controller/operation (config
// false), distinct from the identity mirror in signal-controller/state.

func TestHealth_OperationalStatus(t *T, obs *Observation) {
	op := obs.Device.GetSignalController().GetOperation()
	if op == nil {
		t.Fatalf("operation container missing")
	}
	// LastModeChange should be RFC3339; not empty is sufficient at this
	// layer — YANG validation enforces the date-and-time type.
	if op.GetLastModeChange() == "" {
		t.Errorf("operation/last-mode-change is unset")
	}
	// Also require that we observed at least one heartbeat on the wire.
	for _, e := range obs.Events {
		if strings.HasSuffix(e.Subject, ".operational-status") {
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
	if op.GetFlashActive() {
		t.Errorf("controller is flashing (cause=%s); MMU or programmed flash active", op.GetFlashCause())
	}
}
