// Command yang-proto-gen generates protobuf definitions from the openits
// YANG modules using goyang.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

// goPackageFor returns the `option go_package` value for a file whose proto
// package is pkg: every service now gets its own Go package/directory
// (pkg/proto/<svc-path>/v1) instead of every generated file flattening into
// one shared openitspb package, so two services can each declare a message
// named e.g. "Detector" without colliding — they're different Go packages.
func goPackageFor(pkg string) string {
	return "github.com/openits/openits-models/pkg/proto/" + strings.ReplaceAll(pkg, ".", "/") + ";" + goPkgName(pkg)
}

// goPkgName derives the bare Go package name from a proto package: strip the
// leading "openits.", drop "." and "_" separators, keep the version — e.g.
// "openits.ramp_metering.v1" -> "rampmeteringv1", "openits.types.v1" ->
// "typesv1".
func goPkgName(pkg string) string {
	s := strings.TrimPrefix(pkg, "openits.")
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, "_", "")
	return s
}

// outFile accumulates one output .proto file's content plus enough
// provenance to report a clear error if two YANG notifications collide on
// the same proto message name within it (see validateUniqueMessages).
type outFile struct {
	pkg     string
	pf      *ProtoFile
	origins map[string][]string // proto message name -> "module:notification" origins
}

