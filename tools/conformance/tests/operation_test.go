package tests

import (
	"testing"

	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
)

func bp(b bool) *bool { return &b }

type reqStage = yangpkg.E_OpenitsSignalControl_SignalController_Priority_State_ActiveRequest_Stage

func opDev(preemptFlag, priorityFlag *bool, preemptorActive bool, stage reqStage) *yangpkg.Device {
	dev := &yangpkg.Device{}
	sc := dev.GetOrCreateSignalController()
	op := sc.GetOrCreateOperation()
	op.PreemptActive = preemptFlag
	op.PriorityActive = priorityFlag
	if preemptorActive {
		p := sc.GetOrCreatePreemption().GetOrCreatePreemptor(1)
		p.GetOrCreateState().Active = bp(true)
	}
	if stage != 0 {
		r := sc.GetOrCreatePriority().GetOrCreateState().GetOrCreateActiveRequest("r1")
		r.Stage = stage
	}
	return dev
}

func TestOperation_ActiveFlagsMatchLists_Table(t *testing.T) {
	yes, no := bp(true), bp(false)
	const (
		requested = yangpkg.OpenitsSignalControl_SignalController_Priority_State_ActiveRequest_Stage_requested
		granted   = yangpkg.OpenitsSignalControl_SignalController_Priority_State_ActiveRequest_Stage_granted
		active    = yangpkg.OpenitsSignalControl_SignalController_Priority_State_ActiveRequest_Stage_active
	)
	cases := []struct {
		name string
		dev  *yangpkg.Device
		pass bool
	}{
		{"preempt true, none active", opDev(yes, nil, false, 0), false},
		{"preempt false, one active", opDev(no, nil, true, 0), false},
		{"preempt true, one active", opDev(yes, nil, true, 0), true},
		{"priority true, only requested", opDev(nil, yes, false, requested), false},
		{"priority true, granted", opDev(nil, yes, false, granted), true},
		{"priority false, active", opDev(nil, no, false, active), false},
		{"flags unset", opDev(nil, nil, true, active), true},
	}
	for _, c := range cases {
		r := Run("op", TestOperation_ActiveFlagsMatchLists, &Observation{Device: c.dev})
		if r.Pass != c.pass {
			t.Errorf("%s: pass=%v want %v (%s)", c.name, r.Pass, c.pass, r.Message)
		}
	}
}
