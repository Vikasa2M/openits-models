package tests

import (
	"strings"

	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
)

// ----- identity -----
//
// Meter identity is split into intended config (operator-provisioned) and
// state (applied mirror + device-reported hardware inventory) — the same
// idiom as ESS's station identity. Conformance checks inspect observed
// telemetry, so they read from state.

func TestRMIdentity_MeterID(t *T, obs *Observation) {
	st := obs.Device.GetRampMeter().GetState()
	if st == nil || st.GetId() == "" {
		t.Errorf("state/id is unset")
	}
}

func TestRMIdentity_Firmware(t *T, obs *Observation) {
	st := obs.Device.GetRampMeter().GetState()
	if st == nil || st.GetFirmware() == "" {
		t.Errorf("state/firmware is unset; required for field-service diagnostics")
	}
}

func TestRMIdentity_MakeModel(t *T, obs *Observation) {
	st := obs.Device.GetRampMeter().GetState()
	if st == nil || st.GetMake() == "" || st.GetModel() == "" {
		t.Errorf("state/make and state/model must both be populated")
	}
}

// ----- operational status -----
//
// Commanded metering control also splits into control/config (intended) and
// control/state (applied mirror + live operational rollup: current release
// rate, queue state). Conformance reads control/state.

func TestRMOperational_ModePresent(t *T, obs *Observation) {
	st := obs.Device.GetRampMeter().GetControl().GetState()
	if st == nil {
		t.Fatalf("control/state missing")
	}
	if st.Mode == yangpkg.OpenitsRampMeteringTypes_MeterMode_UNSET ||
		st.Mode == yangpkg.OpenitsRampMeteringTypes_MeterMode_mode_unknown {
		t.Errorf("control/state/mode is unset or unknown")
	}
}

func TestRMOperational_ReleaseRatePositiveWhenActive(t *T, obs *Observation) {
	st := obs.Device.GetRampMeter().GetControl().GetState()
	if st == nil {
		return
	}
	if st.Mode != yangpkg.OpenitsRampMeteringTypes_MeterMode_mode_active {
		return
	}
	if st.CurrentReleaseRateVph == nil || *st.CurrentReleaseRateVph == 0 {
		t.Errorf("meter in active mode but control/state/current-release-rate-vph is zero/unset")
	}
}

func TestRMOperational_ActivePlanExists(t *T, obs *Observation) {
	rm := obs.Device.GetRampMeter()
	st := rm.GetControl().GetState()
	plans := rm.GetPlans()
	if st == nil || plans == nil || st.ActivePlanId == nil {
		return
	}
	if _, ok := plans.Plan[*st.ActivePlanId]; !ok {
		t.Errorf("control/state/active-plan-id %d has no matching plan in /plans/plan",
			*st.ActivePlanId)
	}
}

// ----- plans -----

func TestRMPlans_AtLeastOne(t *T, obs *Observation) {
	p := obs.Device.GetRampMeter().GetPlans()
	if p == nil || len(p.Plan) == 0 {
		t.Errorf("no metering plans configured")
	}
}

// A ramp meter is NOT an intersection through-movement: the intersection
// MUTCD phase minimums (min-green >= 4s, yellow >= 3s, red-clear >= 1s) sum to
// >= 8s, which is physically incompatible with ramp metering — real meters run
// 4-6s headways (600-900+ vph), and those floors alone would cap throughput
// below ~450 vph. What a ramp plan must actually satisfy is that its release
// cycle FITS the headway (TestRMPlans_HeadwayFitsTiming) and that any yellow it
// uses is perceptible — not intersection floors.
func TestRMPlans_ActivePlanTimingSane(t *T, obs *Observation) {
	rm := obs.Device.GetRampMeter()
	st := rm.GetControl().GetState()
	plans := rm.GetPlans()
	if st == nil || plans == nil || st.ActivePlanId == nil {
		return
	}
	plan, ok := plans.Plan[*st.ActivePlanId]
	if !ok {
		return
	}
	pt := plan.GetPhaseTiming()
	if pt == nil {
		return
	}
	if pt.YellowChange != nil && *pt.YellowChange < 0.5 {
		t.Errorf("active plan yellow-change %.1fs is too short to be perceptible", *pt.YellowChange)
	}
	if pt.RedClear != nil && *pt.RedClear < 0.0 {
		t.Errorf("active plan red-clear %.1fs is negative", *pt.RedClear)
	}
}