// Generate loads every YANG module under yangDir, computes shared
// groupings, emits one .proto per service package (plus a shared
// openits/types/v1/types.proto for cross-service messages like WireSource)
// under outDir, and persists the updated field-number lock at lockPath.
// Modules are processed in sorted-name order so output — including which of
// two content-colliding enums keeps the unqualified name (see emit.go) — is
// byte-identical across runs regardless of the nondeterministic map
// iteration LoadModules reads *yang.Modules.Modules through.
func Generate(yangDir, outDir, lockPath string) error {
	_, mods, err := LoadModules(yangDir)
	if err != nil {
		return fmt.Errorf("generate: load yang modules from %s: %w", yangDir, err)
	}
	sort.Slice(mods, func(i, j int) bool { return mods[i].Name < mods[j].Name })

	shared := SharedGroupings(mods)

	lock, err := LoadFieldLock(lockPath)
	if err != nil {
		return fmt.Errorf("generate: load field lock %s: %w", lockPath, err)
	}

	// typesPF is the single shared accumulator every shared-grouping
	// message (WireSource today; any future ≥2-site container-shaped
	// grouping automatically) is centralized into — see emit.go's
	// TypesTarget/emitSharedMessage. Every service ProtoFile below points
	// its Types field at the same target, so no matter which service
	// module first references a given shared grouping, the message lands
	// in types.proto exactly once and every other reference is a
	// cross-package import + qualified name.
	typesPF := &ProtoFile{LockPrefix: typesPackage}
	typesTarget := &TypesTarget{File: typesPF, Package: typesPackage, ImportPath: typesFilePath}
	typesPF.Types = typesTarget

	// claimedEnumNames is keyed per go_package (see goPackageFor), NOT
	// shared globally across the run — see ProtoFile.ClaimedNames. Every
	// generated .proto file now carries a PER-SERVICE `option go_package`,
	// so protoc-gen-go only flattens the files that share one service's
	// go_package into one Go package (e.g. a service's events.proto +
	// state.proto), not the whole corpus: two different services declaring
	// the same bare top-level enum name (e.g. "Severity") is legal — they
	// land in different Go packages — so their claimed-name sets must stay
	// separate too. claimedNamesFor lazily allocates each package's set.
	claimedEnumNames := map[string]map[string]bool{}
	claimedNamesFor := func(pkg string) map[string]bool {
		gp := goPackageFor(pkg)
		m := claimedEnumNames[gp]
		if m == nil {
			m = map[string]bool{}
			claimedEnumNames[gp] = m
		}
		return m
	}
	typesPF.ClaimedNames = claimedNamesFor(typesPackage)

	files := map[string]*outFile{} // relative output path -> accumulator

	// pendingRoot is one top-level message tree — a live notification, or a
	// config/state root container/list — awaiting emission. Both passes
	// below only COLLECT roots (they never call EmitMessage): collision
	// scope must be computed per go_package across every root that shares
	// one (see collisionsByPkg), which requires seeing every root, from
	// both the events pass and the config/state pass, before emitting any
	// of them.
	type pendingRoot struct {
		of      *outFile
		entry   *yang.Entry
		msgName string
		origin  string // "module:notification" or "module:config-state"
	}
	var pending []pendingRoot

	for _, mod := range mods {
		pkg, relFile, ok := pkgFor(mod.Name)
		if !ok {
			if hasLiveNotification(mod) {
				return fmt.Errorf("generate: module %q declares notification(s) but has no service package mapping in pkgmap.go — add a serviceRoute for it", mod.Name)
			}
			continue
		}
		of := files[relFile]
		if of == nil {
			of = &outFile{pkg: pkg, pf: &ProtoFile{Types: typesTarget, ClaimedNames: claimedNamesFor(pkg), LockPrefix: pkg}, origins: map[string][]string{}}
			files[relFile] = of
		}
		for _, c := range sortedChildren(mod) {
			if c.Kind != yang.NotificationEntry {
				continue
			}
			if st := entryStatus(c); st == "deprecated" || st == "obsolete" {
				continue
			}
			pending = append(pending, pendingRoot{of: of, entry: c, msgName: ProtoName(c.Name), origin: mod.Name + ":" + c.Name})
		}
	}

	// Second pass: config/state trees + actions for modules on the opt-in
	// allowlist (configStateRoutes). Empty allowlist => this loop is inert
	// and output stays byte-identical to the events-only generator.
	for _, mod := range mods {
		route, ok := configStateFor(mod.Name)
		if !ok {
			continue
		}
		of := files[route.file]
		if of == nil {
			of = &outFile{pkg: route.pkg, pf: &ProtoFile{Types: typesTarget, ClaimedNames: claimedNamesFor(route.pkg), LockPrefix: route.pkg}, origins: map[string][]string{}}
			files[route.file] = of
		}
		for _, c := range sortedChildren(mod) {
			if c.Kind == yang.NotificationEntry {
				continue
			}
			if c.RPC != nil {
				continue // top-level rpc statements aren't config/state trees
			}
			if st := entryStatus(c); st == "deprecated" || st == "obsolete" {
				continue
			}
			if !c.IsContainer() && !c.IsList() {
				continue
			}
			pending = append(pending, pendingRoot{of: of, entry: c, msgName: ProtoName(c.Name), origin: mod.Name + ":config-state"})
		}
	}

	// Collision scope is per go_package, NOT per top-level tree and NOT
	// global: group every pending root (both events and config/state) by
	// the go_package of the output file it routes to, and compute ONE
	// collisionSet over the union of those roots' trees. A service's
	// events.proto and state.proto share one go_package, so a nested name
	// colliding between them — or between two notifications in the same
	// file — is caught and symmetrically qualified exactly like a collision
	// within a single tree (e.g. ESS's pavement/sensor + diagnostics/sensor
	// -> PavementSensor/DiagnosticsSensor). Two different SERVICES
	// declaring the same nested name (e.g. both "Detector") never even see
	// each other's roots, since they group under different go_packages —
	// this is what keeps cross-service names clean. See
	// collisionSetForRoots (emit.go).
	rootsByPkg := map[string][]*yang.Entry{}
	for _, p := range pending {
		gp := goPackageFor(p.of.pkg)
		rootsByPkg[gp] = append(rootsByPkg[gp], p.entry)
	}
	collisionsByPkg := make(map[string]map[string]bool, len(rootsByPkg))
	for gp, roots := range rootsByPkg {
		collisionsByPkg[gp] = collisionSetForRoots(roots)
	}

	for _, p := range pending {
		of := p.of
		of.pf.Collisions = collisionsByPkg[goPackageFor(of.pkg)]
		before := of.pf.Body.Len()
		EmitMessage(p.entry, p.msgName, lock, shared, of.pf)
		added := of.pf.Body.String()[before:]
		for _, name := range messageNamesIn(added) {
			of.origins[name] = append(of.origins[name], p.origin)
		}
	}

	relFiles := make([]string, 0, len(files))
	for f := range files {
		relFiles = append(relFiles, f)
	}
	sort.Strings(relFiles)

	// Validate every file — including the shared types file — before
	// writing anything, so a collision aborts cleanly with no partial
	// output on disk.
	for _, f := range relFiles {
		of := files[f]
		body := of.pf.Body.String()
		if err := validateUniqueMessages(f, body, of.origins); err != nil {
			return err
		}
		if err := validateUniqueFieldTags(f, body); err != nil {
			return err
		}
	}
	if body := typesPF.Body.String(); body != "" {
		if err := validateUniqueMessages(typesFilePath, body, nil); err != nil {
			return err
		}
		if err := validateUniqueFieldTags(typesFilePath, body); err != nil {
			return err
		}
	}

	// Cross-file message-name uniqueness, scoped per go_package. Two files
	// that share a go_package (e.g. one service's events.proto + state.proto)
	// still flatten into one Go package, and FieldLock keys by BARE message
	// name — so the same message name in two SAME-go_package files is both a
	// Go symbol collision and a field-number-lock conflation.
	// validateUniqueMessages only guards within a file — this guards across
	// them, but only within a go_package: two different services declaring
	// the same message name (e.g. both "Detector") now route to different
	// go_packages and are never compared, which is legal.
	bodies := make(map[string]string, len(relFiles)+1)
	filePkg := make(map[string]string, len(relFiles)+1)
	for _, f := range relFiles {
		bodies[f] = files[f].pf.Body.String()
		filePkg[f] = files[f].pkg
	}
	if body := typesPF.Body.String(); body != "" {
		bodies[typesFilePath] = body
		filePkg[typesFilePath] = typesPackage
	}
	if cols := findCrossFileMessageCollisions(bodies, filePkg); len(cols) > 0 {
		return fmt.Errorf("generate: message name(s) declared in more than one file sharing a go_package "+
			"(Go go_package flatten + field-number-lock conflation): %s", strings.Join(cols, "; "))
	}

	for _, f := range relFiles {
		of := files[f]
		if err := writeProtoFile(filepath.Join(outDir, f), of.pkg, of.pf); err != nil {
			return err
		}
	}
	if typesPF.Body.Len() > 0 {
		if err := writeProtoFile(filepath.Join(outDir, typesFilePath), typesPackage, typesPF); err != nil {
			return err
		}
	}

	if err := lock.Save(lockPath); err != nil {
		return fmt.Errorf("generate: save field lock %s: %w", lockPath, err)
	}

	// JSON Schema emission is strictly additive and runs after every proto
	// file above has been validated and written: a JSON Schema failure must
	// never leave a half-written proto tree, and proto validation failing
	// (caught above, before any file is written) must never produce JSON
	// Schema output for a corpus Generate is about to reject anyway.
	if err := emitJSONSchemas(mods, shared, outDir); err != nil {
		return err
	}
	return nil
}

