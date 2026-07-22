// check-events-layering enforces the events-module layering rule: every
// yang/*-events.yang module may import only
//
//   - openits-types,
//   - ietf-yang-types,
//   - openits-nema-common, or
//   - any module whose name ends in "-types" (a service's own
//     <svc>-types companion, or another service's).
//
// Anything else is a layering VIOLATION — most notably a bare service core
// (e.g. `import openits-dms`), or another *-events module. The whole point
// of splitting event payloads into their own companion module is that
// events depend only on shared leaf/enum/typedef definitions, never on a
// service's full core config/state schema (which would pull the events
// module into that core's change cadence and dependency graph) or on
// another service's event stream.
//
// The check is a line-based textual scan of each file's
// `import <name> { ... }` statements, not a full goyang parse/Process():
// the layering rule is purely about which module names are named in
// import statements, so extracting them with a regex is sufficient. This
// also means the check works even on a module that wouldn't resolve
// cleanly against a full module set (e.g. one with the very layering
// violation being detected), and needs no access to the rest of yang/.
//
// Wired to `make check-events-layering`.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// importRE matches a YANG `import <name> {` statement at the start of a
// (possibly indented) line. A YANG import statement always opens a `{`
// block (it carries at least the mandatory `prefix` substatement), so
// there's no bare `import <name>;` form to also handle.
var importRE = regexp.MustCompile(`^\s*import\s+(\S+)\s*\{`)

// allowedExact is the set of module names an events module may import
// verbatim, beyond the "-types" suffix rule.
var allowedExact = map[string]bool{
	"openits-types":       true,
	"ietf-yang-types":     true,
	"openits-nema-common": true,
}

// This lint used to carry a narrow, named baseline exception ("grandfathered")
// for openits-rsu-events, whose two pre-existing layering violations
// (importing openits-v2x-messaging for srm-request-type, openits-v2x-radio
// for dsrc-channel) predated the lint. That exception — and the Note type
// used to report a skipped-but-visible grandfathered import — has been
// removed: those two identities (plus channel-fault-type and
// v2x-message-type) moved into openits-v2x-radio-types /
// openits-v2x-messaging-types, so openits-rsu-events now imports only
// "-types" modules and needs no exception. Every disallowed import is
// enforced uniformly again.

// Violation is one *-events.yang module importing a module the layering
// rule disallows.
type Violation struct {
	// Module is the events module's basename with the ".yang" extension
	// stripped, e.g. "openits-rsu-events".
	Module string
	// Import is the disallowed imported module name, e.g. "openits-rsu".
	Import string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s.yang imports %s", v.Module, v.Import)
}

func main() {
	dir := "yang"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	violations, fileCount, err := CheckDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "check-events-layering: %v\n", err)
		os.Exit(2)
	}

	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Fprintln(os.Stderr, v.String())
		}
		fmt.Fprintf(os.Stderr, "check-events-layering: %d violation(s) across %d file(s)\n",
			len(violations), fileCount)
		os.Exit(1)
	}

	fmt.Printf("check-events-layering: OK — %d *-events.yang file(s), 0 violations\n", fileCount)
}

// CheckDir globs dir for *-events.yang files, extracts each one's imports,
// and returns every layering violation found (sorted by file, then import
// order within the file), and the number of *-events.yang files scanned.
//
// A non-nil error means the scan itself could not complete (bad dir, a
// file that couldn't be read) — not that a violation was found. A dir with
// no *-events.yang files is a legitimate empty state, not an error.
func CheckDir(dir string) ([]Violation, int, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*-events.yang"))
	if err != nil {
		return nil, 0, fmt.Errorf("glob %s: %w", dir, err)
	}
	sort.Strings(files)

	var violations []Violation
	for _, f := range files {
		imports, err := extractImports(f)
		if err != nil {
			return nil, 0, fmt.Errorf("read %s: %w", f, err)
		}
		modName := strings.TrimSuffix(filepath.Base(f), ".yang")
		violations = append(violations, classifyModule(modName, imports)...)
	}
	return violations, len(files), nil
}

// classifyModule reports modName's disallowed imports (per
// disallowedImports) as violations, in import order.
func classifyModule(modName string, imports []string) []Violation {
	var violations []Violation
	for _, imp := range disallowedImports(imports) {
		violations = append(violations, Violation{Module: modName, Import: imp})
	}
	return violations
}

// extractImports scans path line by line for `import <name> {` statements
// and returns the imported module names in file order.
func extractImports(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var imports []string
	for _, line := range strings.Split(string(data), "\n") {
		if m := importRE.FindStringSubmatch(line); m != nil {
			imports = append(imports, m[1])
		}
	}
	return imports, nil
}

// disallowedImports returns the subset of imports that violate the
// layering rule, preserving their original order.
func disallowedImports(imports []string) []string {
	var bad []string
	for _, imp := range imports {
		if !isAllowedImport(imp) {
			bad = append(bad, imp)
		}
	}
	return bad
}

// isAllowedImport reports whether name may be imported by a *-events.yang
// module under the layering rule: one of the exact shared/base modules, or
// any module whose name ends in "-types" (those modules carry only leaf
// typedefs/identities/groupings-of-leaves, never config/state schema, so
// depending on one doesn't pull an events module into a core's dependency
// graph).
func isAllowedImport(name string) bool {
	if allowedExact[name] {
		return true
	}
	return strings.HasSuffix(name, "-types")
}
