package tests

import (
	"strings"

	commonv1 "github.com/openits/openits-models/pkg/proto/openits/common/v1"
	trafficsensorv1 "github.com/openits/openits-models/pkg/proto/openits/traffic_sensor/v1"
	yangpkg "github.com/openits/openits-models/pkg/yang/openits"
)

// ----- identity -----
//
// Device-reported identity lives in traffic-sensor/state (config false):
// it mirrors the operator-provisioned traffic-sensor/config identity.
// Per the ESS/DMS/ramp-metering conformance precedent, checks that
// assert what the device actually reported read from state, not config.

func TestTrafficSensorIdentity_SensorID(t *T, obs *Observation) {
	st := obs.Device.GetTrafficSensor().GetState()
	if st == nil || st.GetId() == "" {
		t.Errorf("state/id is unset")
	}
}

// ----- configuration -----
//
// lane-numbering-origin declares how this station's lane-id ordering
// maps to physical lane position; leaving it UNSET means a consumer
// cannot tell a leftmost-in-travel station from a from-sensor one and
// risks silently swapping the HOV/inside lane with the shoulder.

func TestTrafficSensorConfig_LaneNumberingOrigin(t *T, obs *Observation) {
	conf := obs.Device.GetTrafficSensor().GetConfiguration()
	if conf == nil || conf.LaneNumberingOrigin == yangpkg.OpenitsTrafficSensor_TrafficSensor_Configuration_LaneNumberingOrigin_UNSET {
		t.Errorf("configuration/lane-numbering-origin is unset")
	}
}

// ----- lanes -----

func TestTrafficSensorLane_ZoneLengthPositive(t *T, obs *Observation) {
	lanes := obs.Device.GetTrafficSensor().GetLanes()
	if lanes == nil || len(lanes.Lane) == 0 {
		t.Errorf("no lanes populated")
		return
	}
	for _, lane := range lanes.Lane {
		cfg := lane.GetConfig()
		if cfg == nil || cfg.GetEffectiveZoneLengthM() <= 0 {
			t.Errorf("lane %v config/effective-zone-length-m must be > 0 (loop/radar footprint required to normalize time occupancy)", lane.GetLaneId())
		}
	}
}

// ----- interval data -----
//
// One reporting interval's aggregate observations for a lane, applied
// under lanes/lane/state/interval (config false).

func TestTrafficSensorInterval_DataQualityPresent(t *T, obs *Observation) {
	lanes := obs.Device.GetTrafficSensor().GetLanes()
	if lanes == nil || len(lanes.Lane) == 0 {
		t.Errorf("no lanes populated")
		return
	}
	for _, lane := range lanes.Lane {
		iv := lane.GetState().GetInterval()
		if iv == nil || iv.DataQuality == yangpkg.OpenitsTrafficSensor_TrafficSensor_Lanes_Lane_State_Interval_DataQuality_UNSET {
			t.Errorf("lane %v state/interval/data-quality is unset", lane.GetLaneId())
		}
	}
}

func TestTrafficSensorInterval_UptimeInRange(t *T, obs *Observation) {
	lanes := obs.Device.GetTrafficSensor().GetLanes()
	if lanes == nil || len(lanes.Lane) == 0 {
		t.Errorf("no lanes populated")
		return
	}
	for _, lane := range lanes.Lane {
		iv := lane.GetState().GetInterval()
		if iv == nil {
			t.Errorf("lane %v state/interval is missing", lane.GetLaneId())
			continue
		}
		up := iv.GetUptimePercent()
		if up < 0 || up > 100 {
			t.Errorf("lane %v uptime-percent %v out of range [0,100]", lane.GetLaneId(), up)
		}
	}
}

