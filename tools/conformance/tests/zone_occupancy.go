package tests

import (
	"strings"

	zoneoccupancyv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/zone_occupancy/v1"
	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
)

// Checks for the openits-zone-occupancy capability.
//
// The capability's live container carries NO `must` constraints between its
// leaves, and that is a deliberate modeling decision: a device may report
// presence without a count, more objects than the capacity, or a breakdown
// that does not sum, and the schema keeps every one of those representable so
// that a misbehaving device is visible on the wire rather than unable to
// speak. That decision moves the burden here. The schema says what a device
// CAN say; conformance says what a well-behaved device SHOULD say. Tests
// below assert only the rules the modules state in prose, and deliberately do
// not re-assert anything the schema already enforces.

// zoneOccupancyOf returns the capability subtree from whichever device profile
// composes it, or nil if this device does not implement it.
//
// Only openits-perception composes the capability today (behind its
// zone-occupancy feature). When a second profile does — a bay magnetometer
// being the obvious one — extend this one function rather than every test.
func zoneOccupancyOf(obs *Observation) *yangpkg.OpenitsPerception_PerceptionSensor_ZoneOccupancy {
	return obs.Device.GetPerceptionSensor().GetZoneOccupancy()
}

// ----- configuration / state coherence -----

// Live occupancy must report on a region the device actually declares. The
// leafref is require-instance false so a just-deleted region is representable
// rather than invalid, but reporting occupancy for a region that was never
// configured is a real defect: a consumer has no capacity, sensing-method, or
// classifies flag to interpret the reading with.
func TestZoneOccupancy_LiveZonesAreConfigured(t *T, obs *Observation) {
	zoc := zoneOccupancyOf(obs)
	if zoc == nil || zoc.GetZones() == nil {
		return
	}
	cfg := zoc.GetConfiguration()
	for id := range zoc.GetZones().Zone {
		if cfg == nil || cfg.GetZone(id) == nil {
			t.Errorf("live occupancy zone %q has no configured counterpart; a consumer cannot interpret the reading without capacity/sensing-method/classifies", id)
		}
	}
}

// A zone that declares classifies=false must report an empty present-class
// list. This is the distinction the `classifies` leaf exists to draw: read
// with an empty list, false means "this device never classifies, the list is
// complete" and true means "the classifier ran and found nothing". A device
// that says false and then emits classes has made that unreadable.
//
// No `must` can express this — it spans the config and state trees — which is
// exactly why it belongs in conformance.
func TestZoneOccupancy_ClassifiesGatesBreakdown(t *T, obs *Observation) {
	zoc := zoneOccupancyOf(obs)
	if zoc == nil || zoc.GetZones() == nil || zoc.GetConfiguration() == nil {
		return
	}
	for id, live := range zoc.GetZones().Zone {
		cfgZone := zoc.GetConfiguration().GetZone(id)
		// Omitted classifies is a third, legitimate state — the device does
		// not state whether it classifies — so only an explicit false binds.
		if cfgZone == nil || cfgZone.Classifies == nil || cfgZone.GetClassifies() {
			continue
		}
		if len(live.PresentClass) > 0 {
			t.Errorf("zone %q declares classifies=false but reports %d present-class entries; that makes an empty list unreadable for every other zone",
				id, len(live.PresentClass))
		}
	}
}

// Where a region classifies, the present-class counts sum to occupancy-count
// with object-unknown as the catch-all, so no present object is dropped from
// the breakdown.
//
// The schema deliberately leaves a non-summing breakdown representable so a
// classifier that dropped one is visible rather than silenced. Conformance is
// where that visibility turns into a finding — the model surfaces it, this
// test judges it.
func TestZoneOccupancy_PresentClassSumsToCount(t *T, obs *Observation) {
	zoc := zoneOccupancyOf(obs)
	if zoc == nil || zoc.GetZones() == nil {
		return
	}
	for id, live := range zoc.GetZones().Zone {
		// The sum rule only applies where a breakdown was actually produced
		// and a count was reported to compare it against. A presence-only
		// sensor reports neither and is not in scope.
		if len(live.PresentClass) == 0 || live.OccupancyCount == nil {
			continue
		}
		var sum uint16
		for _, pc := range live.PresentClass {
			sum += pc.GetCount()
		}
		if sum != live.GetOccupancyCount() {
			t.Errorf("zone %q: sum(present-class count)=%d != occupancy-count %d; the breakdown must account for every present object, with object-unknown as the catch-all",
				id, sum, live.GetOccupancyCount())
		}
	}
}

