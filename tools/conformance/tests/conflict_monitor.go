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