func TestTrafficSensorInterval_SpaceMeanNotAboveTimeMean(t *T, obs *Observation) {
	lanes := obs.Device.GetTrafficSensor().GetLanes()
	if lanes == nil || len(lanes.Lane) == 0 {
		t.Errorf("no lanes populated")
		return
	}
	for _, lane := range lanes.Lane {
		iv := lane.GetState().GetInterval()
		if iv == nil || iv.SpeedAverageKmh == nil || iv.SpeedSpaceMeanKmh == nil {
			t.Errorf("lane %v speed-average-kmh and speed-space-mean-kmh must both be present", lane.GetLaneId())
			continue
		}
		if iv.GetSpeedSpaceMeanKmh() > iv.GetSpeedAverageKmh() {
			t.Errorf("lane %v speed-space-mean-kmh %v > speed-average-kmh %v; space-mean must not exceed time-mean",
				lane.GetLaneId(), iv.GetSpeedSpaceMeanKmh(), iv.GetSpeedAverageKmh())
		}
	}
}

// Per-class volumes are un-interpretable without knowing which bin plan the
// station uses, so the scheme must be declared.
func TestTrafficSensorConfig_ClassificationSchemePresent(t *T, obs *Observation) {
	conf := obs.Device.GetTrafficSensor().GetConfiguration()
	if conf == nil || conf.ClassificationScheme == yangpkg.OpenitsTrafficSensorTypes_ClassificationScheme_UNSET {
		t.Errorf("configuration/classification-scheme is unset; per-class volumes are un-interpretable without the bin plan")
	}
}

// The per-class breakdown must reconcile with total volume: unbinned vehicles
// belong in unclassified-volume, not dropped. sum(class-volume) +
// unclassified-volume == volume.
func TestTrafficSensorInterval_ClassVolumeReconciles(t *T, obs *Observation) {
	lanes := obs.Device.GetTrafficSensor().GetLanes()
	if lanes == nil || len(lanes.Lane) == 0 {
		t.Errorf("no lanes populated")
		return
	}
	for _, lane := range lanes.Lane {
		iv := lane.GetState().GetInterval()
		if iv == nil {
			continue
		}
		if len(iv.ClassVolume) == 0 && iv.UnclassifiedVolume == nil {
			continue // device reports no per-class breakdown for this interval
		}
		var binned uint32
		for _, cv := range iv.ClassVolume {
			binned += cv.GetVolume()
		}
		total := binned + iv.GetUnclassifiedVolume()
		if total != iv.GetVolume() {
			t.Errorf("lane %v: sum(class-volume)=%d + unclassified=%d = %d != total volume %d; per-class breakdown must reconcile with volume",
				lane.GetLaneId(), binned, iv.GetUnclassifiedVolume(), total, iv.GetVolume())
		}
	}
}

// ----- operational health (per-lane + rollup) -----

// Each monitored lane must report its own health so partial-lane failure is
// visible instead of hidden by a single binary sensor status.
func TestTrafficSensorLane_OperationalStatusPresent(t *T, obs *Observation) {
	lanes := obs.Device.GetTrafficSensor().GetLanes()
	if lanes == nil || len(lanes.Lane) == 0 {
		t.Errorf("no lanes populated")
		return
	}
	for _, lane := range lanes.Lane {
		st := lane.GetState()
		if st == nil || st.OperationalStatus == yangpkg.OpenitsTrafficSensorTypes_OperationalStatus_UNSET {
			t.Errorf("lane %v state/operational-status is unset", lane.GetLaneId())
		}
	}
}

// The sensor-level rollup must agree with the per-lane health: active iff every
// reporting lane is active; otherwise not-active (degraded/inactive). A sensor
// that reports active while a lane is down is exactly the partial-failure blind
// spot this cut closes.
func TestTrafficSensorHealth_RollupConsistent(t *T, obs *Observation) {
	ts := obs.Device.GetTrafficSensor()
	lanes := ts.GetLanes()
	if lanes == nil || len(lanes.Lane) == 0 {
		return
	}
	active := yangpkg.OpenitsTrafficSensorTypes_OperationalStatus_active
	unset := yangpkg.OpenitsTrafficSensorTypes_OperationalStatus_UNSET
	allActive, anyReported := true, false
	for _, lane := range lanes.Lane {
		ls := lane.GetState().OperationalStatus
		if ls == unset {
			continue
		}
		anyReported = true
		if ls != active {
			allActive = false
		}
	}
	if !anyReported {
		return
	}
	sensor := ts.GetState().OperationalStatus
	if allActive && sensor != active {
		t.Errorf("all lanes active but sensor-level operational-status is %v, want active", sensor)
	}
	if !allActive && sensor == active {
		t.Errorf("a lane is not active but sensor-level operational-status reports active — partial failure hidden")
	}
}