// emitModuleConfigState walks mod's top-level container/list children — the
// service's config/state root(s) — and emits each as a proto message into
// of, recursing through config/state containers, nested keyed lists, and any
// actions via EmitMessage. Notifications are handled by Generate's separate
// events loop and are skipped here. Children are visited in sortedChildren
// order so output is deterministic. Generate itself no longer calls this
// directly (its config/state pass collects roots into the same
// go_package-scoped collision computation the events pass uses — see
// pendingRoot in Generate); it survives as a single-module, self-contained
// entry point exercised directly by configstate_test.go, computing its own
// per-root collision set exactly as it always has.
func emitModuleConfigState(mod *yang.Entry, of *ProtoFile, lock *FieldLock, shared map[string]string) {
	for _, c := range sortedChildren(mod) {
		if c.Kind == yang.NotificationEntry {
			continue
		}
		if c.RPC != nil {
			continue // top-level rpc statements aren't config/state trees
		}
		if st := entryStatus(c); st == "deprecated" || st == "obsolete" {
			continue
		}
		if !c.IsContainer() && !c.IsList() {
			continue
		}
		// Collisions is scoped per top-level tree (this service root), so two
		// same-named lists under different parents (pavement/sensor +
		// diagnostics/sensor) both qualify symmetrically.
		of.Collisions = collisionSet(c)
		EmitMessage(c, ProtoName(c.Name), lock, shared, of)
	}
}

