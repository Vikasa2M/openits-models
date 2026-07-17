package tests

import (
	"strings"

	yangpkg "github.com/openits/openits-models/pkg/yang/openits"
)

// ----- identity -----

func TestCctvIdentity_CameraID(t *T, obs *Observation) {
	st := obs.Device.GetCamera().GetState()
	if st == nil || st.GetId() == "" {
		t.Errorf("state/id is unset")
	}
}

// ----- PTZ -----

// The camera cannot be "at" a preset that does not exist: if ptz/presets/state
// reports an active-preset, it must be one of the configured presets. Guards
// the state<->config tie behind the preset readback.
func TestCctvPtz_ActivePresetIsDefined(t *T, obs *Observation) {
	ptz := obs.Device.GetCamera().GetPtz()
	if ptz == nil || ptz.GetPresets() == nil {
		return
	}
	presets := ptz.GetPresets()
	stateC := presets.GetState()
	if stateC == nil || stateC.ActivePreset == nil {
		return
	}
	active := stateC.GetActivePreset()
	if _, ok := presets.Preset[active]; !ok {
		t.Errorf("ptz/presets/state/active-preset %d is not a configured preset", active)
	}
}

// A running tour must name which tour is running: run-state 'running' with no
// active-tour is an unattributable state a consumer cannot act on.
func TestCctvTour_RunningImpliesActiveTour(t *T, obs *Observation) {
	ptz := obs.Device.GetCamera().GetPtz()
	if ptz == nil || ptz.GetTours() == nil {
		return
	}
	st := ptz.GetTours().GetState()
	if st == nil {
		return
	}
	if st.RunState == yangpkg.OpenitsCctvTypes_TourRunState_running && st.ActiveTour == nil {
		t.Errorf("tours/state/run-state is 'running' but active-tour is unset")
	}
}

// ----- streams -----

// A stream reporting health 'ok' must report a non-zero bitrate — an "ok"
// stream carrying no bits is a contradiction that would mislead a consumer
// treating it as live.
func TestCctvStream_OkHealthHasBitrate(t *T, obs *Observation) {
	streams := obs.Device.GetCamera().GetStreams()
	if streams == nil {
		return
	}
	for id, s := range streams.Stream {
		st := s.GetState()
		if st == nil || st.Health != yangpkg.OpenitsCctvTypes_StreamHealth_ok {
			continue
		}
		if st.BitrateKbps == nil || st.GetBitrateKbps() == 0 {
			t.Errorf("stream %d reports health 'ok' but a zero/absent bitrate", id)
		}
	}
}

// ----- control ownership -----

// If control is currently held (control/state/holder present), the readback
// must name who holds it — the entire point of ownership arbitration is
// attributing control, and a held camera with no named holder is a state an
// operator cannot reason about.
func TestCctvControl_HeldImpliesHolder(t *T, obs *Observation) {
	ctl := obs.Device.GetCamera().GetControl()
	if ctl == nil || ctl.GetState() == nil {
		return
	}
	h := ctl.GetState().GetHolder()
	if h == nil {
		return
	}
	if h.CurrentHolder == nil || h.GetCurrentHolder() == "" {
		t.Errorf("control is held (state/holder present) but current-holder is unset")
	}
}

// ----- events -----

func TestCctvEvent_LockoutDeniedShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".lockout-denied") {
			continue
		}
		want := "openits.cctv.lockout-denied.v1"
		if e.CEType != want {
			t.Errorf("lockout-denied ce-type %q, want %q", e.CEType, want)
		}
		return
	}
	t.Errorf("no lockout-denied event observed during %s window", obs.Window)
}

func TestCctvEvent_PresetRecalledShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".ptz-preset-recalled") {
			continue
		}
		want := "openits.cctv.ptz-preset-recalled.v1"
		if e.CEType != want {
			t.Errorf("ptz-preset-recalled ce-type %q, want %q", e.CEType, want)
		}
		return
	}
	t.Errorf("no ptz-preset-recalled event observed during %s window", obs.Window)
}

func TestCctvEvent_FaultRaisedShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".fault-raised") {
			continue
		}
		want := "openits.cctv.fault-raised.v1"
		if e.CEType != want {
			t.Errorf("fault-raised ce-type %q, want %q", e.CEType, want)
		}
		return
	}
	t.Errorf("no fault-raised event observed during %s window", obs.Window)
}
