package main

import (
	"context"
	"time"

	commonv1 "github.com/openits/openits-models/pkg/proto/openits/common/v1"
	trafficsensorv1 "github.com/openits/openits-models/pkg/proto/openits/traffic_sensor/v1"
	yangpkg "github.com/openits/openits-models/pkg/yang/openits"
	"github.com/openits/openits-models/tools/conformance/tests"
)

// collectTrafficSensor builds a fully-populated, spec-compliant
// traffic-sensor observation: identity, writable configuration
// (reporting cadence + lane-numbering convention), one monitored lane
// with both intended (config) and applied/observed (state) data, a
// live queue zone, and self-reported diagnostics + an active fault.
func collectTrafficSensor() (*yangpkg.Device, error) {
	dev := &yangpkg.Device{}
	ts := dev.GetOrCreateTrafficSensor()

	// Identity: operator-provisioned intent in config, device-reported
	// mirror + hardware inventory + operational rollup in state (config
	// false) — same config/state idiom as ESS/DMS/ramp-metering identity.
	cfg := ts.GetOrCreateConfig()
	cfg.Id = strPtr("i35-mm214-ts-01")
	cfg.Name = strPtr("I-35 NB @ MM 214 Traffic Sensor")
	cfg.Latitude = f64Ptr(30.2672)
	cfg.Longitude = f64Ptr(-97.7431)
	cfg.RoadReference = strPtr("I-35 NB @ MM 214")

	st := ts.GetOrCreateState()
	st.Id = strPtr("i35-mm214-ts-01")
	st.Name = strPtr("I-35 NB @ MM 214 Traffic Sensor")
	st.Latitude = f64Ptr(30.2672)
	st.Longitude = f64Ptr(-97.7431)
	st.RoadReference = strPtr("I-35 NB @ MM 214")
	st.Make = strPtr("Wavetronix")
	st.Model = strPtr("SmartSensor HD")
	st.Firmware = strPtr("SS-HD-6.1.2")
	st.Serial = strPtr("WVX-2419-0091")
	st.OperationalStatus = yangpkg.OpenitsTrafficSensorTypes_OperationalStatus_active

	// Configuration: reporting cadence + the lane-numbering convention
	// (canonical = leftmost-in-travel) that disambiguates lane-id
	// ordering across vendors.
	conf := ts.GetOrCreateConfiguration()
	conf.DataIntervalS = u16Ptr(60)
	conf.LaneNumberingOrigin = yangpkg.OpenitsTrafficSensor_TrafficSensor_Configuration_LaneNumberingOrigin_leftmost_in_travel
	// Declare which classification (bin) plan the per-class volumes below
	// implement, so the archive never merges incompatible bin plans.
	conf.ClassificationScheme = yangpkg.OpenitsTrafficSensorTypes_ClassificationScheme_scheme_length_based
	// Side-fire mounting side, needed to interpret from-sensor lane numbering.
	conf.MountingSide = yangpkg.OpenitsTrafficSensor_TrafficSensor_Configuration_MountingSide_roadside

	// One monitored lane. The list key `lane-id` is a leafref to
	// ../config/lane-id: NewLane sets the top-level key, but
	// config/lane-id (the intended per-lane setting the leafref
	// resolves against) must ALSO be set or TestYANG_Validate fails
	// with an empty leafref target set.
	lanes := ts.GetOrCreateLanes()
	lane, err := lanes.NewLane(1)
	if err != nil {
		return nil, err
	}
	laneCfg := lane.GetOrCreateConfig()
	laneCfg.LaneId = u8Ptr(1)
	laneCfg.Name = strPtr("northbound-inside")
	laneCfg.Carriageway = strPtr("northbound")
	laneCfg.TravelHeading = u16Ptr(10)
	laneCfg.EffectiveZoneLengthM = f64Ptr(3.0)

	laneSt := lane.GetOrCreateState()
	laneSt.LaneId = u8Ptr(1)
	laneSt.Name = strPtr("northbound-inside")
	laneSt.Carriageway = strPtr("northbound")
	laneSt.TravelHeading = u16Ptr(10)
	laneSt.EffectiveZoneLengthM = f64Ptr(3.0)
	// Per-lane health: this lane is active. The sensor-level rollup below
	// must therefore also be active (no lane is degraded/inactive).
	laneSt.OperationalStatus = yangpkg.OpenitsTrafficSensorTypes_OperationalStatus_active

	interval := laneSt.GetOrCreateInterval()
	interval.IntervalStart = strPtr("2026-04-19T12:00:00Z")
	interval.IntervalDurationS = u16Ptr(60)
	interval.Volume = u32Ptr(42)
	interval.Occupancy = f64Ptr(12.4)
	interval.SpeedAverageKmh = f64Ptr(98.4)
	// Space-mean (harmonic-mean) speed is required to be <= the
	// time-mean average: time-mean systematically overestimates
	// space-mean, never the reverse.
	interval.SpeedSpaceMeanKmh = f64Ptr(97.6)
	interval.Speed_85ThPercentileKmh = f64Ptr(104.2)
	interval.HeadwayAverageS = f64Ptr(2.8)
	interval.GapAverageS = f64Ptr(1.9)
	interval.Density = f64Ptr(11.5)
	interval.FlowRateVph = u32Ptr(2520)
	interval.DataQuality = yangpkg.OpenitsTrafficSensor_TrafficSensor_Lanes_Lane_State_Interval_DataQuality_valid
	interval.UptimePercent = f64Ptr(100.0)

	// Per-class breakdown that reconciles with total volume:
	// sum(class-volume) + unclassified-volume == volume (39+2+1+0 == 42).
	// Unbinned vehicles go to unclassified-volume, never dropped.
	for _, cv := range []struct {
		id  uint8
		vol uint32
	}{{1, 39}, {2, 2}, {3, 1}} {
		e, err := interval.NewClassVolume(cv.id)
		if err != nil {
			return nil, err
		}
		e.Volume = u32Ptr(cv.vol)
	}
	interval.UnclassifiedVolume = u32Ptr(0)
	interval.WrongWayVolume = u32Ptr(0)
	interval.MeanVehicleLengthM = f64Ptr(4.85)
	interval.SpeedStdDevKmh = f64Ptr(7.3)

	presence := laneSt.GetOrCreatePresence()
	presence.Occupied = boolPtr(false)
	presence.LastDetection = strPtr("2026-04-19T12:00:58Z")

	// Live queue-detection rollup for one zone.
	queues := ts.GetOrCreateQueues()
	zone, err := queues.NewQueueZone("qz-1")
	if err != nil {
		return nil, err
	}
	zone.Queueing = boolPtr(true)
	zone.QueueDurationS = u32Ptr(45)
	zone.QueueLengthM = u32Ptr(120)
	zone.BackOfQueueM = u32Ptr(140)

	diag := ts.GetOrCreateDiagnostics()
	diag.SignalQuality = u8Ptr(92)
	diag.InternalTemperatureC = f64Ptr(28.5)
	diag.UptimeS = u64Ptr(2_592_000)
	diag.LastSelfTest = strPtr("2026-04-19T06:00:00Z")
	cal := diag.GetOrCreateCalibration()
	cal.LastCalibrated = strPtr("2026-04-01T09:00:00Z")
	cal.Status = yangpkg.OpenitsTrafficSensor_TrafficSensor_Diagnostics_Calibration_Status_calibrated

	faults := ts.GetOrCreateFaults()
	fault, err := faults.NewFault("f-ts-001")
	if err != nil {
		return nil, err
	}
	fault.Category = yangpkg.OpenitsTrafficSensorTypes_TrafficSensorFaultEventKind_traffic_sensor_fault_rf
	fault.Severity = yangpkg.OpenitsTypes_FaultSeverity_warning
	fault.Description = strPtr("RF front-end signal marginal on lane 1")
	fault.FirstObserved = strPtr("2026-04-19T11:45:00Z")

	return dev, nil
}

