package tests

import (
	"strings"
	"time"

	zoneoccupancyv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/zone_occupancy/v1"
	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
)

// Checks for the openits-zone-occupancy capability.
//
// The capability's live container carries NO `must` constraints between its
// leaves, and that is a deliberate modeling decision: a device may report
// presence without a count, more objects than the region holds, or a breakdown
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
	return zoneOccupancyOfDevice(obs.Device)
}

// zoneOccupancyOfDevice is zoneOccupancyOf for an explicit snapshot, so a
// check can read the post-window state (Observation.DeviceAfter) instead.
func zoneOccupancyOfDevice(dev *yangpkg.Device) *yangpkg.OpenitsPerception_PerceptionSensor_ZoneOccupancy {
	return dev.GetPerceptionSensor().GetZoneOccupancy()
}

// ----- configuration / state coherence -----

// Live occupancy must report on a region the device actually declares. The
// leafref is require-instance false so a just-deleted region is representable
// rather than invalid, but reporting occupancy for a region that was never
// configured is a real defect: a consumer has no sensing-method or
// classifies flag to interpret the reading with.
func TestZoneOccupancy_LiveZonesAreConfigured(t *T, obs *Observation) {
	zoc := zoneOccupancyOf(obs)
	if zoc == nil || zoc.GetZones() == nil {
		return
	}
	cfg := zoc.GetConfiguration()
	for id := range zoc.GetZones().Zone {
		if cfg == nil || cfg.GetZone(id) == nil {
			t.Errorf("live occupancy zone %q has no configured counterpart; a consumer cannot interpret the reading without sensing-method/classifies", id)
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
// pair is precisely what a demand-planning consumer reasons from.
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

// ----- change event -----

// The three transition identities zone-occupancy-changed may carry. The
// notification's `kind` is constrained to the zoc-occupancy-change-event-kind
// sub-base, so this set IS the schema's definition of what counts as a change;
// conformance restates it because proto carries identityrefs as strings and
// the schema's base restriction is not checked by the transport.
const (
	kindZoneOccupied         = "openits-zone-occupancy-types:zoc-zone-occupied"
	kindZoneVacated          = "openits-zone-occupancy-types:zoc-zone-vacated"
	kindZoneOccupancyUpdated = "openits-zone-occupancy-types:zoc-zone-occupancy-updated"
)

// zoneOccupancyChangedEvents returns every decodable zone-occupancy-changed
// event in the observation. Envelope defects (ce-type, payload type) are
// TestZoneOccupancyEvent_ChangedShape's finding alone; this helper skips what
// it cannot decode so one defect is reported once, not under every check that
// needs the events.
func zoneOccupancyChangedEvents(obs *Observation) []*zoneoccupancyv1.ZoneOccupancyChanged {
	var out []*zoneoccupancyv1.ZoneOccupancyChanged
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".zone-occupancy-changed") {
			continue
		}
		if ev, ok := e.Data.(*zoneoccupancyv1.ZoneOccupancyChanged); ok {
			out = append(out, ev)
		}
	}
	return out
}

// newerChange reports whether a supersedes b as the latest transition for a
// region, ordering by (occurred-at, sequence) as event-header specifies. A
// device with one-second clock resolution can emit two transitions in the
// same second; sequence is what separates them.
func newerChange(a, b *zoneoccupancyv1.ZoneOccupancyChanged) bool {
	at, bt := a.GetOccurredAt().AsTime(), b.GetOccurredAt().AsTime()
	if !at.Equal(bt) {
		return at.After(bt)
	}
	return a.GetSequence() > b.GetSequence()
}

// Every change event names a transition, a region, and when. kind must be one
// of the three transition identities (not the interval report's kind, which
// derives from the same capability root but not from the change sub-base) and
// the event-header leaves the schema marks mandatory must be populated.
//
// Absence is not a finding: the event fires on transition, and a region that
// did not change during the window legitimately emits nothing. (The interval
// report is different: it is periodic, so a window longer than its interval
// must contain one.)
func TestZoneOccupancyEvent_ChangedShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasSuffix(e.Subject, ".zone-occupancy-changed") {
			continue
		}
		want := "openits.zone-occupancy.zone-occupancy-changed.v1"
		if e.CEType != want {
			t.Errorf("zone-occupancy-changed ce-type %q, want %q", e.CEType, want)
		}
		ev, ok := e.Data.(*zoneoccupancyv1.ZoneOccupancyChanged)
		if !ok {
			t.Errorf("zone-occupancy-changed Data is %T, want *zoneoccupancyv1.ZoneOccupancyChanged", e.Data)
			continue
		}
		switch ev.GetKind() {
		case kindZoneOccupied, kindZoneVacated, kindZoneOccupancyUpdated:
		default:
			t.Errorf("zone-occupancy-changed kind %q is not a zoc-occupancy-change-event-kind transition (occupied / vacated / updated)", ev.GetKind())
		}
		if ev.GetZoneId() == "" {
			t.Errorf("zone-occupancy-changed has no zone-id; a change event that does not name its region cannot be applied to a twin")
		}
		if ev.GetSourceDeviceId() == "" {
			t.Errorf("zone-occupancy-changed zone %q has no source-device-id (mandatory in event-header)", ev.GetZoneId())
		}
		if ev.GetOccurredAt() == nil {
			t.Errorf("zone-occupancy-changed zone %q has no occurred-at (mandatory in event-header)", ev.GetZoneId())
		}
	}
}

