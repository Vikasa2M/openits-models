package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

// CeType is one derived CloudEvents ce-type: the notification whose JSON
// Schema is that ce-type's payload, plus the service/event dimensions the
// AsyncAPI catalog (Task 2 of the P2b-2 plan) groups by.
type CeType struct {
	Type               string // e.g. "openits.dms.message-activation-failed.v1"
	Service            string // ce-slug, e.g. "dms", "ramp-metering"
	Event              string // notification name, e.g. "message-activation-failed"
	SchemaModule       string // YANG module the notification is defined in
	SchemaNotification string // notification name within SchemaModule (== Event)
}

// ceCandidate is a CeType plus the dedup signal (Bug 2 fix) that doesn't
// belong on the public CeType type: whether the source notification is
// `status deprecated`. BuildCatalog collects one ceCandidate per
// (notification, service) derivation — including duplicates where more
// than one notification derives the same Type — then dedupeCandidates
// reduces that to the one CeType per Type that AsyncAPI actually wants.
type ceCandidate struct {
	CeType
	deprecated bool
}

// serviceInfo pairs a service's ce-type slug with the identity name of its
// "<service>-event-kind" root in the event-kind taxonomy (yang/openits-types.yang
// is the root of the whole tree; every service module derives its own root
// from openits-types:device-event-kind). Root name and slug are equal for
// every service except ramp-metering (root ramp-meter-event-kind, slug
// ramp-metering) and signal-control (root signal-control-event-kind, which
// happens to equal the slug but is listed explicitly for clarity).
type serviceInfo struct {
	slug string
	root string
}

// services is the explicit module-prefix -> service-event-kind-root -> ce-slug
// map from the P2b-2 plan's Global Constraints. It is the authoritative list
// of services a common notification can fan out to.
var services = []serviceInfo{
	{slug: "dms", root: "dms-event-kind"},
	{slug: "ess", root: "ess-event-kind"},
	{slug: "rsu", root: "rsu-event-kind"},
	{slug: "perception", root: "perception-event-kind"},
	{slug: "traffic-sensor", root: "traffic-sensor-event-kind"},
	{slug: "signal-control", root: "signal-control-event-kind"},
	{slug: "ramp-metering", root: "ramp-meter-event-kind"},
	{slug: "reversible-lane", root: "reversible-lane-event-kind"},
	{slug: "cctv", root: "cctv-event-kind"},
}

// BuildCatalog derives the full ce-type catalog from the openits YANG
// event-kind taxonomy.
//
// Two derivation rules (see the P2b-2 plan, Task 1):
//
//   - Per-service notification module: ANY notification-bearing module
//     that is not a common cross-service module (openits-common-*-events)
//     is a per-service module — not just the original
//     openits-<svc>-notifications modules, but also a hyphenated
//     multi-word service module like signal-control's
//     openits-signal-control-events. Every notification in it maps 1:1 to
//     openits.<svc-slug>.<notification>.v1, where <svc-slug> is resolved
//     by the longest-matching "openits-<slug>-" prefix in the service map
//     (routeServiceSlug) — regardless of the notification's YANG `status`.
//     Any `status deprecated` notification is still a live ce-type with a
//     real schema home and is intentionally included: the catalog is a
//     status-agnostic superset used to drive later reconciliation, not a
//     "current only" view. (No deprecated per-service notifications remain
//     today — the fault/mode duplicates were consolidated
//     onto the common notifications — but the status-agnostic rule stands
//     so a future deprecation does not silently drop from the catalog.) A
//     notification-bearing module that matches no service prefix is a
//     taxonomy regression and is reported as an error rather than
//     silently dropped.
//
//   - Common notification module (openits-common-*-events): each
//     notification's `kind` leaf is an identityref whose base (B) names a
//     behavioral class (fault-event-kind / mode-event-kind /
//     comm-health-event-kind). For every service S in the service map,
//     if the identity graph has at least one identity that is transitively
//     derived from both B and S's "<S>-event-kind" root, the notification
//     fans out to openits.<S-slug>.<notification>.v1.
//
// The identity-graph walk itself needs no bespoke traversal: goyang's
// Modules.Process (called by LoadModules) already resolves, for every
// *yang.Identity, the full transitive set of identities derived from it
// into that Identity's Values field (see goyang's identity.go
// resolveIdentities/addChildren). So "B derives X" and "S-root derives X"
// both reduce to "X is in Values", and "does some identity satisfy both"
// reduces to a set-intersection test over two Values slices — computed
// once per (notification, service) pair, not via a hand-rolled recursive
// base-walk.
//
// A ce-type Type can legitimately be produced by more than one source —
// e.g. a deprecated gen-1 per-service notification (signal-control's
// legacy "fault-raised") and the common notification that superseded it
// (openits-common-fault-events:fault-raised, fanned to signal-control) both
// derive openits.signal-control.fault-raised.v1. AsyncAPI needs exactly one
// message per ce-type, so the result is deduped by Type before it's
// returned: see dedupeCandidates.
func BuildCatalog(ms *yang.Modules, mods []*yang.Entry) ([]CeType, error) {
	_ = ms // identityref bases resolve via Entry.Type.IdentityBase, populated during ms.Process() inside LoadModules.

	roots, err := serviceRootIdentities(mods)
	if err != nil {
		return nil, err
	}

	var candidates []ceCandidate
	for _, m := range mods {
		notifs := notificationEntries(m)
		if len(notifs) == 0 {
			continue
		}

		if isCommonEventsModule(m.Name) {
			for _, notif := range notifs {
				base, err := notificationKindBase(notif)
				if err != nil {
					return nil, fmt.Errorf("%s:%s: %w", m.Name, notif.Name, err)
				}
				if base == nil {
					continue
				}
				for _, svc := range services {
					svcRoot := roots[svc.root]
					if identitySetsIntersect(base.Values, svcRoot.Values) {
						candidates = append(candidates, ceCandidate{
							CeType: CeType{
								Type:               ceTypeName(svc.slug, notif.Name),
								Service:            svc.slug,
								Event:              notif.Name,
								SchemaModule:       m.Name,
								SchemaNotification: notif.Name,
							},
							deprecated: notificationDeprecated(notif),
						})
					}
				}
			}
			continue
		}

		// Per-service (Bug 1 fix): every notification-bearing module that
		// isn't common is routed to exactly one service by the
		// longest-matching module-name prefix in the service map. This
		// covers both the original openits-<svc>-notifications modules
		// and sub-domain families like signal-control's
		// openits-signal-control-<sub>-events modules.
		slug, ok := routeServiceSlug(m.Name)
		if !ok {
			return nil, fmt.Errorf("module %s has notifications but does not match any known service prefix in the service map", m.Name)
		}
		for _, notif := range notifs {
			candidates = append(candidates, ceCandidate{
				CeType: CeType{
					Type:               ceTypeName(slug, notif.Name),
					Service:            slug,
					Event:              notif.Name,
					SchemaModule:       m.Name,
					SchemaNotification: notif.Name,
				},
				deprecated: notificationDeprecated(notif),
			})
		}
	}

	return dedupeCandidates(candidates), nil
}

