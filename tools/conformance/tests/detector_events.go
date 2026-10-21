package tests

import (
	"strings"

	signalcontrolv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/signal_control/v1"
)

// pedDetectorSideKinds are the pedestrian-event kinds raised by a
// pedestrian detector; they must name it in pedestrian-detector-id.
var pedDetectorSideKinds = map[string]bool{
	"openits-signal-control-types:ped-detector-on":       true,
	"openits-signal-control-types:ped-detector-off":      true,
	"openits-signal-control-types:ped-detector-failed":   true,
	"openits-signal-control-types:ped-detector-restored": true,
}

// TestDetectorEvents_ReferenceConfiguredDetectors checks that a
// detector-transition names a configured vehicle detector (detector-id)
// and that a detector-side pedestrian-event names a configured pedestrian
// detector (pedestrian-detector-id). yanglint does not enforce
// notification constraints, so the reference rule is enforced here.
func TestDetectorEvents_ReferenceConfiguredDetectors(t *T, obs *Observation) {
	sc := obs.Device.GetSignalController()
	vehicle := map[uint32]bool{}
	if d := sc.GetDetectors(); d != nil {
		for id := range d.Detector {
			vehicle[uint32(id)] = true
		}
	}
	ped := map[uint32]bool{}
	if pd := sc.GetPedestrianDetectors(); pd != nil {
		for id := range pd.PedestrianDetector {
			ped[uint32(id)] = true
		}
	}
	for _, e := range obs.Events {
		switch {
		case strings.HasSuffix(e.Subject, ".detector-transition"):
			ev, ok := e.Data.(*signalcontrolv1.DetectorTransition)
			if !ok {
				t.Errorf("detector-transition on %s has payload %T", e.Subject, e.Data)
				continue
			}
			if !vehicle[ev.GetDetectorId()] {
				t.Errorf("detector-transition on %s names detector-id %d, which is not a configured vehicle detector", e.Subject, ev.GetDetectorId())
			}
		case strings.HasSuffix(e.Subject, ".pedestrian-event"):
			ev, ok := e.Data.(*signalcontrolv1.PedestrianEvent)
			if !ok {
				t.Errorf("pedestrian-event on %s has payload %T", e.Subject, e.Data)
				continue
			}
			if !pedDetectorSideKinds[ev.GetKind()] {
				continue
			}
			if ev.PedestrianDetectorId == nil {
				t.Errorf("pedestrian-event %s on %s carries no pedestrian-detector-id", ev.GetKind(), e.Subject)
			} else if !ped[ev.GetPedestrianDetectorId()] {
				t.Errorf("pedestrian-event %s on %s names pedestrian-detector-id %d, which is not a configured pedestrian detector", ev.GetKind(), e.Subject, ev.GetPedestrianDetectorId())
			}
		}
	}
}
