package tests

// Sequences: the phase sequence library (NTCIP 1202 sequenceTable). A
// ring-sequence orders phases WITHIN one ring, so every ordered phase must
// belong to that ring per phases/phase/config/ring. This is a harness check,
// not a YANG `must`: the invariant needs deref() through each member of a
// leaf-list, which XPath 1.0 cannot express per-node, and ygot's Validate()
// does not evaluate `must` regardless (same precedent as
// TestConflictMonitor_NoSameRingPermissive).

func TestSequences_PhasesMatchRingMembership(t *T, obs *Observation) {
	sc := obs.Device.GetSignalController()
	seqs := sc.GetSequences()
	ph := sc.GetPhases()
	if seqs == nil || ph == nil {
		return
	}
	ringOf := map[uint8]uint8{}
	for _, p := range ph.Phase {
		ringOf[p.GetConfig().GetPhaseNumber()] = p.GetConfig().GetRing()
	}
	for sid, seq := range seqs.Sequence {
		for ring, rs := range seq.RingSequence {
			for _, phase := range rs.OrderedPhases {
				if r, ok := ringOf[phase]; ok && r != ring {
					t.Errorf("sequence %d ring %d orders phase %d, but that phase is assigned to ring %d; "+
						"a controller cannot serve a phase in a ring it is not assigned to", sid, ring, phase, r)
				}
			}
		}
	}
}
