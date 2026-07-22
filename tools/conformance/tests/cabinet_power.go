package tests

import (
	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
)

// Cabinet power is the leading comm-loss indicator; an unreported power source
// defeats the whole point of modeling it as telemetry rather than a fault.
func TestCabinetPower_SourceReported(t *T, obs *Observation) {
	cp := obs.Device.GetSignalController().GetCabinetPower()
	if cp == nil {
		return
	}
	if cp.PowerSource == yangpkg.OpenitsSignalControl_SignalController_CabinetPower_PowerSource_UNSET {
		t.Errorf("cabinet-power/power-source is unset")
	}
}

// A reported battery must report its state-of-charge — a battery whose charge
// cannot be read cannot inform the dispatch decision.
func TestCabinetPower_BatteryChargeReported(t *T, obs *Observation) {
	cp := obs.Device.GetSignalController().GetCabinetPower()
	if cp == nil || cp.GetBattery() == nil {
		return
	}
	if cp.GetBattery().StateOfChargePct == nil {
		t.Errorf("cabinet-power/battery present but state-of-charge-pct is unset")
	}
}

// The dispatch discriminator: on battery, the device must report how much
// runtime remains — "two hours left" vs "on line power" is the maintenance
// decision a fault-only model cannot express. Vacuous while on line power.
func TestCabinetPower_OnBatteryHasRuntime(t *T, obs *Observation) {
	cp := obs.Device.GetSignalController().GetCabinetPower()
	if cp == nil {
		return
	}
	onBattery := yangpkg.OpenitsSignalControl_SignalController_CabinetPower_PowerSource_on_battery
	if cp.PowerSource != onBattery {
		return
	}
	if cp.GetBattery() == nil || cp.GetBattery().RuntimeRemainingMinutes == nil {
		t.Errorf("cabinet-power is on-battery but runtime-remaining-minutes is unset")
	}
}
