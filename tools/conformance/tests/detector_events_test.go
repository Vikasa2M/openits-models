package tests

import (
	"testing"

	signalcontrolv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/signal_control/v1"
	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
	"google.golang.org/protobuf/proto"
)

const subjBase = "openits.us-tx.txdot.d07.signal-control.i35-exit-214."

func detEventDev() *yangpkg.Device {
	dev := &yangpkg.Device{}
	sc := dev.GetOrCreateSignalController()
	sc.GetOrCreateDetectors().GetOrCreateDetector(1)
	sc.GetOrCreatePedestrianDetectors().GetOrCreatePedestrianDetector(1)
	return dev
}

func vehEvent(id uint32) EventEnvelope {
	return EventEnvelope{
		Subject: subjBase + "detector-transition",
		Data: &signalcontrolv1.DetectorTransition{
			Kind:       "openits-signal-control-types:vehicle-detector-on",
			DetectorId: id,
		},
	}
}

func pedEvent(kind string, id *uint32) EventEnvelope {
	return EventEnvelope{
		Subject: subjBase + "pedestrian-event",
		Data:    &signalcontrolv1.PedestrianEvent{Kind: kind, PedestrianDetectorId: id},
	}
}

func TestDetectorEvents_ReferenceConfiguredDetectors_Table(t *testing.T) {
	const (
		on    = "openits-signal-control-types:ped-detector-on"
		begin = "openits-signal-control-types:ped-begin-walk"
	)
	cases := []struct {
		name   string
		events []EventEnvelope
		errs   int
	}{
		{"known vehicle id", []EventEnvelope{vehEvent(1)}, 0},
		{"unknown vehicle id", []EventEnvelope{vehEvent(9)}, 1},
		{"ped-detector-on known id", []EventEnvelope{pedEvent(on, proto.Uint32(1))}, 0},
		{"ped-detector-on unknown id", []EventEnvelope{pedEvent(on, proto.Uint32(9))}, 1},
		{"ped-detector-on no id", []EventEnvelope{pedEvent(on, nil)}, 1},
		{"ped-begin-walk no id", []EventEnvelope{pedEvent(begin, nil)}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Run("det", TestDetectorEvents_ReferenceConfiguredDetectors, &Observation{Device: detEventDev(), Events: c.events})
			if (c.errs == 0) != r.Pass {
				t.Fatalf("pass=%v want errors=%d: %s", r.Pass, c.errs, r.Message)
			}
		})
	}
}
