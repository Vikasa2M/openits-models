// check-augment-collisions fails when two augments target the same YANG
// node. Co-existing augments at the same node can't both graduate (the
// graduation review checklist requires "no other augment in
// yang/augments/ conflicts"), so detecting collisions early surfaces the
// resolution conversation before three NoIs accumulate.
//
// The tool parses every .yang file under yang/augments/ with goyang and
// resolves each `augment "<path>"` statement's target path to a
// normalized (module-name, node-path) identity: every prefixed segment of
// the path is looked up in that module's own `import ... { prefix ...; }`
// declarations and rewritten to the imported module's real name. This
// matters because two augment modules are each free to pick their own
// local alias for the same imported core module — e.g. one importing
// openits-dms as `prefix dms` and targeting "/dms:sign", another
// importing the same openits-dms as `prefix d` and targeting "/d:sign" —
// and a literal string comparison of "/dms:sign" vs "/d:sign" would miss
// that they claim the identical core node. Normalizing through each
// module's own import map closes that blind spot.
//
// Wired to `make check-augment-collisions`. Exits non-zero when any
// collision is found — this is a CI gate, not just a report.
package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

func main() {
	root := "yang/augments"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	os.Exit(run(root, os.Stdout))
}

// run does the work behind main and returns the process exit code:
//   - 0: augments parsed cleanly and no collisions were found.
//   - 1: augments parsed cleanly but at least one collision was found.
//   - 2: the tool itself could not complete (bad root dir, unreadable
//     file, etc.) — distinct from 1 so CI logs can tell "the check ran
//     and found a problem" apart from "the check couldn't run".
//
// Kept separate from main so tests can drive it directly and assert on
// both the returned code and the printed report, without spawning a
// subprocess or calling os.Exit from within a test binary.
func run(root string, stdout io.Writer) int {
	targets, collisions, err := FindCollisions(root)
	if err != nil {
		fmt.Fprintf(stdout, "check-augment-collisions: %v\n", err)
		return 2
	}

	if len(targets) == 0 {
		fmt.Fprintln(stdout, "check-augment-collisions: no augment files found")
		return 0
	}

	for _, c := range collisions {
		fmt.Fprintf(stdout, "COLLISION: normalized target %s is augmented by:\n", c.Normalized)
		for _, f := range c.Files {
			fmt.Fprintf(stdout, "  - %s\n", f)
		}
		fmt.Fprintln(stdout, "  These augments cannot both graduate. Resolve via the TSC graduation review.")
	}

	if len(collisions) == 0 {
		fmt.Fprintf(stdout, "check-augment-collisions: %d target(s) checked across %d file(s), no collisions\n",
			len(targets), countFiles(targets))
		return 0
	}
	fmt.Fprintf(stdout, "check-augment-collisions: %d collision(s) found among %d target(s)\n", len(collisions), len(targets))
	return 1
}

func countFiles(targets []AugmentTarget) int {
	seen := make(map[string]bool)
	for _, t := range targets {
		seen[t.File] = true
	}
	return len(seen)
}

// AugmentTarget is one `augment "<path>"` statement, resolved to its
// prefix-normalized node identity.
type AugmentTarget struct {
	// File is the basename of the augment .yang file this target came
	// from, e.g. "example-signal-control-vehicle-counts.yang".
	File string
	// RawPath is the path exactly as declared in the augment statement,
	// using that module's own local prefix aliases (e.g. "/dms:sign").
	RawPath string
	// Normalized is RawPath with every segment's prefix resolved
	// through the declaring module's import map to the imported
	// module's real name (e.g. "/openits-dms:sign"). Two
	// AugmentTargets from different files with equal Normalized values
	// claim the same core node and therefore collide.
	Normalized string
}

// Collision is one normalized node path claimed by more than one distinct
// augment file.
type Collision struct {
	Normalized string
	// Files is the sorted, deduped set of augment file basenames that
	// claim Normalized.
	Files []string
}

// FindCollisions parses every .yang file under root and returns every
// resolved augment target plus the collisions among them (targets from 2+
// distinct files sharing the same Normalized path).
//
// Each file is parsed in isolation — a fresh *yang.Modules per file, and
// only that file is Read into it — so this needs no access to the real
// base OpenITS modules and cannot be confused by an unrelated augment
// file's own import aliases. What's needed from each augment module (its
// own `import ... { prefix ...; }` declarations and its own `augment`
// statements) is available directly from goyang's parse; resolving the
// imported module's full schema is not required to compute the
// normalized identity.
//
// A non-nil error means FindCollisions itself could not complete (e.g.
// root doesn't exist, or a .yang file fails to parse) — every augment
// file must be syntactically valid for the collision check to be
// trustworthy, so unlike check-deviations's per-file isolation, a broken
// file here is fatal to the whole run rather than a skippable finding.
func FindCollisions(root string) ([]AugmentTarget, []Collision, error) {
	files, err := listAugmentFiles(root)
	if err != nil {
		return nil, nil, fmt.Errorf("list augment files under %s: %w", root, err)
	}

	var targets []AugmentTarget
	for _, f := range files {
		fileTargets, err := augmentTargetsInFile(f)
		if err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", f, err)
		}
		targets = append(targets, fileTargets...)
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Normalized != targets[j].Normalized {
			return targets[i].Normalized < targets[j].Normalized
		}
		return targets[i].File < targets[j].File
	})

	// Group by normalized path -> deduped, sorted set of contributing
	// files. A path claimed twice by the SAME file (e.g. two `augment`
	// blocks in one module targeting the same node — unusual, but not
	// what this tool is checking for) doesn't count as a collision.
	byPath := make(map[string]map[string]bool)
	for _, t := range targets {
		if byPath[t.Normalized] == nil {
			byPath[t.Normalized] = make(map[string]bool)
		}
		byPath[t.Normalized][t.File] = true
	}

	var collisions []Collision
	for path, fileSet := range byPath {
		if len(fileSet) <= 1 {
			continue
		}
		files := make([]string, 0, len(fileSet))
		for f := range fileSet {
			files = append(files, f)
		}
		sort.Strings(files)
		collisions = append(collisions, Collision{Normalized: path, Files: files})
	}
	sort.Slice(collisions, func(i, j int) bool { return collisions[i].Normalized < collisions[j].Normalized })

	return targets, collisions, nil
}

func listAugmentFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(d.Name()) != ".yang" {
			return nil
		}
		out = append(out, path)
		return nil
	})
	sort.Strings(out)
	return out, err
}

// augmentTargetsInFile parses one augment .yang file and returns one
// AugmentTarget per `augment` statement it declares, normalized through
// that file's own import-prefix map.
func augmentTargetsInFile(path string) ([]AugmentTarget, error) {
	ms := yang.NewModules()
	if err := ms.Read(path); err != nil {
		return nil, err
	}

	m := soleModule(ms)
	if m == nil {
		return nil, fmt.Errorf("no module found in %s", path)
	}

	prefixToModule := importPrefixMap(m)
	base := filepath.Base(path)

	targets := make([]AugmentTarget, 0, len(m.Augment))
	for _, aug := range m.Augment {
		normalized, unresolved := normalizeAugmentPath(aug.Name, prefixToModule)
		if len(unresolved) > 0 {
			return nil, fmt.Errorf(
				"augment %q: prefix(es) %s not declared by any `import ... { prefix ...; }` in %s",
				aug.Name, strings.Join(unresolved, ", "), base)
		}
		targets = append(targets, AugmentTarget{
			File:       base,
			RawPath:    aug.Name,
			Normalized: normalized,
		})
	}
	return targets, nil
}

// soleModule returns the one real *yang.Module a single-file goyang parse
// produced. ms.Modules files every module under both its bare name and
// its "name@revision" alias (both point at the same *yang.Module), so
// this picks the entry whose map key equals the module's own Name,
// mirroring tools/check-deviations's validateOneDeviation.
func soleModule(ms *yang.Modules) *yang.Module {
	for key, m := range ms.Modules {
		if key == m.Name {
			return m
		}
	}
	return nil
}

// importPrefixMap maps every local prefix alias m declares — via its own
// `import ... { prefix ...; }` statements, plus its own self-prefix — to
// the real module name that prefix refers to.
func importPrefixMap(m *yang.Module) map[string]string {
	out := make(map[string]string, len(m.Import)+2)
	// Segments in the target path with no explicit "prefix:" refer to
	// the augmenting module's own namespace (RFC 7950); map the
	// empty-prefix case to the module itself.
	out[""] = m.Name
	for _, imp := range m.Import {
		if imp.Prefix == nil {
			continue
		}
		out[imp.Prefix.Name] = imp.Name
	}
	// The module's own declared prefix always resolves to itself, and must
	// win over any import: set it AFTER the import loop so that a module
	// which (invalidly) reuses its own prefix as an import alias still
	// resolves self-referencing segments to itself. RFC 7950 forbids such
	// a collision, so this is defensive, but it keeps resolution correct
	// regardless of import order.
	if p := m.GetPrefix(); p != "" {
		out[p] = m.Name
	}
	return out
}

// normalizeAugmentPath rewrites every "prefix:node" segment of an
// absolute augment target path (e.g. "/dms:sign/dms:text") by resolving
// its prefix through prefixToModule to the real module name (e.g.
// "/openits-dms:sign/openits-dms:text"). Segments whose prefix isn't in
// prefixToModule are reported back via unresolved (their prefixes) rather
// than silently passed through, so a malformed or stale augment can't
// masquerade as a distinct — or falsely colliding — target.
func normalizeAugmentPath(raw string, prefixToModule map[string]string) (normalized string, unresolved []string) {
	segments := strings.Split(strings.TrimPrefix(strings.TrimSpace(raw), "/"), "/")
	out := make([]string, 0, len(segments))
	seenUnresolved := make(map[string]bool)
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		prefix, node, ok := strings.Cut(seg, ":")
		if !ok {
			// No explicit prefix: the segment belongs to the augmenting
			// module's own namespace (RFC 7950 node-identifier without a
			// prefix means "current module"). Resolve through the
			// module's own self-prefix entry.
			prefix, node = "", seg
		}
		mod, ok := prefixToModule[prefix]
		if !ok {
			if !seenUnresolved[prefix] {
				seenUnresolved[prefix] = true
				unresolved = append(unresolved, prefix)
			}
			continue
		}
		out = append(out, mod+":"+node)
	}
	if len(unresolved) > 0 {
		sort.Strings(unresolved)
		return "", unresolved
	}
	return "/" + strings.Join(out, "/"), nil
}
