package tests

import (
	"strings"

	perceptionv1 "github.com/openits/openits-models/pkg/proto/openits/perception/v1"
	yangpkg "github.com/openits/openits-models/pkg/yang/openits"
)

// ----- identity -----

func TestPerceptionIdentity_SensorID(t *T, obs *Observation) {
	st := obs.Device.GetPerceptionSensor().GetState()
	if st == nil || st.GetId() == "" {
		t.Errorf("state/id is unset")
	}
}

// ----- incident semantics -----

// Incident severity must be on the incident-severity axis (traffic impact), not
// the equipment fault-severity axis; an unset value means it was never set.
func TestPerceptionIncident_SeverityPresent(t *T, obs *Observation) {
	inc := obs.Device.GetPerceptionSensor().GetIncidents()
	if inc == nil || len(inc.Incident) == 0 {
		return
	}
	for _, i := range inc.Incident {
		if i.Severity == yangpkg.OpenitsPerceptionTypes_IncidentSeverity_UNSET {
			t.Errorf("incident %q severity is unset (must be on the incident-severity axis)", i.GetIncidentId())
		}
	}
}

// The incident inventory must carry detection detail (confidence), proving it
// mirrors the notification rather than dropping fields.
func TestPerceptionIncident_ConfidencePresent(t *T, obs *Observation) {
	inc := obs.Device.GetPerceptionSensor().GetIncidents()
	if inc == nil || len(inc.Incident) == 0 {
		return
	}
	for _, i := range inc.Incident {
		if i.Confidence == nil {
			t.Errorf("incident %q has no confidence; the inventory must mirror the notification, not drop detection detail", i.GetIncidentId())
		}
	}
}

// ----- track lifecycle -----

// Every live track must report a lifecycle state so consumers keep tentative
// and coasting (ghost) tracks out of zone counts.
func TestPerceptionTrack_LifecyclePresent(t *T, obs *Observation) {
	objs := obs.Device.GetPerceptionSensor().GetObjects()
	if objs == nil || len(objs.Track) == 0 {
		return
	}
	for _, tr := range objs.Track {
		if tr.Lifecycle == yangpkg.OpenitsPerception_PerceptionSensor_Objects_Track_Lifecycle_UNSET {
			t.Errorf("track %d has no lifecycle (tentative/confirmed/coasting)", tr.GetTrackId())
		}
	}
}

// ----- event shapes -----

func TestPerceptionEvent_IncidentDetectedShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".zone-incident-detected") {
			continue
		}
		want := "openits.perception.zone-incident-detected.v1"
		if e.CEType != want {
			t.Errorf("zone-incident-detected ce-type %q, want %q", e.CEType, want)
		}
		d, ok := e.Data.(*perceptionv1.ZoneIncidentDetected)
		if !ok {
			t.Errorf("zone-incident-detected Data is %T, want *perceptionv1.ZoneIncidentDetected", e.Data)
			return
		}
		if d.GetKind() == "" {
			t.Errorf("zone-incident-detected Data kind is empty")
		}
		if d.GetIncidentId() == "" {
			t.Errorf("zone-incident-detected Data incident-id is empty")
		}
		return
	}
	t.Errorf("no zone-incident-detected event observed during %s window", obs.Window)
}

// The zone interval report's per-class breakdown must reconcile with the
// crossed (throughput) volume — the sum rule that keeps the crossed and
// observed populations honest.
func TestPerceptionEvent_IntervalCrossedReconciles(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".zone-interval-report") {
			continue
		}
		want := "openits.perception.zone-interval-report.v1"
		if e.CEType != want {
			t.Errorf("zone-interval-report ce-type %q, want %q", e.CEType, want)
		}
		report, ok := e.Data.(*perceptionv1.ZoneIntervalReport)
		if !ok {
			t.Errorf("zone-interval-report Data is %T, want *perceptionv1.ZoneIntervalReport", e.Data)
			return
		}
		zones := report.GetZone()
		if len(zones) == 0 {
			t.Errorf("zone-interval-report Data has no zones")
			return
		}
		for _, z := range zones {
			var sum uint32
			for _, cc := range z.GetClassCount() {
				sum += cc.GetCount()
			}
			if sum != z.GetCrossedVolume() {
				t.Errorf("zone %q: sum(class-count)=%d != crossed-volume %d; breakdown must reconcile with crossed throughput",
					z.GetZoneId(), sum, z.GetCrossedVolume())
			}
		}
		return
	}
	t.Errorf("no zone-interval-report event observed during %s window", obs.Window)
}

func TestPerceptionEvent_FaultRaisedShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".fault-raised") {
			continue
		}
		want := "openits.perception.fault-raised.v1"
		if e.CEType != want {
			t.Errorf("fault-raised ce-type %q, want %q", e.CEType, want)
		}
		return
	}
	t.Errorf("no fault-raised event observed during %s window", obs.Window)
}
