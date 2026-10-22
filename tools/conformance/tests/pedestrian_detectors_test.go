package tests

import (
	"strings"
	"testing"

	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
)

func pedDev(phases []uint8, pedCalls map[uint8]uint8) *yangpkg.Device {
	dev := &yangpkg.Device{}
	sc := dev.GetOrCreateSignalController()
	for _, n := range phases {
		sc.GetOrCreatePhases().GetOrCreatePhase(n)
	}
	for id, call := range pedCalls {
		d := sc.GetOrCreatePedestrianDetectors().GetOrCreatePedestrianDetector(id)
		d.GetOrCreateConfig().CallPhase = &call
	}
	return dev
}

func TestPedestrianDetectors_CallPhaseResolves_Table(t *testing.T) {
	cases := []struct {
		name     string
		dev      *yangpkg.Device
		wantPass bool
		wantMsg  string
	}{
		{"resolves", pedDev([]uint8{2}, map[uint8]uint8{1: 2}), true, ""},
		{"missing phase", pedDev([]uint8{2}, map[uint8]uint8{1: 6}), false, "pedestrian detector 1"},
		{"no list", pedDev([]uint8{2}, nil), true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Run("ped", TestPedestrianDetectors_CallPhaseResolves, &Observation{Device: c.dev})
			if r.Pass != c.wantPass {
				t.Fatalf("pass=%v want %v: %s", r.Pass, c.wantPass, r.Message)
			}
			if c.wantMsg != "" && !strings.Contains(r.Message, c.wantMsg) {
				t.Fatalf("message %q lacks %q", r.Message, c.wantMsg)
			}
		})
	}
}