// occupied-since records when the current unbroken occupancy began, and is
// omitted while the region is vacant and reset when it empties. A vacant
// region still carrying the leaf yields a dwell that never ends.
func TestZoneOccupancy_OccupiedSinceOnlyWhenPresent(t *T, obs *Observation) {
	zoc := zoneOccupancyOf(obs)
	if zoc == nil || zoc.GetZones() == nil {
		return
	}
	for id, live := range zoc.GetZones().Zone {
		// Unset presence is a device that does not report it; only an
		// explicit "nothing is here" contradicts an occupancy start time.
		if live.Presence == nil || live.GetPresence() {
			continue
		}
		if live.OccupiedSince != nil {
			t.Errorf("zone %q reports presence=false but still carries occupied-since %q; it must be cleared when the region empties or consumers derive an unbounded dwell",
				id, live.GetOccupiedSince())
		}
	}
}

// Every live reading must say when it was taken. Without measured-at a
// consumer cannot compare the reading against the device's cadence, and is
// forced to assume anything it received is fresh.
func TestZoneOccupancy_MeasuredAtPresent(t *T, obs *Observation) {
	zoc := zoneOccupancyOf(obs)
	if zoc == nil || zoc.GetZones() == nil {
		return
	}
	for id, live := range zoc.GetZones().Zone {
		if live.MeasuredAt == nil {
			t.Errorf("live occupancy zone %q has no measured-at; a consumer cannot judge whether the reading is still current", id)
		}
	}
}

// ----- event shape -----

// The capability's own notification, published on the capability's service
// token rather than the composing device's.
func TestZoneOccupancyEvent_IntervalReportShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".zone-occupancy-interval-report") {
			continue
		}
		want := "openits.zone-occupancy.zone-occupancy-interval-report.v1"
		if e.CEType != want {
			t.Errorf("zone-occupancy-interval-report ce-type %q, want %q", e.CEType, want)
		}
		report, ok := e.Data.(*zoneoccupancyv1.ZoneOccupancyIntervalReport)
		if !ok {
			t.Errorf("zone-occupancy-interval-report Data is %T, want *zoneoccupancyv1.ZoneOccupancyIntervalReport", e.Data)
			return
		}
		if report.GetKind() == "" {
			t.Errorf("zone-occupancy-interval-report Data kind is empty")
		}
		if len(report.GetZone()) == 0 {
			t.Errorf("zone-occupancy-interval-report Data has no zones")
		}
		return
	}
	t.Errorf("no zone-occupancy-interval-report event observed during %s window", obs.Window)
}

// The interval breakdown carries the same sum rule as the live state: where
// the region classifies, observed-class accounts for every observed object.
func TestZoneOccupancyEvent_ObservedClassReconciles(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".zone-occupancy-interval-report") {
			continue
		}
		report, ok := e.Data.(*zoneoccupancyv1.ZoneOccupancyIntervalReport)
		if !ok {
			return // shape is TestZoneOccupancyEvent_IntervalReportShape's finding
		}
		for _, z := range report.GetZone() {
			if len(z.GetObservedClass()) == 0 {
				continue // a non-classifying region; the sum rule does not apply
			}
			var sum uint32
			for _, oc := range z.GetObservedClass() {
				sum += oc.GetCount()
			}
			if sum != z.GetObservedCount() {
				t.Errorf("zone %q: sum(observed-class count)=%d != observed-count %d; the breakdown must account for every observed object",
					z.GetZoneId(), sum, z.GetObservedCount())
			}
		}
		return
	}
}

// Peak simultaneous occupancy cannot exceed the number of distinct objects
// seen across the whole interval — the peak is a subset of the population, at
// one instant. A report violating this has miscounted one of the two, and the
// pair is precisely what a capacity-planning consumer reasons from.
func TestZoneOccupancyEvent_PeakWithinObserved(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".zone-occupancy-interval-report") {
			continue
		}
		report, ok := e.Data.(*zoneoccupancyv1.ZoneOccupancyIntervalReport)
		if !ok {
			return
		}
		for _, z := range report.GetZone() {
			// Both default to zero when unreported, which trivially satisfies
			// the inequality — no need to distinguish absent from zero here.
			if uint32(z.GetPeakOccupancyCount()) > z.GetObservedCount() {
				t.Errorf("zone %q: peak-occupancy-count %d exceeds observed-count %d; simultaneous occupancy cannot exceed the distinct population it is drawn from",
					z.GetZoneId(), z.GetPeakOccupancyCount(), z.GetObservedCount())
			}
		}
		return
	}
}