func subscribeTrafficSensor(ctx context.Context, out chan<- tests.EventEnvelope, window time.Duration) error {
	base := "openits.us-tx.txdot.d07.traffic-sensor.i35-mm214-ts-01"
	src := "urn:openits:traffic-sensor:us-tx:txdot:d07:i35-mm214-ts-01"
	events := []tests.EventEnvelope{
		{
			Subject:  base + ".traffic-interval-report",
			CEType:   "openits.traffic-sensor.traffic-interval-report.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VEA",
			CETime:   time.Now().UTC(),
			Data: &trafficsensorv1.TrafficIntervalReport{
				Kind: "openits-traffic-sensor-types:ts-traffic-interval-report",
				Lane: []*trafficsensorv1.TrafficIntervalReportLane{
					{
						LaneId:            1,
						Name:              "northbound-inside",
						Carriageway:       "northbound",
						Volume:            42,
						Occupancy:         "12.4",
						SpeedAverageKmh:   "98.4",
						SpeedSpaceMeanKmh: "97.6",
						DataQuality:       trafficsensorv1.DataQuality_DATA_QUALITY_VALID,
						UptimePercent:     "100.0",
					},
				},
			},
		},
		{
			Subject:  base + ".queue-state-changed",
			CEType:   "openits.traffic-sensor.queue-state-changed.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VEB",
			CETime:   time.Now().UTC(),
			Data: &trafficsensorv1.QueueStateChanged{
				Kind:           "openits-traffic-sensor-types:ts-queue-state-changed",
				ZoneId:         "qz-1",
				Queueing:       true,
				QueueDurationS: 45,
			},
		},
		{
			Subject:  base + ".traffic-sensor-status-report",
			CEType:   "openits.traffic-sensor.traffic-sensor-status-report.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VEC",
			CETime:   time.Now().UTC(),
			Data: &trafficsensorv1.TrafficSensorStatusReport{
				Kind:              "openits-traffic-sensor-types:ts-status-report",
				Name:              "I-35 NB @ MM 214 Traffic Sensor",
				OperationalStatus: trafficsensorv1.OperationalStatus_OPERATIONAL_STATUS_ACTIVE,
				Latitude:          "30.2672",
				Longitude:         "-97.7431",
			},
		},
		{
			Subject:  base + ".fault-raised",
			CEType:   "openits.traffic-sensor.fault-raised.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R6VED",
			CETime:   time.Now().UTC(),
			Data: &commonv1.FaultRaised{
				FaultId: "f-ts-001",
				Kind:    "openits-traffic-sensor-types:traffic-sensor-fault-rf",
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
