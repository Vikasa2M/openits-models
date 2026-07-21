package tests

import (
	"strings"

	commonv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/common/v1"
	reversiblelanev1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/reversible_lane/v1"
	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
)

// ----- identity -----
//
// Device-reported identity lives in reversible-lane/state (config
// false): it mirrors the operator-provisioned reversible-lane/config
// identity and adds device-hardware (make/model/firmware/serial) —
// same config/state idiom as every other service.

func TestReversibleLaneIdentity_FacilityID(t *T, obs *Observation) {
	st := obs.Device.GetReversibleLane().GetState()
	if st == nil || st.GetId() == "" {
		t.Errorf("state/id is unset")
	}
}

// ----- interlocks -----
//
// Interlocks are a declared config set (typed kind + required flag)
// paired with a config-false satisfied readback. Every declared
// interlock must carry a typed kind and an explicit required flag —
// an untyped or unqualified interlock can't be reasoned about by a
// controller deciding whether to block a changeover.

func TestReversibleLaneInterlock_HasKind(t *T, obs *Observation) {
	il := obs.Device.GetReversibleLane().GetInterlocks()
	if il == nil || len(il.Interlock) == 0 {
		t.Errorf("no interlocks populated")
		return
	}
	for id, entry := range il.Interlock {
		cfg := entry.GetConfig()
		if cfg == nil {
			t.Errorf("interlock %q missing config", id)
			continue
		}
		if cfg.Kind == yangpkg.OpenitsReversibleLaneTypes_InterlockKind_UNSET {
			t.Errorf("interlock %q config/kind is unset", id)
		}
		if cfg.Required == nil {
			t.Errorf("interlock %q config/required is unset", id)
		}
	}
}

// ----- control / changeover clearance gate -----
//
// control/state/changeover-permitted is the machine-observable rollup
// of every REQUIRED interlock's satisfaction; blocking-interlocks
// names the ones currently blocking a changeover. The two must be
// mutually consistent: permitted true implies no blockers, and a
// non-empty blocker list implies permitted is false.

func TestReversibleLaneControl_ChangeoverPermittedPresent(t *T, obs *Observation) {
	cs := obs.Device.GetReversibleLane().GetControl().GetState()
	if cs == nil {
		t.Fatalf("control/state container missing")
	}
	if cs.ChangeoverPermitted == nil {
		t.Errorf("control/state/changeover-permitted is unset")
	}
}

func TestReversibleLaneControl_BlockingConsistent(t *T, obs *Observation) {
	cs := obs.Device.GetReversibleLane().GetControl().GetState()
	if cs == nil {
		t.Fatalf("control/state container missing")
	}
	if cs.ChangeoverPermitted == nil {
		t.Fatalf("control/state/changeover-permitted is unset")
	}
	permitted := *cs.ChangeoverPermitted
	blocked := len(cs.BlockingInterlocks) > 0
	if permitted && blocked {
		t.Errorf("changeover-permitted=true but blocking-interlocks is non-empty: %v", cs.BlockingInterlocks)
	}
	if !permitted && !blocked {
		t.Errorf("changeover-permitted=false but blocking-interlocks is empty; no reason given for the refusal")
	}
}

// ----- segments / lanes -----
//
// Per MUTCD Chapter 4M, a commanded green-arrow or flashing-yellow-X
// toward one direction requires a steady red-X toward the other — the
// module's config-true must on segments/segment/lane/config. gate-state
// is the barrier-gate readback for the lane, expected present whenever
// the lane reports live state.

func isGreenOrFlashingYellowX(v yangpkg.E_OpenitsReversibleLaneTypes_LcsIndication) bool {
	return v == yangpkg.OpenitsReversibleLaneTypes_LcsIndication_green_arrow ||
		v == yangpkg.OpenitsReversibleLaneTypes_LcsIndication_flashing_yellow_x
}

func TestReversibleLaneLane_GreenImpliesOpposingRedX(t *T, obs *Observation) {
	segs := obs.Device.GetReversibleLane().GetSegments()
	if segs == nil || len(segs.Segment) == 0 {
		t.Errorf("no segments populated")
		return
	}
	checked := false
	for segID, seg := range segs.Segment {
		for laneID, lane := range seg.Lane {
			cfg := lane.GetConfig()
			if cfg == nil {
				t.Errorf("segment %q lane %q missing config", segID, laneID)
				continue
			}
			checked = true
			a, b := cfg.LcsDirectionA, cfg.LcsDirectionB
			if isGreenOrFlashingYellowX(a) && b != yangpkg.OpenitsReversibleLaneTypes_LcsIndication_red_x {
				t.Errorf("segment %q lane %q: lcs-direction-a=%v requires lcs-direction-b=red-x, got %v", segID, laneID, a, b)
			}
			if isGreenOrFlashingYellowX(b) && a != yangpkg.OpenitsReversibleLaneTypes_LcsIndication_red_x {
				t.Errorf("segment %q lane %q: lcs-direction-b=%v requires lcs-direction-a=red-x, got %v", segID, laneID, b, a)
			}
		}
	}
	if !checked {
		t.Errorf("no lanes populated to check")
	}
}