// The kind and the payload must tell the same story. presence is mandatory
// on this notification precisely so this is decidable on a proto3 wire: an
// occupied or updated transition reports presence; a vacated one reports
// absence, has no occupied-since (it resets when the region empties), an
// empty present-class list, and a zero count if it counts at all, exactly as
// the live state reads for a vacant region.
func TestZoneOccupancyEvent_ChangedKindAgreesWithPayload(t *T, obs *Observation) {
	for _, ev := range zoneOccupancyChangedEvents(obs) {
		switch ev.GetKind() {
		case kindZoneOccupied, kindZoneOccupancyUpdated:
			if !ev.GetPresence() {
				t.Errorf("zone %q: kind %s but presence is false; the transition names an occupied region", ev.GetZoneId(), ev.GetKind())
			}
		case kindZoneVacated:
			if ev.GetPresence() {
				t.Errorf("zone %q: kind zoc-zone-vacated but presence is true", ev.GetZoneId())
			}
			if ev.GetOccupiedSince() != nil {
				t.Errorf("zone %q: kind zoc-zone-vacated but occupied-since %q is still carried; it must be cleared when the region empties", ev.GetZoneId(), ev.GetOccupiedSince().AsTime().Format(time.RFC3339))
			}
			if len(ev.GetPresentClass()) > 0 {
				t.Errorf("zone %q: kind zoc-zone-vacated but %d present-class entries are carried; nothing is present in a vacated region", ev.GetZoneId(), len(ev.GetPresentClass()))
			}
			if ev.GetOccupancyCount() != 0 {
				t.Errorf("zone %q: kind zoc-zone-vacated but occupancy-count is %d; a counting device reports 0 for a vacant region", ev.GetZoneId(), ev.GetOccupancyCount())
			}
		}
	}
}

// A change event must report on a region the device actually declares, the
// same rule TestZoneOccupancy_LiveZonesAreConfigured applies to the state tree,
// for the same reason: without the configured sensing-method and classifies
// flag a consumer cannot interpret the payload.
func TestZoneOccupancyEvent_ChangedZoneIsConfigured(t *T, obs *Observation) {
	zoc := zoneOccupancyOf(obs)
	if zoc == nil || zoc.GetConfiguration() == nil {
		return // no configuration exposed; nothing to check against
	}
	for _, ev := range zoneOccupancyChangedEvents(obs) {
		if zoc.GetConfiguration().GetZone(ev.GetZoneId()) == nil {
			t.Errorf("zone-occupancy-changed names zone %q, which the device does not configure; a consumer cannot interpret the reading without sensing-method/classifies", ev.GetZoneId())
		}
	}
}

