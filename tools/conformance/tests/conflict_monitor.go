package tests

// Conflict monitor: the MMU permissive matrix (NEMA TS-2 conflict
// monitor), config-only. Each permissive names a channel pair allowed to
// show green simultaneously; both channels must actually exist, and the
// pair must be stored canonically (channel-a < channel-b) so each
// compatible pair appears exactly once — the invariant the YANG `must`
// on this list enforces at the schema level, checked again here against
// the actual collected data.

func TestConflictMonitor_AtLeastOnePermissive(t *T, obs *Observation) {
	cm := obs.Device.GetSignalController().GetConflictMonitor()
	if cm == nil || len(cm.Permissive) == 0 {
		t.Fatalf("no permissives configured")
	}
}

func TestConflictMonitor_PermissiveResolvesChannels(t *T, obs *Observation) {
	sc := obs.Device.GetSignalController()
	cm := sc.GetConflictMonitor()
	if cm == nil {
		return
	}
	channels := map[uint16]bool{}
	if ch := sc.GetChannels(); ch != nil {
		for n := range ch.Channel {
			channels[n] = true
		}
	}
	for key, p := range cm.Permissive {
		a, b := p.GetChannelA(), p.GetChannelB()
		if !channels[a] {
			t.Errorf("permissive %v channel-a %d does not resolve to a configured channel", key, a)
		}
		if !channels[b] {
			t.Errorf("permissive %v channel-b %d does not resolve to a configured channel", key, b)
		}
	}
}

func TestConflictMonitor_PermissiveCanonicalOrder(t *T, obs *Observation) {
	cm := obs.Device.GetSignalController().GetConflictMonitor()
	if cm == nil {
		return
	}
	for key, p := range cm.Permissive {
		a, b := p.GetChannelA(), p.GetChannelB()
		if !(a < b) {
			t.Errorf("permissive %v is not canonical: channel-a %d must be < channel-b %d", key, a, b)
		}
	}
}

// TestConflictMonitor_NoSameRingPermissive enforces the same-ring cross-check
// that is deliberately NOT a YANG `must` (a correct XPath expression needs a
// double deref() through the channel `choice source` and is undefined for
// overlap-sourced channels; ygot's Validate() does not evaluate `must`
// regardless). Two channels driven by phases in the SAME ring can never be
// green together (a ring serves one phase at a time), so a permissive between
// them is meaningless and masks a programming error. Channels whose source is
// an overlap (no single ring) are skipped. Early-returns on absent data.
func TestConflictMonitor_NoSameRingPermissive(t *T, obs *Observation) {
	sc := obs.Device.GetSignalController()
	cm := sc.GetConflictMonitor()
	ch := sc.GetChannels()
	ph := sc.GetPhases()
	if cm == nil || ch == nil || ph == nil {
		return
	}
	ringOf := map[uint8]uint8{}
	for _, p := range ph.Phase {
		ringOf[p.GetConfig().GetPhaseNumber()] = p.GetConfig().GetRing()
	}
	// Map channel-number -> ring of its driving phase (overlap-sourced skipped).
	chRing := map[uint16]uint8{}
	for n, c := range ch.Channel {
		if phase := c.GetPhase(); phase != 0 {
			if r, ok := ringOf[phase]; ok {
				chRing[n] = r
			}
		}
	}
	for key, p := range cm.Permissive {
		a, b := p.GetChannelA(), p.GetChannelB()
		ra, aok := chRing[a]
		rb, bok := chRing[b]
		if aok && bok && ra == rb {
			t.Errorf("permissive %v pairs channels %d and %d, both driven by ring %d phases; "+
				"same-ring phases are never green together, so this permissive is meaningless", key, a, b, ra)
		}
	}
}