// emitJSONSchemas writes one deterministic schema.json per live
// (non-deprecated, non-obsolete) notification into outDir, alongside the
// .proto file its module's notifications are collected into, then prunes
// any *.schema.json left over from a notification that is no longer live
// (deleted from the YANG outright, or deprecated/obsoleted out) — see
// pruneOrphanSchemas. It reuses pkgFor's module routing — the same routing
// the proto loop above uses — so a module with no service mapping
// (typedef/grouping-only, e.g. openits-types) is silently skipped here
// exactly as it is for proto, and a module with a live notification but no
// route has already made Generate fail before this runs. The output
// filename is module- and notification-qualified (e.g.
// "openits-common-fault-events.fault-raised.schema.json") rather than
// notification-name-only, so two modules routed to the same output file can
// never collide even if a future module reused a notification name
// protected by an active (non-deprecated) declaration.
func emitJSONSchemas(mods []*yang.Entry, shared map[string]string, outDir string) error {
	written := map[string]bool{} // absolute schema.json path -> written this run
	dirs := map[string]bool{}    // every directory schemas were (or could have been) written into this run

	for _, mod := range mods {
		_, relFile, ok := pkgFor(mod.Name)
		if !ok {
			continue
		}
		dir := filepath.Join(outDir, filepath.Dir(relFile))
		dirs[dir] = true
		for _, c := range sortedChildren(mod) {
			if c.Kind != yang.NotificationEntry {
				continue
			}
			if st := entryStatus(c); st == "deprecated" || st == "obsolete" {
				continue
			}
			schema := EmitJSONSchema(c, shared)
			path := filepath.Join(dir, mod.Name+"."+c.Name+".schema.json")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("generate: mkdir %s: %w", dir, err)
			}
			if err := os.WriteFile(path, MarshalSchemaDeterministic(schema), 0o644); err != nil {
				return fmt.Errorf("generate: write %s: %w", path, err)
			}
			written[path] = true
		}
	}
	return pruneOrphanSchemas(dirs, written)
}

// pruneOrphanSchemas deletes every *.schema.json file under each of dirs
// that this run's emitJSONSchemas loop did not (re)write — i.e. written.
// Without this, `make gen` is append-only for schema.json: EmitJSONSchema
// only ever adds a file for each live notification, so a schema.json left
// behind by a notification that was since deleted or deprecated out of the
// YANG lingers on disk indefinitely, describing API surface that no longer
// exists (this happened when 6 per-service fault/mode notifications were
// deleted, leaving 6 such orphans behind uncaught). Pruning is scoped strictly to
// "*.schema.json" — it never touches .proto, .pb.go, or anything else in
// dir. dirs and each directory's matches are both visited in sorted order
// so which files get removed, and in what order, is stable across runs.
func pruneOrphanSchemas(dirs map[string]bool, written map[string]bool) error {
	sortedDirs := make([]string, 0, len(dirs))
	for d := range dirs {
		sortedDirs = append(sortedDirs, d)
	}
	sort.Strings(sortedDirs)

	for _, dir := range sortedDirs {
		matches, err := filepath.Glob(filepath.Join(dir, "*.schema.json"))
		if err != nil {
			return fmt.Errorf("generate: glob %s: %w", dir, err)
		}
		sort.Strings(matches)
		for _, m := range matches {
			if written[m] {
				continue
			}
			if err := os.Remove(m); err != nil {
				return fmt.Errorf("generate: remove orphaned schema %s: %w", m, err)
			}
		}
	}
	return nil
}

// hasLiveNotification reports whether mod declares at least one
// non-deprecated, non-obsolete notification — used to distinguish a module
// that's legitimately unmapped (typedef/grouping-only, e.g. openits-types)
// from one that's missing a serviceRoute and would otherwise silently drop
// its notifications from the generated output.
func hasLiveNotification(mod *yang.Entry) bool {
	for _, c := range sortedChildren(mod) {
		if c.Kind != yang.NotificationEntry {
			continue
		}
		if st := entryStatus(c); st == "deprecated" || st == "obsolete" {
			continue
		}
		return true
	}
	return false
}

// entryStatus returns e's YANG `status` substatement argument ("current",
// "deprecated", "obsolete"), or "" if unset (which RFC 7950 treats as
// "current"). goyang's *yang.Entry has no direct Status accessor, so this
// reads it off the underlying statement the same way declOrder does.
func entryStatus(e *yang.Entry) string {
	if e == nil || e.Node == nil {
		return ""
	}
	st := e.Node.Statement()
	if st == nil {
		return ""
	}
	for _, sub := range st.SubStatements() {
		if sub.Keyword == "status" {
			return sub.Argument
		}
	}
	return ""
}