// rmServedLaneCount counts non-bypass (metered) lanes — the lanes over which a
// release cycle is shared. Matches the schema must's
// count(lanes/lane[not(config/bypass='true')]); a lane marked bypass in either
// config or state is excluded.
func rmServedLaneCount(rm *yangpkg.OpenitsRampMetering_RampMeter) int {
	l := rm.GetLanes()
	if l == nil {
		return 0
	}
	n := 0
	for _, lane := range l.Lane {
		if lane.GetConfig().GetBypass() || lane.GetState().GetBypass() {
			continue
		}
		n++
	}
	return n
}

// rmAlternateRelease reports whether the meter coordinates its release lanes in
// an alternating sequence (each lane's cycle spans all served lanes).
func rmAlternateRelease(rm *yangpkg.OpenitsRampMetering_RampMeter) bool {
	return rm.GetLanes().GetConfig().GetReleaseCoordination() ==
		yangpkg.OpenitsRampMetering_RampMeter_Lanes_Config_ReleaseCoordination_alternate
}

func TestRMPlans_HeadwayFitsTiming(t *T, obs *Observation) {
	rm := obs.Device.GetRampMeter()
	plans := rm.GetPlans()
	if plans == nil {
		return
	}
	served := rmServedLaneCount(rm)
	alt := rmAlternateRelease(rm)
	for id, plan := range plans.Plan {
		pt := plan.GetPhaseTiming()
		if pt == nil || plan.HeadwayS == nil || pt.MinGreen == nil || pt.YellowChange == nil || pt.RedClear == nil {
			continue
		}
		// Minimum feasible cycle includes min-green, not just clearance:
		// a headway shorter than min-green+yellow+red-clear certifies a
		// cycle that cannot physically complete. For alternate release each
		// lane's cycle spans all served lanes, so the feasible cycle is
		// headway * served-lanes; for simultaneous release it is the headway.
		cycle := float64(*pt.MinGreen) + *pt.YellowChange + *pt.RedClear
		feasible := *plan.HeadwayS
		if alt && served > 0 {
			feasible = *plan.HeadwayS * float64(served)
		}
		if cycle > feasible {
			t.Errorf("plan %d min feasible cycle %.1fs (min-green+yellow+red-clear) > feasible cycle %.1fs; cannot complete one release cycle",
				id, cycle, feasible)
		}
	}
}

// The commanding authority (central / TOD / local) must be reported so a
// consumer knows who is driving the meter — the NTCIP 1207 control dimension.
func TestRMControl_CommandSourcePresent(t *T, obs *Observation) {
	st := obs.Device.GetRampMeter().GetControl().GetState()
	if st == nil {
		return
	}
	if st.CommandSource == yangpkg.OpenitsTypes_ControlSource_UNSET {
		t.Errorf("control/state/command-source is unset; the commanding authority (central/TOD/local) is unknown")
	}
}

// Queue-override hysteresis: when a plan configures a clear (deactivation)
// threshold, it must be below the activation threshold — an equal/higher clear
// threshold defeats the anti-flap band and the override chatters.
func TestRMPlans_QueueOverrideHysteresis(t *T, obs *Observation) {
	plans := obs.Device.GetRampMeter().GetPlans()
	if plans == nil {
		return
	}
	for id, plan := range plans.Plan {
		if plan.QueueOverrideClearThresholdVehicles == nil || plan.QueueOverrideThresholdVehicles == nil {
			continue
		}
		if *plan.QueueOverrideClearThresholdVehicles >= *plan.QueueOverrideThresholdVehicles {
			t.Errorf("plan %d queue-override clear threshold %d >= activation %d; hysteresis band is defeated (override will flap)",
				id, *plan.QueueOverrideClearThresholdVehicles, *plan.QueueOverrideThresholdVehicles)
		}
	}
}

