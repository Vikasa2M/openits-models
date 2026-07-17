package main

import (
	"context"
	"time"

	commonv1 "github.com/openits/openits-models/pkg/proto/openits/common/v1"
	reversiblelanev1 "github.com/openits/openits-models/pkg/proto/openits/reversible_lane/v1"
	yangpkg "github.com/openits/openits-models/pkg/yang/openits"
	"github.com/openits/openits-models/tools/conformance/tests"
)

// collectReversibleLane builds a fully-populated, spec-compliant
// reversible-lane observation: identity (including the two configured
// travel directions, which the module's musts require to be concrete
// cardinal opposites), a commanded changeover with its clearance-gate
// readback (changeover-permitted + blocking-interlocks), one
// segment/lane pair carrying a commanded LCS display that satisfies
// the green-implies-opposing-red-X invariant, and one declared,
// required, satisfied interlock.
func collectReversibleLane() (*yangpkg.Device, error) {
	dev := &yangpkg.Device{}
	rl := dev.GetOrCreateReversibleLane()

	// Identity: operator-provisioned intent in config, device-reported
	// mirror + hardware inventory in state — same config/state idiom as
	// every other service. direction-a/direction-b must be concrete and
	// cardinal opposites; northbound/southbound satisfies both musts.
	cfg := rl.GetOrCreateConfig()
	cfg.Id = strPtr("i395-nova-reversible")
	cfg.Name = strPtr("I-395 NOVA Reversible Lanes")
	cfg.Latitude = f64Ptr(38.8462)
	cfg.Longitude = f64Ptr(-77.0575)
	cfg.DirectionA = yangpkg.OpenitsReversibleLaneTypes_TravelDirection_northbound
	cfg.DirectionB = yangpkg.OpenitsReversibleLaneTypes_TravelDirection_southbound

	st := rl.GetOrCreateState()
	st.Id = strPtr("i395-nova-reversible")
	st.Name = strPtr("I-395 NOVA Reversible Lanes")
	st.Latitude = f64Ptr(38.8462)
	st.Longitude = f64Ptr(-77.0575)
	st.Make = strPtr("Transcore")
	st.Model = strPtr("LMS-4000")
	st.Firmware = strPtr("LMS-4000 3.1.0")
	st.Serial = strPtr("TC-2419-0044")

	// Control: commanded changeover to open northbound; control/state
	// mirrors the applied result and reports the changeover clearance
	// gate — changeover-permitted true with no blocking-interlocks,
	// consistent with the one interlock declared below being satisfied.
	control := rl.GetOrCreateControl()
	ctrlCfg := control.GetOrCreateConfig()
	ctrlCfg.TargetState = yangpkg.OpenitsReversibleLane_ReversibleLane_Control_Config_TargetState_open
	ctrlCfg.TargetDirection = yangpkg.OpenitsReversibleLaneTypes_TravelDirection_northbound

	ctrlSt := control.GetOrCreateState()
	ctrlSt.CurrentState = yangpkg.OpenitsReversibleLaneTypes_LaneFlowState_open
	ctrlSt.OpenDirection = yangpkg.OpenitsReversibleLaneTypes_TravelDirection_northbound
	ctrlSt.ChangeoverPermitted = boolPtr(true)
	// blocking-interlocks intentionally left empty: consistent with
	// changeover-permitted=true.

	// Segments/lanes: the segment-id/lane-id leafrefs resolve against
	// config/segment-id and config/lane-id respectively — both the
	// top-level key AND the nested config leaf must be set or
	// TestYANG_Validate fails with an empty leafref target set.
	segments := rl.GetOrCreateSegments()
	seg, err := segments.NewSegment("seg-1")
	if err != nil {
		return nil, err
	}
	segCfg := seg.GetOrCreateConfig()
	segCfg.SegmentId = strPtr("seg-1")
	segCfg.Name = strPtr("Segment 1 - Springfield to Pentagon")

	lane, err := seg.NewLane("lane-1")
	if err != nil {
		return nil, err
	}
	laneCfg := lane.GetOrCreateConfig()
	laneCfg.LaneId = strPtr("lane-1")
	// Commanded green-arrow toward direction-a requires a steady red-X
	// toward direction-b (MUTCD 4M) — the green-implies-opposing-red-X
	// invariant enforced by the lane config's module musts.
	laneCfg.LcsDirectionA = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_green_arrow
	laneCfg.LcsDirectionB = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_red_x

	laneSt := lane.GetOrCreateState()
	laneSt.LcsDirectionA = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_green_arrow
	laneSt.LcsDirectionB = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_red_x
	laneSt.GateState = yangpkg.OpenitsReversibleLane_GateState_open

	// Interlocks: one declared, required, satisfied interlock. The
	// facility's changeover-permitted=true readback above is only
	// consistent because every required interlock here is satisfied.
	interlocks := rl.GetOrCreateInterlocks()
	il, err := interlocks.NewInterlock("sweep-1")
	if err != nil {
		return nil, err
	}
	ilCfg := il.GetOrCreateConfig()
	ilCfg.InterlockId = strPtr("sweep-1")
	ilCfg.Kind = yangpkg.OpenitsReversibleLaneTypes_InterlockKind_sweep_confirmed
	ilCfg.Required = boolPtr(true)
	ilSt := il.GetOrCreateState()
	ilSt.Satisfied = boolPtr(true)

	return dev, nil
}

