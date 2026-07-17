package tests

import (
	"strings"

	yangpkg "github.com/openits/openits-models/pkg/yang/openits"
)

// ----- identity -----
//
// Device-reported identity lives in sign/state (config false): it mirrors
// the operator-provisioned sign/config identity and adds device-hardware
// (make/model/firmware/serial) + technology + face geometry. Per the
// ESS/ramp-metering conformance precedent, checks that assert what the
// device actually reported read from state, not config.

func TestDMSIdentity_SignID(t *T, obs *Observation) {
	st := obs.Device.GetSign().GetState()
	if st == nil || st.GetId() == "" {
		t.Errorf("state/id is unset")
	}
}

func TestDMSIdentity_Firmware(t *T, obs *Observation) {
	st := obs.Device.GetSign().GetState()
	if st == nil || st.GetFirmware() == "" {
		t.Errorf("state/firmware is unset; required for field-service diagnostics")
	}
}

func TestDMSIdentity_MakeModel(t *T, obs *Observation) {
	st := obs.Device.GetSign().GetState()
	if st == nil || st.GetMake() == "" || st.GetModel() == "" {
		t.Errorf("state/make and state/model must both be populated")
	}
}

func TestDMSIdentity_Location(t *T, obs *Observation) {
	st := obs.Device.GetSign().GetState()
	if st == nil {
		t.Errorf("state missing")
		return
	}
	lat, lon := st.GetLatitude(), st.GetLongitude()
	if lat == 0 && lon == 0 {
		t.Errorf("state/latitude and /longitude both zero; implausible")
	}
	if lat < -90 || lat > 90 {
		t.Errorf("latitude %v out of range [-90,90]", lat)
	}
	if lon < -180 || lon > 180 {
		t.Errorf("longitude %v out of range [-180,180]", lon)
	}
}

func TestDMSIdentity_Dimensions(t *T, obs *Observation) {
	st := obs.Device.GetSign().GetState()
	if st == nil {
		t.Errorf("state missing")
		return
	}
	w, h := st.GetSignWidthPixels(), st.GetSignHeightPixels()
	if w == 0 || h == 0 {
		t.Errorf("sign-width-pixels=%d, sign-height-pixels=%d; both required", w, h)
	}
}

// Stuck pixels (on/off) are a breakdown of failed pixels, so their sum cannot
// exceed the total failed count — otherwise the diagnostic counts contradict.
func TestDMSDiagnostics_StuckPixelsWithinFailed(t *T, obs *Observation) {
	diag := obs.Device.GetSign().GetDiagnostics()
	if diag == nil || diag.PixelsFailed == nil {
		return
	}
	if diag.PixelsStuckOn == nil && diag.PixelsStuckOff == nil {
		return
	}
	stuck := diag.GetPixelsStuckOn() + diag.GetPixelsStuckOff()
	if stuck > diag.GetPixelsFailed() {
		t.Errorf("pixels stuck-on(%d)+stuck-off(%d)=%d exceeds pixels-failed(%d); the breakdown contradicts the total",
			diag.GetPixelsStuckOn(), diag.GetPixelsStuckOff(), stuck, diag.GetPixelsFailed())
	}
}

// Every scheduled day-plan must contain at least one action; an empty day-plan
// schedules nothing (mirrors the config-tree must, which ygot Validate does not
// enforce).
func TestDMSSchedule_DayPlanHasAction(t *T, obs *Observation) {
	sched := obs.Device.GetSign().GetSchedule()
	if sched == nil {
		return
	}
	for _, dp := range sched.DayPlan {
		if len(dp.Action) == 0 {
			t.Errorf("schedule day-plan %d has no actions; it schedules nothing", dp.GetDayPlanId())
		}
	}
}

// ----- capability advertisement -----

// A sign must advertise its matrix type so central can decide whether
// arbitrary text positioning / graphics are possible before composing a
// message.
func TestDMSCapabilities_SignTypePresent(t *T, obs *Observation) {
	caps := obs.Device.GetSign().GetState().GetCapabilities()
	if caps == nil || caps.SignType == yangpkg.OpenitsDms_Sign_State_Capabilities_SignType_UNSET {
		t.Errorf("sign/state/capabilities/sign-type is unset; central cannot validate message fit")
	}
}

// A character- or line-matrix sign must advertise its character-cell height,
// otherwise central cannot tell how many text rows fit and messages truncate.
func TestDMSCapabilities_CharMatrixHasCellSize(t *T, obs *Observation) {
	caps := obs.Device.GetSign().GetState().GetCapabilities()
	if caps == nil {
		return
	}
	st := caps.SignType
	if st == yangpkg.OpenitsDms_Sign_State_Capabilities_SignType_char_matrix ||
		st == yangpkg.OpenitsDms_Sign_State_Capabilities_SignType_line_matrix {
		if caps.CharacterHeightPixels == nil || *caps.CharacterHeightPixels == 0 {
			t.Errorf("sign-type is %v but character-height-pixels is unset/zero; central cannot compute text fit", st)
		}
	}
}