// Headway must be consistent with the release rate: headway ~= 3600 *
// vehicles-per-green / release-rate-vph. A rate and headway that disagree
// mean the meter's advertised throughput and its actual cycle contradict.
func TestRMPlans_HeadwayConsistentWithRate(t *T, obs *Observation) {
	rm := obs.Device.GetRampMeter()
	plans := rm.GetPlans()
	if plans == nil {
		return
	}
	served := rmServedLaneCount(rm)
	if served == 0 {
		// No served lanes declared: the band would collapse; the schema must
		// short-circuits this case too.
		return
	}
	alt := rmAlternateRelease(rm)
	for id, plan := range plans.Plan {
		if plan.ReleaseRateVph == nil || plan.HeadwayS == nil {
			continue
		}
		vpg := 1.0
		if plan.VehiclesPerGreen != nil {
			vpg = float64(*plan.VehiclesPerGreen)
		}
		product := float64(*plan.ReleaseRateVph) * *plan.HeadwayS
		// Simultaneous release: aggregate rate serves all lanes at once, so the
		// band scales by served-lanes; alternate release: the aggregate is
		// 3600*vpg/headway (per-release interval), band is the single-lane band.
		lo, hi := 3060*vpg, 4140*vpg
		if !alt {
			lo *= float64(served)
			hi *= float64(served)
		}
		if product < lo || product > hi {
			t.Errorf("plan %d: release-rate %d * headway %.1f = %.0f outside [%.0f,%.0f] for %.0f veh/green over %d served lane(s)",
				id, *plan.ReleaseRateVph, *plan.HeadwayS, product, lo, hi, vpg, served)
		}
	}
}

// ----- lanes / detectors -----
//
// Lane and detector config both split into config (intended) and state
// (applied mirror + live readings), the same idiom as everywhere else;
// conformance reads the observed value from state.

func TestRMLanes_AtLeastOneMeteredLane(t *T, obs *Observation) {
	l := obs.Device.GetRampMeter().GetLanes()
	if l == nil {
		t.Fatalf("lanes container missing")
	}
	for _, lane := range l.Lane {
		if !lane.GetState().GetBypass() {
			return
		}
	}
	t.Errorf("no metered lane (bypass=false) configured")
}

func TestRMLanes_DemandAndPassageDetectors(t *T, obs *Observation) {
	l := obs.Device.GetRampMeter().GetLanes()
	if l == nil {
		return
	}
	var demand, passage bool
	for _, lane := range l.Lane {
		if lane.GetState().GetBypass() {
			continue
		}
		for _, det := range lane.Detector {
			switch det.GetState().Role {
			case yangpkg.OpenitsRampMetering_DetectorRole_demand:
				demand = true
			case yangpkg.OpenitsRampMetering_DetectorRole_passage:
				passage = true
			}
		}
	}
	if !demand {
		t.Errorf("no demand-role detector on any metered lane")
	}
	if !passage {
		t.Errorf("no passage-role detector on any metered lane")
	}
}

// ----- event shapes -----

func TestRMEvent_ModeChangedShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".mode-changed") {
			continue
		}
		want := "openits.ramp-metering.mode-changed.v1"
		if e.CEType != want {
			t.Errorf("mode-changed ce-type %q, want %q", e.CEType, want)
		}
		return
	}
}

func TestRMEvent_ReleaseRateChangedShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".release-rate-changed") {
			continue
		}
		want := "openits.ramp-metering.release-rate-changed.v1"
		if e.CEType != want {
			t.Errorf("release-rate-changed ce-type %q, want %q", e.CEType, want)
		}
		return
	}
}

func TestRMEvent_QueueOverrideActivatedShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".queue-override-activated") {
			continue
		}
		want := "openits.ramp-metering.queue-override-activated.v1"
		if e.CEType != want {
			t.Errorf("queue-override-activated ce-type %q, want %q", e.CEType, want)
		}
		return
	}
}
