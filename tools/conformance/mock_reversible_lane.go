package main

import (
	"context"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"

	commonv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/common/v1"
	reversiblelanev1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/reversible_lane/v1"
	typesv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/types/v1"
	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
	"github.com/Vikasa2M/openits-models/tools/conformance/tests"
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
	cfg.DirectionA = yangpkg.OpenitsTypes_CarriagewayDirection_direction_northbound
	cfg.DirectionB = yangpkg.OpenitsTypes_CarriagewayDirection_direction_southbound

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
	ctrlCfg.TargetDirection = yangpkg.OpenitsTypes_CarriagewayDirection_direction_northbound

	ctrlSt := control.GetOrCreateState()
	ctrlSt.CurrentState = yangpkg.OpenitsReversibleLaneTypes_LaneFlowState_open
	ctrlSt.OpenDirection = yangpkg.OpenitsTypes_CarriagewayDirection_direction_northbound
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
	// Commanded downward-green-arrow toward direction-a requires a steady red-X
	// toward direction-b (MUTCD 4T.04) — the green-implies-opposing-red-X
	// invariant enforced by the lane config's module musts.
	laneCfg.LcsDirectionA = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_downward_green_arrow
	laneCfg.LcsDirectionB = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_red_x

	laneSt := lane.GetOrCreateState()
	laneSt.LcsDirectionA = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_downward_green_arrow
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

	// Gates: one warning gate under central control, and one barrier
	// whose position feedback has failed. gate-id is the list key and
	// the config leaf the key leafref points at. segment-id leafrefs
	// seg-1, which the segment list above already declares.
	gates := rl.GetOrCreateGates()
	wg, err := gates.NewGate("WG-1")
	if err != nil {
		return nil, err
	}
	wgCfg := wg.GetOrCreateConfig()
	wgCfg.GateId = strPtr("WG-1")
	wgCfg.GateType = yangpkg.OpenitsReversibleLaneTypes_GateType_gate_type_warning
	wgCfg.Name = strPtr("Warning gate 1")
	wgCfg.SegmentId = strPtr("seg-1")
	wgCfg.ControlsEntryFor = yangpkg.OpenitsTypes_CarriagewayDirection_direction_northbound
	wgSt := wg.GetOrCreateState()
	wgSt.Position = yangpkg.OpenitsReversibleLaneTypes_GatePosition_open
	wgSt.OperatingMode = yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneGateMode_reversible_lane_gate_mode_automatic
	wgSt.ControlSource = yangpkg.OpenitsTypes_ControlSource_control_central
	wgSt.InterlockBypassed = boolPtr(false)
	wgSt.OpenInhibited = boolPtr(false)
	wgSt.CloseInhibited = boolPtr(true)
	wgSt.ObservedAt = strPtr(time.Now().UTC().Format(time.RFC3339))

	bg, err := gates.NewGate("BG-2")
	if err != nil {
		return nil, err
	}
	bgCfg := bg.GetOrCreateConfig()
	bgCfg.GateId = strPtr("BG-2")
	bgCfg.GateType = yangpkg.OpenitsReversibleLaneTypes_GateType_gate_type_barrier
	bgCfg.Name = strPtr("Barrier gate 2")
	bgCfg.ControlsEntryFor = yangpkg.OpenitsTypes_CarriagewayDirection_direction_southbound
	bgSt := bg.GetOrCreateState()
	bgSt.Position = yangpkg.OpenitsReversibleLaneTypes_GatePosition_unknown
	bgSt.OperatingMode = yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneGateMode_reversible_lane_gate_mode_offline
	bgSt.OpenInhibited = boolPtr(false)
	bgSt.CloseInhibited = boolPtr(false)
	bgSt.ObservedAt = strPtr(time.Now().UTC().Format(time.RFC3339))

	// Every gate-fault identity is a legal inventory entry. Each names
	// its gate through the shared component reference (rl-component-gate
	// + gate-id). The feedback-loss entry names BG-2, whose position is
	// unknown.
	faults := rl.GetOrCreateFaults()
	gateFaults := []struct {
		id   string
		kind yangpkg.E_OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind
		gate string
	}{
		{"f-wg-1-close", yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind_reversible_lane_fault_gate_failed_to_close, "WG-1"},
		{"f-wg-1-open", yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind_reversible_lane_fault_gate_failed_to_open, "WG-1"},
		{"f-wg-1-estop", yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind_reversible_lane_fault_gate_emergency_stop, "WG-1"},
		{"f-wg-1-obstruction", yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind_reversible_lane_fault_gate_obstruction, "WG-1"},
		{"f-wg-1-drive", yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind_reversible_lane_fault_gate_drive_fault, "WG-1"},
		{"f-bg-2-feedback", yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind_reversible_lane_fault_gate_feedback_loss, "BG-2"},
	}
	for _, gf := range gateFaults {
		fault, err := faults.NewFault(gf.id)
		if err != nil {
			return nil, err
		}
		fault.FaultId = strPtr(gf.id)
		fault.Kind = gf.kind
		fault.Severity = yangpkg.OpenitsTypes_FaultSeverity_critical
		fault.FirstObserved = strPtr("2026-10-14T15:55:00Z")
		comp := fault.GetOrCreateComponent()
		comp.ComponentClass = yangpkg.OpenitsTypes_ComponentClass_rl_component_gate
		comp.ComponentId = strPtr(gf.gate)
	}

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
				PreviousState: reversiblelanev1.LaneFlowState_LANE_FLOW_STATE_CLOSED.Enum(),
				NewState:      reversiblelanev1.LaneFlowState_LANE_FLOW_STATE_OPEN.Enum(),
				NewDirection:  proto.String("openits-types:direction-northbound"),
				InitiatedBy:   proto.String("tmc-ops"),
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
				FromDirection: proto.String("openits-types:direction-southbound"),
				ToDirection:   proto.String("openits-types:direction-northbound"),
				TimeoutS:      proto.Uint32(900),
				SequenceStep:  proto.String("sweep-verify"),
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
				SegmentId:     proto.String("seg-1"),
				LcsDirectionA: reversiblelanev1.LcsIndication_LCS_INDICATION_DOWNWARD_GREEN_ARROW.Enum(),
				LcsDirectionB: reversiblelanev1.LcsIndication_LCS_INDICATION_DOWNWARD_GREEN_ARROW.Enum(),
			},
		},
		// A coherent changeover on lane-1: the closing direction steps green ->
		// yellow X -> red X (MUTCD 4T.04), then the opening direction goes red -> green.
		{
			Subject:  base + ".lcs-indication-changed",
			CEType:   "openits.reversible-lane.lcs-indication-changed.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VFH",
			CETime:   time.Now().UTC(),
			Data: &reversiblelanev1.LcsIndicationChanged{
				Kind:               "openits-reversible-lane-types:rl-lcs-indication-changed",
				SegmentId:          "seg-1",
				LaneId:             "lane-1",
				Face:               "openits-types:direction-southbound",
				PreviousIndication: reversiblelanev1.LcsIndication_LCS_INDICATION_DOWNWARD_GREEN_ARROW.Enum(),
				Indication:         reversiblelanev1.LcsIndication_LCS_INDICATION_YELLOW_X,
				OccurredAt:         timestamppb.New(time.Now().UTC()),
			},
		},
		{
			Subject:  base + ".lcs-indication-changed",
			CEType:   "openits.reversible-lane.lcs-indication-changed.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VFJ",
			CETime:   time.Now().UTC(),
			Data: &reversiblelanev1.LcsIndicationChanged{
				Kind:               "openits-reversible-lane-types:rl-lcs-indication-changed",
				SegmentId:          "seg-1",
				LaneId:             "lane-1",
				Face:               "openits-types:direction-southbound",
				PreviousIndication: reversiblelanev1.LcsIndication_LCS_INDICATION_YELLOW_X.Enum(),
				Indication:         reversiblelanev1.LcsIndication_LCS_INDICATION_RED_X,
				OccurredAt:         timestamppb.New(time.Now().UTC()),
			},
		},
		{
			Subject:  base + ".lcs-indication-changed",
			CEType:   "openits.reversible-lane.lcs-indication-changed.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VFK",
			CETime:   time.Now().UTC(),
			Data: &reversiblelanev1.LcsIndicationChanged{
				Kind:               "openits-reversible-lane-types:rl-lcs-indication-changed",
				SegmentId:          "seg-1",
				LaneId:             "lane-1",
				Face:               "openits-types:direction-northbound",
				PreviousIndication: reversiblelanev1.LcsIndication_LCS_INDICATION_RED_X.Enum(),
				Indication:         reversiblelanev1.LcsIndication_LCS_INDICATION_DOWNWARD_GREEN_ARROW,
				OccurredAt:         timestamppb.New(time.Now().UTC()),
			},
		},
		{
			Subject:  base + ".fault-raised",
			CEType:   "openits.reversible-lane.fault-raised.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VFD",
			CETime:   time.Now().UTC(),
			Data: &commonv1.FaultRaised{
				FirstObserved: timestamppb.New(time.Date(2026, 4, 19, 11, 45, 0, 0, time.UTC)),
				FaultId:       "f-rl-001",
				Kind:          "openits-reversible-lane-types:reversible-lane-fault-interlock",
				Severity:      commonv1.FaultSeverity_FAULT_SEVERITY_MAJOR,
			},
		},
		{
			// A gate-scope fault names its gate through the shared
			// component reference, not inside fault-id.
			Subject:  base + ".fault-raised",
			CEType:   "openits.reversible-lane.fault-raised.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VFG",
			CETime:   time.Now().UTC(),
			Data: &commonv1.FaultRaised{
				FirstObserved: timestamppb.New(time.Date(2026, 10, 14, 15, 55, 0, 0, time.UTC)),
				FaultId:       "f-bg-2-feedback",
				Kind:          "openits-reversible-lane-types:reversible-lane-fault-gate-feedback-loss",
				Severity:      commonv1.FaultSeverity_FAULT_SEVERITY_CRITICAL,
				Component: &typesv1.ComponentRef{
					ComponentClass: "openits-reversible-lane-types:rl-component-gate",
					ComponentId:    "BG-2",
				},
			},
		},
		{
			Subject:  base + ".gate-status-changed",
			CEType:   "openits.reversible-lane.gate-status-changed.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VFE",
			CETime:   time.Now().UTC(),
			Data: &reversiblelanev1.GateStatusChanged{
				Kind:              "openits-reversible-lane-types:rl-gate-status-changed",
				GateId:            "WG-1",
				Position:          reversiblelanev1.GatePosition_GATE_POSITION_OPEN,
				OperatingMode:     proto.String("openits-reversible-lane-types:reversible-lane-gate-mode-automatic"),
				ControlSource:     proto.String("openits-types:control-central"),
				InterlockBypassed: proto.Bool(false),
				OpenInhibited:     proto.Bool(false),
				CloseInhibited:    proto.Bool(true),
				OccurredAt:        timestamppb.New(time.Now().UTC()),
			},
		},
		{
			Subject:  base + ".gate-status-changed",
			CEType:   "openits.reversible-lane.gate-status-changed.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VFF",
			CETime:   time.Now().UTC(),
			Data: &reversiblelanev1.GateStatusChanged{
				Kind:           "openits-reversible-lane-types:rl-gate-status-changed",
				GateId:         "BG-2",
				Position:       reversiblelanev1.GatePosition_GATE_POSITION_UNKNOWN,
				OperatingMode:  proto.String("openits-reversible-lane-types:reversible-lane-gate-mode-offline"),
				OpenInhibited:  proto.Bool(false),
				CloseInhibited: proto.Bool(false),
				OccurredAt:     timestamppb.New(time.Now().UTC()),
			},
		},
	}
	stampHeaders(events, "mock-boot-1")
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