// ceTypeName formats the openits CloudEvents ce-type string.
func ceTypeName(slug, notif string) string {
	return "openits." + slug + "." + notif + ".v1"
}

// routeServiceSlug returns the service slug that owns a per-service
// (non-common) notification-bearing module, resolved as the
// longest-matching "openits-<slug>-" prefix of moduleName against the
// service map. Longest match matters because a module name could in
// principle stack a sub-domain suffix onto a service prefix that is
// itself hyphenated (e.g. a hypothetical
// "openits-signal-control-<sub>-events" must resolve to the
// "signal-control" slug, not fail to match, and no other service slug in
// the map is a prefix of "signal-control" today — the longest-match rule
// is future-proofing against that becoming possible). No such stacked
// module exists today: signal-control's 11 former per-domain event
// modules were folded into the single openits-signal-control-events by
// cut 3b, but the rule still guards against a future split.
//
// Examples: "openits-dms-events" -> "dms";
// "openits-ramp-metering-events" -> "ramp-metering";
// "openits-signal-control-events" -> "signal-control".
func routeServiceSlug(moduleName string) (string, bool) {
	best := ""
	for _, svc := range services {
		prefix := "openits-" + svc.slug + "-"
		if strings.HasPrefix(moduleName, prefix) && len(svc.slug) > len(best) {
			best = svc.slug
		}
	}
	return best, best != ""
}

// isCommonEventsModule reports whether name is a common (cross-service)
// notification module: "openits-common-*-events", e.g.
// "openits-common-fault-events".
func isCommonEventsModule(name string) bool {
	return strings.HasPrefix(name, "openits-common-") && strings.HasSuffix(name, "-events")
}