// A sign must report how its brightness is governed (photocell / timer /
// manual), so a consumer knows whether brightness-setpoint is an auto-dim
// ceiling or a fixed level.
func TestDMSControl_IlluminationControlPresent(t *T, obs *Observation) {
	st := obs.Device.GetSign().GetControl().GetState()
	if st == nil {
		return
	}
	if st.IlluminationControl == yangpkg.OpenitsDms_Sign_Control_Config_IlluminationControl_UNSET {
		t.Errorf("control/state/illumination-control is unset; can't tell auto-dim from a fixed brightness level")
	}
}

// ----- operational status -----
//
// Commanded/applied sign-control state lives under sign/control/state
// (config false): control-mode + brightness mirror the commanded
// control/config, applied by the sign.

func TestDMSOperational_ModePresent(t *T, obs *Observation) {
	cs := obs.Device.GetSign().GetControl().GetState()
	if cs == nil {
		t.Fatalf("control/state container missing")
	}
	if cs.ControlMode == yangpkg.OpenitsDmsTypes_DmsControlMode_UNSET {
		t.Errorf("control/state/control-mode is unset")
	}
}

func TestDMSOperational_BrightnessInRange(t *T, obs *Observation) {
	cs := obs.Device.GetSign().GetControl().GetState()
	if cs == nil {
		return
	}
	if cs.BrightnessSetpoint != nil && *cs.BrightnessSetpoint > 100 {
		t.Errorf("brightness-setpoint %d > 100", *cs.BrightnessSetpoint)
	}
	if cs.BrightnessCurrent != nil && *cs.BrightnessCurrent > 100 {
		t.Errorf("brightness-current %d > 100", *cs.BrightnessCurrent)
	}
}

func TestDMSOperational_DisplayStatePresent(t *T, obs *Observation) {
	cs := obs.Device.GetSign().GetControl().GetState()
	if cs == nil {
		t.Fatalf("control/state container missing")
	}
	if cs.DisplayState == yangpkg.OpenitsDmsTypes_SignMode_UNSET {
		t.Errorf("control/state/display-state is unset")
	}
}

func TestDMSFallback_PowerLossActivePresent(t *T, obs *Observation) {
	cs := obs.Device.GetSign().GetControl().GetState()
	if cs == nil {
		t.Fatalf("control/state container missing")
	}
	if cs.PowerLossActive == nil {
		t.Errorf("control/state/power-loss-active is unset")
	}
}

func TestDMSOperational_Heartbeat(t *T, obs *Observation) {
	for _, e := range obs.Events {
		// DMS heartbeats ride on mode-changed during the window;
		// we accept that as proof of life for the mock driver path.
		if strings.HasSuffix(e.Subject, ".mode-changed") {
			return
		}
	}
	t.Errorf("no mode-changed event observed during %s window", obs.Window)
}

// ----- messages -----

func TestDMSMessages_BufferIntegrity(t *T, obs *Observation) {
	m := obs.Device.GetSign().GetMessages()
	if m == nil || len(m.Slot) == 0 {
		t.Errorf("no message slots populated")
		return
	}
	for k, slot := range m.Slot {
		cfg := slot.GetConfig()
		if cfg.GetMultiString() == "" {
			continue // empty slot is allowed
		}
		if cfg.GetCrc() == 0 {
			t.Errorf("slot (memory-type=%v, slot-number=%d) has MULTI text but zero CRC",
				k.MemoryType, k.SlotNumber)
		}
	}
}

func TestDMSMessages_SlotStatusValid(t *T, obs *Observation) {
	// The activated slot's device-reported message-status (slot/state,
	// config false) must confirm the message was accepted as valid.
	active := obs.Device.GetSign().GetControl().GetState().GetActive()
	if active == nil {
		t.Fatalf("active message missing")
	}
	if active.MemoryType == yangpkg.OpenitsDmsTypes_MessageMemoryType_blank {
		return // intentionally blank; no slot to check
	}
	if active.SlotNumber == nil {
		t.Errorf("active.slot-number unset while memory-type=%v", active.MemoryType)
		return
	}
	m := obs.Device.GetSign().GetMessages()
	slot := m.GetSlot(active.MemoryType, *active.SlotNumber)
	if slot == nil {
		t.Errorf("active message references slot (%v, %d) not present in inventory",
			active.MemoryType, *active.SlotNumber)
		return
	}
	status := slot.GetState().GetStatus()
	if status == yangpkg.OpenitsDmsTypes_DmsMessageStatus_UNSET {
		t.Errorf("slot (%v, %d) state/status is unset", active.MemoryType, *active.SlotNumber)
		return
	}
	if status != yangpkg.OpenitsDmsTypes_DmsMessageStatus_valid {
		t.Errorf("slot (%v, %d) state/status = %v, want valid", active.MemoryType, *active.SlotNumber, status)
	}
}

