package tests

// Pedestrian detectors are numbered independently of vehicle detectors and
// each calls exactly one phase. The model's leafref already rejects a
// call-phase with no matching phase in instance data; this check gives a
// live device the same guarantee on its observed state.

func TestPedestrianDetectors_CallPhaseResolves(t *T, obs *Observation) {
	sc := obs.Device.GetSignalController()
	pd := sc.GetPedestrianDetectors()
	if pd == nil {
		return
	}
	phases := map[uint8]bool{}
	if ph := sc.GetPhases(); ph != nil {
		for n := range ph.Phase {
			phases[n] = true
		}
	}
	for id, d := range pd.PedestrianDetector {
		call := d.GetConfig().GetCallPhase()
		if !phases[call] {
			t.Errorf("pedestrian detector %d call-phase %d does not resolve to a configured phase", id, call)
		}
	}
}
