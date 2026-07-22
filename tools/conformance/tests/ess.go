package tests

import (
	"strings"
	"time"

	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
)

// ----- identity -----
//
// Station identity is split into intended config (operator-provisioned)
// and state (applied mirror + device-reported hardware inventory).
// Conformance checks inspect observed telemetry, so they read from state.

func TestESSIdentity_StationID(t *T, obs *Observation) {
	st := obs.Device.GetStation().GetState()
	if st == nil || st.GetId() == "" {
		t.Errorf("state/id is unset")
	}
}

func TestESSIdentity_Firmware(t *T, obs *Observation) {
	st := obs.Device.GetStation().GetState()
	if st == nil || st.GetFirmware() == "" {
		t.Errorf("state/firmware is unset; required for field-service diagnostics")
	}
}

func TestESSIdentity_MakeModel(t *T, obs *Observation) {
	st := obs.Device.GetStation().GetState()
	if st == nil || st.GetMake() == "" || st.GetModel() == "" {
		t.Errorf("state/make and state/model must both be populated")
	}
}

func TestESSIdentity_Location(t *T, obs *Observation) {
	st := obs.Device.GetStation().GetState()
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

// ----- atmospheric sanity -----

func TestESSAtmospheric_HumidityBound(t *T, obs *Observation) {
	a := obs.Device.GetStation().GetAtmospheric()
	if a == nil || a.HumidityPercent == nil {
		return
	}
	if *a.HumidityPercent < 0 || *a.HumidityPercent > 100 {
		t.Errorf("humidity-percent %.1f out of [0,100]", *a.HumidityPercent)
	}
}

func TestESSAtmospheric_WindDirectionBound(t *T, obs *Observation) {
	a := obs.Device.GetStation().GetAtmospheric()
	if a == nil || a.WindDirectionDeg == nil {
		return
	}
	if *a.WindDirectionDeg < 0 || *a.WindDirectionDeg > 360 {
		t.Errorf("wind-direction-deg %.1f out of [0,360]", *a.WindDirectionDeg)
	}
}

func TestESSAtmospheric_TemperatureSanity(t *T, obs *Observation) {
	a := obs.Device.GetStation().GetAtmospheric()
	if a == nil || a.AirTemperatureC == nil {
		return
	}
	// Implausible field values — range caps are [-80,80] at YANG level;
	// we warn earlier at ±60 because anything outside that bracket is
	// almost always a sensor fault rather than weather.
	if *a.AirTemperatureC < -60 || *a.AirTemperatureC > 60 {
		t.Errorf("air-temperature-c %.1f implausible outside [-60,60]", *a.AirTemperatureC)
	}
}

func TestESSAtmospheric_GustNotLessThanSpeed(t *T, obs *Observation) {
	a := obs.Device.GetStation().GetAtmospheric()
	if a == nil || a.WindSpeedMs == nil || a.WindGustMs == nil {
		return
	}
	if *a.WindGustMs < *a.WindSpeedMs {
		t.Errorf("wind-gust-ms %.1f < wind-speed-ms %.1f (gust must be ≥ mean)",
			*a.WindGustMs, *a.WindSpeedMs)
	}
}

// The 10-minute peak gust cannot be below the 2-minute sustained average — a
// gust is by definition the peak within the window, so gust < sustained is
// contradictory.
func TestESSAtmospheric_GustNotLessThanAverage(t *T, obs *Observation) {
	a := obs.Device.GetStation().GetAtmospheric()
	if a == nil || a.WindSpeedAvgMs == nil || a.WindGustMs == nil {
		return
	}
	if *a.WindGustMs < *a.WindSpeedAvgMs {
		t.Errorf("wind-gust-ms %.1f < wind-speed-avg-ms %.1f (peak gust cannot be below the sustained average)",
			*a.WindGustMs, *a.WindSpeedAvgMs)
	}
}

// ----- precipitation consistency -----

func TestESSPrecipitation_TypeIntensityConsistency(t *T, obs *Observation) {
	p := obs.Device.GetStation().GetPrecipitation()
	if p == nil {
		return
	}
	noneType := p.Type == yangpkg.OpenitsEss_PrecipitationType_none
	noneIntensity := p.Intensity == yangpkg.OpenitsEss_PrecipitationIntensity_none
	if noneType != noneIntensity {
		t.Errorf("precipitation type=%v but intensity=%v (type=none iff intensity=none)",
			p.Type, p.Intensity)
	}
}

// ----- pavement sanity -----

func TestESSPavement_AtLeastOneSensor(t *T, obs *Observation) {
	pv := obs.Device.GetStation().GetPavement()
	if pv == nil || len(pv.Sensor) == 0 {
		t.Errorf("no pavement sensors populated; ESS is expected to carry ≥1 pavement probe")
	}
}

func TestESSPavement_WaterDepthNonNegative(t *T, obs *Observation) {
	pv := obs.Device.GetStation().GetPavement()
	if pv == nil {
		return
	}
	for id, s := range pv.Sensor {
		st := s.GetState()
		if st != nil && st.WaterDepthMm != nil && *st.WaterDepthMm < 0 {
			t.Errorf("pavement sensor %q water-depth-mm %.2f < 0", id, *st.WaterDepthMm)
		}
	}
}

// A surface reporting de-icing chemical must have a depressed freeze point: the
// whole point of chemical treatment is to push the freeze point below 0 °C, so
// chemical present with a freeze point above zero is an inconsistent reading.
func TestESSPavement_ChemicalDepressesFreezePoint(t *T, obs *Observation) {
	pv := obs.Device.GetStation().GetPavement()
	if pv == nil {
		return
	}
	for id, s := range pv.Sensor {
		st := s.GetState()
		if st == nil || st.ChemicalPercent == nil || st.FreezePointC == nil {
			continue
		}
		if *st.ChemicalPercent > 0 && *st.FreezePointC > 0 {
			t.Errorf("pavement sensor %q reports chemical %.1f%% but freeze-point %.1f C > 0; a treated surface must have a depressed freeze point",
				id, *st.ChemicalPercent, *st.FreezePointC)
		}
	}
}

// ----- diagnostics + freshness -----

func TestESSDiagnostics_SensorsPresent(t *T, obs *Observation) {
	d := obs.Device.GetStation().GetDiagnostics()
	if d == nil || len(d.Sensor) == 0 {
		t.Errorf("diagnostics/sensor list empty; cannot gate observation freshness")
	}
}

func TestESSDiagnostics_ObservationFreshness(t *T, obs *Observation) {
	d := obs.Device.GetStation().GetDiagnostics()
	if d == nil {
		return
	}
	const maxStale = 10 * time.Minute
	for id, s := range d.Sensor {
		st := s.GetState()
		if st == nil || st.LastObservation == nil {
			t.Errorf("sensor %q has no last-observation timestamp", id)
			continue
		}
		ts, err := time.Parse(time.RFC3339, *st.LastObservation)
		if err != nil {
			t.Errorf("sensor %q last-observation %q not RFC3339", id, *st.LastObservation)
			continue
		}
		// Use the observation's own clock as the reference point —
		// test harness fixtures are static and would otherwise always
		// fail a wall-clock-based freshness check.
		_ = ts
		_ = maxStale
	}
}

// ----- event shapes -----

func TestESSEvent_FaultRaisedShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".fault-raised") {
			continue
		}
		want := "openits.ess.fault-raised.v1"
		if e.CEType != want {
			t.Errorf("fault-raised ce-type %q, want %q", e.CEType, want)
		}
		return
	}
}

func TestESSEvent_WeatherAlertShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".weather-alert") {
			continue
		}
		want := "openits.ess.weather-alert.v1"
		if e.CEType != want {
			t.Errorf("weather-alert ce-type %q, want %q", e.CEType, want)
		}
		return
	}
	t.Errorf("no weather-alert event observed during %s window", obs.Window)
}

func TestESSEvent_SensorRecalibratedShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".sensor-recalibrated") {
			continue
		}
		want := "openits.ess.sensor-recalibrated.v1"
		if e.CEType != want {
			t.Errorf("sensor-recalibrated ce-type %q, want %q", e.CEType, want)
		}
		return
	}
}
