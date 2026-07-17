package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

// ProtoFile accumulates a file's imports and message bodies.
type ProtoFile struct {
	Imports map[string]bool
	Body    strings.Builder

	// EmittedShared tracks, by proto message name, which shared-grouping
	// messages have already been written to (some ancestor's) Body — each
	// is written at most once no matter how many fields/messages
	// reference it. EmitMessage recurses by handing nested
	// containers/cases a fresh child ProtoFile (see child()) whose Body is
	// scoped to that recursion but whose Emitted* maps are the *same* map
	// values as the caller's, so the "emitted once" guarantee holds across
	// the whole recursion tree, not just one call.
	EmittedShared map[string]bool

	// EmittedEnums tracks already-emitted enums by their base proto name
	// (e.g. "Prior", derived from the YANG leaf/typedef identifier before
	// any module-qualification — see emitEnum): for each base name, the
	// set of distinct value-set signatures emitted under it, and the
	// final proto name each was given. A YANG identifier is not a
	// reliable global key on its own — two unrelated modules can each
	// declare an inline `enumeration` on a same-named leaf (e.g. `prior`)
	// with disjoint value sets — so a bare-name registry would let the
	// second overwrite/collide with the first once both land in a shared
	// file. Keying by (base name, content signature) keeps the common
	// case (identical enum reused across messages) deduped under the bare
	// name exactly as before, while a genuinely different enum that
	// happens to share the base name gets its own module-qualified name
	// so both emit distinctly.
	EmittedEnums map[string]map[string]string

	// ClaimedNames tracks, across every output ProtoFile that shares this
	// file's go_package (not the whole run — see Generate's
	// claimedNamesFor), every top-level enum name already declared by any
	// of them. Every generated .proto file's `option go_package` is now
	// derived per-service (see goPackageFor in main.go), so protoc-gen-go
	// only flattens the files that share ONE service's go_package into one
	// Go package — e.g. a service's events.proto and state.proto — not the
	// whole corpus: a bare enum name (e.g. "Severity") that is perfectly
	// fine to reuse *within* one .proto file (see EmittedEnums) becomes a Go
	// symbol collision the moment a *different* output file sharing that
	// go_package also declares it, but two different services declaring the
	// same bare enum name is legal (different Go packages). ClaimedNames is
	// the cross-file counterpart to EmittedEnums's within-file dedup:
	// emitEnum forces module qualification (see enumQualifiedSource)
	// whenever the bare name is already claimed by another file in the same
	// go_package, not only when this file has already used it. nil (the
	// default, used by every pre-Task-8 caller/test) disables the check and
	// preserves prior behavior.
	ClaimedNames map[string]bool

	// Collisions is the set of bare nested-message names (ProtoName of a
	// list/container that is not "config"/"state") that appear more than once
	// within the top-level message tree currently being emitted — precomputed
	// by collisionSet and set by the caller (Generate, or a test) before the
	// top-level EmitMessage call, then propagated unchanged down the whole
	// recursion via child(). nestedMessageName parent-qualifies EVERY instance
	// of a name in this set (symmetric, order-independent: neither sibling
	// keeps the bare name), and leaves non-colliding names bare. Because the
	// set is computed once over the whole tree, two same-named lists under
	// different parents (e.g. pavement/sensor and diagnostics/sensor) both see
	// the collision and both qualify. A nil set (the default for callers/tests
	// with no possible collision) leaves every non-config/state name bare,
	// preserving byte-identical output for the collision-free corpus.
	Collisions map[string]bool

	// LockPrefix is the proto package (e.g. "openits.ramp_metering.v1") every
	// message emitted into this ProtoFile is keyed under in the field-number
	// lock — the lock key is LockPrefix + "." + msgName (see EmitMessage's
	// lockKey). FieldLock keys by message name alone, so without this two
	// different services that each declare a same-named container (e.g. a
	// control/config that both render as "ControlConfig") would share ONE lock
	// bucket and get their semantically-distinct fields co-mingled into one tag
	// space — a wire-safety hazard invisible in the lock, since it tracks only
	// the shared bare name. Scoping the key by the service's proto package gives
	// each service's messages their own independent tag space (two services
	// declaring "ControlConfig" is legal — they're different proto packages, and
	// their generated Go types already live in different Go packages via
	// goPackageFor). Propagated unchanged down the whole EmitMessage recursion
	// via child(). Empty (the default, used by hermetic fixtures/tests that emit
	// a single tree in isolation) keys by the bare message name, preserving the
	// original single-namespace behavior.
	LockPrefix string

	// Types, if non-nil, is the target file/package every shared-grouping
	// message (see emitSharedMessage) is physically written into, instead
	// of this ProtoFile's own Body — set by Generate (Task 7) so a message
	// like WireSource is centralized in one shared `openits.types.v1` file
	// across every service's output rather than living in whichever
	// service file happens to reference it first. When Types.File == this
	// ProtoFile (the types file emitting into itself), the reference is
	// unqualified and no cross-file import is added. nil (the default,
	// used by every pre-Task-7 caller/test) preserves the original
	// behavior: the shared message is written directly into this
	// ProtoFile and referenced by its bare name.
	Types *TypesTarget
}

