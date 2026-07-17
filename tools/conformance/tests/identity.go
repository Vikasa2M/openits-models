package tests

// Device-reported identity lives in signal-controller/state (config
// false): it mirrors the operator-provisioned signal-controller/config
// identity and adds device-hardware (make/model/firmware). Per the
// ESS/DMS/ramp-metering conformance precedent, checks that assert what
// the device actually reported read from state, not config.

func TestIdentity_ControllerID(t *T, obs *Observation) {
	st := obs.Device.GetSignalController().GetState()
	if st == nil || st.GetId() == "" {
		t.Errorf("state/id is unset")
	}
}

func TestIdentity_Firmware(t *T, obs *Observation) {
	st := obs.Device.GetSignalController().GetState()
	if st == nil || st.GetFirmware() == "" {
		t.Errorf("state/firmware is unset; required for field-service diagnostics")
	}
}

func TestIdentity_MakeModel(t *T, obs *Observation) {
	st := obs.Device.GetSignalController().GetState()
	if st == nil || st.GetMake() == "" || st.GetModel() == "" {
		t.Errorf("state/make and state/model must both be populated")
	}
}

func TestIdentity_Location(t *T, obs *Observation) {
	st := obs.Device.GetSignalController().GetState()
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
