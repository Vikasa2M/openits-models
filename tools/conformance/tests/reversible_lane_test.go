package tests

import (
	"testing"

	reversiblelanev1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/reversible_lane/v1"
	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
)

const (
	pDark  = reversiblelanev1.LcsIndication_LCS_INDICATION_DARK
	pRed   = reversiblelanev1.LcsIndication_LCS_INDICATION_RED_X
	pYel   = reversiblelanev1.LcsIndication_LCS_INDICATION_YELLOW_X
	pGreen = reversiblelanev1.LcsIndication_LCS_INDICATION_GREEN_ARROW
	pTwo   = reversiblelanev1.LcsIndication_LCS_INDICATION_WHITE_TWO_WAY_LEFT_TURN_ARROW
)

const (
	faceNB = "openits-types:direction-northbound"
	faceSB = "openits-types:direction-southbound"
)

func lcsChangeOn(face string, seq uint64, prev *reversiblelanev1.LcsIndication, cur reversiblelanev1.LcsIndication) EventEnvelope {
	return EventEnvelope{
		Subject: "openits.us-va.vdot.nova.reversible-lane.rl-1.lcs-indication-changed",
		Data: &reversiblelanev1.LcsIndicationChanged{
			SegmentId: "seg-1", LaneId: "lane-1", Face: face, Sequence: seq,
			PreviousIndication: prev, Indication: cur,
		},
	}
}

func lcsChange(prev *reversiblelanev1.LcsIndication, cur reversiblelanev1.LcsIndication) EventEnvelope {
	return lcsChangeOn(faceNB, 0, prev, cur)
}

