package tests

import (
	"strings"

	yangpkg "github.com/openits/openits-models/pkg/yang/openits"
)

// TestPreemption_RailTrackClearance re-implements, in Go, the cut-2c
// config-true must "not(type = 'railroad') or track-clearance" on the
// preemptor data tree (ygot's Validate() does not evaluate XPath `must`
// statements, so this is the only place the MUTCD Ch. 8C invariant is
// actually checked end-to-end). A mutation that drops the track-
// clearance container, or zeros its green-seconds, on a railroad
// preemptor fails this check. This is a data-tree check, distinct from
// TestPreemption_EventFired/EventTypeShape below, which check the
// preemption-activated notification event, not the commanded table.
func TestPreemption_RailTrackClearance(t *T, obs *Observation) {
	pre := obs.Device.GetSignalController().GetPreemption()
	if pre == nil {
		return
	}
	for _, pr := range pre.Preemptor {
		cfg := pr.GetConfig()
		if cfg.GetType() != yangpkg.OpenitsSignalControlTypes_PreemptionType_preempt_railroad {
			continue
		}
		if cfg.GetTrackClearance() == nil || cfg.GetTrackClearance().GetGreenSeconds() == 0 {
			t.Errorf("railroad preemptor %d has no track-clearance green time", pr.GetPreemptorId())
		}
	}
}

func TestPreemption_EventFired(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if strings.HasSuffix(e.Subject, ".preemption-activated") {
			return
		}
	}
	t.Errorf("no preemption-activated event observed during %s window", obs.Window)
}

func TestPreemption_EventTypeShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".preemption-activated") {
			continue
		}
		want := "openits.signal-control.preemption-activated.v1"
		if e.CEType != want {
			t.Errorf("preemption ce-type %q, want %q", e.CEType, want)
		}
		return
	}
}
