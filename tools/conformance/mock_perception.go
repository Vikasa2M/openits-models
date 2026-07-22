package main

import (
	"context"
	"time"

	commonv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/common/v1"
	perceptionv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/perception/v1"
	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
	"github.com/Vikasa2M/openits-models/tools/conformance/tests"
)

// collectPerception builds a fully-populated, spec-compliant perception-sensor
// observation: identity, a configured incident zone (>=3-vertex polygon), live
// object tracks with lifecycle + epoch, per-zone live analytics, an active
// incident carrying the full inventory on the incident-severity axis, and
// self-reported diagnostics + a fault.
func collectPerception() (*yangpkg.Device, error) {
	dev := &yangpkg.Device{}
	pc := dev.GetOrCreatePerceptionSensor()

	cfg := pc.GetOrCreateConfig()
	cfg.Id = strPtr("eb-travel-lanes-cam-03")
	cfg.Name = strPtr("US-75 EB @ Mockingbird Perception Camera")
	cfg.Latitude = f64Ptr(32.8360200)
	cfg.Longitude = f64Ptr(-96.7679600)
	cfg.RoadReference = strPtr("US-75 EB @ Mockingbird")

	st := pc.GetOrCreateState()
	st.Id = strPtr("eb-travel-lanes-cam-03")
	st.Name = strPtr("US-75 EB @ Mockingbird Perception Camera")
	st.Latitude = f64Ptr(32.8360200)
	st.Longitude = f64Ptr(-96.7679600)
	st.RoadReference = strPtr("US-75 EB @ Mockingbird")
	st.Make = strPtr("TrafficVision")
	st.Model = strPtr("TV-Edge-4")
	st.Firmware = strPtr("TVE-4.2.1")

	// Configuration: one incident zone whose geometry is a >=3-vertex polygon.
	conf := pc.GetOrCreateConfiguration()
	conf.DataIntervalS = u16Ptr(300)
	zoneCfg, err := conf.NewZone("eb-travel-lanes")
	if err != nil {
		return nil, err
	}
	zoneCfg.ZoneId = strPtr("eb-travel-lanes")
	zoneCfg.Name = strPtr("EB travel lanes")
	zoneCfg.Function = yangpkg.OpenitsPerception_ZoneFunction_incident
	zoneCfg.LegalHeading = u16Ptr(92)
	for i, ll := range [][2]float64{{32.8560, -96.7280}, {32.8561, -96.7278}, {32.8559, -96.7277}, {32.8558, -96.7279}} {
		v, err := zoneCfg.NewVertex(uint8(i))
		if err != nil {
			return nil, err
		}
		v.Latitude = f64Ptr(ll[0])
		v.Longitude = f64Ptr(ll[1])
	}

	// Live tracks: lifecycle + the epoch that disambiguates recycled ids.
	objs := pc.GetOrCreateObjects()
	objs.TrackCount = u16Ptr(1)
	objs.TrackEpoch = u32Ptr(3)
	track, err := objs.NewTrack(1042)
	if err != nil {
		return nil, err
	}
	track.Lifecycle = yangpkg.OpenitsPerception_PerceptionSensor_Objects_Track_Lifecycle_confirmed
	track.Class = yangpkg.OpenitsPerceptionTypes_ObjectClass_object_passenger_vehicle
	track.ClassConfidence = u8Ptr(94)
	track.Latitude = f64Ptr(32.8560100)
	track.Longitude = f64Ptr(-96.7279500)
	track.Heading = u16Ptr(92)
	track.SpeedKmh = f64Ptr(88.4)
	track.ObservedAt = strPtr("2026-04-19T12:00:58Z")

	// Live per-zone analytics (leafref zone-id -> a configured zone).
	zones := pc.GetOrCreateZones()
	zoneSt, err := zones.NewZone("eb-travel-lanes")
	if err != nil {
		return nil, err
	}
	zoneSt.ZoneId = strPtr("eb-travel-lanes")
	zoneSt.OccupancyCount = u16Ptr(4)
	zoneSt.Presence = boolPtr(true)
	zoneSt.AverageSpeedKmh = f64Ptr(86.1)

	diag := pc.GetOrCreateDiagnostics()
	diag.PointsPerSecond = u32Ptr(240000)
	diag.BlockagePercent = u8Ptr(2)
	diag.InternalTemperatureC = f64Ptr(37.5)
	diag.UptimeS = u64Ptr(1_209_600)

	// One active incident carrying the full inventory on the incident-severity
	// axis (NOT the equipment fault-severity axis).
	inc := pc.GetOrCreateIncidents()
	incident, err := inc.NewIncident("inc-2026-04-19-0007")
	if err != nil {
		return nil, err
	}
	incident.ZoneId = strPtr("eb-travel-lanes")
	incident.Type = yangpkg.OpenitsPerceptionTypes_IncidentType_incident_stopped_vehicle
	incident.Severity = yangpkg.OpenitsPerceptionTypes_IncidentSeverity_intermediate
	incident.TrackId = u32Ptr(1042)
	incident.ObjectClass = yangpkg.OpenitsPerceptionTypes_ObjectClass_object_passenger_vehicle
	incident.SpeedKmh = f64Ptr(0.0)
	incident.Confidence = u8Ptr(88)
	incident.Latitude = f64Ptr(32.8560100)
	incident.Longitude = f64Ptr(-96.7279500)
	incident.FirstObserved = strPtr("2026-04-19T11:58:12Z")

	faults := pc.GetOrCreateFaults()
	fault, err := faults.NewFault("f-pcp-001")
	if err != nil {
		return nil, err
	}
	fault.Category = yangpkg.OpenitsPerceptionTypes_PerceptionFaultEventKind_perception_fault_blockage
	fault.Severity = yangpkg.OpenitsTypes_FaultSeverity_warning
	fault.Description = strPtr("Partial lens blockage on the lower field of view")
	fault.FirstObserved = strPtr("2026-04-19T11:45:00Z")

	return dev, nil
}