// TypesTarget names the shared accumulator, proto package, and import path
// EmitMessage's shared-grouping path (see emitSharedMessage) routes a
// shared message's single physical emission into.
type TypesTarget struct {
	File       *ProtoFile
	Package    string
	ImportPath string
}

func (p *ProtoFile) addImport(path string) {
	if p.Imports == nil {
		p.Imports = map[string]bool{}
	}
	p.Imports[path] = true
}

// child returns a new ProtoFile that shares p's Emitted* dedup maps
// (allocating them on p first if necessary) but starts with a fresh
// Body/Imports, for use as the `out` of a recursive EmitMessage call whose
// output gets merged back into p's Body by the caller.
func (p *ProtoFile) child() *ProtoFile {
	if p.EmittedShared == nil {
		p.EmittedShared = map[string]bool{}
	}
	if p.EmittedEnums == nil {
		p.EmittedEnums = map[string]map[string]string{}
	}
	return &ProtoFile{EmittedShared: p.EmittedShared, EmittedEnums: p.EmittedEnums, ClaimedNames: p.ClaimedNames, Collisions: p.Collisions, Types: p.Types, LockPrefix: p.LockPrefix}
}

// sortedChildren returns an entry's data children in deterministic order.
// This IS the canonical order the field-number lock keys off. Order follows
// the original YANG source (declaration order), recovered from the
// underlying statement's substatements via declOrder; children with no
// resolvable source position (e.g. produced by grouping/augment expansion,
// unhandled until a later task) fall back to name order so the result stays
// deterministic either way. The lock guarantees tag numbers never move even
// if order changes later.
func sortedChildren(e *yang.Entry) []*yang.Entry {
	names := make([]string, 0, len(e.Dir))
	for n := range e.Dir {
		names = append(names, n)
	}
	order := declOrder(e)
	sort.Slice(names, func(i, j int) bool {
		oi, oki := order[names[i]]
		oj, okj := order[names[j]]
		if oki && okj {
			return oi < oj
		}
		if oki != okj {
			return oki // known source position sorts before unknown
		}
		return names[i] < names[j]
	})
	out := make([]*yang.Entry, 0, len(names))
	for _, n := range names {
		out = append(out, e.Dir[n])
	}
	return out
}

// declOrder maps each direct child's name to its index among e's underlying
// statement's substatements, i.e. its position in the original YANG source.
// Children without a matching substatement argument are simply absent from
// the map. Only data-definition and grouping statements are indexed; other
// substatements (description, units, config, etc.) are skipped.
func declOrder(e *yang.Entry) map[string]int {
	order := map[string]int{}
	if e == nil || e.Node == nil {
		return order
	}
	st := e.Node.Statement()
	if st == nil {
		return order
	}
	// Only record substatements with data-definition or grouping keywords.
	dataDefKeywords := map[string]bool{
		"leaf":      true,
		"leaf-list": true,
		"container": true,
		"list":      true,
		"choice":    true,
		"case":      true,
		"uses":      true,
		"anydata":   true,
		"anyxml":    true,
		"grouping":  true,
	}
	for i, sub := range st.SubStatements() {
		if dataDefKeywords[sub.Keyword] {
			if _, ok := order[sub.Argument]; !ok {
				order[sub.Argument] = i
			}
		}
	}
	return order
}

