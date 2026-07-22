package main

import (
	"context"
	"time"

	commonv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/common/v1"
	dmsv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/dms/v1"
	essv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/ess/v1"
	rampmeteringv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/ramp_metering/v1"
	rsuv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/rsu/v1"
	signalcontrolv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/signal_control/v1"
	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
	"github.com/Vikasa2M/openits-models/tools/conformance/tests"
)

// mockDriver produces a fully-populated, spec-compliant observation so
// the harness can be exercised end-to-end without a real device.  Real
// vendors will fail one or more of the checks; that is the point.
type mockDriver struct {
	kind   string
	window time.Duration
}

func newMockDriver(kind string, window time.Duration) *mockDriver {
	return &mockDriver{kind: kind, window: window}
}

func (m *mockDriver) Collect(_ context.Context) (*yangpkg.Device, error) {
	switch m.kind {
	case "ramp-metering":
		return collectRM()
	case "rsu":
		return collectRSU()
	case "ess":
		return collectESS()
	case "dms":
		return collectDMS()
	case "traffic-sensor":
		return collectTrafficSensor()
	case "reversible-lane":
		return collectReversibleLane()
	case "perception":
		return collectPerception()
	case "cctv":
		return collectCctv()
	default:
		return collectASC()
	}
}

func collectASC() (*yangpkg.Device, error) {
	dev := &yangpkg.Device{}
	sc := dev.GetOrCreateSignalController()

	// Identity: operator-provisioned intent in config, device-reported
	// mirror + hardware inventory in state — same config/state idiom as
	// ESS/DMS/ramp-metering identity.
	cfg := sc.GetOrCreateConfig()
	cfg.Id = strPtr("i35-exit-214")
	cfg.Name = strPtr("I-35 @ Exit 214")
	cfg.Latitude = f64Ptr(30.2672)
	cfg.Longitude = f64Ptr(-97.7431)

	st := sc.GetOrCreateState()
	st.Id = strPtr("i35-exit-214")
	st.Name = strPtr("I-35 @ Exit 214")
	st.Latitude = f64Ptr(30.2672)
	st.Longitude = f64Ptr(-97.7431)
	st.Make = strPtr("Econolite")
	st.Model = strPtr("Cobalt")
	st.Firmware = strPtr("Econolite Cobalt 9.2.3")

	// Operational rollup (state-only): mode/flash-active/flash-cause/mmu/
	// last-mode-change moved off the identity tree into their own
	// container, distinct from the applied-identity "state" above.
	op := sc.GetOrCreateOperation()
	op.FlashActive = boolPtr(false)
	op.LastModeChange = strPtr("2026-04-18T14:00:00Z")

	// Coordination: config-only timing-plan library + state readback
	// naming the active plan. Plan 3 carries the cut-2c coordination-
	// semantics leaves and a per-ring split table (NEMA dual-ring:
	// phases 1-4 = ring 1, phases 5-8 = ring 2, per ringFor below)
	// sized to satisfy both new config-true musts: each ring's splits
	// sum to exactly the cycle length, and every split clears its
	// phase's minimum service time (min-green 5s + yellow 3.5s +
	// red-clear 1.5s = 10s; none of these phases carry ped-recall/
	// rest-in-walk, so the walk+ped-clear term of the must is zero).
	coord := sc.GetOrCreateCoordination()
	tp, err := coord.NewTimingPlan(3)
	if err != nil {
		return nil, err
	}
	tp.CycleLength = u16Ptr(120)
	tp.CoordinatedPhases = []uint8{2, 6}
	tp.OffsetReference = yangpkg.OpenitsSignalControl_SignalController_Coordination_TimingPlan_OffsetReference_begin_of_green
	tp.TransitionMode = yangpkg.OpenitsSignalControl_SignalController_Coordination_TimingPlan_TransitionMode_shortway
	tp.ForceOffMode = yangpkg.OpenitsSignalControl_SignalController_Coordination_TimingPlan_ForceOffMode_fixed
	for _, n := range []uint8{1, 2, 3, 4, 5, 6, 7, 8} {
		s, err := tp.NewSplit(n)
		if err != nil {
			return nil, err
		}
		s.SplitSeconds = u16Ptr(30) // 4 phases/ring * 30s = 120s = cycle-length
		if n == 2 || n == 6 {
			s.SplitMode = yangpkg.OpenitsSignalControl_SignalController_Coordination_TimingPlan_Split_SplitMode_coordinated_fixed
		} else {
			s.SplitMode = yangpkg.OpenitsSignalControl_SignalController_Coordination_TimingPlan_Split_SplitMode_minimum_recall
		}
	}
	coordSt := coord.GetOrCreateState()
	coordSt.ActivePlan = u8Ptr(3)
	coordSt.CycleState = yangpkg.OpenitsSignalControl_SignalController_Coordination_State_CycleState_in_step

	// Timebase (NTCIP 1201): the "weekday" day-plan activates coordination
	// plan 3 at 06:00 and drops to flash at 23:00 (exercising both arms of
	// the action/activate choice: a plan leafref and a special-operation
	// enum). A schedule-entry maps Mon-Fri onto that day-plan, and clock
	// state reports a GNSS-synced controller with a small positive drift —
	// exercising the config-false clock readback alongside the config-true
	// day-plan/schedule-entry lists.
	tb := sc.GetOrCreateTimebase()
	dp, err := tb.NewDayPlan(1)
	if err != nil {
		return nil, err
	}
	dp.Name = strPtr("weekday")
	a1, err := dp.NewAction("06:00:00")
	if err != nil {
		return nil, err
	}
	a1.TimingPlan = u8Ptr(3) // references the coordination plan created above
	a2, err := dp.NewAction("23:00:00")
	if err != nil {
		return nil, err
	}
	a2.SpecialOperation = yangpkg.OpenitsSignalControl_SignalController_Timebase_DayPlan_Action_SpecialOperation_flash

	se, err := tb.NewScheduleEntry(1)
	if err != nil {
		return nil, err
	}
	se.DaysOfWeek = []yangpkg.E_OpenitsSignalControl_SignalController_Timebase_ScheduleEntry_DaysOfWeek{
		yangpkg.OpenitsSignalControl_SignalController_Timebase_ScheduleEntry_DaysOfWeek_monday,
		yangpkg.OpenitsSignalControl_SignalController_Timebase_ScheduleEntry_DaysOfWeek_tuesday,
		yangpkg.OpenitsSignalControl_SignalController_Timebase_ScheduleEntry_DaysOfWeek_wednesday,
		yangpkg.OpenitsSignalControl_SignalController_Timebase_ScheduleEntry_DaysOfWeek_thursday,
		yangpkg.OpenitsSignalControl_SignalController_Timebase_ScheduleEntry_DaysOfWeek_friday,
	}
	se.DayPlan = u8Ptr(1)

	clk := tb.GetOrCreateClock()
	clk.CurrentTime = strPtr("2026-07-14T12:00:00Z")
	clk.TimeSource = yangpkg.OpenitsSignalControlTypes_TimeSource_time_source_gnss
	clk.SyncStatus = yangpkg.OpenitsSignalControl_SignalController_Timebase_Clock_SyncStatus_synced
	clk.OffsetMs = i32Ptr(12)

	ph := sc.GetOrCreatePhases()
	for _, n := range []uint8{1, 2, 3, 4, 5, 6, 7, 8} {
		p, err := ph.NewPhase(n)
		if err != nil {
			return nil, err
		}
		pc := p.GetOrCreateConfig()
		pc.PhaseNumber = &n
		pc.Ring = u8Ptr(ringFor(n))
		pc.Barrier = u8Ptr(barrierFor(n))
		pc.Enabled = boolPtr(true)
		t := pc.GetOrCreateTiming()
		t.MinGreen = f64Ptr(5)
		t.MaxGreen = f64Ptr(45)
		t.YellowChange = f64Ptr(3.5)
		t.RedClear = f64Ptr(1.5)
		t.Passage = f64Ptr(2.0)
		t.Walk = u16Ptr(7)
		t.PedClear = u16Ptr(20)
	}

	// Detectors: config/state split, same idiom as everywhere else.
	// Populates the NTCIP 1202 vehicleDetectorTable completeness added in
	// cut 2a (delay/extend/mode/fail-action) plus the per-detector V/O/S
	// measurement container, so that surface is actually exercised by the
	// harness instead of sitting untouched since the detector-type
	// identity landed in cut 1.
	dets := sc.GetOrCreateDetectors()
	det, err := dets.NewDetector(1)
	if err != nil {
		return nil, err
	}
	detCfg := det.GetOrCreateConfig()
	detCfg.DetectorId = u16Ptr(1)
	detCfg.Type = yangpkg.OpenitsSignalControlTypes_DetectorType_detector_inductive_loop
	detCfg.AssignedPhases = []uint8{2}
	detCfg.Enabled = boolPtr(true)
	detCfg.Delay = u16Ptr(3)
	detCfg.Extend = f64Ptr(1.5)
	detCfg.Mode = yangpkg.OpenitsSignalControl_DetectorMode_presence
	detCfg.FailAction = yangpkg.OpenitsSignalControl_DetectorFailAction_max_recall
	detSt := det.GetOrCreateState()
	detSt.Active = boolPtr(true)
	detSt.LastActivation = strPtr("2026-04-18T14:05:32Z")
	detSt.ActuationCount = u64Ptr(1421)
	detSt.Fault = boolPtr(false)
	meas := detSt.GetOrCreateMeasurement()
	meas.Volume = u64Ptr(842)
	meas.Occupancy = f64Ptr(12.5)
	meas.SpeedKmh = f64Ptr(61)

	// Overlaps: config/state split (NTCIP overlapTable). One FYA
	// (flashing-yellow-arrow) left-turn overlap: included-phases names the
	// protected left phase, and the fya presence container adds the
	// opposing through phase that drives the permissive indication —
	// exercising the paired config/state and the fya sub-container
	// together.
	overlaps := sc.GetOrCreateOverlaps()
	ol, err := overlaps.NewOverlap(1)
	if err != nil {
		return nil, err
	}
	olCfg := ol.GetOrCreateConfig()
	olCfg.OverlapNumber = u8Ptr(1)
	olCfg.Name = strPtr("NB left FYA")
	olCfg.Type = yangpkg.OpenitsSignalControlTypes_OverlapType_overlap_fya
	olCfg.IncludedPhases = []uint8{5}
	olCfg.TrailingGreen = f64Ptr(0.0)
	olCfg.TrailingYellow = f64Ptr(3.0)
	olCfg.TrailingRedClear = f64Ptr(1.0)
	fya := olCfg.GetOrCreateFya()
	fya.ProtectedLeftPhase = u8Ptr(5)
	fya.OpposingThroughPhase = u8Ptr(2)
	olSt := ol.GetOrCreateState()
	olSt.CurrentInterval = yangpkg.OpenitsSignalControl_OverlapIntervalType_flashing_yellow_arrow
	olSt.Active = boolPtr(true)

	// Channels: the load-switch mapping (NTCIP channelTable). Channel 1 is
	// phase-sourced, channel 2 is overlap-sourced (the FYA overlap above)
	// — exercising both arms of the `choice source`.
	channels := sc.GetOrCreateChannels()
	ch1, err := channels.NewChannel(1)
	if err != nil {
		return nil, err
	}
	ch1.Phase = u8Ptr(2)
	ch1.Movement = yangpkg.OpenitsSignalControlTypes_ChannelMovement_channel_vehicle
	ch1.FlashState = yangpkg.OpenitsSignalControl_ChannelFlashState_yellow

	ch2, err := channels.NewChannel(2)
	if err != nil {
		return nil, err
	}
	ch2.Overlap = u8Ptr(1)
	ch2.Movement = yangpkg.OpenitsSignalControlTypes_ChannelMovement_channel_vehicle
	ch2.FlashState = yangpkg.OpenitsSignalControl_ChannelFlashState_red

	// Conflict monitor: the MMU permissive matrix, one pair naming
	// channels 1 and 2 compatible, stored canonically (channel-a <
	// channel-b) as the YANG `must` on this list requires.
	cm := sc.GetOrCreateConflictMonitor()
	if _, err := cm.NewPermissive(1, 2); err != nil {
		return nil, err
	}

	// Preemption: commanded preempt table (cut 2c restructure from a
	// state-only readback into a paired config/state preemptor list).
	// One railroad preemptor with a track-clearance interval, exercising
	// the config-true must that every railroad preemptor define
	// track-clearance (MUTCD Ch. 8C). Not currently active, so state
	// carries no active-since/source-id.
	pre := sc.GetOrCreatePreemption()
	pr, err := pre.NewPreemptor(1)
	if err != nil {
		return nil, err
	}
	prc := pr.GetOrCreateConfig()
	prc.PreemptorId = u8Ptr(1)
	prc.Type = yangpkg.OpenitsSignalControlTypes_PreemptionType_preempt_railroad
	prc.PriorityOrder = u8Ptr(1)
	prc.DelaySeconds = u16Ptr(0)
	prc.MinGreenBeforeEntry = u16Ptr(5)
	tc := prc.GetOrCreateTrackClearance()
	tc.Phases = []uint8{2, 6}
	tc.GreenSeconds = u16Ptr(8)
	prc.DwellPhases = []uint8{2, 6}
	prc.MinDwellSeconds = u16Ptr(10)
	prc.ExitPhases = []uint8{2, 6}
	prc.MaxPresenceSeconds = u16Ptr(180)
	prSt := pr.GetOrCreateState()
	prSt.Active = boolPtr(false)
	prSt.CurrentStage = yangpkg.OpenitsSignalControl_PreemptStage_none

	// Cabinet power / UPS (platform grouping): a cabinet on line power with a
	// healthy battery, so the on-battery leading indicator and the
	// runtime-remaining dispatch discriminator are exercised.
	cp := sc.GetOrCreateCabinetPower()
	cp.PowerSource = yangpkg.OpenitsSignalControl_SignalController_CabinetPower_PowerSource_on_line
	cp.TransferCount = u32Ptr(4)
	cp.LineVoltageV = f64Ptr(121.4)
	cp.LineFrequencyHz = f64Ptr(60.0)
	bat := cp.GetOrCreateBattery()
	bat.StateOfChargePct = u8Ptr(97)
	bat.RuntimeRemainingMinutes = u16Ptr(180)
	bat.VoltageV = f64Ptr(27.3)
	bat.TemperatureC = f64Ptr(24.5)
	bat.ChargerFault = boolPtr(false)
	bat.TestState = yangpkg.OpenitsSignalControl_SignalController_CabinetPower_Battery_TestState_passed
	bat.LastTest = strPtr("2026-07-15T02:00:00Z")
	cp.DoorOpen = boolPtr(false)
	cp.PolicePanelOpen = boolPtr(false)

	return dev, nil
}