func TestReversibleLane_IndicationSequence_Table(t *testing.T) {
	cases := []struct {
		name      string
		prev, cur reversiblelanev1.LcsIndication
		pass      bool
	}{
		{"green to yellow", pGreen, pYel, true},
		{"green to red", pGreen, pRed, true},
		{"yellow to red", pYel, pRed, true},
		{"green to dark", pGreen, pDark, false},
		{"yellow to green", pYel, pGreen, false},
		{"yellow to dark", pYel, pDark, false},
		{"red to green", pRed, pGreen, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prev := c.prev
			r := Run("seq", TestReversibleLane_IndicationSequence, &Observation{Events: []EventEnvelope{lcsChange(&prev, c.cur)}})
			if r.Pass != c.pass {
				t.Fatalf("pass=%v want %v: %s", r.Pass, c.pass, r.Message)
			}
		})
	}
	t.Run("absent previous with no history passes", func(t *testing.T) {
		r := Run("seq", TestReversibleLane_IndicationSequence, &Observation{Events: []EventEnvelope{lcsChange(nil, pDark)}})
		if !r.Pass {
			t.Fatalf("want pass: %s", r.Message)
		}
	})
	t.Run("unknown to X passes", func(t *testing.T) {
		u := reversiblelanev1.LcsIndication_LCS_INDICATION_UNKNOWN
		r := Run("seq", TestReversibleLane_IndicationSequence, &Observation{Events: []EventEnvelope{lcsChange(&u, pRed)}})
		if !r.Pass {
			t.Fatalf("want pass: %s", r.Message)
		}
	})
	t.Run("green to white two-way with opposite red passes", func(t *testing.T) {
		r := Run("seq", TestReversibleLane_IndicationSequence, &Observation{Events: []EventEnvelope{
			lcsChangeOn(faceNB, 1, nil, pGreen), lcsChangeOn(faceSB, 2, nil, pRed), lcsChangeOn(faceNB, 3, nil, pTwo),
		}})
		if !r.Pass {
			t.Fatalf("want pass (MUTCD 4T.04 para 05): %s", r.Message)
		}
	})
	t.Run("green to white two-way with opposite green fails", func(t *testing.T) {
		r := Run("seq", TestReversibleLane_IndicationSequence, &Observation{Events: []EventEnvelope{
			lcsChangeOn(faceNB, 1, nil, pGreen), lcsChangeOn(faceSB, 2, nil, pGreen), lcsChangeOn(faceNB, 3, nil, pTwo),
		}})
		if r.Pass {
			t.Fatal("want fail: opposite face is not red X")
		}
	})
	t.Run("green to white two-way with opposite unseen fails", func(t *testing.T) {
		r := Run("seq", TestReversibleLane_IndicationSequence, &Observation{Events: []EventEnvelope{
			lcsChangeOn(faceNB, 1, nil, pGreen), lcsChangeOn(faceNB, 2, nil, pTwo),
		}})
		if r.Pass {
			t.Fatal("want fail: opposite face unknown is treated as not red")
		}
	})
	t.Run("green to white two-way with opposite unknown fails", func(t *testing.T) {
		u, red := reversiblelanev1.LcsIndication_LCS_INDICATION_UNKNOWN, pRed
		r := Run("seq", TestReversibleLane_IndicationSequence, &Observation{Events: []EventEnvelope{
			lcsChangeOn(faceNB, 1, nil, pGreen), lcsChangeOn(faceSB, 2, nil, pRed), lcsChangeOn(faceSB, 3, &red, u), lcsChangeOn(faceNB, 4, nil, pTwo),
		}})
		if r.Pass {
			t.Fatal("want fail: opposite readback dropped out, so it is not known to be red")
		}
	})
	t.Run("events are ordered by sequence", func(t *testing.T) {
		r := Run("seq", TestReversibleLane_IndicationSequence, &Observation{Events: []EventEnvelope{
			lcsChangeOn(faceNB, 3, nil, pTwo), lcsChangeOn(faceSB, 2, nil, pRed), lcsChangeOn(faceNB, 1, nil, pGreen),
		}})
		if !r.Pass {
			t.Fatalf("want pass once sorted by sequence: %s", r.Message)
		}
	})
	t.Run("green to unknown passes and clears memory", func(t *testing.T) {
		u, green := reversiblelanev1.LcsIndication_LCS_INDICATION_UNKNOWN, pGreen
		r := Run("seq", TestReversibleLane_IndicationSequence, &Observation{Events: []EventEnvelope{
			lcsChange(nil, pGreen), lcsChange(&green, u), lcsChange(nil, pDark),
		}})
		if !r.Pass {
			t.Fatalf("want pass: unknown is a dropout, and dark after it has no remembered green: %s", r.Message)
		}
	})
	t.Run("chain break fails", func(t *testing.T) {
		y := pYel
		r := Run("seq", TestReversibleLane_IndicationSequence, &Observation{Events: []EventEnvelope{
			lcsChange(nil, pGreen), lcsChange(&y, pRed),
		}})
		if r.Pass {
			t.Fatal("want fail: second event's previous (yellow) differs from remembered green")
		}
	})
	t.Run("absent previous uses remembered value", func(t *testing.T) {
		r := Run("seq", TestReversibleLane_IndicationSequence, &Observation{Events: []EventEnvelope{
			lcsChange(nil, pGreen), lcsChange(nil, pDark),
		}})
		if r.Pass {
			t.Fatal("want fail: green then dark with absent previous")
		}
		r = Run("seq", TestReversibleLane_IndicationSequence, &Observation{Events: []EventEnvelope{
			lcsChange(nil, pGreen), lcsChange(nil, pYel), lcsChange(nil, pRed),
		}})
		if !r.Pass {
			t.Fatalf("want pass for green, yellow, red: %s", r.Message)
		}
	})
	t.Run("same-value repeat is no change", func(t *testing.T) {
		g := pGreen
		r := Run("seq", TestReversibleLane_IndicationSequence, &Observation{Events: []EventEnvelope{
			lcsChange(nil, pGreen), lcsChange(&g, pGreen),
		}})
		if !r.Pass {
			t.Fatalf("want pass: %s", r.Message)
		}
	})
	t.Run("no events passes", func(t *testing.T) {
		if !Run("seq", TestReversibleLane_IndicationSequence, &Observation{}).Pass {
			t.Fatal("want pass: a quiet window has no changes")
		}
	})
}

