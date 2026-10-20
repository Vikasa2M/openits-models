package tests

import yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"

// TestOperation_ActiveFlagsMatchLists checks the derived preempt-active
// and priority-active flags against the lists they summarise. YANG cannot
// express the count cleanly, so the rule is enforced here. An unset flag
// is not checked.
func TestOperation_ActiveFlagsMatchLists(t *T, obs *Observation) {
	sc := obs.Device.GetSignalController()
	op := sc.GetOperation()
	if op == nil {
		return
	}
	if op.PreemptActive != nil {
		want := false
		if pre := sc.GetPreemption(); pre != nil {
			for _, p := range pre.Preemptor {
				if p.GetState().GetActive() {
					want = true
				}
			}
		}
		if *op.PreemptActive != want {
			t.Errorf("operation/preempt-active = %v, but a preemptor is active = %v", *op.PreemptActive, want)
		}
	}
	if op.PriorityActive != nil {
		want := false
		if st := sc.GetPriority().GetState(); st != nil {
			for _, r := range st.ActiveRequest {
				switch r.GetStage() {
				case yangpkg.OpenitsSignalControl_SignalController_Priority_State_ActiveRequest_Stage_granted,
					yangpkg.OpenitsSignalControl_SignalController_Priority_State_ActiveRequest_Stage_active:
					want = true
				}
			}
		}
		if *op.PriorityActive != want {
			t.Errorf("operation/priority-active = %v, but a request is granted or active = %v", *op.PriorityActive, want)
		}
	}
}