func (m *mockDriver) Subscribe(ctx context.Context, out chan<- tests.EventEnvelope) error {
	switch m.kind {
	case "ramp-metering":
		return subscribeRM(ctx, out, m.window)
	case "rsu":
		return subscribeRSU(ctx, out, m.window)
	case "ess":
		return subscribeESS(ctx, out, m.window)
	case "dms":
		return subscribeDMS(ctx, out, m.window)
	case "traffic-sensor":
		return subscribeTrafficSensor(ctx, out, m.window)
	case "reversible-lane":
		return subscribeReversibleLane(ctx, out, m.window)
	case "perception":
		return subscribePerception(ctx, out, m.window)
	case "cctv":
		return subscribeCctv(ctx, out, m.window)
	default:
		return subscribeASC(ctx, out, m.window)
	}
}

func subscribeASC(ctx context.Context, out chan<- tests.EventEnvelope, window time.Duration) error {
	// Emit one of each event type on the expected subject/ce-type.
	base := "openits.us-tx.txdot.d07.signal-control.i35-exit-214"
	events := []tests.EventEnvelope{
		{
			Subject:  base + ".fault-raised",
			CEType:   "openits.signal-control.fault-raised.v1",
			CESource: "urn:openits:controller:us-tx:txdot:d07:i35-exit-214",
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6V8W",
			CETime:   time.Now().UTC(),
			Data: &commonv1.FaultRaised{
				FaultId:  "f-001",
				Severity: commonv1.FaultSeverity_FAULT_SEVERITY_WARNING,
			},
		},
		{
			Subject:  base + ".operational-status",
			CEType:   "openits.signal-control.operational-status.v1",
			CESource: "urn:openits:controller:us-tx:txdot:d07:i35-exit-214",
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6V8X",
			CETime:   time.Now().UTC(),
			// No generated payload message backs this heartbeat: unlike
			// every other event here, operational-status was never a YANG
			// `notification` in the old hand-authored proto either, so
			// tools/yang-proto-gen (which only emits from notifications)
			// has nothing to generate for it. The envelope alone (subject
			// + ce-type) is what TestHealth_OperationalStatus checks.
			Data: nil,
		},
		{
			Subject:  base + ".preemption-activated",
			CEType:   "openits.signal-control.preemption-activated.v1",
			CESource: "urn:openits:controller:us-tx:txdot:d07:i35-exit-214",
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6V8Y",
			CETime:   time.Now().UTC(),
			Data: &signalcontrolv1.PreemptionActivated{
				Type: "openits-signal-control-types:preempt-emergency-vehicle",
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
	// Hold until window expires so the harness records a realistic
	// observation period.
	select {
	case <-ctx.Done():
	case <-time.After(window):
	}
	return nil
}

func collectDMS() (*yangpkg.Device, error) {
	dev := &yangpkg.Device{}
	sign := dev.GetOrCreateSign()

	// Identity: operator-provisioned intent in sign/config, device-reported
	// mirror + hardware inventory + face geometry in sign/state (config
	// false) — same config/state idiom as ESS/ramp-metering identity.
	cfg := sign.GetOrCreateConfig()
	cfg.Id = strPtr("i35-mm214-nb-dms")
	cfg.Name = strPtr("I-35 NB @ MM 214")
	cfg.Latitude = f64Ptr(30.2672)
	cfg.Longitude = f64Ptr(-97.7431)

	st := sign.GetOrCreateState()
	st.Id = strPtr("i35-mm214-nb-dms")
	st.Name = strPtr("I-35 NB @ MM 214")
	st.Latitude = f64Ptr(30.2672)
	st.Longitude = f64Ptr(-97.7431)
	st.Make = strPtr("Daktronics")
	st.Model = strPtr("Vanguard VF-2320")
	st.Firmware = strPtr("VX-4.3.2")
	st.Serial = strPtr("VF2320-2419-0088")
	st.Technology = yangpkg.OpenitsDms_Sign_State_Technology_led
	st.SignWidthPixels = u32Ptr(144)
	st.SignHeightPixels = u32Ptr(27)
	// Capability advertisement so central can validate message fit/markup.
	caps := st.GetOrCreateCapabilities()
	caps.SignType = yangpkg.OpenitsDms_Sign_State_Capabilities_SignType_full_matrix
	caps.CharacterHeightPixels = u32Ptr(7)
	caps.CharacterWidthPixels = u32Ptr(0)
	caps.ColorCapability = yangpkg.OpenitsDms_Sign_State_Capabilities_ColorCapability_color
	caps.MaxPages = u8Ptr(3)
	caps.BeaconCapable = boolPtr(true)
	caps.SupportedMultiTags = []yangpkg.E_OpenitsDmsTypes_MultiTag{
		yangpkg.OpenitsDmsTypes_MultiTag_multi_tag_new_line,
		yangpkg.OpenitsDmsTypes_MultiTag_multi_tag_new_page,
		yangpkg.OpenitsDmsTypes_MultiTag_multi_tag_font,
	}
	font, err := caps.NewFont(1)
	if err != nil {
		return nil, err
	}
	font.Name = strPtr("default-7line")
	font.CharacterHeightPixels = u32Ptr(7)

	// Message library (config-only): the slot activated below. The
	// message body (MULTI text, priority, owner, CRC, beacon) lives under
	// slot/config; slot/state carries the device-reported message-status
	// (config false).
	m := sign.GetOrCreateMessages()
	slot, err := m.NewSlot(yangpkg.OpenitsDmsTypes_MessageMemoryType_changeable, 1)
	if err != nil {
		return nil, err
	}
	slotCfg := slot.GetOrCreateConfig()
	slotCfg.MemoryType = yangpkg.OpenitsDmsTypes_MessageMemoryType_changeable
	slotCfg.SlotNumber = u16Ptr(1)
	slotCfg.MultiString = strPtr("[jp3]CRASH AHEAD[nl]USE CAUTION")
	slotCfg.Priority = u8Ptr(200)
	slotCfg.Owner = strPtr("tmc-ops")
	slotCfg.Crc = u32Ptr(2863311530)
	slotCfg.Beacon = yangpkg.OpenitsDms_Sign_Control_State_Active_Beacon_flashing
	slotSt := slot.GetOrCreateState()
	slotSt.Status = yangpkg.OpenitsDmsTypes_DmsMessageStatus_valid

	// Commanded control: intent in control/config (including the
	// active-message activation command), applied/actual in control/state
	// (config false) — control-mode, brightness mirror, and the
	// active-message readback that must match the library slot above.
	control := sign.GetOrCreateControl()
	ctrlCfg := control.GetOrCreateConfig()
	ctrlCfg.ControlMode = yangpkg.OpenitsDmsTypes_DmsControlMode_dms_control_central
	ctrlCfg.BrightnessSetpoint = u8Ptr(85)
	ctrlCfg.IlluminationControl = yangpkg.OpenitsDms_Sign_Control_Config_IlluminationControl_photocell

	activation := ctrlCfg.GetOrCreateActiveMessage()
	activation.MemoryType = yangpkg.OpenitsDmsTypes_MessageMemoryType_changeable
	activation.SlotNumber = u16Ptr(1)
	activation.Indefinite = boolPtr(true) // display until superseded (duration-s must be >=1 when finite)
	activation.ActivationPriority = u8Ptr(200)
	activation.Crc = u32Ptr(2863311530)
	activation.Owner = strPtr("tmc-ops")

	fallback := ctrlCfg.GetOrCreateFallback()
	commLoss := fallback.GetOrCreateCommLoss()
	commLoss.CommLossTimeoutS = u32Ptr(300)
	commLoss.MemoryType = yangpkg.OpenitsDmsTypes_MessageMemoryType_blank
	fallback.GetOrCreateEndOfDuration().MemoryType = yangpkg.OpenitsDmsTypes_MessageMemoryType_blank
	fallback.GetOrCreatePowerLoss().MemoryType = yangpkg.OpenitsDmsTypes_MessageMemoryType_blank

	ctrlSt := control.GetOrCreateState()
	ctrlSt.ControlMode = yangpkg.OpenitsDmsTypes_DmsControlMode_dms_control_central
	ctrlSt.DisplayState = yangpkg.OpenitsDmsTypes_SignMode_mode_normal
	ctrlSt.BrightnessSetpoint = u8Ptr(85)
	ctrlSt.IlluminationControl = yangpkg.OpenitsDms_Sign_Control_Config_IlluminationControl_photocell
	ctrlSt.LastModeChange = strPtr("2026-04-18T14:00:00Z")
	ctrlSt.BrightnessCurrent = u8Ptr(80)
	ctrlSt.CommLossActive = boolPtr(false)
	ctrlSt.PowerLossActive = boolPtr(false)

	active := ctrlSt.GetOrCreateActive()
	active.MemoryType = yangpkg.OpenitsDmsTypes_MessageMemoryType_changeable
	active.SlotNumber = u16Ptr(1)
	active.ActivatedAt = strPtr("2026-04-18T14:05:12Z")
	active.Source = strPtr("tmc-ops")
	active.MultiString = strPtr("[jp3]CRASH AHEAD[nl]USE CAUTION")
	active.Priority = u8Ptr(200)
	active.Owner = strPtr("tmc-ops")
	active.Crc = u32Ptr(2863311530)
	active.Beacon = yangpkg.OpenitsDms_Sign_Control_State_Active_Beacon_flashing

	// A higher-priority message is active; it displaced slot 4, which the
	// sign will restore when this activation clears (priority-queue restore).
	preempted := ctrlSt.GetOrCreatePreempted()
	preempted.MemoryType = yangpkg.OpenitsDmsTypes_MessageMemoryType_changeable
	preempted.SlotNumber = u16Ptr(4)

	diag := sign.GetOrCreateDiagnostics()
	diag.PixelsTotal = u32Ptr(3888)
	diag.PixelsFailed = u32Ptr(4)
	diag.PixelsStuckOn = u32Ptr(1)
	diag.PixelsStuckOff = u32Ptr(2)

	// Time-based scheduling: one day-plan activated on weekday mornings.
	sched := sign.GetOrCreateSchedule()
	dp, err := sched.NewDayPlan(1)
	if err != nil {
		return nil, err
	}
	dp.Name = strPtr("weekday-am")
	act, err := dp.NewAction("06:00:00")
	if err != nil {
		return nil, err
	}
	act.MemoryType = yangpkg.OpenitsDmsTypes_MessageMemoryType_changeable
	act.SlotNumber = u16Ptr(2)
	se, err := sched.NewScheduleEntry(1)
	if err != nil {
		return nil, err
	}
	se.DayPlan = u8Ptr(1) // empty calendar axes => applies every day
	schedSt := sched.GetOrCreateState()
	schedSt.ActiveDayPlanId = u8Ptr(1)
	schedSt.NextActionAt = strPtr("2026-04-19T06:00:00Z")
	diag.LampsTotal = u16Ptr(0)
	diag.LampsFailed = u16Ptr(0)
	diag.LastSelfTest = strPtr("2026-04-18T12:00:00Z")

	faults := sign.GetOrCreateFaults()
	fault, err := faults.NewFault("f-dms-001")
	if err != nil {
		return nil, err
	}
	fault.Category = yangpkg.OpenitsDmsTypes_DmsFaultEventKind_dms_fault_pixel
	fault.Severity = yangpkg.OpenitsTypes_FaultSeverity_warning
	fault.Description = strPtr("4 of 3888 pixels failed self-test")
	fault.FirstObserved = strPtr("2026-04-18T12:00:00Z")

	return dev, nil
}

func subscribeDMS(ctx context.Context, out chan<- tests.EventEnvelope, window time.Duration) error {
	base := "openits.us-tx.txdot.d07.dms.i35-mm214-nb-dms"
	src := "urn:openits:sign:us-tx:txdot:d07:i35-mm214-nb-dms"
	events := []tests.EventEnvelope{
		{
			Subject:  base + ".fault-raised",
			CEType:   "openits.dms.fault-raised.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VAA",
			CETime:   time.Now().UTC(),
			Data: &commonv1.FaultRaised{
				FaultId: "f-dms-001",
				Kind:    "openits-dms-types:dms-fault-pixel",
			},
		},
		{
			Subject:  base + ".mode-changed",
			CEType:   "openits.dms.mode-changed.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VAB",
			CETime:   time.Now().UTC(),
			Data: &commonv1.ModeChanged{
				Prior:   "blank",
				Current: "normal",
				Kind:    "openits-dms-types:dms-mode-event-kind",
			},
		},
		{
			Subject:  base + ".message-activation-failed",
			CEType:   "openits.dms.message-activation-failed.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VAC",
			CETime:   time.Now().UTC(),
			Data: &dmsv1.MessageActivationFailed{
				AttemptedMemoryType: dmsv1.MessageMemoryType_MESSAGE_MEMORY_TYPE_CHANGEABLE,
				AttemptedSlotNumber: 2,
				Reason:              "unsupported MULTI tag",
				ErrorType:           dmsv1.ErrorType_ERROR_TYPE_UNSUPPORTED_TAG,
				ErrorPosition:       17,
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

func u32Ptr(v uint32) *uint32 { return &v }

func collectESS() (*yangpkg.Device, error) {
	dev := &yangpkg.Device{}
	station := dev.GetOrCreateStation()

	cfg := station.GetOrCreateConfig()
	cfg.Id = strPtr("i70-rwis-mm312")
	cfg.Name = strPtr("I-70 EB @ MM 312 RWIS")
	cfg.Latitude = f64Ptr(39.6703)
	cfg.Longitude = f64Ptr(-105.5256)
	cfg.Elevation = f64Ptr(2520.0)
	cfg.RoadReference = strPtr("I-70 EB @ MP 312")

	st := station.GetOrCreateState()
	st.Id = strPtr("i70-rwis-mm312")
	st.Name = strPtr("I-70 EB @ MM 312 RWIS")
	st.Latitude = f64Ptr(39.6703)
	st.Longitude = f64Ptr(-105.5256)
	st.Elevation = f64Ptr(2520.0)
	st.RoadReference = strPtr("I-70 EB @ MP 312")
	st.Make = strPtr("Vaisala")
	st.Model = strPtr("RWS200")
	st.Firmware = strPtr("3.9.1")

	// Sensor mounting heights so height-dependent observations are comparable.
	essConf := station.GetOrCreateConfiguration()
	essConf.WindSensorHeightM = f64Ptr(10.0)
	essConf.AirTemperatureSensorHeightM = f64Ptr(2.0)
	essConf.VisibilitySensorHeightM = f64Ptr(3.0)

	atm := station.GetOrCreateAtmospheric()
	atm.ObservedAt = strPtr("2026-04-19T12:00:00Z")
	atm.SensorId = strPtr("atmos-1")
	atm.Quality = yangpkg.OpenitsEss_Station_Atmospheric_Quality_valid
	atm.AirTemperatureC = f64Ptr(-4.5)
	atm.DewpointC = f64Ptr(-6.0)
	atm.HumidityPercent = f64Ptr(82.0)
	atm.PressureHpa = f64Ptr(740.5)
	atm.WindSpeedMs = f64Ptr(6.2)
	atm.WindSpeedAvgMs = f64Ptr(7.5)
	atm.WindGustMs = f64Ptr(11.0)
	atm.WindDirectionDeg = f64Ptr(280.0)

	precip := station.GetOrCreatePrecipitation()
	precip.ObservedAt = strPtr("2026-04-19T12:00:00Z")
	precip.SensorId = strPtr("precip-1")
	precip.Type = yangpkg.OpenitsEss_PrecipitationType_freezing_rain
	precip.Intensity = yangpkg.OpenitsEss_PrecipitationIntensity_light
	precip.RateMmH = f64Ptr(0.80)
	precip.AccumulatedMm = f64Ptr(12.4)
	precip.Accumulated_1HMm = f64Ptr(3.0)
	precip.Accumulated_24HMm = f64Ptr(22.0)
	precip.SnowDepthMm = f64Ptr(45.0)
	precip.SnowRateMmH = f64Ptr(2.5)

	vis := station.GetOrCreateVisibility()
	vis.ObservedAt = strPtr("2026-04-19T12:00:00Z")
	vis.SensorId = strPtr("vis-1")
	vis.RangeM = u32Ptr(4800)
	vis.Situation = yangpkg.OpenitsEss_VisibilitySituation_blowing_snow

	pv := station.GetOrCreatePavement()
	pvs, _ := pv.NewSensor("pv-eb-inside")
	pvCfg := pvs.GetOrCreateConfig()
	pvCfg.SensorId = strPtr("pv-eb-inside")
	pvCfg.LaneReference = strPtr("eastbound-inside")
	pvSt := pvs.GetOrCreateState()
	pvSt.LaneReference = strPtr("eastbound-inside")
	pvSt.ObservedAt = strPtr("2026-04-19T12:00:00Z")
	pvSt.SurfaceTemperatureC = f64Ptr(-2.3)
	pvSt.Condition = yangpkg.OpenitsEss_PavementCondition_ice_warning
	pvSt.FreezePointC = f64Ptr(-6.5)
	pvSt.WaterDepthMm = f64Ptr(0.20)
	pvSt.SalinityPpm = u32Ptr(0)
	// Winter road-state: low grip + de-icer on the surface + a thin ice film.
	pvSt.GripCoefficient = f64Ptr(0.35)
	pvSt.ChemicalPercent = f64Ptr(12.5)
	pvSt.ChemicalFactor = f64Ptr(3.2)
	pvSt.IceDepthMm = f64Ptr(0.8)
	pvSt.SubsurfaceTemperatureC = f64Ptr(1.1)
	pvSt.SubsurfaceDepthMm = u16Ptr(300)

	diag := station.GetOrCreateDiagnostics()
	for _, sid := range []string{"atmos-1", "precip-1", "vis-1", "pv-eb-inside"} {
		s, _ := diag.NewSensor(sid)
		dCfg := s.GetOrCreateConfig()
		dCfg.SensorId = strPtr(sid)
		dCfg.SampleIntervalS = u32Ptr(60)
		dSt := s.GetOrCreateState()
		dSt.Type = strPtr("atmospheric")
		if sid == "pv-eb-inside" {
			dSt.Type = strPtr("pavement")
		}
		dSt.Health = yangpkg.OpenitsEss_SensorHealth_ok
		dSt.LastObservation = strPtr("2026-04-19T12:00:00Z")
		dSt.LastCalibration = strPtr("2026-01-15T10:00:00Z")
		dSt.NextCalibration = strPtr("2027-01-15T10:00:00Z")
		dSt.SampleIntervalS = u32Ptr(60)
	}
	return dev, nil
}

func subscribeESS(ctx context.Context, out chan<- tests.EventEnvelope, window time.Duration) error {
	base := "openits.us-co.codot.all.ess.i70-rwis-mm312"
	src := "urn:openits:station:us-co:codot:all:i70-rwis-mm312"
	events := []tests.EventEnvelope{
		{
			Subject:  base + ".fault-raised",
			CEType:   "openits.ess.fault-raised.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VBA",
			CETime:   time.Now().UTC(),
			Data: &commonv1.FaultRaised{
				FaultId: "f-ess-001",
				Kind:    "openits-ess-types:ess-fault-calibration-drift",
			},
		},
		{
			Subject:  base + ".weather-alert",
			CEType:   "openits.ess.weather-alert.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VBB",
			CETime:   time.Now().UTC(),
			Data: &essv1.WeatherAlert{
				ThresholdId:    "ice-warning",
				ObservedValue:  "-2.3",
				ThresholdValue: "0.0",
				Unit:           essv1.Unit_UNIT_CELSIUS,
				Direction:      essv1.Direction_DIRECTION_ENTERED,
				SensorId:       "surface-1",
			},
		},
		{
			Subject:  base + ".sensor-recalibrated",
			CEType:   "openits.ess.sensor-recalibrated.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VBC",
			CETime:   time.Now().UTC(),
			Data: &essv1.SensorRecalibrated{
				SensorId:     "atmos-1",
				CalibratedBy: "field-tech-42",
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

func strPtr(s string) *string   { return &s }
func u8Ptr(v uint8) *uint8      { return &v }
func u16Ptr(v uint16) *uint16   { return &v }
func i32Ptr(v int32) *int32     { return &v }
func f64Ptr(v float64) *float64 { return &v }
func boolPtr(v bool) *bool      { return &v }

// ringFor assigns NEMA dual-ring mapping (1-4 → ring 1, 5-8 → ring 2).
func ringFor(n uint8) uint8 {
	if n <= 4 {
		return 1
	}
	return 2
}

// barrierFor assigns NEMA barrier mapping (1-2, 5-6 → barrier 1; 3-4, 7-8 → barrier 2).
func barrierFor(n uint8) uint8 {
	switch n {
	case 1, 2, 5, 6:
		return 1
	default:
		return 2
	}
}

func collectRSU() (*yangpkg.Device, error) {
	dev := &yangpkg.Device{}
	r := dev.GetOrCreateRsu()

	// Identity: operator-provisioned intent in config, device-reported
	// mirror in state — same config/state idiom as ASC/DMS/ESS/ramp-metering
	// identity. RSU's identity grouping carries no device-hardware leaves;
	// those (firmware/hardware/serial) live directly on state via the
	// device-hardware grouping, same as the other services.
	cfg := r.GetOrCreateConfig()
	cfg.Id = strPtr("rsu-i35-mm214")
	cfg.Name = strPtr("I-35 NB @ MM 214")
	cfg.RoadReference = strPtr("I-35 NB @ MM 214")
	cfg.Latitude = f64Ptr(30.2672)
	cfg.Longitude = f64Ptr(-97.7431)

	// Surveyed antenna reference position (GNSS integrity): the precise
	// coordinate the live fix is compared against to detect spoofing / drift.
	surveyed := r.GetOrCreateGnss().GetOrCreateSurveyedPosition()
	surveyed.Latitude = f64Ptr(30.2672)
	surveyed.Longitude = f64Ptr(-97.7431)

	st := r.GetOrCreateState()
	st.Id = strPtr("rsu-i35-mm214")
	st.Name = strPtr("I-35 NB @ MM 214")
	st.RoadReference = strPtr("I-35 NB @ MM 214")
	st.Latitude = f64Ptr(30.2672)
	st.Longitude = f64Ptr(-97.7431)
	st.Firmware = strPtr("Kapsch RSU-4.9.2")
	st.FirmwareBuild = strPtr("2026.03-release")
	st.HardwareVersion = strPtr("Rev-C")
	st.Serial = strPtr("KSN-2419-0077")

	ch := r.GetOrCreateChannels()
	c, err := ch.NewChannel("172")
	if err != nil {
		return nil, err
	}
	c.RadioTech = yangpkg.OpenitsV2XRadio_RadioTech_radio_dsrc
	chCfg := c.GetOrCreateConfig()
	chCfg.DsrcChannelNumber = u8Ptr(172) // valid only because radio-tech is DSRC (when tie)
	chCfg.Enabled = boolPtr(true)
	chCfg.Mode = yangpkg.OpenitsV2XRadio_ChannelMode_mode_continuous
	chCfg.Primary = boolPtr(true)
	chCfg.TxPower = i8Ptr(20)
	// Message types this DSRC channel carries: SPaT/MAP plus RTCM SC-104
	// corrections (SAE J2735 rtcmCorrections) — exercises the v2x-message-type
	// vocabulary, including the correction traffic an RSU broadcasts alongside
	// safety/geometry messages.
	chCfg.MessageTypes = []yangpkg.E_OpenitsV2XMessagingTypes_V2XMessageType{
		yangpkg.OpenitsV2XMessagingTypes_V2XMessageType_msg_spat,
		yangpkg.OpenitsV2XMessagingTypes_V2XMessageType_msg_map,
		yangpkg.OpenitsV2XMessagingTypes_V2XMessageType_msg_rtcm,
	}
	dcc := chCfg.GetOrCreateDcc()
	dcc.Policy = yangpkg.OpenitsRsu_Rsu_Channels_Channel_Config_Dcc_Policy_adaptive
	dcc.CbrTargetPercent = u8Ptr(65)
	chSt := c.GetOrCreateState()
	chSt.Operational = boolPtr(true)

	diag := r.GetOrCreateDiagnostics()
	diag.GpsStatus = yangpkg.OpenitsRsuTypes_GpsFixStatus_fix_3d
	diag.SatellitesVisible = u8Ptr(9)
	// GNSS positioning health: strong geometry (low HDOP), disciplining pulse
	// present, and a small deviation from the surveyed reference (healthy — no
	// spoofing/drift). position-deviation-m is only meaningful because a
	// surveyed position is configured above.
	diag.Hdop = f64Ptr(0.9)
	diag.PpsPresent = boolPtr(true)
	diag.PositionDeviationM = f64Ptr(0.35)
	diag.TimeSource = yangpkg.OpenitsRsuTypes_TimeSource_gps
	diag.UptimeSeconds = u64Ptr(864_000)
	diag.RestartCount = u32Ptr(3)
	diag.LastRestartReason = yangpkg.OpenitsTypes_RestartReason_restart_upgrade
	diag.LastRestartTime = strPtr("2026-06-02T03:15:00Z")
	diag.ConfigHash = strPtr("sha256:8f2a1c9d")

	// Vehicle-analytics (config false, behind if-feature onboard-detection,
	// enabled by default): RSU-onboard aggregate vehicle counts/speeds
	// derived from received BSMs over a rolling window, plus the
	// sample-basis metadata that qualifies the estimate (window size,
	// sample count, penetration estimate).
	va := diag.GetOrCreateVehicleAnalytics()
	basis := va.GetOrCreateSampleBasis()
	basis.WindowType = yangpkg.OpenitsRsu_Rsu_Diagnostics_VehicleAnalytics_SampleBasis_WindowType_rolling
	basis.StatsWindowSeconds = u32Ptr(300)
	basis.SampleCount = u32Ptr(420)
	basis.PenetrationEstimatePct = f64Ptr(8.0)
	basis.ComputedAt = strPtr("2026-06-02T03:15:00Z")

	counts := va.GetOrCreateCounts()
	counts.CountBasis = yangpkg.OpenitsRsu_Rsu_Diagnostics_VehicleAnalytics_Counts_CountBasis_observation_sessions
	counts.Vehicles_1Min = u32Ptr(14)
	counts.Vehicles_1Hr = u32Ptr(612)
	counts.Vehicles_24Hr = u32Ptr(9840)

	speed := va.GetOrCreateSpeedMetrics()
	speed.AverageKmh = f64Ptr(97.4)
	speed.Percentile_85Kmh = f64Ptr(104.8)

	// SRM/SSM grant authority (config): the deployable NTCIP 1211 path — grants
	// arbitrated by the controller's Priority Request Server (asc-device is
	// therefore required by the grant-authority must), with EVP auto-granted so
	// emergency vehicles never wait on the PRS queue.
	srmSsm := r.GetOrCreateMessages().GetOrCreateSrmSsm()
	srmCfg := srmSsm.GetOrCreateConfig()
	srmCfg.GrantAuthority = yangpkg.OpenitsRsu_Rsu_Messages_SrmSsm_Config_GrantAuthority_controller_prs
	srmCfg.AscDevice = strPtr("asc-i35-mm214")
	srmCfg.EvpAutoGrant = boolPtr(true)

	// An active priority request the PRS has granted, carrying its decision
	// provenance (decision-authority) so the grant is auditable and the TSP
	// grant rate is attributable (NTCIP 1211).
	req, err := srmSsm.GetOrCreateActiveRequests().NewRequest("srm-000482")
	if err != nil {
		return nil, err
	}
	req.VehicleId = strPtr("bus-cap-metro-7")
	req.RequestType = yangpkg.OpenitsV2XMessagingTypes_SrmRequestType_srm_priority_request
	req.Status = yangpkg.OpenitsV2XMessagingTypes_SrmRequestStatus_approved
	req.DecisionAuthority = yangpkg.OpenitsV2XMessagingTypes_DecisionAuthority_controller_prs

	// SRM/SSM operator decisions (config): the approve/deny round-trip added
	// by cut B (srm-ssm/decisions), replacing the retired rsu-approve-srm
	// RPC. Correlated to an active priority request by request-id.
	dec, err := srmSsm.GetOrCreateDecisions().NewDecision("srm-000482")
	if err != nil {
		return nil, err
	}
	dec.Action = yangpkg.OpenitsRsu_Rsu_Messages_SrmSsm_Decisions_Decision_Action_approve
	dec.Reason = strPtr("transit priority")

	// TIM (Store-and-Repeat): a work-zone advisory, currently broadcasting and
	// not yet expired (broadcasting => not expired; the schema separates these
	// so post-outage replay of a lapsed TIM is an expressible defect).
	tim := r.GetOrCreateMessages().GetOrCreateTim()
	timCfg := tim.GetOrCreateConfig()
	timCfg.Enabled = boolPtr(true)
	timCfg.SuppressExpired = boolPtr(true)
	timMsg, err := tim.GetOrCreateActive().NewMessage("tim-workzone-01")
	if err != nil {
		return nil, err
	}
	tmCfg := timMsg.GetOrCreateConfig()
	tmCfg.Name = strPtr("work zone advisory")
	tmCfg.Enabled = boolPtr(true)
	tmCfg.Priority = u8Ptr(5)
	tmCfg.ItisCode = []uint16{1025, 7186} // structured ITIS codes (J2540.2), not a string
	tmCfg.StartTime = strPtr("2026-04-19T06:00:00Z")
	tmSt := timMsg.GetOrCreateState()
	tmSt.BroadcastCount = u64Ptr(1200)
	tmSt.Broadcasting = boolPtr(true)
	tmSt.Expired = boolPtr(false)

	// SCMS: an application certificate carrying its PSID permissions + geo
	// validity, so a consumer can verify not just that a cert exists but that
	// it may sign for the PSID (SPaT/MAP) in this region.
	sec := r.GetOrCreateSecurity()
	sec.GetOrCreateConfig().GetOrCreateMisbehaviorReporting().Enabled = boolPtr(true)
	secSt := sec.GetOrCreateState()
	secSt.EnrollmentStatus = yangpkg.OpenitsRsu_Rsu_Security_State_EnrollmentStatus_enrolled
	secSt.DaysToAppCertExpiry = i32Ptr(48) // app-cert expiry is the RSU-critical figure
	secSt.AppCertRenewalActive = boolPtr(false)
	mbr := secSt.GetOrCreateMisbehaviorReporting()
	mbr.ReportsGenerated = u64Ptr(10)
	mbr.ReportsSent = u64Ptr(8)
	mbr.ReportsPending = u32Ptr(2)
	cert, err := sec.GetOrCreateCertificates().NewCertificate("app-cert-spat")
	if err != nil {
		return nil, err
	}
	cst := cert.GetOrCreateState()
	cst.Type = yangpkg.OpenitsRsu_Rsu_Security_Certificates_Certificate_State_Type_application
	cst.ValidFrom = strPtr("2026-01-01T00:00:00Z")
	cst.ValidUntil = strPtr("2026-12-31T23:59:59Z")
	for _, p := range []struct {
		psid uint32
		ssp  string
	}{{0x8002, "01"}, {0x8003, ""}} { // 0x8002 SPaT, 0x8003 MAP
		perm, err := cst.NewPermissions(p.psid)
		if err != nil {
			return nil, err
		}
		perm.Ssp = strPtr(p.ssp)
	}
	geo := cst.GetOrCreateGeographicValidity()
	geo.RegionType = yangpkg.OpenitsRsu_Rsu_Security_Certificates_Certificate_State_GeographicValidity_RegionType_identified
	geo.IdentifiedRegionId = u32Ptr(840) // US

	return dev, nil
}

func subscribeRSU(ctx context.Context, out chan<- tests.EventEnvelope, window time.Duration) error {
	base := "openits.us-tx.txdot.d07.rsu.rsu-i35-mm214"
	src := "urn:openits:rsu:us-tx:txdot:d07:rsu-i35-mm214"
	events := []tests.EventEnvelope{
		{
			Subject:  base + ".rsu-srm-received",
			CEType:   "openits.rsu.rsu-srm-received.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VCA",
			CETime:   time.Now().UTC(),
			Data: &rsuv1.RsuSrmReceived{
				RequestId:    "srm-001",
				VehicleId:    "ev-austin-42",
				RequestType:  "openits-v2x-messaging-types:srm-preemption-request",
				Approach:     2,
				EtaSeconds:   12,
				VehicleClass: "emergency",
			},
		},
		{
			Subject:  base + ".rsu-certificate-expiring",
			CEType:   "openits.rsu.rsu-certificate-expiring.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VCB",
			CETime:   time.Now().UTC(),
			Data: &rsuv1.RsuCertificateExpiring{
				CertificateId:   "cert-abc-123",
				CertificateType: "pseudonym",
				DaysUntilExpiry: 3,
			},
		},
		{
			Subject:  base + ".rsu-channel-fault",
			CEType:   "openits.rsu.rsu-channel-fault.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VCC",
			CETime:   time.Now().UTC(),
			Data: &rsuv1.RsuChannelFault{
				ChannelId:         "184",
				DsrcChannelNumber: 184,
				FaultType:         "openits-v2x-radio-types:channel-fault-interference",
				Message:           "elevated BER on adjacent channel 184",
			},
		},
		{
			Subject:  base + ".rsu-gps-status-change",
			CEType:   "openits.rsu.rsu-gps-status-change.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VCD",
			CETime:   time.Now().UTC(),
			Data: &rsuv1.RsuGpsStatusChange{
				PreviousStatus: rsuv1.GpsFixStatus_GPS_FIX_STATUS_FIX_2D,
				NewStatus:      rsuv1.GpsFixStatus_GPS_FIX_STATUS_FIX_3D,
				Satellites:     9,
			},
		},
		{
			Subject:  base + ".rsu-security-event",
			CEType:   "openits.rsu.rsu-security-event.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VCE",
			CETime:   time.Now().UTC(),
			Data: &rsuv1.RsuSecurityEvent{
				EventType: "openits-rsu-types:sec-invalid-signature",
				Source:    "misbehavior-detector",
				Message:   "ECDSA verify failed on BSM from vehicle pseudonym-abc",
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

func u64Ptr(v uint64) *uint64 { return &v }
func i8Ptr(v int8) *int8      { return &v }

func collectRM() (*yangpkg.Device, error) {
	dev := &yangpkg.Device{}
	rm := dev.GetOrCreateRampMeter()

	// Identity is split into intended config (operator-provisioned) and
	// state (applied mirror + device-reported hardware inventory), same
	// idiom as ESS's station identity.
	cfg := rm.GetOrCreateConfig()
	cfg.Id = strPtr("i405-nb-western-ave")
	cfg.RoadReference = strPtr("I-405 NB @ Western Ave on-ramp")
	cfg.Latitude = f64Ptr(47.6205)
	cfg.Longitude = f64Ptr(-122.3493)

	st := rm.GetOrCreateState()
	st.Id = strPtr("i405-nb-western-ave")
	st.RoadReference = strPtr("I-405 NB @ Western Ave on-ramp")
	st.Latitude = f64Ptr(47.6205)
	st.Longitude = f64Ptr(-122.3493)
	st.Make = strPtr("Siemens")
	st.Model = strPtr("m60-ramp")
	st.Firmware = strPtr("9.4.1")

	plans := rm.GetOrCreatePlans()
	plan, err := plans.NewPlan(2)
	if err != nil {
		return nil, err
	}
	plan.Name = strPtr("am-peak")
	plan.ReleaseRateVph = u16Ptr(720)
	plan.HeadwayS = f64Ptr(5.0) // 3600/720 = 5.0s at one veh/green
	plan.VehiclesPerGreen = u8Ptr(1)
	plan.QueueOverrideThresholdVehicles = u16Ptr(25)
	plan.QueueOverrideClearThresholdVehicles = u16Ptr(15) // hysteresis: clear below activate
	plan.QueueOverrideRateVph = u16Ptr(900)
	pt := plan.GetOrCreatePhaseTiming()
	// Ramp-meter cycle: short green, brief yellow/red-clear, all within the
	// 5s headway (min-green+yellow+red-clear = 4.0 <= 5.0).
	pt.MinGreen = f64Ptr(1)
	pt.MaxGreen = f64Ptr(30)
	pt.YellowChange = f64Ptr(2.0)
	pt.RedClear = f64Ptr(1.0)
	pt.Passage = f64Ptr(0.5)

	// Commanded metering control: control/config is the intended command,
	// control/state is the applied mirror + live operational rollup
	// (current-release-rate-vph, queue state) — see openits-ramp-metering's
	// "control" container.
	control := rm.GetOrCreateControl()
	ctrlCfg := control.GetOrCreateConfig()
	ctrlCfg.Mode = yangpkg.OpenitsRampMeteringTypes_MeterMode_mode_active
	ctrlCfg.ActivePlanId = u8Ptr(2)
	ctrlCfg.CommandSource = yangpkg.OpenitsTypes_ControlSource_control_central

	ctrlSt := control.GetOrCreateState()
	ctrlSt.Mode = yangpkg.OpenitsRampMeteringTypes_MeterMode_mode_active
	ctrlSt.ActivePlanId = u8Ptr(2)
	ctrlSt.CommandSource = yangpkg.OpenitsTypes_ControlSource_control_central
	ctrlSt.LastModeChange = strPtr("2026-04-19T07:00:00Z")
	ctrlSt.CurrentReleaseRateVph = u16Ptr(720)
	ctrlSt.QueueLengthCurrentVehicles = u16Ptr(8)
	ctrlSt.QueueOverrideActive = boolPtr(false)

	lanes := rm.GetOrCreateLanes()
	lanes.GetOrCreateConfig().ReleaseCoordination = yangpkg.OpenitsRampMetering_RampMeter_Lanes_Config_ReleaseCoordination_simultaneous
	lane, err := lanes.NewLane("meter-1")
	if err != nil {
		return nil, err
	}
	laneCfg := lane.GetOrCreateConfig()
	laneCfg.LaneId = strPtr("meter-1")
	laneCfg.Bypass = boolPtr(false)
	laneCfg.BypassOperation = yangpkg.OpenitsRampMetering_RampMeter_Lanes_Lane_Config_BypassOperation_free_flow
	laneCfg.MinReleaseRateVph = u16Ptr(240)
	laneCfg.MaxReleaseRateVph = u16Ptr(1000)
	laneCfg.VehiclesPerGreen = u8Ptr(1)
	laneSt := lane.GetOrCreateState()
	laneSt.Bypass = boolPtr(false)
	laneSt.HeadState = yangpkg.OpenitsRampMetering_RampMeter_Lanes_Lane_State_HeadState_red
	laneSt.CurrentReleaseRateVph = u16Ptr(720)
	laneSt.LastRelease = strPtr("2026-04-19T07:05:03Z")
	for _, d := range []struct {
		id   string
		role yangpkg.E_OpenitsRampMetering_DetectorRole
	}{
		{"demand", yangpkg.OpenitsRampMetering_DetectorRole_demand},
		{"passage", yangpkg.OpenitsRampMetering_DetectorRole_passage},
		{"queue", yangpkg.OpenitsRampMetering_DetectorRole_queue},
	} {
		det, err := lane.NewDetector(d.id)
		if err != nil {
			return nil, err
		}
		detCfg := det.GetOrCreateConfig()
		detCfg.DetectorId = strPtr(d.id)
		detCfg.Role = d.role
		detSt := det.GetOrCreateState()
		detSt.Role = d.role
		detSt.Active = boolPtr(true)
		detSt.LastUpdate = strPtr("2026-04-19T07:05:00Z")
	}

	diag := rm.GetOrCreateDiagnostics()
	diag.ControllerUptimeS = u32Ptr(345_600)
	diag.LastSelfTest = strPtr("2026-04-19T06:55:00Z")
	diag.SignalHeadFaults = u16Ptr(0)

	return dev, nil
}

func subscribeRM(ctx context.Context, out chan<- tests.EventEnvelope, window time.Duration) error {
	base := "openits.us-wa.wsdot.nw-region.ramp-metering.i405-nb-western-ave"
	src := "urn:openits:ramp-meter:us-wa:wsdot:nw-region:i405-nb-western-ave"
	events := []tests.EventEnvelope{
		{
			Subject:  base + ".mode-changed",
			CEType:   "openits.ramp-metering.mode-changed.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VDA",
			CETime:   time.Now().UTC(),
			Data: &commonv1.ModeChanged{
				Prior:   "on-standby",
				Current: "active",
				Reason:  "schedule",
				Kind:    "openits-ramp-metering-types:ramp-meter-mode-event-kind",
			},
		},
		{
			Subject:  base + ".release-rate-changed",
			CEType:   "openits.ramp-metering.release-rate-changed.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VDB",
			CETime:   time.Now().UTC(),
			Data: &rampmeteringv1.ReleaseRateChanged{
				PreviousRateVph: 600,
				NewRateVph:      720,
				PlanId:          2,
				Cause:           rampmeteringv1.RateChangeCause_RATE_CHANGE_CAUSE_TRAFFIC_RESPONSIVE,
			},
		},
		{
			Subject:  base + ".queue-override-activated",
			CEType:   "openits.ramp-metering.queue-override-activated.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VDC",
			CETime:   time.Now().UTC(),
			Data: &rampmeteringv1.QueueOverrideActivated{
				QueueLengthVehicles: 27,
				ThresholdVehicles:   25,
				PlanId:              2,
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
