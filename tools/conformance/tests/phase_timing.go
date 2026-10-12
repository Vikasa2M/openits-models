package tests

// Phase-timing floors the harness checks. MinYellowChange is the MUTCD
// minimum. Red clearance has no MUTCD minimum: the interval is optional, and
// the core model deliberately permits zero (only the strict MUTCD deviation
// module requires one), so the harness checks only that it is not negative.
// MinGreenMinimum is a conservative operational floor, not an MUTCD value;
// non-standard geometries may justify lower.
const (
	MinYellowChange = 3.0 // seconds
	MinGreenMinimum = 4   // seconds; operational floor
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

func TestPhaseTiming_RedClearNonNegative(t *T, obs *Observation) {
	ph := obs.Device.GetSignalController().GetPhases()
	if ph == nil {
		return
	}
	for n, p := range ph.Phase {
		if p.GetConfig().GetTiming() == nil {
			continue
		}
		r := p.GetConfig().GetTiming().GetRedClear()
		if r < 0 {
			t.Errorf("phase %d red-clear %.2fs is negative", n, r)
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
			t.Errorf("phase %d min-green %ds < operational floor %ds", n, g, MinGreenMinimum)
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
