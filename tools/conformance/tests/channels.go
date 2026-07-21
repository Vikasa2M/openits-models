package tests

// Channels: the load-switch table (NTCIP channelTable), config-only.
// Each channel's `choice source` names either a phase or an overlap that
// must actually exist — this is the mapping the conflict-monitor
// permissive matrix and the physical load switches key off of.

func TestChannels_AtLeastOne(t *T, obs *Observation) {
	ch := obs.Device.GetSignalController().GetChannels()
	if ch == nil || len(ch.Channel) == 0 {
		t.Fatalf("no channels configured")
	}
}

func TestChannels_SourceResolves(t *T, obs *Observation) {
	sc := obs.Device.GetSignalController()
	ch := sc.GetChannels()
	if ch == nil {
		return
	}
	phases := map[uint8]bool{}
	if ph := sc.GetPhases(); ph != nil {
		for n := range ph.Phase {
			phases[n] = true
		}
	}
	overlaps := map[uint8]bool{}
	if ov := sc.GetOverlaps(); ov != nil {
		for id := range ov.Overlap {
			overlaps[id] = true
		}
	}
	for n, c := range ch.Channel {
		phase, overlap := c.GetPhase(), c.GetOverlap()
		switch {
		case phase != 0 && overlap != 0:
			t.Errorf("channel %d names both a phase and an overlap source; choice source allows only one", n)
		case phase != 0:
			if !phases[phase] {
				t.Errorf("channel %d source phase %d does not resolve to a configured phase", n, phase)
			}
		case overlap != 0:
			if !overlaps[overlap] {
				t.Errorf("channel %d source overlap %d does not resolve to a configured overlap", n, overlap)
			}
		default:
			t.Errorf("channel %d names no source (choice source unset)", n)
		}
	}
}