// writeProtoFile renders pf as a complete .proto file (syntax, package,
// go_package, sorted deduped imports, body) and writes it to path, creating
// parent directories as needed. go_package is derived from pkg (see
// goPackageFor) so every file gets its own per-service go_package instead of
// the single shared one every file used to carry.
func writeProtoFile(path, pkg string, pf *ProtoFile) error {
	var b strings.Builder
	b.WriteString("syntax = \"proto3\";\n\n")
	fmt.Fprintf(&b, "package %s;\n\n", pkg)
	fmt.Fprintf(&b, "option go_package = %q;\n\n", goPackageFor(pkg))
	if len(pf.Imports) > 0 {
		imports := make([]string, 0, len(pf.Imports))
		for imp := range pf.Imports {
			imports = append(imports, imp)
		}
		sort.Strings(imports)
		for _, imp := range imports {
			fmt.Fprintf(&b, "import %q;\n", imp)
		}
		b.WriteString("\n")
	}
	b.WriteString(pf.Body.String())

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("generate: mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("generate: write %s: %w", path, err)
	}
	return nil
}

// messageHeaderRe matches the header line EmitMessage always produces for a
// top-level message declaration: `fmt.Fprintf(&out.Body, "message %s {\n...",
// msgName, ...)`. fieldTagRe matches a field/oneof-member line's trailing
// `= <tag>;`.
var (
	messageHeaderRe = regexp.MustCompile(`^message (\w+) \{$`)
	fieldTagRe      = regexp.MustCompile(`= (\d+);$`)
)

// messageBlock is one top-level "message NAME { ... }" declaration
// extracted from an assembled ProtoFile body, along with every field/oneof
// tag number assigned inside it.
type messageBlock struct {
	name string
	tags []int
}

// extractMessageBlocks scans body — the fully assembled contents of one
// output .proto file — for every message declaration EmitMessage produced,
// including messages nested inside another (a multi-node choice case emits
// its message nested inside the enclosing message). It is brace-depth aware:
// each field/oneof tag is attributed to its NEAREST enclosing message, so a
// nested message's own tags count toward that nested message, not its parent
// (a oneof brace is transparent — its members share the enclosing message's
// tag space). Message headers are matched regardless of indentation. A name
// can appear more than once in the returned slice if two emissions produced
// it; that's exactly the condition validateUniqueMessages /
// validateUniqueFieldTags exist to catch.
func extractMessageBlocks(body string) []messageBlock {
	lines := strings.Split(body, "\n")
	var blocks []messageBlock
	// Scope stack: one frame per open brace. A frame holds the index into
	// blocks when it is a message body, or -1 for any other brace (oneof,
	// etc.). Tags attribute to the nearest frame whose index is >= 0.
	var stack []int
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			continue
		case strings.HasSuffix(line, "{"):
			if m := messageHeaderRe.FindStringSubmatch(line); m != nil {
				blocks = append(blocks, messageBlock{name: m[1]})
				stack = append(stack, len(blocks)-1)
			} else {
				stack = append(stack, -1)
			}
		case line == "}":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		default:
			tm := fieldTagRe.FindStringSubmatch(line)
			if tm == nil {
				continue
			}
			n, err := strconv.Atoi(tm[1])
			if err != nil {
				continue
			}
			for k := len(stack) - 1; k >= 0; k-- {
				if stack[k] >= 0 {
					blocks[stack[k]].tags = append(blocks[stack[k]].tags, n)
					break
				}
			}
		}
	}
	return blocks
}

// messageNamesIn returns every top-level message name declared in text, in
// order of appearance (duplicates included) — used by Generate to attribute
// each message name a given top-level EmitMessage call produced back to the
// YANG notification that produced it.
func messageNamesIn(text string) []string {
	var names []string
	for _, line := range strings.Split(text, "\n") {
		if m := messageHeaderRe.FindStringSubmatch(line); m != nil {
			names = append(names, m[1])
		}
	}
	return names
}

