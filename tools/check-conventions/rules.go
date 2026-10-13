package main

import (
	"fmt"
	"slices"
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
	seenEnums := map[string]bool{} // typedef enums are reported once, globally
	for _, m := range mods {
		out = append(out, ruleBeforeAfter(m)...)
		out = append(out, ruleUnknownZero(m, seenEnums)...)
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

// moduleName is the module a module or submodule belongs to.
func moduleName(m *yang.Module) string {
	if m.BelongsTo != nil {
		return m.BelongsTo.Name
	}
	return m.Name
}

// typedefModule returns the module that defines the typedef a leaf's type
// names, resolving a prefixed type name through the leaf's imports.
func typedefModule(e *yang.Entry) (module, typedef string) {
	typedef = e.Type.Name
	l, ok := e.Node.(*yang.Leaf)
	if !ok || l.Type == nil {
		return moduleName(yang.RootNode(e.Node)), typedef
	}
	if i := strings.Index(l.Type.Name, ":"); i >= 0 {
		if mod := yang.FindModuleByPrefix(e.Node, l.Type.Name[:i]); mod != nil {
			return moduleName(mod), l.Type.Name[i+1:]
		}
	}
	return moduleName(yang.RootNode(e.Node)), typedef
}

// ruleUnknownZero: an enumeration read from a device (config false, or in
// a notification) has unknown at value 0. A typedef is reported once, for
// the module that defines it (seen is shared across modules).
func ruleUnknownZero(m *yang.Entry, seen map[string]bool) []Violation {
	var out []Violation
	walk(m, func(e *yang.Entry) {
		if !isLeaf(e) || e.Type == nil || e.Type.Kind != yang.Yenum || e.Type.Enum == nil {
			return
		}
		if !e.ReadOnly() && !inNotification(e) {
			return
		}
		module, where := m.Name, e.Path()
		if e.Type.Name != "enumeration" {
			var td string
			module, td = typedefModule(e)
			where = "typedef " + td
		}
		k := module + "\x00" + where
		if seen[k] {
			return
		}
		seen[k] = true
		v, ok := e.Type.Enum.NameMap()["unknown"]
		switch {
		case !ok:
			out = append(out, Violation{"unknown-zero", module, where,
				"enumeration read from a device has no unknown member; add unknown with value 0"})
		case v != 0:
			out = append(out, Violation{"unknown-zero", module, where,
				fmt.Sprintf("unknown has value %d; it must be 0", v)})
		}
	})
	return out
}

// shortUnits maps each short unit suffix to the units strings a leaf with
// that suffix may carry.
var shortUnits = map[string][]string{
	"s": {"seconds", "s"}, "ms": {"milliseconds"}, "us": {"microseconds"},
	"min": {"minutes"}, "h": {"hours"}, "pct": {"percent"}, "deg": {"degrees"},
	"kmh": {"km/h"}, "vph": {"vehicles/hour", "vehicles-per-hour"},
	"c": {"celsius"}, "m": {"meters"}, "mm": {"millimeters"}, "cm": {"centimeters"},
	"km": {"kilometers"}, "v": {"volts"}, "a": {"amperes"}, "w": {"watts"},
	"hz": {"hertz"}, "db": {"dB"}, "dbm": {"dBm"}, "lux": {"lux"},
	"kbps": {"kilobits/second"}, "bytes": {"bytes"},
	"wm2": {"watts-per-square-meter"}, "mi": {"miles"},
	"ppm": {"parts-per-million"}, "hpa": {"hectopascals"}, "dbi": {"dBi"},
}

// ambiguousSuffixes are single letters that also end ordinary A/B labels
// (direction-a, channel-w), so they count as unit suffixes only when the
// leaf has a units statement.
var ambiguousSuffixes = map[string]bool{"a": true, "v": true, "w": true}

// countUnits are units that count things rather than measure a quantity;
// such a leaf needs no unit suffix in its name.
var countUnits = map[string]bool{
	"vehicles": true, "objects": true, "veh/km": true,
	"counts/minute": true, "points-per-second": true, "bytes": true,
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

// ruleUnits: a leaf with a unit carries a short unit suffix and a matching
// units statement.
func ruleUnits(m *yang.Entry) []Violation {
	var out []Violation
	walk(m, func(e *yang.Entry) {
		if !isLeaf(e) {
			return
		}
		suf := lastToken(e.Name)
		units := unitsOf(e)
		accepted, isSuffix := shortUnits[suf]
		switch {
		case longUnits[suf] != "":
			out = append(out, Violation{"units", m.Name, e.Path(),
				fmt.Sprintf("spelled-out unit suffix -%s; use -%s", suf, longUnits[suf])})
		case isSuffix && units == "":
			if !ambiguousSuffixes[suf] {
				out = append(out, Violation{"units", m.Name, e.Path(),
					fmt.Sprintf("name ends in unit suffix -%s but there is no units statement", suf)})
			}
		case isSuffix:
			if !slices.Contains(accepted, units) {
				out = append(out, Violation{"units", m.Name, e.Path(),
					fmt.Sprintf("suffix -%s but units %q; expected one of %s", suf, units, strings.Join(accepted, ", "))})
			}
		case units != "" && !countUnits[units]:
			out = append(out, Violation{"units", m.Name, e.Path(),
				fmt.Sprintf("has units %q but no unit suffix in its name", units)})
		}
	})
	return out
}

// ruleCounters: a monotonic counter read from a device is named *-total or
// *-errors and typed counter64. Notification leaves are skipped because
// notifications carry no counters.
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
// or report sub-base, never a service root. A service root is an identity
// based directly on device-event-kind and defined outside openits-types;
// the event-kind identities openits-types defines (fault-event-kind,
// mode-event-kind, ...) are cross-service bases the shared events modules
// are required to use. Only module-level notifications are checked.
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
		base := kind.Type.IdentityBase
		if moduleName(yang.RootNode(base)) == "openits-types" {
			continue
		}
		for _, b := range base.Base {
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