func subscribePerception(ctx context.Context, out chan<- tests.EventEnvelope, window time.Duration) error {
	base := "openits.us-tx.txdot.d07.perception.eb-travel-lanes-cam-03"
	src := "urn:openits:perception:us-tx:txdot:d07:eb-travel-lanes-cam-03"
	events := []tests.EventEnvelope{
		{
			Subject:  base + ".zone-incident-detected",
			CEType:   "openits.perception.zone-incident-detected.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R7AAA",
			CETime:   time.Now().UTC(),
			Data: &perceptionv1.ZoneIncidentDetected{
				Kind:        "openits-perception-types:pcp-zone-incident-detected",
				IncidentId:  "inc-2026-04-19-0007",
				ZoneId:      "eb-travel-lanes",
				Type:        "openits-perception-types:incident-stopped-vehicle",
				Severity:    perceptionv1.IncidentSeverity_INCIDENT_SEVERITY_INTERMEDIATE,
				TrackId:     1042,
				ObjectClass: "openits-perception-types:object-passenger-vehicle",
				SpeedKmh:    "0.0",
				Confidence:  88,
			},
		},
		{
			Subject:  base + ".zone-interval-report",
			CEType:   "openits.perception.zone-interval-report.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R7BBB",
			CETime:   time.Now().UTC(),
			Data: &perceptionv1.ZoneIntervalReport{
				Kind: "openits-perception-types:pcp-zone-interval-report",
				Zone: []*perceptionv1.ZoneIntervalReportZone{
					{
						ZoneId:            "eb-travel-lanes",
						IntervalDurationS: 300,
						CrossedVolume:     47,
						ObservedCount:     50,
						OccupancyPercent:  "34.5",
						AverageSpeedKmh:   "88.3",
						ClassCount: []*perceptionv1.ClassCount{
							{Class: "openits-perception-types:object-passenger-vehicle", Count: 42},
							{Class: "openits-perception-types:object-truck", Count: 5},
						},
					},
				},
			},
		},
		{
			Subject:  base + ".fault-raised",
			CEType:   "openits.perception.fault-raised.v1",
			CESource: src,
			CEID:     "01HXYR3K9T8M2NAEQF5P4R7CCC",
			CETime:   time.Now().UTC(),
			Data: &commonv1.FaultRaised{
				FaultId: "f-pcp-001",
				Kind:    "openits-perception-types:perception-fault-blockage",
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