func TestDMSMessages_ActiveBeaconMatchesSlot(t *T, obs *Observation) {
	// The active-message readback's beacon state must match the beacon
	// configured on the library slot it was activated from — same
	// readback-consistency idiom as TestDMSMessages_ActiveMatchesSlot.
	active := obs.Device.GetSign().GetControl().GetState().GetActive()
	if active == nil {
		t.Fatalf("active message missing")
	}
	if active.MemoryType == yangpkg.OpenitsDmsTypes_MessageMemoryType_blank {
		return // intentionally blank; nothing to cross-check
	}
	if active.SlotNumber == nil {
		t.Errorf("active.slot-number unset while memory-type=%v", active.MemoryType)
		return
	}
	m := obs.Device.GetSign().GetMessages()
	slot := m.GetSlot(active.MemoryType, *active.SlotNumber)
	if slot == nil {
		t.Errorf("active message references slot (%v, %d) not present in inventory",
			active.MemoryType, *active.SlotNumber)
		return
	}
	if active.GetBeacon() != slot.GetConfig().GetBeacon() {
		t.Errorf("active.beacon=%v diverges from slot (%v, %d) config/beacon=%v",
			active.GetBeacon(), active.MemoryType, *active.SlotNumber, slot.GetConfig().GetBeacon())
	}
}

func TestDMSMessages_ActiveMatchesSlot(t *T, obs *Observation) {
	// The active-message readback lives under control/state (config false);
	// the message library it's activated from lives under sign/messages.
	active := obs.Device.GetSign().GetControl().GetState().GetActive()
	if active == nil {
		t.Fatalf("active message missing")
	}
	m := obs.Device.GetSign().GetMessages()
	if active.MemoryType == yangpkg.OpenitsDmsTypes_MessageMemoryType_blank {
		return // intentionally blank; nothing to cross-check
	}
	if active.SlotNumber == nil {
		t.Errorf("active.slot-number unset while memory-type=%v", active.MemoryType)
		return
	}
	src := m.GetSlot(active.MemoryType, *active.SlotNumber)
	if src == nil {
		t.Errorf("active message references slot (%v, %d) not present in inventory",
			active.MemoryType, *active.SlotNumber)
		return
	}
	if src.GetConfig().GetMultiString() != active.GetMultiString() {
		t.Errorf("active MULTI diverges from slot MULTI for (%v, %d)",
			active.MemoryType, *active.SlotNumber)
	}
}

// ----- diagnostics -----

func TestDMSDiagnostics_PixelsFailedBound(t *T, obs *Observation) {
	d := obs.Device.GetSign().GetDiagnostics()
	if d == nil {
		return
	}
	if d.GetPixelsFailed() > d.GetPixelsTotal() {
		t.Errorf("pixels-failed %d > pixels-total %d", d.GetPixelsFailed(), d.GetPixelsTotal())
	}
}

func TestDMSDiagnostics_LampsFailedBound(t *T, obs *Observation) {
	d := obs.Device.GetSign().GetDiagnostics()
	if d == nil {
		return
	}
	if d.GetLampsFailed() > d.GetLampsTotal() {
		t.Errorf("lamps-failed %d > lamps-total %d", d.GetLampsFailed(), d.GetLampsTotal())
	}
}

// ----- events -----

func TestDMSEvent_FaultRaisedShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".fault-raised") {
			continue
		}
		want := "openits.dms.fault-raised.v1"
		if e.CEType != want {
			t.Errorf("fault-raised ce-type %q, want %q", e.CEType, want)
		}
		return
	}
}

func TestDMSEvent_ModeChangedShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".mode-changed") {
			continue
		}
		want := "openits.dms.mode-changed.v1"
		if e.CEType != want {
			t.Errorf("mode-changed ce-type %q, want %q", e.CEType, want)
		}
		return
	}
}

func TestDMSEvent_ActivationFailedShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".message-activation-failed") {
			continue
		}
		want := "openits.dms.message-activation-failed.v1"
		if e.CEType != want {
			t.Errorf("message-activation-failed ce-type %q, want %q", e.CEType, want)
		}
		return
	}
}
