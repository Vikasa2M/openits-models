package tests

import (
	"sort"
	"strings"
	"time"

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
// gate-state is the barrier-gate readback for the lane, expected present
// whenever the lane reports live state. The MUTCD 4T.04 command rules
// (a downward green arrow needs the opposing red X, and the other
// prohibited pairs) are checked by TestReversibleLane_NoProhibitedCommand.

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

func TestReversibleLaneGate_HasIdentity(t *T, obs *Observation) {
	gates := obs.Device.GetReversibleLane().GetGates()
	if gates == nil || len(gates.Gate) == 0 {
		t.Errorf("no gates populated")
		return
	}
	for id, g := range gates.Gate {
		cfg := g.GetConfig()
		if cfg == nil || cfg.GetGateId() == "" {
			t.Errorf("gate %q config/gate-id is unset", id)
		}
	}
}

// faultGateID returns the gate a fault-inventory entry names through the
// shared component reference (component-class rl-component-gate), or ""
// when the entry names no gate.
func faultGateID(f *yangpkg.OpenitsReversibleLane_ReversibleLane_Faults_Fault) string {
	c := f.GetComponent()
	if c == nil || c.ComponentClass != yangpkg.OpenitsTypes_ComponentClass_rl_component_gate {
		return ""
	}
	return c.GetComponentId()
}

func TestReversibleLaneFault_FeedbackLossPositionUnknown(t *T, obs *Observation) {
	rl := obs.Device.GetReversibleLane()
	faults := rl.GetFaults()
	if faults == nil {
		return
	}
	for _, f := range faults.Fault {
		if f.GetKind() != yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind_reversible_lane_fault_gate_feedback_loss {
			continue
		}
		gateID := faultGateID(f)
		if gateID == "" {
			t.Errorf("feedback-loss fault %q names no gate: component must be rl-component-gate with the gate-id", f.GetFaultId())
			continue
		}
		gates := rl.GetGates()
		g := (*yangpkg.OpenitsReversibleLane_ReversibleLane_Gates_Gate)(nil)
		if gates != nil {
			g = gates.Gate[gateID]
		}
		if g == nil || g.GetState() == nil {
			t.Errorf("feedback-loss fault %q names gate %q, which has no state", f.GetFaultId(), gateID)
			continue
		}
		if g.GetState().Position != yangpkg.OpenitsReversibleLaneTypes_GatePosition_unknown {
			t.Errorf("gate %q has feedback-loss raised but position is %s; position MUST be unknown", gateID, g.GetState().Position)
		}
	}
}

// gateFaultKind is the base reversible-lane-fault-gate and every
// identity declared with that base. ygot enums do not carry the identity
// derivation graph, so the set is the schema's own list.
func gateFaultKind(c yangpkg.E_OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind) bool {
	switch c {
	case yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind_reversible_lane_fault_gate,
		yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind_reversible_lane_fault_gate_drive_fault,
		yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind_reversible_lane_fault_gate_emergency_stop,
		yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind_reversible_lane_fault_gate_failed_to_close,
		yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind_reversible_lane_fault_gate_failed_to_open,
		yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind_reversible_lane_fault_gate_feedback_loss,
		yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind_reversible_lane_fault_gate_obstruction:
		return true
	default:
		return false
	}
}

// TestReversibleLaneFault_GateFaultNamesExistingGate: every gate-scope
// fault in the inventory names its gate through component
// (rl-component-gate + gate-id), and that gate is configured.
func TestReversibleLaneFault_GateFaultNamesExistingGate(t *T, obs *Observation) {
	rl := obs.Device.GetReversibleLane()
	faults := rl.GetFaults()
	if faults == nil {
		return
	}
	var gates map[string]*yangpkg.OpenitsReversibleLane_ReversibleLane_Gates_Gate
	if g := rl.GetGates(); g != nil {
		gates = g.Gate
	}
	for _, f := range faults.Fault {
		if !gateFaultKind(f.GetKind()) {
			continue
		}
		gateID := faultGateID(f)
		if gateID == "" {
			t.Errorf("fault %q kind %s is gate-scoped but its component does not name a gate (want component-class rl-component-gate)", f.GetFaultId(), f.GetKind())
			continue
		}
		if gates[gateID] == nil {
			t.Errorf("fault %q kind %s names gate %q, which is not in gates/gate", f.GetFaultId(), f.GetKind(), gateID)
		}
	}
}

// TestReversibleLaneEvent_GateFaultRaisedNamesGate: a fault-raised whose
// kind is a reversible-lane gate fault names its gate through component.
func TestReversibleLaneEvent_GateFaultRaisedNamesGate(t *T, obs *Observation) {
	const prefix = "openits-reversible-lane-types:reversible-lane-fault-gate"
	for _, e := range obs.Events {
		fr, ok := e.Data.(*commonv1.FaultRaised)
		if !ok || !strings.HasPrefix(fr.GetKind(), prefix) {
			continue
		}
		c := fr.GetComponent()
		if c.GetComponentClass() != "openits-reversible-lane-types:rl-component-gate" || c.GetComponentId() == "" {
			t.Errorf("fault-raised %q kind %s does not name its gate via component (rl-component-gate + gate-id)", fr.GetFaultId(), fr.GetKind())
		}
	}
}

func TestReversibleLaneEvent_GateStatusChangedMirrorsLiveState(t *T, obs *Observation) {
	dev := obs.DeviceAfter
	if dev == nil {
		dev = obs.Device
	}
	gates := dev.GetReversibleLane().GetGates()
	var events []*reversiblelanev1.GateStatusChanged
	for _, e := range obs.Events {
		ev, ok := e.Data.(*reversiblelanev1.GateStatusChanged)
		if ok {
			events = append(events, ev)
		}
	}
	if len(events) == 0 {
		t.Errorf("no gate-status-changed event observed during %s window", obs.Window)
		return
	}
	if gates == nil {
		return
	}
	for id, g := range gates.Gate {
		live := g.GetState()
		if live == nil || live.ObservedAt == nil {
			continue
		}
		measured, err := time.Parse(time.RFC3339Nano, live.GetObservedAt())
		if err != nil {
			continue
		}
		var latest *reversiblelanev1.GateStatusChanged
		for _, ev := range events {
			if ev.GetGateId() != id || ev.GetOccurredAt() == nil || ev.GetOccurredAt().AsTime().After(measured) {
				continue
			}
			if latest == nil || gateChangeNewer(ev, latest) {
				latest = ev
			}
		}
		if latest == nil {
			continue
		}
		if live.Position != yangpkg.OpenitsReversibleLaneTypes_GatePosition_UNSET && !gatePositionAgrees(live.Position, latest.GetPosition()) {
			t.Errorf("gate %q: live position=%s but the latest gate-status-changed says %s", id, live.Position, latest.GetPosition())
		}
		if live.OperatingMode != yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneGateMode_UNSET && latest.GetOperatingMode() != gateModeIdentity(live.OperatingMode) {
			t.Errorf("gate %q: live operating-mode=%s but the latest gate-status-changed says %q", id, live.OperatingMode, latest.GetOperatingMode())
		}
		if live.ControlSource != yangpkg.OpenitsTypes_ControlSource_UNSET {
			want, ok := controlSourceIdentity(live.ControlSource)
			if !ok {
				t.Errorf("gate %q: live control-source %s has no identity string", id, live.ControlSource)
			} else if latest.GetControlSource() != want {
				t.Errorf("gate %q: live control-source=%s but the latest gate-status-changed says %q", id, live.ControlSource, latest.GetControlSource())
			}
		}
		if live.InterlockBypassed != nil && live.GetInterlockBypassed() != latest.GetInterlockBypassed() {
			t.Errorf("gate %q: live interlock-bypassed=%t but the latest gate-status-changed says %t", id, live.GetInterlockBypassed(), latest.GetInterlockBypassed())
		}
		if live.OpenInhibited != nil && live.GetOpenInhibited() != latest.GetOpenInhibited() {
			t.Errorf("gate %q: live open-inhibited=%t but the latest gate-status-changed says %t", id, live.GetOpenInhibited(), latest.GetOpenInhibited())
		}
		if live.CloseInhibited != nil && live.GetCloseInhibited() != latest.GetCloseInhibited() {
			t.Errorf("gate %q: live close-inhibited=%t but the latest gate-status-changed says %t", id, live.GetCloseInhibited(), latest.GetCloseInhibited())
		}
	}
}

func gateChangeNewer(a, b *reversiblelanev1.GateStatusChanged) bool {
	at, bt := a.GetOccurredAt().AsTime(), b.GetOccurredAt().AsTime()
	if !at.Equal(bt) {
		return at.After(bt)
	}
	return a.GetSequence() > b.GetSequence()
}

func gatePositionAgrees(live yangpkg.E_OpenitsReversibleLaneTypes_GatePosition, ev reversiblelanev1.GatePosition) bool {
	switch live {
	case yangpkg.OpenitsReversibleLaneTypes_GatePosition_unknown:
		return ev == reversiblelanev1.GatePosition_GATE_POSITION_UNKNOWN
	case yangpkg.OpenitsReversibleLaneTypes_GatePosition_closing:
		return ev == reversiblelanev1.GatePosition_GATE_POSITION_CLOSING
	case yangpkg.OpenitsReversibleLaneTypes_GatePosition_closed:
		return ev == reversiblelanev1.GatePosition_GATE_POSITION_CLOSED
	case yangpkg.OpenitsReversibleLaneTypes_GatePosition_opening:
		return ev == reversiblelanev1.GatePosition_GATE_POSITION_OPENING
	case yangpkg.OpenitsReversibleLaneTypes_GatePosition_open:
		return ev == reversiblelanev1.GatePosition_GATE_POSITION_OPEN
	case yangpkg.OpenitsReversibleLaneTypes_GatePosition_intermediate:
		return ev == reversiblelanev1.GatePosition_GATE_POSITION_INTERMEDIATE
	default:
		return false
	}
}

func gateModeIdentity(v yangpkg.E_OpenitsReversibleLaneTypes_ReversibleLaneGateMode) string {
	switch v {
	case yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneGateMode_reversible_lane_gate_mode_automatic:
		return "openits-reversible-lane-types:reversible-lane-gate-mode-automatic"
	case yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneGateMode_reversible_lane_gate_mode_offline:
		return "openits-reversible-lane-types:reversible-lane-gate-mode-offline"
	case yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneGateMode_reversible_lane_gate_mode_manual:
		return "openits-reversible-lane-types:reversible-lane-gate-mode-manual"
	case yangpkg.OpenitsReversibleLaneTypes_ReversibleLaneGateMode_reversible_lane_gate_mode_unknown:
		return "openits-reversible-lane-types:reversible-lane-gate-mode-unknown"
	default:
		return ""
	}
}

func controlSourceIdentity(v yangpkg.E_OpenitsTypes_ControlSource) (string, bool) {
	switch v {
	case yangpkg.OpenitsTypes_ControlSource_control_central:
		return "openits-types:control-central", true
	case yangpkg.OpenitsTypes_ControlSource_control_local_backup:
		return "openits-types:control-local-backup", true
	case yangpkg.OpenitsTypes_ControlSource_control_manual:
		return "openits-types:control-manual", true
	case yangpkg.OpenitsTypes_ControlSource_control_failed:
		return "openits-types:control-failed", true
	case yangpkg.OpenitsTypes_ControlSource_control_schedule:
		return "openits-types:control-schedule", true
	case yangpkg.OpenitsTypes_ControlSource_control_other:
		return "openits-types:control-other", true
	default:
		return "", false
	}
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

// direction-open-permitted gates the lcs-reverse step of a changeover: while
// it is false, no lane may show a proceed indication (downward-green-arrow,
// either white left-turn arrow, or the legacy flashing-yellow-x a field head
// may still report) toward the
// direction the facility is changing over to. At rest the flag is routinely
// false (the sweep interlock is not satisfied while traffic is flowing), so
// the check applies only while a changeover is running, and only to the
// lcs head facing the new direction. Traffic still flowing in the old
// direction is not a violation.
func TestRL_DirectionOpenGateBlocksReverse(t *T, obs *Observation) {
	rl := obs.Device.GetReversibleLane()
	st := rl.GetControl().GetState()
	if st == nil || st.DirectionOpenPermitted == nil || *st.DirectionOpenPermitted {
		return // permitted, or not reported: nothing to gate
	}
	if st.CurrentState != yangpkg.OpenitsReversibleLaneTypes_LaneFlowState_in_transition {
		return
	}
	to := st.GetTransition().GetToDirection()
	cfg := rl.GetConfig()
	if to == 0 || cfg == nil {
		return
	}
	segs := rl.GetSegments()
	if segs == nil {
		return
	}
	proceed := func(i yangpkg.E_OpenitsReversibleLaneTypes_LcsIndication) bool {
		return i == yangpkg.OpenitsReversibleLaneTypes_LcsIndication_downward_green_arrow ||
			i == yangpkg.OpenitsReversibleLaneTypes_LcsIndication_white_two_way_left_turn_arrow ||
			i == yangpkg.OpenitsReversibleLaneTypes_LcsIndication_white_one_way_left_turn_arrow ||
			i == yangpkg.OpenitsReversibleLaneTypes_LcsIndication_flashing_yellow_x
	}
	for segID, seg := range segs.Segment {
		for laneID, lane := range seg.Lane {
			ls := lane.GetState()
			if ls == nil {
				continue
			}
			switch {
			case to == cfg.DirectionA && proceed(ls.GetLcsDirectionA()):
				t.Errorf("segment %q lane %q shows %v toward the new direction (direction-a) while direction-open-permitted is false",
					segID, laneID, ls.GetLcsDirectionA())
			case to == cfg.DirectionB && proceed(ls.GetLcsDirectionB()):
				t.Errorf("segment %q lane %q shows %v toward the new direction (direction-b) while direction-open-permitted is false",
					segID, laneID, ls.GetLcsDirectionB())
			}
		}
	}
}

// ----- lane-use signal sequencing (MUTCD 4T.04) -----

// lcsTransitionLegal reports whether a head may change from prev to cur.
// MUTCD 4T.04 paragraph 03: a downward green arrow ends with a yellow X or a
// red X, and a yellow X ends with a red X. Paragraph 05 adds one exception:
// a downward green arrow may change to a white two-way left-turn arrow when
// the red X continues to be shown to the opposite direction, so
// oppositeRed says whether the other face of the lane currently shows a red X.
// Every other transition is unconstrained by this check.
func lcsTransitionLegal(prev, cur reversiblelanev1.LcsIndication, oppositeRed bool) bool {
	switch prev {
	case reversiblelanev1.LcsIndication_LCS_INDICATION_DOWNWARD_GREEN_ARROW:
		if cur == reversiblelanev1.LcsIndication_LCS_INDICATION_WHITE_TWO_WAY_LEFT_TURN_ARROW {
			return oppositeRed
		}
		return cur == reversiblelanev1.LcsIndication_LCS_INDICATION_YELLOW_X ||
			cur == reversiblelanev1.LcsIndication_LCS_INDICATION_RED_X
	case reversiblelanev1.LcsIndication_LCS_INDICATION_YELLOW_X:
		return cur == reversiblelanev1.LcsIndication_LCS_INDICATION_RED_X
	}
	return true
}

// TestReversibleLane_IndicationSequence walks lcs-indication-changed events and
// refuses a head that leaves a downward green arrow or a yellow X for anything
// other than the closing sequence MUTCD 4T.04 requires (a green arrow may also
// become a white two-way left-turn arrow while the opposite face shows a red X,
// paragraph 05; the clearance duration that paragraph also requires is not yet
// checked). Events are processed in the event-header sequence order within an
// epoch (arrival order breaks ties and separates epochs).
// Each (segment, lane, face) is one head and the check is stateful per head:
// the last observed indication is remembered. An event whose previous-indication
// is present but differs from the remembered value is a chain break and fails.
// An event with no previous-indication takes the remembered value (if any) as
// its previous. A repeat of the same value is treated as no change and passes.
// A change into unknown is a readback dropout, not a signal violation: it is
// not checked and clears the head's remembered indication. The opposite face is
// the other head seen for the same segment and lane; if none has been seen it
// is treated as not red. A window with no lcs-indication-changed event passes:
// a quiet window has no changes to check.
func TestReversibleLane_IndicationSequence(t *T, obs *Observation) {
	type head struct{ seg, lane, face string }
	var evs []*reversiblelanev1.LcsIndicationChanged
	for _, e := range obs.Events {
		if ev, ok := e.Data.(*reversiblelanev1.LcsIndicationChanged); ok {
			evs = append(evs, ev)
		}
	}
	epochOrder := map[string]int{}
	for _, ev := range evs {
		if _, ok := epochOrder[ev.GetSequenceEpoch()]; !ok {
			epochOrder[ev.GetSequenceEpoch()] = len(epochOrder)
		}
	}
	sort.SliceStable(evs, func(i, j int) bool {
		ei, ej := epochOrder[evs[i].GetSequenceEpoch()], epochOrder[evs[j].GetSequenceEpoch()]
		if ei != ej {
			return ei < ej
		}
		return evs[i].GetSequence() < evs[j].GetSequence()
	})
	last := map[head]reversiblelanev1.LcsIndication{}
	oppositeRed := func(h head) bool {
		seen := false
		for o, ind := range last {
			if o.seg != h.seg || o.lane != h.lane || o.face == h.face {
				continue
			}
			seen = true
			if ind != reversiblelanev1.LcsIndication_LCS_INDICATION_RED_X {
				return false
			}
		}
		return seen
	}
	for _, ev := range evs {
		h := head{ev.GetSegmentId(), ev.GetLaneId(), ev.GetFace()}
		cur := ev.GetIndication()
		remembered, have := last[h]
		var prev reversiblelanev1.LcsIndication
		known := false
		if ev.PreviousIndication != nil {
			prev, known = ev.GetPreviousIndication(), true
			if have && prev != remembered {
				t.Errorf("segment %q lane %q face %q: chain break: event says previous %v but the last observed indication was %v",
					h.seg, h.lane, h.face, prev, remembered)
			}
		} else if have {
			prev, known = remembered, true
		}
		if cur == reversiblelanev1.LcsIndication_LCS_INDICATION_UNKNOWN {
			delete(last, h)
			continue
		}
		last[h] = cur
		if known && prev != cur && !lcsTransitionLegal(prev, cur, oppositeRed(h)) {
			t.Errorf("segment %q lane %q face %q: %v -> %v violates the MUTCD 4T.04 closing sequence (green arrow -> yellow X or red X, or a white two-way arrow only while the opposite face shows a red X; yellow X -> red X)",
				h.seg, h.lane, h.face, prev, cur)
		}
	}
}

// lcsProhibitedPair names the MUTCD 4T.04 paragraph 02 prohibition a commanded
// pair breaks, or "" when the pair is allowed. It also refuses the legacy
// flashing-yellow-x and the readback-only unknown, which may be reported but
// never commanded.
func lcsProhibitedPair(a, b yangpkg.E_OpenitsReversibleLaneTypes_LcsIndication) string {
	const (
		green = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_downward_green_arrow
		red   = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_red_x
		yel   = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_yellow_x
		fl    = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_flashing_yellow_x
		unk   = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_unknown
		two   = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_white_two_way_left_turn_arrow
		one   = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_white_one_way_left_turn_arrow
	)
	switch {
	case a == unk || b == unk:
		return "unknown commanded (a readback-only value)"
	case a == fl || b == fl:
		return "legacy flashing-yellow-x commanded"
	case (a == green && b != red) || (b == green && a != red):
		return "downward green arrow without a steady red X to the opposite direction"
	case a == yel && b == yel:
		return "yellow X to both directions (4T.04 B)"
	case a == one && b == one:
		return "white one-way left-turn arrow to both directions (4T.04 C)"
	case (a == two && b == one) || (a == one && b == two):
		return "white two-way with white one-way left-turn arrow (4T.04 F)"
	case (a == one && b == yel) || (a == yel && b == one):
		return "white one-way left-turn arrow against yellow X (4T.04 G)"
	}
	return ""
}

// TestReversibleLane_NoProhibitedCommand checks every lane's commanded
// lcs-direction-a/b pair against the MUTCD 4T.04 paragraph 02 prohibitions.
func TestReversibleLane_NoProhibitedCommand(t *T, obs *Observation) {
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
				continue
			}
			checked = true
			if why := lcsProhibitedPair(cfg.LcsDirectionA, cfg.LcsDirectionB); why != "" {
				t.Errorf("segment %q lane %q commands (%v, %v): %s", segID, laneID, cfg.LcsDirectionA, cfg.LcsDirectionB, why)
			}
		}
	}
	if !checked {
		t.Errorf("no lane config populated to check")
	}
}