func TestReversibleLaneLane_GateStatePresent(t *T, obs *Observation) {
	segs := obs.Device.GetReversibleLane().GetSegments()
	if segs == nil || len(segs.Segment) == 0 {
		t.Errorf("no segments populated")
		return
	}
	checked := false
	for segID, seg := range segs.Segment {
		for laneID, lane := range seg.Lane {
			st := lane.GetState()
			if st == nil {
				t.Errorf("segment %q lane %q missing state", segID, laneID)
				continue
			}
			checked = true
			if st.GateState == yangpkg.OpenitsReversibleLane_GateState_UNSET {
				t.Errorf("segment %q lane %q: state/gate-state is unset", segID, laneID)
			}
		}
	}
	if !checked {
		t.Errorf("no lane state populated to check")
	}
}

// ----- event shapes -----

func TestReversibleLaneEvent_LcsConflictShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".lcs-conflict-detected") {
			continue
		}
		want := "openits.reversible-lane.lcs-conflict-detected.v1"
		if e.CEType != want {
			t.Errorf("lcs-conflict-detected ce-type %q, want %q", e.CEType, want)
		}
		conflict, ok := e.Data.(*reversiblelanev1.LcsConflictDetected)
		if !ok {
			t.Errorf("lcs-conflict-detected Data is %T, want *reversiblelanev1.LcsConflictDetected", e.Data)
			return
		}
		if conflict.GetSegmentId() == "" {
			t.Errorf("lcs-conflict-detected Data segment-id is empty")
		}
		if conflict.GetLcsDirectionA() == reversiblelanev1.LcsIndication_LCS_INDICATION_UNKNOWN {
			t.Errorf("lcs-conflict-detected Data lcs-direction-a is unset")
		}
		if conflict.GetLcsDirectionB() == reversiblelanev1.LcsIndication_LCS_INDICATION_UNKNOWN {
			t.Errorf("lcs-conflict-detected Data lcs-direction-b is unset")
		}
		return
	}
	t.Errorf("no lcs-conflict-detected event observed during %s window", obs.Window)
}

func TestReversibleLaneEvent_FaultRaisedShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".fault-raised") {
			continue
		}
		want := "openits.reversible-lane.fault-raised.v1"
		if e.CEType != want {
			t.Errorf("fault-raised ce-type %q, want %q", e.CEType, want)
		}
		fr, ok := e.Data.(*commonv1.FaultRaised)
		if !ok {
			t.Errorf("fault-raised Data is %T, want *commonv1.FaultRaised", e.Data)
			return
		}
		if fr.GetFaultId() == "" {
			t.Errorf("fault-raised Data fault-id is empty")
		}
		if fr.GetKind() == "" {
			t.Errorf("fault-raised Data kind is empty")
		}
		return
	}
	t.Errorf("no fault-raised event observed during %s window", obs.Window)
}

// When the lane group is explicitly not permitted to open toward a direction
// (a transition / interlock state), no lane may show a proceed (green-arrow)
// indication in either direction. Guards the open-permit safety interlock.
func TestRL_DirectionOpenGateBlocksReverse(t *T, obs *Observation) {
	rl := obs.Device.GetReversibleLane()
	if rl == nil || rl.GetControl() == nil || rl.GetControl().GetState() == nil {
		return
	}
	st := rl.GetControl().GetState()
	if st.DirectionOpenPermitted == nil || *st.DirectionOpenPermitted {
		return // permitted, or not reported — nothing to gate
	}
	segs := rl.GetSegments()
	if segs == nil {
		return
	}
	green := yangpkg.OpenitsReversibleLaneTypes_LcsIndication_green_arrow
	for _, seg := range segs.Segment {
		for _, lane := range seg.Lane {
			ls := lane.GetState()
			if ls == nil {
				continue
			}
			if ls.GetLcsDirectionA() == green || ls.GetLcsDirectionB() == green {
				t.Errorf("direction-open-permitted is false but a lane shows a green-arrow indication")
			}
		}
	}
}
