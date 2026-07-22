package tests

func TestYANG_Validate(t *T, obs *Observation) {
	if err := obs.Device.Validate(); err != nil {
		t.Errorf("device state fails YANG validation: %v", err)
	}
}