func laneDev(a, b yangpkg.E_OpenitsReversibleLaneTypes_LcsIndication) *yangpkg.Device {
	dev := &yangpkg.Device{}
	seg, _ := dev.GetOrCreateReversibleLane().GetOrCreateSegments().NewSegment("seg-1")
	lane, _ := seg.NewLane("lane-1")
	cfg := lane.GetOrCreateConfig()
	cfg.LcsDirectionA, cfg.LcsDirectionB = a, b
	return dev
}

func TestReversibleLane_NoProhibitedCommand_Table(t *testing.T) {
	const (
		red   = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_red_x
		yel   = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_yellow_x
		fl    = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_flashing_yellow_x
		green = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_green_arrow
		two   = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_white_two_way_left_turn_arrow
		one   = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_white_one_way_left_turn_arrow
	)
	cases := []struct {
		name string
		a, b yangpkg.E_OpenitsReversibleLaneTypes_LcsIndication
		pass bool
	}{
		{"legal green over red", green, red, true},
		{"legal two-way both", two, two, true},
		{"legal yellow over red", yel, red, true},
		{"green without opposing red", green, yel, false},
		{"green both", green, green, false},
		{"yellow both (B)", yel, yel, false},
		{"one-way both (C)", one, one, false},
		{"two-way with one-way (F)", two, one, false},
		{"one-way with two-way (F)", one, two, false},
		{"one-way with yellow (G)", one, yel, false},
		{"yellow with one-way (G)", yel, one, false},
		{"legacy flashing yellow", fl, red, false},
		{"unknown commanded", yangpkg.OpenitsReversibleLaneTypes_LcsIndication_unknown, red, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Run("cmd", TestReversibleLane_NoProhibitedCommand, &Observation{Device: laneDev(c.a, c.b)})
			if r.Pass != c.pass {
				t.Fatalf("pass=%v want %v: %s", r.Pass, c.pass, r.Message)
			}
		})
	}
}

func TestRL_DirectionOpenGateBlocksReverse_Table(t *testing.T) {
	const (
		red = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_red_x
		two = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_white_two_way_left_turn_arrow
		one = yangpkg.OpenitsReversibleLaneTypes_LcsIndication_white_one_way_left_turn_arrow
	)
	cases := []struct {
		name string
		a    yangpkg.E_OpenitsReversibleLaneTypes_LcsIndication
		pass bool
	}{
		{"red toward the new direction passes", red, true},
		{"white two-way toward the new direction fails", two, false},
		{"white one-way toward the new direction fails", one, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dev := &yangpkg.Device{}
			rl := dev.GetOrCreateReversibleLane()
			rl.GetOrCreateConfig().DirectionA = yangpkg.OpenitsTypes_CarriagewayDirection_direction_northbound
			rl.GetOrCreateConfig().DirectionB = yangpkg.OpenitsTypes_CarriagewayDirection_direction_southbound
			cs := rl.GetOrCreateControl().GetOrCreateState()
			permitted := false
			cs.DirectionOpenPermitted = &permitted
			cs.CurrentState = yangpkg.OpenitsReversibleLaneTypes_LaneFlowState_in_transition
			cs.GetOrCreateTransition().ToDirection = yangpkg.OpenitsTypes_CarriagewayDirection_direction_northbound
			seg, _ := rl.GetOrCreateSegments().NewSegment("seg-1")
			lane, _ := seg.NewLane("lane-1")
			lane.GetOrCreateState().LcsDirectionA = c.a
			r := Run("gate", TestRL_DirectionOpenGateBlocksReverse, &Observation{Device: dev})
			if r.Pass != c.pass {
				t.Fatalf("pass=%v want %v: %s", r.Pass, c.pass, r.Message)
			}
		})
	}
}
