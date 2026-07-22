package tests

// TestTimebase_ReferencesResolve re-implements, in Go, the referential
// integrity NTCIP 1201's time-base tables depend on but that YANG's
// `leafref require-instance true` only checks against the schema, not the
// live tree (ygot's Validate() does not evaluate leafref/must, so this is
// the only place these invariants are actually checked end-to-end): every
// schedule-entry's day-plan resolves to a real day-plan, and every
// day-plan action's timing-plan resolves to a real coordination plan. A
// mutation that points either reference at a nonexistent id fails this
// check.
func TestTimebase_ReferencesResolve(t *T, obs *Observation) {
	sc := obs.Device.GetSignalController()
	tb := sc.GetTimebase()
	if tb == nil {
		return
	}
	dayPlans := map[uint8]bool{}
	for _, dp := range tb.DayPlan {
		dayPlans[dp.GetDayPlanId()] = true
	}
	plans := map[uint8]bool{}
	if coord := sc.GetCoordination(); coord != nil {
		for _, tp := range coord.TimingPlan {
			plans[tp.GetPlanId()] = true
		}
	}
	for _, se := range tb.ScheduleEntry {
		if dp := se.GetDayPlan(); dp != 0 && !dayPlans[dp] {
			t.Errorf("schedule-entry %d references missing day-plan %d", se.GetScheduleId(), dp)
		}
	}
	for _, dp := range tb.DayPlan {
		for _, a := range dp.Action {
			if p := a.GetTimingPlan(); p != 0 && !plans[p] {
				t.Errorf("day-plan %d action %q references missing timing-plan %d", dp.GetDayPlanId(), a.GetStartTime(), p)
			}
		}
	}
}