func subscribeReversibleLane(ctx context.Context, out chan<- tests.EventEnvelope, window time.Duration) error {
	base := "openits.us-va.vdot.nova.reversible-lane.i395-nova-reversible"
	src := "urn:openits:reversible-lane:us-va:vdot:nova:i395-nova-reversible"
	events := []tests.EventEnvelope{
		{
			Subject:  base + ".lane-state-changed",
			CEType:   "openits.reversible-lane.lane-state-changed.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VFA",
			CETime:   time.Now().UTC(),
			Data: &reversiblelanev1.LaneStateChanged{
				Kind:          "openits-reversible-lane-types:rl-lane-state-changed",
				PreviousState: reversiblelanev1.LaneFlowState_LANE_FLOW_STATE_CLOSED,
				NewState:      reversiblelanev1.LaneFlowState_LANE_FLOW_STATE_OPEN,
				NewDirection:  reversiblelanev1.TravelDirection_TRAVEL_DIRECTION_NORTHBOUND,
				InitiatedBy:   "tmc-ops",
			},
		},
		{
			Subject:  base + ".transition-timeout",
			CEType:   "openits.reversible-lane.transition-timeout.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VFB",
			CETime:   time.Now().UTC(),
			Data: &reversiblelanev1.TransitionTimeout{
				Kind:          "openits-reversible-lane-types:rl-transition-timeout",
				FromDirection: reversiblelanev1.TravelDirection_TRAVEL_DIRECTION_SOUTHBOUND,
				ToDirection:   reversiblelanev1.TravelDirection_TRAVEL_DIRECTION_NORTHBOUND,
				TimeoutS:      900,
				SequenceStep:  "sweep-verify",
			},
		},
		{
			Subject:  base + ".lcs-conflict-detected",
			CEType:   "openits.reversible-lane.lcs-conflict-detected.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VFC",
			CETime:   time.Now().UTC(),
			Data: &reversiblelanev1.LcsConflictDetected{
				Kind:          "openits-reversible-lane-types:rl-lcs-conflict-detected",
				SegmentId:     "seg-1",
				LcsDirectionA: reversiblelanev1.LcsIndication_LCS_INDICATION_GREEN_ARROW,
				LcsDirectionB: reversiblelanev1.LcsIndication_LCS_INDICATION_GREEN_ARROW,
			},
		},
		{
			Subject:  base + ".fault-raised",
			CEType:   "openits.reversible-lane.fault-raised.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VFD",
			CETime:   time.Now().UTC(),
			Data: &commonv1.FaultRaised{
				FaultId: "f-rl-001",
				Kind:    "openits-reversible-lane-types:reversible-lane-fault-interlock",
			},
		},
	}
	for _, e := range events {
		select {
		case <-ctx.Done():
			return nil
		case out <- e:
		}
	}
	select {
	case <-ctx.Done():
	case <-time.After(window):
	}
	return nil
}