// notificationEntries returns every `notification` child of module entry m,
// sorted by name for deterministic iteration.
func notificationEntries(m *yang.Entry) []*yang.Entry {
	var out []*yang.Entry
	for _, c := range m.Dir {
		if c.Kind == yang.NotificationEntry {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// notificationKindBase returns the resolved identityref base identity (B)
// of a common notification's `kind` leaf. Returns (nil, nil) if the
// notification has no `kind` leaf (none exist today in the common-events
// modules, but a future notification without one is not itself an error —
// it simply doesn't fan out via the taxonomy). Returns an error if `kind`
// exists but isn't an identityref, since that would signal a real schema
// mismatch against the derivation rule.
func notificationKindBase(notif *yang.Entry) (*yang.Identity, error) {
	kind, ok := notif.Dir["kind"]
	if !ok {
		return nil, nil
	}
	if kind.Type == nil || kind.Type.IdentityBase == nil {
		return nil, fmt.Errorf("kind leaf is not an identityref")
	}
	return kind.Type.IdentityBase, nil
}

// notificationDeprecated reports whether notif carries `status deprecated`
// in YANG source. Used only to rank dedup priority (Bug 2 fix, see
// dedupeCandidates): between two notifications that derive the same
// ce-type Type, the non-deprecated one wins.
//
// *yang.Entry doesn't carry status directly, but for a notification
// entry (Kind == yang.NotificationEntry) its Node field is always the
// originating *yang.Notification (see goyang's entry.go), which does —
// as a *yang.Value whose Name is the status keyword ("current",
// "deprecated", or "obsolete"; YANG defaults to "current" when the
// `status` statement is omitted, i.e. Status == nil).
func notificationDeprecated(notif *yang.Entry) bool {
	n, ok := notif.Node.(*yang.Notification)
	if !ok || n.Status == nil {
		return false
	}
	return n.Status.Name == "deprecated"
}

// serviceRootIdentities resolves every service's "<service>-event-kind"
// root identity (from the services map) to its *yang.Identity, by scanning
// each module Entry's directly-declared Identities. Returns an error if a
// root is missing (taxonomy regression) or defined more than once (naming
// collision) so BuildCatalog fails loudly instead of silently picking one.
func serviceRootIdentities(mods []*yang.Entry) (map[string]*yang.Identity, error) {
	want := make(map[string]bool, len(services))
	for _, svc := range services {
		want[svc.root] = true
	}

	out := make(map[string]*yang.Identity, len(services))
	for _, m := range mods {
		for _, id := range m.Identities {
			if !want[id.Name] {
				continue
			}
			if prev, dup := out[id.Name]; dup {
				return nil, fmt.Errorf("service-event-kind root identity %q defined in both %s and %s", id.Name, yang.RootNode(prev).GetPrefix(), m.Name)
			}
			out[id.Name] = id
		}
	}
	for _, svc := range services {
		if _, ok := out[svc.root]; !ok {
			return nil, fmt.Errorf("service-event-kind root identity %q not found in loaded modules", svc.root)
		}
	}
	return out, nil
}

// identitySetsIntersect reports whether any identity appears in both a and
// b. Used to test "some identity is derived from both B and S-root",
// i.e. B and S-root have a common descendant.
func identitySetsIntersect(a, b []*yang.Identity) bool {
	set := make(map[*yang.Identity]bool, len(a))
	for _, id := range a {
		set[id] = true
	}
	for _, id := range b {
		if set[id] {
			return true
		}
	}
	return false
}

// dedupeCandidates collapses candidates down to one CeType per Type
// (Bug 2 fix) and returns them sorted by Type. AsyncAPI needs exactly one
// message per ce-type, but the derivation rules can legitimately produce
// the same Type from more than one notification — most commonly a
// deprecated gen-1 per-service notification and the common notification
// that superseded it (e.g. signal-control's legacy, `status deprecated`
// "fault-raised" in openits-signal-control-events vs.
// openits-common-fault-events:fault-raised fanned to signal-control), but
// also a deprecated legacy notification vs. its non-deprecated successor
// module (e.g. signal-control's legacy "preemption-activated" vs.
// openits-signal-control-events:preemption-activated).
//
// Among candidates sharing a Type, dedupeCandidatePriority ranks:
// non-deprecated beats deprecated; among non-deprecated (or among
// deprecated), a common-events source beats a per-service one — common
// modules are the canonical cross-service source once a notification has
// been migrated there. Remaining ties (identical rank) are broken by
// SchemaModule so output is byte-stable across runs.
func dedupeCandidates(candidates []ceCandidate) []CeType {
	best := make(map[string]ceCandidate, len(candidates))
	for _, c := range candidates {
		prev, ok := best[c.Type]
		if !ok || dedupeCandidateWins(c, prev) {
			best[c.Type] = c
		}
	}

	cat := make([]CeType, 0, len(best))
	for _, c := range best {
		cat = append(cat, c.CeType)
	}
	sort.Slice(cat, func(i, j int) bool { return cat[i].Type < cat[j].Type })
	return cat
}

// dedupeCandidateWins reports whether candidate a should replace candidate
// b as the surviving source for their shared Type. See dedupeCandidates
// for the priority rules.
func dedupeCandidateWins(a, b ceCandidate) bool {
	ra, rb := dedupeCandidatePriority(a), dedupeCandidatePriority(b)
	if ra != rb {
		return ra > rb
	}
	return a.SchemaModule < b.SchemaModule
}

// dedupeCandidatePriority scores a candidate for dedup ranking: higher
// wins. Non-deprecated outranks deprecated; a common-events source
// outranks a per-service source at the same deprecation tier.
func dedupeCandidatePriority(c ceCandidate) int {
	score := 0
	if !c.deprecated {
		score += 2
	}
	if isCommonEventsModule(c.SchemaModule) {
		score++
	}
	return score
}
