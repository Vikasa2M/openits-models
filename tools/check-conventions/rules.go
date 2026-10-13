package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

// Violation is one place a module breaks a house-contract convention
// (docs/reference/yang-reference-conventions.md, "The pre-1.0 house
// contract", rule 6).
type Violation struct {
	Rule   string // rule id: before-after, unknown-zero, units, counters, kind-base, references
	Module string // module name
	Path   string // schema path, "typedef <name>", or "line <n>"
	Detail string // what is wrong, for the person fixing it
}

func (v Violation) key() string { return v.Rule + "\x00" + v.Module + "\x00" + v.Path }

// checkSchema runs every rule that works on the parsed schema.
func checkSchema(mods []*yang.Entry) []Violation {
	var out []Violation
	for _, m := range mods {
		out = append(out, ruleBeforeAfter(m)...)
		out = append(out, ruleUnknownZero(m)...)
		out = append(out, ruleUnits(m)...)
		out = append(out, ruleCounters(m)...)
		out = append(out, ruleKindBase(m)...)
	}
	return out
}

// walk calls fn for e and every entry below it, depth first, children in
// name order so output is deterministic.
func walk(e *yang.Entry, fn func(*yang.Entry)) {
	fn(e)
	names := make([]string, 0, len(e.Dir))
	for n := range e.Dir {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		walk(e.Dir[n], fn)
	}
}

func inNotification(e *yang.Entry) bool {
	for p := e; p != nil; p = p.Parent {
		if p.Kind == yang.NotificationEntry {
			return true
		}
	}
	return false
}

func isLeaf(e *yang.Entry) bool { return e.Kind == yang.LeafEntry }

// lastToken is the part of a hyphenated name after the final hyphen, or
// "" for a name with no hyphen.
func lastToken(name string) string {
	i := strings.LastIndex(name, "-")
	if i < 0 {
		return ""
	}
	return name[i+1:]
}

var beforeAfterBad = []string{"prior", "new", "old", "current"}

// ruleBeforeAfter: in notifications, the earlier value of a transition is
// previous-<x> and the later value is <x>.
func ruleBeforeAfter(m *yang.Entry) []Violation {
	var out []Violation
	walk(m, func(e *yang.Entry) {
		if !isLeaf(e) || !inNotification(e) {
			return
		}
		for _, p := range beforeAfterBad {
			if e.Name == p || strings.HasPrefix(e.Name, p+"-") {
				out = append(out, Violation{"before-after", m.Name, e.Path(),
					fmt.Sprintf("%q: name the earlier value previous-<x> and the later value <x>", e.Name)})
				return
			}
		}
		if x, ok := strings.CutPrefix(e.Name, "previous-"); ok {
			if _, sib := e.Parent.Dir[x]; !sib {
				out = append(out, Violation{"before-after", m.Name, e.Path(),
					fmt.Sprintf("%q has no sibling %q carrying the later value", e.Name, x)})
			}
		}
	})
	return out
}

// ruleUnknownZero: an enumeration read from a device (config false, or in
// a notification) has unknown at value 0. A typedef is reported once per
// using module.
func ruleUnknownZero(m *yang.Entry) []Violation {
	var out []Violation
	seen := map[string]bool{}
	walk(m, func(e *yang.Entry) {
		if !isLeaf(e) || e.Type == nil || e.Type.Kind != yang.Yenum || e.Type.Enum == nil {
			return
		}
		if !e.ReadOnly() && !inNotification(e) {
			return
		}
		where := e.Path()
		if e.Type.Name != "enumeration" {
			where = "typedef " + e.Type.Name
		}
		if seen[where] {
			return
		}
		seen[where] = true
		v, ok := e.Type.Enum.NameMap()["unknown"]
		switch {
		case !ok:
			out = append(out, Violation{"unknown-zero", m.Name, where,
				"enumeration read from a device has no unknown member; add unknown with value 0"})
		case v != 0:
			out = append(out, Violation{"unknown-zero", m.Name, where,
				fmt.Sprintf("unknown has value %d; it must be 0", v)})
		}
	})
	return out
}