// A zone reporting a queue must report its length, else the queue signal is
// un-actionable (a TMC cannot see spillback risk).
func TestTrafficSensorQueue_LengthWhenQueueing(t *T, obs *Observation) {
	queues := obs.Device.GetTrafficSensor().GetQueues()
	if queues == nil || len(queues.QueueZone) == 0 {
		return
	}
	for _, z := range queues.QueueZone {
		if z.GetQueueing() && z.QueueLengthM == nil {
			t.Errorf("queue-zone %q is queueing but queue-length-m is unset", z.GetZoneId())
		}
	}
}

// ----- calibration + mounting (detail) -----

// A sensor should report a concrete calibration state; 'unknown' leaves a
// consumer unable to trust speed/length/classification metrics that depend on
// a valid calibration.
func TestTrafficSensorDiag_CalibrationStatusKnown(t *T, obs *Observation) {
	diag := obs.Device.GetTrafficSensor().GetDiagnostics()
	if diag == nil {
		return
	}
	cal := diag.GetCalibration()
	unset := yangpkg.OpenitsTrafficSensor_TrafficSensor_Diagnostics_Calibration_Status_UNSET
	unknown := yangpkg.OpenitsTrafficSensor_TrafficSensor_Diagnostics_Calibration_Status_unknown
	if cal == nil || cal.Status == unset || cal.Status == unknown {
		t.Errorf("diagnostics/calibration/status is unknown/unset; metrics are un-trustable without a reported calibration state")
	}
}

// from-sensor lane numbering is only interpretable if the mounting side is
// declared: "the lane nearest the sensor" needs to know which side that is.
func TestTrafficSensorConfig_MountingSideForFromSensor(t *T, obs *Observation) {
	conf := obs.Device.GetTrafficSensor().GetConfiguration()
	if conf == nil {
		return
	}
	fromSensor := yangpkg.OpenitsTrafficSensor_TrafficSensor_Configuration_LaneNumberingOrigin_from_sensor
	unknown := yangpkg.OpenitsTrafficSensor_TrafficSensor_Configuration_MountingSide_unknown
	if conf.LaneNumberingOrigin == fromSensor && conf.MountingSide == unknown {
		t.Errorf("lane-numbering-origin=from-sensor but mounting-side is unknown; lane numbers cannot be remapped to physical position")
	}
}

// ----- event shapes -----

func TestTrafficSensorEvent_IntervalReportShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".traffic-interval-report") {
			continue
		}
		want := "openits.traffic-sensor.traffic-interval-report.v1"
		if e.CEType != want {
			t.Errorf("traffic-interval-report ce-type %q, want %q", e.CEType, want)
		}
		report, ok := e.Data.(*trafficsensorv1.TrafficIntervalReport)
		if !ok {
			t.Errorf("traffic-interval-report Data is %T, want *trafficsensorv1.TrafficIntervalReport", e.Data)
			return
		}
		lanes := report.GetLane()
		if len(lanes) == 0 {
			t.Errorf("traffic-interval-report Data has no lanes")
			return
		}
		if lanes[0].GetVolume() == 0 {
			t.Errorf("traffic-interval-report Data lane %d volume is zero", lanes[0].GetLaneId())
		}
		return
	}
	t.Errorf("no traffic-interval-report event observed during %s window", obs.Window)
}

func TestTrafficSensorEvent_FaultRaisedShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".fault-raised") {
			continue
		}
		want := "openits.traffic-sensor.fault-raised.v1"
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