// validateUniqueMessages fails if body — one fully assembled output .proto
// file's contents — declares the same top-level message name more than
// once: protoc rejects a file with two `message X { ... }` declarations
// outright, so this must be caught before writing the file at all. origins,
// when non-nil, maps each colliding message name to the "module:
// notification" YANG source(s) that produced it (see Generate), so the
// error names the actual colliding YANG origins rather than just the proto
// symptom; pass nil (e.g. for the shared types file, which has no
// per-notification origin tracking) to fall back to an occurrence count.
func validateUniqueMessages(fileLabel, body string, origins map[string][]string) error {
	counts := map[string]int{}
	for _, b := range extractMessageBlocks(body) {
		counts[b.name]++
	}
	var dupNames []string
	for n, c := range counts {
		if c > 1 {
			dupNames = append(dupNames, n)
		}
	}
	if len(dupNames) == 0 {
		return nil
	}
	sort.Strings(dupNames)
	parts := make([]string, 0, len(dupNames))
	for _, n := range dupNames {
		if o := origins[n]; len(o) > 0 {
			parts = append(parts, fmt.Sprintf("%s (from %s)", n, strings.Join(o, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("%s (%d occurrences)", n, counts[n]))
		}
	}
	return fmt.Errorf("generate: %s: duplicate message name(s): %s", fileLabel, strings.Join(parts, "; "))
}

// validateUniqueFieldTags fails if any single message block in body assigns
// the same proto field tag to more than one field/oneof member — the
// wire-format symptom of two differently-named YANG leaves whose names
// collapse to the same proto field name (e.g. "foo-bar" and "foo_bar" both
// become "foo_bar") and so were silently assigned the same FieldLock tag by
// FieldLock.Assign (which dedupes by field *name*, not YANG identifier).
func validateUniqueFieldTags(fileLabel, body string) error {
	for _, b := range extractMessageBlocks(body) {
		seen := map[int]int{}
		for _, t := range b.tags {
			seen[t]++
		}
		var dupTags []int
		for t, c := range seen {
			if c > 1 {
				dupTags = append(dupTags, t)
			}
		}
		if len(dupTags) > 0 {
			sort.Ints(dupTags)
			return fmt.Errorf("generate: %s: message %s has duplicate field tag(s) %v — two YANG leaf names likely collapsed to the same proto field name",
				fileLabel, b.name, dupTags)
		}
	}
	return nil
}

// findCrossFileMessageCollisions returns, sorted, any bare message name
// declared in more than one output file that share a go_package. bodies maps
// a file label to that file's assembled proto body; filePkg maps the same
// label to the proto package (and hence, via goPackageFor, the go_package)
// the file routes to. Every generated .proto now carries a PER-SERVICE
// go_package (see goPackageFor), so protoc-gen-go only flattens the files
// sharing one service's go_package into one Go package — two files in
// DIFFERENT go_packages may legally declare the same bare message name (e.g.
// two services both declaring "Detector"); only a same-go_package collision is
// still a Go symbol clash + field-number-lock conflation (FieldLock keys by
// bare message name).
func findCrossFileMessageCollisions(bodies map[string]string, filePkg map[string]string) []string {
	groups := map[string][]string{} // go_package -> file labels
	for label := range bodies {
		gp := goPackageFor(filePkg[label])
		groups[gp] = append(groups[gp], label)
	}

	var out []string
	for _, labels := range groups {
		nameFiles := map[string]map[string]bool{}
		for _, label := range labels {
			for _, b := range extractMessageBlocks(bodies[label]) {
				if nameFiles[b.name] == nil {
					nameFiles[b.name] = map[string]bool{}
				}
				nameFiles[b.name][label] = true
			}
		}
		for name, fs := range nameFiles {
			if len(fs) < 2 {
				continue
			}
			flabels := make([]string, 0, len(fs))
			for f := range fs {
				flabels = append(flabels, f)
			}
			sort.Strings(flabels)
			out = append(out, fmt.Sprintf("%s (in %s)", name, strings.Join(flabels, ", ")))
		}
	}
	sort.Strings(out)
	return out
}

func main() {
	yangDir := flag.String("yang", "yang", "directory containing the openits YANG modules")
	outDir := flag.String("out", "api/proto", "output directory for generated .proto files (with -asyncapi, the directory asyncapi.yaml is written into)")
	lockPath := flag.String("lock", "tools/yang-proto-gen/field-numbers.yaml", "path to the field-number lock file")
	asyncAPI := flag.Bool("asyncapi", false, "emit asyncapi.yaml from the derived ce-type catalog + JSON Schemas into -out, instead of generating proto")
	flag.Parse()

	if *asyncAPI {
		if err := GenerateAsyncAPI(*yangDir, filepath.Join(*outDir, "asyncapi.yaml")); err != nil {
			fmt.Fprintln(os.Stderr, "yang-proto-gen:", err)
			os.Exit(1)
		}
		return
	}

	if err := Generate(*yangDir, *outDir, *lockPath); err != nil {
		fmt.Fprintln(os.Stderr, "yang-proto-gen:", err)
		os.Exit(1)
	}
}
