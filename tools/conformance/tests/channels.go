package tests

import (
	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
)

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

// TestChannels_NoConflictingFlashYellow enforces the MUTCD flash-color
// compatibility rule the schema cannot: flashing yellow is displayed only to
// non-conflicting movements (conflicting approaches flash red), so no two
// channels may both be programmed flash-state yellow unless the MMU
// permissive record lists them as a compatible (never-conflicting) pair.
// This is a harness check because a config `must` cannot reach across into
// the config-false conflict-monitor readback (same precedent as
// TestConflictMonitor_NoSameRingPermissive). Early-returns on absent data.
func TestChannels_NoConflictingFlashYellow(t *T, obs *Observation) {
	sc := obs.Device.GetSignalController()
	ch := sc.GetChannels()
	if ch == nil {
		return
	}
	var yellows []uint16
	for n, c := range ch.Channel {
		if c.GetFlashState() == yangpkg.OpenitsSignalControl_ChannelFlashState_yellow {
			yellows = append(yellows, n)
		}
	}
	if len(yellows) < 2 {
		return
	}
	permissive := map[[2]uint16]bool{}
	if cm := sc.GetConflictMonitor(); cm != nil {
		for _, p := range cm.Permissive {
			a, b := p.GetChannelA(), p.GetChannelB()
			permissive[[2]uint16{a, b}] = true
			permissive[[2]uint16{b, a}] = true
		}
	}
	for i := 0; i < len(yellows); i++ {
		for j := i + 1; j < len(yellows); j++ {
			a, b := yellows[i], yellows[j]
			if !permissive[[2]uint16{a, b}] {
				t.Errorf("channels %d and %d both flash yellow but are not a compatible pair in the "+
					"MMU permissive record; conflicting movements must flash red (MUTCD flashing operation)", a, b)
			}
		}
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