// ProtoName converts a kebab/snake-case YANG identifier to UpperCamelCase for
// use as a proto message name.
func ProtoName(y string) string {
	parts := strings.FieldsFunc(y, func(r rune) bool { return r == '-' || r == '_' })
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

// fieldName converts a kebab-case YANG identifier to snake_case for use as a
// proto field name.
func fieldName(y string) string { return strings.ReplaceAll(y, "-", "_") }

// lockKey returns the field-number-lock key for a message named msgName emitted
// into a ProtoFile whose proto package is prefix. The key is package-qualified
// (e.g. "openits.ramp_metering.v1.ControlConfig") so two services that each
// declare a same-named container get independent tag spaces rather than sharing
// one bare-name bucket — see ProtoFile.LockPrefix. An empty prefix falls back to
// the bare message name for hermetic callers that emit a single tree in
// isolation (fixtures/tests).
func lockKey(prefix, msgName string) string {
	if prefix == "" {
		return msgName
	}
	return prefix + "." + msgName
}

// configStateNames is the set of YANG local names that are always
// parent-qualified by nestedMessageName. The OpenConfig-style config/state
// idiom repeats a "config" and a "state" container at EVERY node in a tree,
// so a bare ProtoName("config")/ProtoName("state") would collide (as
// "Config"/"State") at every node. Unconditional qualification keeps each
// message name stable regardless of which node the generator visits first.
var configStateNames = map[string]bool{"config": true, "state": true}

// collisionSet walks the whole message tree rooted at root and returns the
// set of bare nested-message names (ProtoName of a list/container that is not
// "config"/"state") that occur more than once anywhere in the tree. This is
// the pre-pass nestedMessageName consults: a name in this set is
// parent-qualified at EVERY occurrence (symmetric), so two same-named lists
// under different parents (pavement/sensor + diagnostics/sensor) both qualify
// to PavementSensor/DiagnosticsSensor regardless of declaration order — a
// rename can never be triggered by reordering the YANG. Actions (c.RPC) are
// skipped (their request/response messages are named separately). config/state
// containers are traversed (deeper lists can collide) but never counted, since
// they are always parent-qualified anyway. A collision-free tree returns an
// empty set, so every name stays bare and output is byte-identical to before.
func collisionSet(root *yang.Entry) map[string]bool {
	return collisionSetForRoots([]*yang.Entry{root})
}

// collisionSetForRoots is the go-package-scoped generalization of
// collisionSet: it unions the nested-message-name counts across EVERY root
// in roots — not just one top-level tree — before deciding which names
// collide. Generate calls this once per go_package group, over every
// notification root AND every config/state root that routes to an output
// file sharing that go_package (see Generate), so a name colliding between
// two different top-level trees that happen to land in the same Go package
// (e.g. a nested "sensor" list reachable from both a service's events.proto
// and its state.proto) is caught and symmetrically qualified exactly like a
// collision within one tree, while two trees that route to DIFFERENT
// go_packages never see each other's names at all — each root's own
// collisionSet call is just the len(roots) == 1 case of this.
func collisionSetForRoots(roots []*yang.Entry) map[string]bool {
	counts := map[string]int{}
	var walk func(e *yang.Entry)
	walk = func(e *yang.Entry) {
		for _, c := range sortedChildren(e) {
			if c.RPC != nil {
				continue
			}
			if !c.IsList() && !c.IsContainer() {
				continue
			}
			if !configStateNames[c.Name] {
				counts[ProtoName(c.Name)]++
			}
			walk(c)
		}
	}
	for _, root := range roots {
		walk(root)
	}
	out := map[string]bool{}
	for n, ct := range counts {
		if ct > 1 {
			out[n] = true
		}
	}
	return out
}

// nestedMessageName returns the proto message name for nested list/container
// child c under a message named parentMsgName. config/state are always
// parent-qualified (the "config" child of "Phase" becomes "PhaseConfig", the
// "state" child of "SignalController" becomes "SignalControllerState"). Every
// other nested message keeps its bare ProtoName UNLESS that bare name collides
// (appears more than once in this top-level tree — see collisionSet), in which
// case it is parent-qualified at every occurrence (e.g. two lists both named
// "sensor" under "alpha"/"beta" become "AlphaSensor"/"BetaSensor"). Symmetric
// and order-independent; a collision-free tree keeps every name bare.
func nestedMessageName(parentMsgName string, c *yang.Entry, collisions map[string]bool) string {
	if configStateNames[c.Name] {
		return parentMsgName + ProtoName(c.Name)
	}
	bare := ProtoName(c.Name)
	if collisions[bare] {
		return parentMsgName + bare
	}
	return bare
}

// indentBlock left-pads every non-empty line of s by n spaces, used to nest a
// generated sub-message body inside its enclosing message.
func indentBlock(s string, n int) string {
	if s == "" {
		return ""
	}
	pad := strings.Repeat(" ", n)
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// EmitMessage appends a proto message for e (notification/container/list) to
// out, recursing into nested containers/lists/choice-cases. shared is the
// grouping-identity->message-name map from SharedGroupings: when a child
// container originates from a shared grouping's `uses`, EmitMessage emits a
// reference field to the shared message (written once, see
// ProtoFile.EmittedShared) instead of inlining it. Pass a nil/empty map for
// callers with no shared groupings (e.g. synthetic single-file fixtures).
func EmitMessage(e *yang.Entry, msgName string, lock *FieldLock, shared map[string]string, out *ProtoFile) {
	children := sortedChildren(e)

	// Gather every numberable field name in canonical order: top-level fields
	// plus every choice-case's leaf/nested-message names, so all of them share
	// one tag sequence assigned in a single lock.Assign call. choice/case
	// entries themselves are not numberable (they never appear on the wire as
	// a field), only their resolved oneof members are.
	names := make([]string, 0, len(children))
	for _, c := range children {
		if c.RPC != nil {
			continue // actions carry no wire field in the enclosing message
		}
		if c.Kind == yang.ChoiceEntry {
			for _, cs := range sortedChildren(c) {
				caseChildren := sortedChildren(cs)
				if len(caseChildren) == 1 {
					names = append(names, fieldName(caseChildren[0].Name))
				} else {
					names = append(names, fieldName(cs.Name))
				}
			}
			continue
		}
		names = append(names, fieldName(c.Name))
	}
	tags := lock.Assign(lockKey(out.LockPrefix, msgName), names, reservedFieldTags(children, shared))

	var nested strings.Builder       // sibling messages (lists, containers)
	var nestedInside strings.Builder // messages nested inside this one (multi-node choice cases)
	var body strings.Builder
	for _, c := range children {
		fn := fieldName(c.Name)
		tag := tags[fn]
		switch {
		case c.RPC != nil:
			emitAction(c, lock, shared, &nested, out)
		case c.IsLeaf():
			pt := leafFieldType(c, &nested, out)
			fmt.Fprintf(&body, "  %s %s = %d;\n", pt, fn, tag)
		case c.IsLeafList():
			pt := leafFieldType(c, &nested, out)
			fmt.Fprintf(&body, "  repeated %s %s = %d;\n", pt, fn, tag)
		case c.IsList():
			sub := nestedMessageName(msgName, c, out.Collisions)
			subFile := out.child()
			EmitMessage(c, sub, lock, shared, subFile)
			nested.WriteString(subFile.Body.String())
			for imp := range subFile.Imports {
				out.addImport(imp)
			}
			fmt.Fprintf(&body, "  repeated %s %s = %d;\n", sub, fn, tag)
		case c.IsContainer():
			if grpName, ok := groupingOf(c); ok {
				if sharedMsg, isShared := shared[grpName]; isShared {
					refName := emitSharedMessage(c, sharedMsg, lock, shared, out)
					fmt.Fprintf(&body, "  %s %s = %d;\n", refName, fn, tag)
					continue
				}
			}
			sub := nestedMessageName(msgName, c, out.Collisions)
			subFile := out.child()
			EmitMessage(c, sub, lock, shared, subFile)
			nested.WriteString(subFile.Body.String())
			for imp := range subFile.Imports {
				out.addImport(imp)
			}
			fmt.Fprintf(&body, "  %s %s = %d;\n", sub, fn, tag)
		case c.Kind == yang.ChoiceEntry:
			fmt.Fprintf(&body, "  oneof %s {\n", fn)
			for _, cs := range sortedChildren(c) {
				caseChildren := sortedChildren(cs)
				if len(caseChildren) == 1 {
					m := caseChildren[0]
					mn := fieldName(m.Name)
					pt := leafFieldType(m, &nested, out)
					fmt.Fprintf(&body, "    %s %s = %d;\n", pt, mn, tags[mn])
				} else {
					// Multi-node case: emit the message NESTED inside this
					// message (so its Go type is Parent_Case, keeping these
					// helper types namespaced under their oneof rather than
					// leaking as flat top-level protos) and reference it as a
					// single oneof member.
					sub := ProtoName(cs.Name)
					subFile := out.child()
					EmitMessage(cs, sub, lock, shared, subFile)
					nestedInside.WriteString(subFile.Body.String())
					for imp := range subFile.Imports {
						out.addImport(imp)
					}
					mn := fieldName(cs.Name)
					fmt.Fprintf(&body, "    %s %s = %d;\n", sub, mn, tags[mn])
				}
			}
			fmt.Fprint(&body, "  }\n")
		}
	}
	inner := body.String()
	if nestedInside.Len() > 0 {
		inner += indentBlock(nestedInside.String(), 2)
	}
	fmt.Fprintf(&out.Body, "message %s {\n%s}\n\n", msgName, inner)
	out.Body.WriteString(nested.String())
}

// emitAction emits the request/response messages for YANG action a into
// nested (as sibling top-level messages of the enclosing message), and adds
// no field to the enclosing message — an action is an operation, not tree
// data. goyang models an action as an entry whose input/output live on
// a.RPC.Input / a.RPC.Output rather than in a.Dir, so this emits directly
// from those. Both messages are always emitted (either may be empty) so the
// request/response pairing is predictable. Action names are not
// parent-qualified: each service's actions live in that service's own proto
// package, and within a service the action verbs are distinct.
func emitAction(a *yang.Entry, lock *FieldLock, shared map[string]string, nested *strings.Builder, out *ProtoFile) {
	base := ProtoName(a.Name)
	emitActionSide(a.RPC.Input, base+"Request", lock, shared, nested, out)
	emitActionSide(a.RPC.Output, base+"Response", lock, shared, nested, out)
}

// emitActionSide emits one action message (request or response) named
// msgName from side (an *yang.Entry for the action's input or output, which
// may be nil for an action that declares no input or no output — in which
// case an empty message is emitted). It reuses the generic EmitMessage so
// nested containers/lists/enums inside an action's input/output are handled
// identically to anywhere else.
func emitActionSide(side *yang.Entry, msgName string, lock *FieldLock, shared map[string]string, nested *strings.Builder, out *ProtoFile) {
	if side == nil {
		fmt.Fprintf(nested, "message %s {\n}\n\n", msgName)
		return
	}
	subFile := out.child()
	EmitMessage(side, msgName, lock, shared, subFile)
	nested.WriteString(subFile.Body.String())
	for imp := range subFile.Imports {
		out.addImport(imp)
	}
}

// reservedFieldTags inspects e's direct children — the same set EmitMessage
// is about to hand to FieldLock.Assign — and reports which of them, if any,
// must receive FieldLock's two reserved tags: "kind" -> reservedKind, only
// when that leaf's YANG type is identityref, and "source" -> reservedSource,
// only when that field is the message-typed reference the `wire-source`
// grouping produces (i.e. c.IsContainer() and groupingOf(c) is literally
// "wire-source", the same shared-grouping identity emitSharedMessage's
// caller below checks before routing it into the shared WireSource
// message). This is the fix for the wire-contract defect where Assign used
// to reserve tag 100 for ANY field named "source" (99 for "kind") purely by
// name: two RSU notification leaves are plain `leaf source { type string;
// }` domain strings that happen to share the name with the WireSource
// grouping's container field, and previously collided with tag 100 too,
// breaking the invariant "tag 100 == WireSource message" (see
// fieldnum.go). A field named kind/source that doesn't match its reserved
// shape (e.g. a scalar `leaf source { type string; }`, or a hypothetical
// scalar `leaf kind { type string; }`) is simply absent from the returned
// map, so Assign hands it a normal sequential tag like any other field.
//
// Only e's direct (non-choice-case) children are inspected: no notification
// in the corpus — and no recognized shape — puts kind/source inside a
// choice/case.
func reservedFieldTags(children []*yang.Entry, shared map[string]string) map[string]int {
	reserved := map[string]int{}
	for _, c := range children {
		fn := fieldName(c.Name)
		switch {
		case fn == "kind" && c.IsLeaf() && c.Type.Kind == yang.Yidentityref:
			reserved[fn] = reservedKind
		case fn == "source" && c.IsContainer():
			if grpName, ok := groupingOf(c); ok && grpName == "wire-source" {
				if _, isShared := shared[grpName]; isShared {
					reserved[fn] = reservedSource
				}
			}
		}
	}
	return reserved
}

// emitSharedMessage writes the shared message sharedMsg (for grouping-usage
// container c) into its target accumulator at most once — the first caller
// (across the whole EmitMessage recursion tree rooted at whatever top-level
// ProtoFile out's Emitted* maps trace back to, per child()) generates and
// writes it: every later call — from the same or a different top-level
// message, in the same or a different output file — is a no-op. Every user
// still gets its own reference field written by the caller; only the
// message definition itself is deduped. Returns the name the caller's
// reference field should use: the bare sharedMsg name if out has no
// separate Types target (or is itself the types file), or a
// package-qualified name (plus a cross-file import added to out) if the
// message was routed into a different ProtoFile via out.Types.
func emitSharedMessage(c *yang.Entry, sharedMsg string, lock *FieldLock, shared map[string]string, out *ProtoFile) string {
	target := out
	crossFile := out.Types != nil && out.Types.File != out
	if crossFile {
		target = out.Types.File
	}
	if target.EmittedShared == nil {
		target.EmittedShared = map[string]bool{}
	}
	if !target.EmittedShared[sharedMsg] {
		target.EmittedShared[sharedMsg] = true
		subFile := target.child()
		EmitMessage(c, sharedMsg, lock, shared, subFile)
		target.Body.WriteString(subFile.Body.String())
		for imp := range subFile.Imports {
			target.addImport(imp)
		}
	}
	if !crossFile {
		return sharedMsg
	}
	out.addImport(out.Types.ImportPath)
	return out.Types.Package + "." + sharedMsg
}

// leafFieldType returns the proto field type for leaf/leaf-list c. YANG
// enumerations get a proto enum emitted (once — see emitEnum) into nested
// and are referenced by its name. A leafref is resolved to its target
// leaf's type so a list key typed `leafref { path "../config/<key>" }`
// renders as the target scalar (e.g. uint32), not a string. Every other
// type maps through ProtoScalar, registering the timestamp import as needed.
func leafFieldType(c *yang.Entry, nested *strings.Builder, out *ProtoFile) string {
	if c.Type.Kind == yang.Yleafref {
		if target := resolveLeafref(c); target != nil {
			return leafFieldType(target, nested, out)
		}
		// Unresolvable target: keep the historical string fallback.
		return "string"
	}
	if c.Type.Kind == yang.Yenum {
		return emitEnum(c, nested, out)
	}
	pt, ts := ProtoScalar(c.Type)
	if ts {
		out.addImport("google/protobuf/timestamp.proto")
	}
	return pt
}

// resolveLeafref follows leaf c's leafref path to the target leaf entry, or
// returns nil if the path is empty or does not resolve. Any leafref
// predicate (a "[...]" segment, e.g. a key qualifier) is stripped before
// resolution: predicates constrain WHICH instance is referenced, not the
// target node's type. Config/state list keys use simple predicate-free
// paths ("../config/<key>"), so this returns the target key leaf whose
// scalar type the key should adopt.
//
// Resolution is DATA-tree-aware. A leafref path is written against the data
// tree, where `choice`/`case` nodes are transparent (RFC 7950 §6.5, §9.9);
// goyang's Entry tree keeps choice/case as real nodes, and its Entry.Find
// walks that schema tree (one e.Parent per ".."). So for a leafref leaf
// declared inside a case, a naive Find under-counts every "../" step by the
// choice/case nesting depth and falls through to the string fallback — which
// is exactly how channelTable's `phase` source (leafref -> phase-number)
// wrongly rendered as proto string. We instead walk the relative path here,
// skipping choice/case ancestors on ".." and looking through them on descent.
// Absolute paths (which need module-prefix resolution and never hit the
// choice/case ascent bug) are delegated to Find unchanged.
func resolveLeafref(c *yang.Entry) *yang.Entry {
	path := stripLeafrefPredicates(c.Type.Path)
	if path == "" {
		return nil
	}
	if strings.HasPrefix(path, "/") {
		return c.Find(path)
	}
	e := c
	for _, part := range strings.Split(path, "/") {
		switch part {
		case "", ".":
			// empty segment (e.g. a trailing "/") or current-node ref: skip
		case "..":
			e = dataParent(e)
		default:
			if i := strings.IndexByte(part, ':'); i >= 0 {
				part = part[i+1:] // drop any "prefix:" qualifier
			}
			e = dataChild(e, part)
		}
		if e == nil {
			return nil
		}
	}
	return e
}

// stripLeafrefPredicates removes any "[...]" predicate segments from a leafref
// path; predicates constrain which instance is referenced, not the target
// node's type, and the path walker below does not parse them.
func stripLeafrefPredicates(path string) string {
	var b strings.Builder
	depth := 0
	for _, r := range path {
		switch r {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// dataParent returns e's nearest ancestor in the data tree: its Parent with
// any intervening choice/case nodes skipped (they are transparent in
// data-tree paths).
func dataParent(e *yang.Entry) *yang.Entry {
	p := e.Parent
	for p != nil && (p.IsChoice() || p.IsCase()) {
		p = p.Parent
	}
	return p
}

// dataChild returns the child named `name` in e's data tree, looking through
// any choice/case nodes (transparent in data-tree paths) to reach leaves
// declared inside a case.
func dataChild(e *yang.Entry, name string) *yang.Entry {
	if e == nil {
		return nil
	}
	if ch, ok := e.Dir[name]; ok {
		return ch
	}
	for _, ch := range e.Dir {
		if ch.IsChoice() || ch.IsCase() {
			if got := dataChild(ch, name); got != nil {
				return got
			}
		}
	}
	return nil
}

// enumSource returns the kebab-case identifier an enum's proto name/value
// prefix should derive from: the typedef name for a named typedef (e.g.
// "fault-severity"), or the leaf's own name for an inline
// `type enumeration { ... }` leaf, which has no typedef identity (goyang
// reports its Type.Name as the literal "enumeration").
func enumSource(c *yang.Entry) string {
	if c.Type.Name != "" && c.Type.Name != "enumeration" {
		return c.Type.Name
	}
	return c.Name
}

// enumSignature returns a stable, content-derived fingerprint of c's
// enumeration values (each member's name and assigned number). Two enums
// with the same signature are the same enum in every way that matters for
// the wire format, even if they were declared in different places; two
// enums with different signatures are genuinely different enums and must
// not be merged just because they happen to share a YANG identifier (see
// EmittedEnums).
func enumSignature(c *yang.Entry) string {
	nm := c.Type.Enum.NameMap()
	names := make([]string, 0, len(nm))
	for n := range nm {
		names = append(names, n)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%s=%d", n, nm[n]))
	}
	return strings.Join(parts, "|")
}

// enumQualifiedSource returns src (see enumSource) qualified by c's owning
// YANG module — e.g. "openits-rsu-events-prior" — for use when a
// different enum has already claimed the bare src as its base name. Falls
// back to src unqualified if the owning module can't be resolved, which is
// not expected for any entry reachable from a fully processed module tree.
func enumQualifiedSource(c *yang.Entry, src string) string {
	mod, err := c.InstantiatingModule()
	if err != nil || mod == "" {
		return src
	}
	return mod + "-" + src
}

// emitEnum writes a top-level proto enum for leaf c's YANG enumeration type
// into nested, unless an enum with the same content was already emitted
// anywhere in out's recursion tree (tracked via out.EmittedEnums, see
// child()). Returns the enum's proto type name either way, for use as the
// field type.
//
// Enums are deduped by (base name, content signature), not by name alone:
// the first enum emitted under a given YANG-identifier-derived base name
// (e.g. "Prior") keeps that bare name, exactly as before. If a *different*
// enum later shares the same base name — e.g. a `prior` leaf in one module
// with values idle/active/fault and an unrelated `prior` leaf in another
// module with values disabled/advisory/active/suspended — it is not the
// same enum and must not overwrite or reuse the first one's declaration;
// it gets its own name qualified by its owning module (see
// enumQualifiedSource) so both emit as distinct proto types instead of
// colliding on the type name (and, via the value prefix below, on value
// names too).
//
// Proto3 requires the first enum value to be 0. Many YANG enumerations
// already have a member sitting at value 0 (whatever was declared first,
// with no explicit `value`, e.g. an explicit `unspecified` member or just
// the first member of an unqualified list like info/warning/critical): that
// member is emitted as the proto zero value under its own normal name, and
// every other member keeps its own YANG value unchanged — no shifting. Only
// when no YANG member occupies value 0 (e.g. every member declares an
// explicit nonzero `value`) does emitEnum prepend a synthetic
// `<PREFIX>_UNSPECIFIED = 0` sentinel to satisfy proto3, leaving every real
// member at its own YANG value.
//
// This also fixes the double-definition bug where a YANG enum already
// declares a member literally named `unspecified` (mapping to the proto
// value name <PREFIX>_UNSPECIFIED): previously the synthetic sentinel was
// always emitted regardless, colliding with that member's own value and
// making protoc reject the file. Since such a member almost always has YANG
// value 0, it now naturally takes the zero slot itself and no synthetic is
// emitted. As a last-resort guard against the pathological case of an
// `unspecified` member with a nonzero value (so the zero slot is otherwise
// unoccupied and a synthetic would be emitted too), the synthetic is
// dropped rather than emitted a second time under the same name.
func emitEnum(c *yang.Entry, nested *strings.Builder, out *ProtoFile) string {
	src := enumSource(c)
	baseName := ProtoName(src)
	sig := enumSignature(c)

	if out.EmittedEnums == nil {
		out.EmittedEnums = map[string]map[string]string{}
	}
	variants := out.EmittedEnums[baseName]
	if name, ok := variants[sig]; ok {
		return name
	}

	// qualifiedSrc drives both the type name and the value prefix, so a
	// colliding enum gets a distinct name AND distinct value identifiers
	// (proto enum values share their enclosing file's flat namespace,
	// same as the reason the PREFIX_VALUE convention exists below).
	//
	// Qualification triggers on either of two conditions: a different
	// enum already emitted into *this* file under baseName (len(variants)
	// > 0 — the original within-file check), or baseName already claimed
	// by *another* output file in this run (out.ClaimedNames — see
	// ProtoFile.ClaimedNames). The latter matters even on this leaf's
	// first occurrence in this file: two different files each hitting
	// "severity" for the first time must not both claim the bare name.
	qualifiedSrc := src
	name := baseName
	_, nameClaimedElsewhere := out.ClaimedNames[baseName]
	if len(variants) > 0 || nameClaimedElsewhere {
		qualifiedSrc = enumQualifiedSource(c, src)
		name = ProtoName(qualifiedSrc)
	}

	if variants == nil {
		variants = map[string]string{}
		out.EmittedEnums[baseName] = variants
	}
	variants[sig] = name
	if out.ClaimedNames != nil {
		out.ClaimedNames[name] = true
	}

	prefix := strings.ToUpper(strings.ReplaceAll(qualifiedSrc, "-", "_"))
	type enumVal struct {
		protoName string
		val       int64
	}
	nm := c.Type.Enum.NameMap()
	vals := make([]enumVal, 0, len(nm))
	for n, v := range nm {
		protoName := fmt.Sprintf("%s_%s", prefix, strings.ToUpper(strings.ReplaceAll(n, "-", "_")))
		vals = append(vals, enumVal{protoName, v})
	}
	sort.Slice(vals, func(i, j int) bool { return vals[i].val < vals[j].val })

	// zeroIdx is the index (if any) of the real YANG member already sitting
	// at value 0. YANG guarantees at most one member per value, so there is
	// never more than one candidate.
	zeroIdx := -1
	for i, v := range vals {
		if v.val == 0 {
			zeroIdx = i
			break
		}
	}

	fmt.Fprintf(nested, "enum %s {\n", name)
	emitted := map[string]bool{}
	if zeroIdx >= 0 {
		// A real member already occupies 0: emit it under its own name,
		// first, and no synthetic sentinel is needed at all.
		fmt.Fprintf(nested, "  %s = 0;\n", vals[zeroIdx].protoName)
		emitted[vals[zeroIdx].protoName] = true
	} else {
		// No member occupies 0: synthesize a sentinel to satisfy proto3,
		// unless doing so would collide with a real member's own proto
		// value name (the pathological case of an `unspecified` member
		// that was given an explicit nonzero value) — in that case the
		// synthetic is dropped rather than emitted a second time.
		synthetic := prefix + "_UNSPECIFIED"
		collides := false
		for _, v := range vals {
			if v.protoName == synthetic {
				collides = true
				break
			}
		}
		if !collides {
			fmt.Fprintf(nested, "  %s = 0;\n", synthetic)
			emitted[synthetic] = true
		}
	}
	for i, v := range vals {
		if i == zeroIdx || emitted[v.protoName] {
			continue
		}
		emitted[v.protoName] = true
		fmt.Fprintf(nested, "  %s = %d;\n", v.protoName, v.val)
	}
	fmt.Fprint(nested, "}\n\n")

	return name
}