// The change event carries the same breakdown as the live state and inherits
// its sum rule: where classes are reported, they account for every present
// object, with object-unknown as the catch-all.
func TestZoneOccupancyEvent_ChangedPresentClassSumsToCount(t *T, obs *Observation) {
	for _, ev := range zoneOccupancyChangedEvents(obs) {
		if len(ev.GetPresentClass()) == 0 || ev.GetOccupancyCount() == 0 {
			// No breakdown, or no count to reconcile against. The proto
			// binding carries occupancy-count without presence tracking, so
			// an omitted count (a presence-only sensor that still classifies)
			// reads as 0 here and cannot be told from a reported zero. The
			// live-state twin check skips on nil for the same case; a genuine
			// zero beside a non-empty breakdown is left to the datastore side.
			continue
		}
		var sum uint32
		for _, pc := range ev.GetPresentClass() {
			sum += pc.GetCount()
		}
		if sum != ev.GetOccupancyCount() {
			t.Errorf("zone %q: sum(present-class count)=%d != occupancy-count %d in zone-occupancy-changed; the breakdown must account for every present object",
				ev.GetZoneId(), sum, ev.GetOccupancyCount())
		}
	}
}

// The live container is the digital-twin rollup of the change stream, so the
// two must agree: for each region, the latest transition the reading could
// have reflected (occurred-at <= measured-at, ordered by occurred-at then
// sequence) must report the same presence and count as the reading. A
// mismatch means the device changed state without emitting the transition,
// which is precisely the defect a consumer maintaining a twin cannot detect
// on its own.
//
// The reading is the post-window snapshot. Every event in the window
// postdates the pre-window snapshot, so against that one this check would
// compare nothing on a live device and pass vacuously; it falls back to it
// only when the harness could not take a second read.
func TestZoneOccupancyEvent_ChangedMirrorsLiveState(t *T, obs *Observation) {
	dev := obs.DeviceAfter
	if dev == nil {
		dev = obs.Device
	}
	zoc := zoneOccupancyOfDevice(dev)
	if zoc == nil || zoc.GetZones() == nil {
		return
	}
	events := zoneOccupancyChangedEvents(obs)
	for id, live := range zoc.GetZones().Zone {
		if live.MeasuredAt == nil {
			continue // TestZoneOccupancy_MeasuredAtPresent's finding
		}
		measured, err := time.Parse(time.RFC3339Nano, live.GetMeasuredAt())
		if err != nil {
			continue // a malformed date-and-time is TestYANG_Validate's finding
		}
		var latest *zoneoccupancyv1.ZoneOccupancyChanged
		for _, ev := range events {
			if ev.GetZoneId() != id || ev.GetOccurredAt() == nil || ev.GetOccurredAt().AsTime().After(measured) {
				continue // another region, or newer than the reading
			}
			if latest == nil || newerChange(ev, latest) {
				latest = ev
			}
		}
		if latest == nil {
			continue
		}
		if live.Presence != nil && live.GetPresence() != latest.GetPresence() {
			t.Errorf("zone %q: live presence=%t but the latest zone-occupancy-changed (%s) says presence=%t; the state container is the rollup of the change stream and must agree with it",
				id, live.GetPresence(), latest.GetKind(), latest.GetPresence())
		}
		// An occupied event carrying count 0 is an omitted count on the proto
		// wire (see TestZoneOccupancyEvent_ChangedPresentClassSumsToCount), so
		// only a reported count is compared; a vacated event's 0 is a real 0.
		countOmitted := latest.GetOccupancyCount() == 0 && latest.GetPresence()
		if live.OccupancyCount != nil && !countOmitted && uint32(live.GetOccupancyCount()) != latest.GetOccupancyCount() {
			t.Errorf("zone %q: live occupancy-count=%d but the latest zone-occupancy-changed (%s) says %d; a count change is a transition and must have been emitted",
				id, live.GetOccupancyCount(), latest.GetKind(), latest.GetOccupancyCount())
		}
	}
}
