package main

import "github.com/openconfig/goyang/pkg/yang"

// SharedGroupings walks every module's Entry tree and finds YANG groupings
// that are BOTH:
//  1. container-shaped — the grouping's body is a single container (e.g.
//     `grouping wire-source { container source {...} } `), and
//  2. `uses`-ed at 2 or more distinct sites across the module set.
//
// Only such groupings are "shared": Task 6's emitter contract is that a
// container-shaped grouping gets emitted once as a top-level message and
// referenced by every user, instead of being inlined per use. A *flat*
// grouping (body is bare leaves, e.g. `geo-location`) is never shared,
// regardless of how many sites use it: RFC 7950 `uses` semantics splice the
// grouping's members directly into the parent's data tree, so the proto
// tree emitted from any one usage site must match — the leaves become
// direct fields of every user, not a nested/shared message. Only a
// container-shaped grouping's single nested container can be pulled out as
// a shared message, because that container already appears as its own
// nested message under every user regardless of whether it's shared.
//
// The returned map is keyed by the grouping's bare (unprefixed) name — e.g.
// "wire-source" — and maps to the proto message name it should be emitted
// as — e.g. "WireSource". Flat groupings, and groupings used at only one
// site, are left for the normal inlining path and are absent from the map.
func SharedGroupings(mods []*yang.Entry) map[string]string {
	counts := map[string]int{}
	containerShaped := map[string]bool{}
	for _, m := range mods {
		countGroupingUsages(m, counts, containerShaped)
	}
	shared := map[string]string{}
	for name, n := range counts {
		if n >= 2 && containerShaped[name] {
			shared[name] = ProtoName(name)
		}
	}
	return shared
}

// countGroupingUsages recursively visits e's descendants, incrementing
// counts[grouping] once per `uses` SITE — not once per member the
// grouping's body declares. A `uses` site is identified by grouping e's
// direct children by the grouping they trace back to (via groupingOf): all
// members spliced in by one `uses` statement land together in the using
// entry's Dir, so a single grouping-name group within one e.Dir pass is
// exactly one site, regardless of whether the grouping's body is one
// container (1 member) or several bare leaves (N members). Without this
// grouping step, a flat grouping with N leaves would be counted N times per
// site — which is the bug this function fixes: `system-info` and
// `comm-link-state`, each `uses`-d at exactly one real site, were
// previously counted ~9x (once per leaf) and so incorrectly cleared the
// `>= 2` shared threshold.
//
// containerShaped[name] records whether that site's members are shaped
// like a container-shaped grouping (exactly one member, itself a
// container) — see SharedGroupings for why only that shape can be shared.
func countGroupingUsages(e *yang.Entry, counts map[string]int, containerShaped map[string]bool) {
	siteMembers := map[string][]*yang.Entry{}
	for _, c := range e.Dir {
		if name, ok := groupingOf(c); ok {
			siteMembers[name] = append(siteMembers[name], c)
		}
	}
	for name, members := range siteMembers {
		counts[name]++
		containerShaped[name] = len(members) == 1 && members[0].IsContainer()
	}
	for _, c := range e.Dir {
		countGroupingUsages(c, counts, containerShaped)
	}
}

// groupingOf reports whether entry c is the direct top of a `uses`
// expansion — i.e. c was declared immediately inside a grouping body, so c
// is one of the nodes a `uses` statement pulled in. goyang expands `uses` by
// splicing the grouping's data-definition nodes directly into the using
// entry's Dir; c's underlying Node still points back to its original
// position in the grouping's statement tree, so c.Node.ParentNode() is the
// *yang.Grouping itself. Descendants nested further inside c (e.g. leaves
// inside a `uses`-d container) have c's container/list/choice node as their
// parent, not the grouping, so they don't also match — each usage site is
// counted exactly once regardless of how many fields the grouping declares.
func groupingOf(c *yang.Entry) (string, bool) {
	if c.Node == nil {
		return "", false
	}
	grp, ok := c.Node.ParentNode().(*yang.Grouping)
	if !ok {
		return "", false
	}
	return grp.NName(), true
}