// shortUnits are the unit suffixes a leaf name may end in.
var shortUnits = map[string]bool{
	"s": true, "ms": true, "us": true, "min": true, "h": true,
	"pct": true, "deg": true, "kmh": true, "vph": true,
	"c": true, "m": true, "mm": true, "cm": true, "km": true,
	"v": true, "a": true, "w": true, "hz": true,
	"db": true, "dbm": true, "lux": true, "kbps": true, "bytes": true, "wm2": true,
}

// longUnits are spelled-out unit suffixes the convention replaces.
var longUnits = map[string]string{
	"seconds": "s", "milliseconds": "ms", "microseconds": "us", "minutes": "min",
	"hours": "h", "percent": "pct", "degrees": "deg", "meters": "m", "celsius": "c",
}

func unitsOf(e *yang.Entry) string {
	// goyang v1.6.3 does not copy a leaf's units statement onto Entry.Units;
	// read it from the source node.
	if l, ok := e.Node.(*yang.Leaf); ok && l.Units != nil && l.Units.Name != "" {
		return l.Units.Name
	}
	if e.Units != "" {
		return e.Units
	}
	if e.Type != nil {
		return e.Type.Units
	}
	return ""
}

// ruleUnits: a leaf with a unit carries both a short unit suffix and a
// units statement.
func ruleUnits(m *yang.Entry) []Violation {
	var out []Violation
	walk(m, func(e *yang.Entry) {
		if !isLeaf(e) {
			return
		}
		suf := lastToken(e.Name)
		units := unitsOf(e)
		switch {
		case longUnits[suf] != "":
			out = append(out, Violation{"units", m.Name, e.Path(),
				fmt.Sprintf("spelled-out unit suffix -%s; use -%s", suf, longUnits[suf])})
		case units != "" && !shortUnits[suf]:
			out = append(out, Violation{"units", m.Name, e.Path(),
				fmt.Sprintf("has units %q but no unit suffix in its name", units)})
		case shortUnits[suf] && units == "":
			out = append(out, Violation{"units", m.Name, e.Path(),
				fmt.Sprintf("name ends in unit suffix -%s but there is no units statement", suf)})
		}
	})
	return out
}

// ruleCounters: a monotonic counter read from a device is named *-total or
// *-errors and typed counter64.
func ruleCounters(m *yang.Entry) []Violation {
	var out []Violation
	walk(m, func(e *yang.Entry) {
		if !isLeaf(e) || !e.ReadOnly() || e.Type == nil {
			return
		}
		if !strings.HasSuffix(e.Name, "-total") && !strings.HasSuffix(e.Name, "-errors") {
			return
		}
		if e.Type.Name != "counter64" {
			out = append(out, Violation{"counters", m.Name, e.Path(),
				fmt.Sprintf("monotonic counter typed %s; use yang:counter64", e.Type.Name)})
		}
	})
	return out
}

// ruleKindBase: a notification's kind is constrained to its own transition
// or report sub-base, never a service root (an identity based directly on
// device-event-kind).
func ruleKindBase(m *yang.Entry) []Violation {
	var out []Violation
	for _, n := range m.Dir {
		if n.Kind != yang.NotificationEntry {
			continue
		}
		kind, ok := n.Dir["kind"]
		if !ok || kind.Type == nil || kind.Type.IdentityBase == nil {
			continue
		}
		for _, b := range kind.Type.IdentityBase.Base {
			name := b.Name
			if i := strings.LastIndex(name, ":"); i >= 0 {
				name = name[i+1:]
			}
			if name == "device-event-kind" {
				out = append(out, Violation{"kind-base", m.Name, kind.Path(),
					fmt.Sprintf("kind is typed to the service root %s; constrain it to this notification's own sub-base",
						kind.Type.IdentityBase.Name)})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
