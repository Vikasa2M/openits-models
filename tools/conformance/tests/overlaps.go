package tests

// Overlaps: config/state split (NTCIP overlapTable). An FYA
// (flashing-yellow-arrow) overlap layers a protected/permissive left
// turn on top of the plain overlap-drives-off-included-phases model: the
// fya container's protected-left-phase is what drives the overlap green
// (so it must itself be one of the overlap's included-phases), and both
// the protected and opposing-through phase must resolve to phases the
// controller actually has configured.

func TestOverlaps_AtLeastOne(t *T, obs *Observation) {
	ov := obs.Device.GetSignalController().GetOverlaps()
	if ov == nil || len(ov.Overlap) == 0 {
		t.Fatalf("no overlaps configured")
	}
}

func TestOverlaps_FYAProtectedAndOpposingPhasesResolve(t *T, obs *Observation) {
	sc := obs.Device.GetSignalController()
	ov := sc.GetOverlaps()
	if ov == nil {
		return
	}
	phases := map[uint8]bool{}
	if ph := sc.GetPhases(); ph != nil {
		for n := range ph.Phase {
			phases[n] = true
		}
	}
	for id, o := range ov.Overlap {
		fya := o.GetConfig().GetFya()
		if fya == nil {
			continue // not an FYA overlap; nothing to check
		}
		protected := fya.GetProtectedLeftPhase()
		if protected == 0 {
			t.Errorf("overlap %s is FYA but protected-left-phase is unset", id)
		} else if !phases[protected] {
			t.Errorf("overlap %s protected-left-phase %d does not resolve to a configured phase", id, protected)
		}

		opposing := fya.GetOpposingThroughPhase()
		if opposing == 0 {
			t.Errorf("overlap %s is FYA but opposing-through-phase is unset", id)
		} else if !phases[opposing] {
			t.Errorf("overlap %s opposing-through-phase %d does not resolve to a configured phase", id, opposing)
		}

		if protected != 0 {
			included := false
			for _, p := range o.GetConfig().GetIncludedPhases() {
				if p == protected {
					included = true
					break
				}
			}
			if !included {
				t.Errorf("overlap %s protected-left-phase %d is not in included-phases %v", id, protected, o.GetConfig().GetIncludedPhases())
			}
		}
	}
}
