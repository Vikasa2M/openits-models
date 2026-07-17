package tests

// MUTCD minimums for phase timing.  These are conservative and cover
// virtually every intersection; non-standard geometries may allow
// lower values with engineering justification.
const (
	MinYellowChange = 3.0 // seconds
	MinRedClear     = 1.0 // seconds
	MinGreenMinimum = 4   // seconds; pedestrian safety floor
	MaxGreenSaneHi  = 300 // seconds; anything above is almost certainly misconfigured
	PedClearSaneHi  = 120 // seconds
)

func TestPhaseTiming_HasPhases(t *T, obs *Observation) {
	ph := obs.Device.GetSignalController().GetPhases()
	if ph == nil || len(ph.Phase) == 0 {
		t.Fatalf("no phases configured")
	}
	if len(ph.Phase) < 2 {
		t.Errorf("controller reports %d phases; signalized intersections have ≥2", len(ph.Phase))
	}
}

func TestPhaseTiming_YellowChangeMinimum(t *T, obs *Observation) {
	ph := obs.Device.GetSignalController().GetPhases()
	if ph == nil {
		return
	}
	for n, p := range ph.Phase {
		if p.GetConfig().GetTiming() == nil {
			continue
		}
		y := p.GetConfig().GetTiming().GetYellowChange()
		if y < MinYellowChange {
			t.Errorf("phase %d yellow-change %.2fs < MUTCD minimum %.2fs", n, y, MinYellowChange)
		}
	}
}

func TestPhaseTiming_RedClearMinimum(t *T, obs *Observation) {
	ph := obs.Device.GetSignalController().GetPhases()
	if ph == nil {
		return
	}
	for n, p := range ph.Phase {
		if p.GetConfig().GetTiming() == nil {
			continue
		}
		r := p.GetConfig().GetTiming().GetRedClear()
		if r < MinRedClear {
			t.Errorf("phase %d red-clear %.2fs < MUTCD minimum %.2fs", n, r, MinRedClear)
		}
	}
}

func TestPhaseTiming_MinGreenMinimum(t *T, obs *Observation) {
	ph := obs.Device.GetSignalController().GetPhases()
	if ph == nil {
		return
	}
	for n, p := range ph.Phase {
		if p.GetConfig().GetTiming() == nil {
			continue
		}
		g := int(p.GetConfig().GetTiming().GetMinGreen())
		if g < MinGreenMinimum {
			t.Errorf("phase %d min-green %ds < pedestrian safety floor %ds", n, g, MinGreenMinimum)
		}
	}
}

func TestPhaseTiming_MaxGreenSane(t *T, obs *Observation) {
	ph := obs.Device.GetSignalController().GetPhases()
	if ph == nil {
		return
	}
	for n, p := range ph.Phase {
		if p.GetConfig().GetTiming() == nil {
			continue
		}
		g := int(p.GetConfig().GetTiming().GetMaxGreen())
		if g > MaxGreenSaneHi {
			t.Errorf("phase %d max-green %ds exceeds %ds (almost certainly misconfigured)", n, g, MaxGreenSaneHi)
		}
		if g == 0 {
			t.Errorf("phase %d max-green is zero; phase can never terminate on gap-out", n)
		}
	}
}

func TestPhaseTiming_PedClearSane(t *T, obs *Observation) {
	ph := obs.Device.GetSignalController().GetPhases()
	if ph == nil {
		return
	}
	for n, p := range ph.Phase {
		if p.GetConfig().GetTiming() == nil {
			continue
		}
		c := int(p.GetConfig().GetTiming().GetPedClear())
		if c > PedClearSaneHi {
			t.Errorf("phase %d ped-clear %ds exceeds %ds", n, c, PedClearSaneHi)
		}
	}
}
