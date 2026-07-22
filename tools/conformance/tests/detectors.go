package tests

// Detector config/state split the same idiom as everywhere else: intended
// configuration (type/assigned-phases/delay/extend/mode/fail-action) lives
// in config, device-reported readback (active/fault/measurement) in state.
// These checks close the cut-1 gap where the detector-type identity had
// no conformance coverage at all.

func TestDetectors_AtLeastOne(t *T, obs *Observation) {
	d := obs.Device.GetSignalController().GetDetectors()
	if d == nil || len(d.Detector) == 0 {
		t.Fatalf("no detectors configured")
	}
}

func TestDetectors_AssignedToPhase(t *T, obs *Observation) {
	d := obs.Device.GetSignalController().GetDetectors()
	if d == nil {
		return
	}
	for id, det := range d.Detector {
		if len(det.GetConfig().GetAssignedPhases()) == 0 {
			t.Errorf("detector %d calls no phase (assigned-phases empty)", id)
		}
	}
}

func TestDetectors_MeasurementReported(t *T, obs *Observation) {
	d := obs.Device.GetSignalController().GetDetectors()
	if d == nil {
		return
	}
	for id, det := range d.Detector {
		if !det.GetState().GetActive() {
			continue
		}
		m := det.GetState().GetMeasurement()
		if m == nil {
			t.Errorf("detector %d is active but reports no volume/occupancy/speed measurement", id)
		}
	}
}
