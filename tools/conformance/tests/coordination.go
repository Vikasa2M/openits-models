package tests

// TestCoordination_SplitsWithinCycle re-implements, in Go, the cut-2c
// config-true must "sum(split[ring=N]/split-seconds) <= cycle-length"
// (ygot's Validate() does not evaluate XPath `must` statements, so this
// is the only place that invariant is actually checked end-to-end). A
// mutation that over-allocates any ring's splits past the plan's cycle
// length fails this check.
func TestCoordination_SplitsWithinCycle(t *T, obs *Observation) {
	c := obs.Device.GetSignalController().GetCoordination()
	ph := obs.Device.GetSignalController().GetPhases()
	if c == nil || ph == nil {
		return
	}
	ringOf := map[uint8]uint8{}
	for _, p := range ph.Phase {
		ringOf[p.GetConfig().GetPhaseNumber()] = p.GetConfig().GetRing()
	}
	for _, tp := range c.TimingPlan {
		sum := map[uint8]uint32{}
		for _, s := range tp.Split {
			sum[ringOf[s.GetPhaseNumber()]] += uint32(s.GetSplitSeconds())
		}
		for ring, total := range sum {
			if total > uint32(tp.GetCycleLength()) {
				t.Errorf("plan %d ring %d splits sum %ds > cycle %ds",
					tp.GetPlanId(), ring, total, tp.GetCycleLength())
			}
		}
	}
}

func TestCoordination_ActivePlan(t *T, obs *Observation) {
	c := obs.Device.GetSignalController().GetCoordination()
	if c == nil {
		t.Errorf("coordination container missing")
		return
	}
	st := c.GetState()
	if st == nil || st.GetActivePlan() == 0 {
		t.Errorf("active-plan is 0; controller reports no coordination pattern")
	}
}

func TestCoordination_NEMADualRing(t *T, obs *Observation) {
	ph := obs.Device.GetSignalController().GetPhases()
	if ph == nil {
		return
	}
	var r1, r2 int
	for _, p := range ph.Phase {
		switch p.GetConfig().GetRing() {
		case 1:
			r1++
		case 2:
			r2++
		}
	}
	if r1 == 0 || r2 == 0 {
		t.Errorf("NEMA dual-ring violated: ring1=%d, ring2=%d", r1, r2)
	}
}

func TestCoordination_BarrierAssignment(t *T, obs *Observation) {
	ph := obs.Device.GetSignalController().GetPhases()
	if ph == nil {
		return
	}
	for n, p := range ph.Phase {
		b := p.GetConfig().GetBarrier()
		if b == 0 {
			t.Errorf("phase %d barrier is 0; NEMA requires barrier 1 or 2", n)
		}
	}
}
