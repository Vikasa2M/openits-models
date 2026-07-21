package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

// index.json is a neutral, machine-readable self-description of the OpenITS
// standard: one plain-JSON document that any consumer (the open-its.org
// website, vendors, third-party tooling) can read with ZERO knowledge of any
// particular consumer, to discover which services and modules exist, what
// event ce-types each service emits, which revisions have been published, and
// where every revision's registry snapshot lives. It is the second neutral
// emitter alongside asyncapi.go — same load-derive-marshal shape, same
// byte-stability discipline — but it targets discovery/tooling rather than the
// NATS event API, so it belongs beside the registry it indexes
// (schema-registry/index.json) rather than in the NATS binding profile.
//
// Two hard invariants (mirroring EmitAsyncAPI's determinism contract):
//   - NO timestamps and NO ordering nondeterminism. Every array is sorted;
//     every map is marshaled by encoding/json (which sorts object keys) or
//     from an explicitly-sorted source. Two regenerations over the same
//     yang/ + schema-registry/ inputs are byte-identical, so `make check-gen`
//     can gate the committed file.
//   - It is GENERATED, never hand-edited. schema-registry/index.json carries
//     the same DO-NOT-EDIT contract as every other generated artifact.
const indexStandard = "OpenITS"

// indexFormatVersion is the schema version of index.json's OWN shape (not any
// OpenITS module revision): a consumer keys its parser off this, and it is
// bumped only when the index's structure changes incompatibly. It is a
// constant — never a build timestamp — so it never perturbs byte-stability.
const indexFormatVersion = "1"

// Index is the top-level index.json document.
//
// Field ordering note: encoding/json marshals struct fields in declaration
// order, so the struct layout below IS the on-disk key order — deterministic
// without any post-processing. Registry is a map, which encoding/json emits
// with sorted keys, so it is deterministic too.
type Index struct {
	Standard string `json:"standard"`
	// Version of index.json's own format (see indexFormatVersion).
	IndexVersion string `json:"indexVersion"`
	// One entry per OpenITS service/domain, sorted by slug.
	Services []ServiceIndex `json:"services"`
	// Shared/foundation modules (the types/common/nema layer, platform
	// modules, and vendor/example type modules) — every loaded OpenITS
	// module NOT scoped to a single service. Sorted by module name.
	Foundation []ModuleIndex `json:"foundation"`
	// Registry map: module name -> revision date -> that snapshot's file
	// paths (relative to schema-registry/, i.e. relative to this index's own
	// location, so they resolve directly when served at
	// schemas.open-its.org/index.json). Independent of the current yang/
	// corpus: it is a historical index of every published snapshot, so it
	// legitimately includes modules (e.g. signal-control's former per-domain
	// event modules) that have since been folded away in yang/.
	Registry map[string]map[string][]string `json:"registry"`
}

// ServiceIndex is one service/domain's public descriptor: identity + a
// per-module breakdown plus service-level rollups.
//
// Revisions aggregation decision: this project versions each composing module
// (core / -types / -events) on its OWN revision cadence — a -types change
// bumps only the -types module, not the core — so there is no single "service
// revision". The honest representation is therefore per-module: Modules
// carries {name, namespace, revisions, refStd} for each composing module, and
// the service-level Revisions/RefStd fields are a deduped, sorted ROLLUP
// (union) across those modules, offered as a convenience for consumers that
// want a coarse "what dates has anything in this service moved" view without
// walking the sub-list. Namespace/Description come from the service's CORE
// module (openits-<slug>), the module a consumer means by "the dms model".
type ServiceIndex struct {
	Slug        string `json:"slug"`
	Namespace   string `json:"namespace"`
	Description string `json:"description"`
	// Union of every composing module's revisions, deduped + sorted ascending.
	Revisions []string `json:"revisions"`
	// The CloudEvents ce-type strings this service emits (e.g.
	// "openits.dms.fault-raised.v1"), from the YANG-derived catalog
	// (BuildCatalog), sorted.
	Events []string `json:"events"`
	// Union of every composing module's normative references, deduped + sorted.
	RefStd []string `json:"refStd"`
	// Per-composing-module metadata, sorted by module name.
	Modules []ModuleIndex `json:"modules"`
}

// ModuleIndex is one YANG module's public descriptor. refStd is the module's
// normative-reference set (see moduleRefStd); revisions are its declared
// revision dates, sorted ascending.
type ModuleIndex struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Revisions []string `json:"revisions"`
	RefStd    []string `json:"refStd"`
}

// BuildIndex assembles the Index from the loaded YANG corpus, the derived
// ce-type catalog, and the on-disk registry tree. registryDir is the
// schema-registry/ directory; its paths in the returned Index are relative to
// it (see Index.Registry). mods is LoadModules's output; cat is BuildCatalog's.
func BuildIndex(mods []*yang.Entry, cat []CeType, registryDir string) (*Index, error) {
	// Group every loaded OpenITS module by the service it is scoped to (or
	// "" for foundation). Only openits-* modules are described — vendored
	// ietf-* imports are not part of the standard's published surface.
	byService := map[string][]*yang.Entry{}
	var foundation []*yang.Entry
	for _, m := range mods {
		if !strings.HasPrefix(m.Name, "openits-") {
			continue
		}
		if slug, ok := moduleServiceSlug(m.Name); ok {
			byService[slug] = append(byService[slug], m)
		} else {
			foundation = append(foundation, m)
		}
	}

	// ce-types grouped by service slug.
	eventsBySvc := map[string][]string{}
	for _, c := range cat {
		eventsBySvc[c.Service] = append(eventsBySvc[c.Service], c.Type)
	}

	byName := make(map[string]*yang.Entry, len(mods))
	for _, m := range mods {
		byName[m.Name] = m
	}

	services := make([]ServiceIndex, 0, len(servicesList()))
	for _, slug := range servicesList() {
		core, ok := byName["openits-"+slug]
		if !ok {
			// A service in the catalog's service map with no core module is
			// a taxonomy regression — fail loudly rather than emit a
			// service descriptor with an empty namespace/description.
			return nil, fmt.Errorf("build index: service %q has no core module openits-%s in the loaded corpus", slug, slug)
		}

		modEntries := byService[slug]
		sort.Slice(modEntries, func(i, j int) bool { return modEntries[i].Name < modEntries[j].Name })

		modules := make([]ModuleIndex, 0, len(modEntries))
		revSet := map[string]bool{}
		refSet := map[string]bool{}
		for _, m := range modEntries {
			mi := moduleIndex(m)
			modules = append(modules, mi)
			for _, r := range mi.Revisions {
				revSet[r] = true
			}
			for _, r := range mi.RefStd {
				refSet[r] = true
			}
		}

		events := append([]string(nil), eventsBySvc[slug]...)
		sort.Strings(events)

		services = append(services, ServiceIndex{
			Slug:        slug,
			Namespace:   moduleNamespace(core),
			Description: moduleDescription(core),
			Revisions:   sortedKeys(revSet),
			Events:      nonNil(events),
			RefStd:      sortedKeys(refSet),
			Modules:     modules,
		})
	}
	// servicesList() is already sorted, but sort defensively so the output
	// order never depends on that helper's ordering.
	sort.Slice(services, func(i, j int) bool { return services[i].Slug < services[j].Slug })

	sort.Slice(foundation, func(i, j int) bool { return foundation[i].Name < foundation[j].Name })
	fdn := make([]ModuleIndex, 0, len(foundation))
	for _, m := range foundation {
		fdn = append(fdn, moduleIndex(m))
	}

	registry, err := scanRegistry(registryDir)
	if err != nil {
		return nil, err
	}

	return &Index{
		Standard:     indexStandard,
		IndexVersion: indexFormatVersion,
		Services:     services,
		Foundation:   fdn,
		Registry:     registry,
	}, nil
}

// moduleServiceSlug assigns an OpenITS module name to the service that owns it,
// by longest-matching service slug: the module is either the service core
// ("openits-<slug>") or a service sub-module ("openits-<slug>-..."). Longest
// match matters for the same reason routeServiceSlug documents it — a
// hyphenated slug like "signal-control" must win over any shorter slug that is
// a prefix of it. Returns ("", false) for a foundation/shared module
// (openits-common-*, openits-types, openits-nema-common, openits-vendor-*,
// openits-v2x-*, …) that maps to no single service.
func moduleServiceSlug(moduleName string) (string, bool) {
	best := ""
	for _, svc := range services {
		if moduleName == "openits-"+svc.slug || strings.HasPrefix(moduleName, "openits-"+svc.slug+"-") {
			if len(svc.slug) > len(best) {
				best = svc.slug
			}
		}
	}
	return best, best != ""
}

// servicesList returns the service slugs, sorted, from the catalog's
// authoritative service map.
func servicesList() []string {
	out := make([]string, 0, len(services))
	for _, svc := range services {
		out = append(out, svc.slug)
	}
	sort.Strings(out)
	return out
}

// moduleIndex builds one module's descriptor from its Entry.
func moduleIndex(m *yang.Entry) ModuleIndex {
	return ModuleIndex{
		Name:      m.Name,
		Namespace: moduleNamespace(m),
		Revisions: moduleRevisions(m),
		RefStd:    moduleRefStd(m),
	}
}

// moduleNode returns the *yang.Module AST node backing a module Entry, or nil
// if the Entry isn't module-rooted (should never happen for LoadModules's
// output, but keeps the accessors below nil-safe).
func moduleNode(m *yang.Entry) *yang.Module {
	if m == nil || m.Node == nil {
		return nil
	}
	mod, _ := m.Node.(*yang.Module)
	return mod
}

// moduleNamespace returns the module's namespace URN (e.g.
// "urn:openits:yang:dms"), or "" if unset.
func moduleNamespace(m *yang.Entry) string {
	if ns := m.Namespace(); ns != nil {
		return ns.Name
	}
	return ""
}

// moduleDescription returns the module's description text, or "".
func moduleDescription(m *yang.Entry) string {
	return m.Description
}

// moduleRevisions returns the module's declared revision dates, sorted
// ascending. YANG source lists revisions newest-first by convention; the
// output is oldest-first for a stable, human-obvious chronological order.
func moduleRevisions(m *yang.Entry) []string {
	mod := moduleNode(m)
	if mod == nil {
		return []string{}
	}
	revs := make([]string, 0, len(mod.Revision))
	for _, r := range mod.Revision {
		revs = append(revs, r.Name)
	}
	sort.Strings(revs)
	return revs
}

// moduleRefStd returns a module's normative references: the content of its
// top-level `reference` statement plus every revision's `reference`, each
// split on ";", trimmed of surrounding whitespace and a trailing ".", then
// deduped and sorted.
//
// Sourcing decision: module-level and revision-level `reference` statements
// are exactly the normative-standard citations (e.g. "NTCIP 1203 v03",
// "RFC 7950 section 11") — they name the external standards the module
// realizes or the evolution rules it follows. Leaf-level references are
// intentionally NOT aggregated: they are per-node provenance, too granular for
// a service/module-level "what standards does this implement" view, and would
// bloat the set non-deterministically as leaves churn. A module with no
// module- or revision-level reference yields an empty list (a valid,
// deterministic answer), not a fallback scrape of descriptions.
func moduleRefStd(m *yang.Entry) []string {
	mod := moduleNode(m)
	if mod == nil {
		return []string{}
	}
	set := map[string]bool{}
	add := func(v *yang.Value) {
		if v == nil {
			return
		}
		for _, ref := range normalizeRefs(v.Name) {
			set[ref] = true
		}
	}
	add(mod.Reference)
	for _, r := range mod.Revision {
		add(r.Reference)
	}
	return sortedKeys(set)
}

// normalizeRefs splits one `reference` statement's raw text into individual
// normative citations, then normalizes each: collapse internal whitespace runs
// (including the newlines YANG wraps long strings across) to single spaces,
// trim, drop a single trailing ".", and drop empties.
//
// Citations within one statement are ";"-separated by convention (e.g.
// "NTCIP 1207 (ramp metering); MUTCD 11th ed. 4F.17"), but a ";" INSIDE
// parentheses is part of a single citation's parenthetical, not a separator
// (e.g. "NTCIP 1203:2010 (DMS functional reference; fault classes)" is ONE
// reference). So the split is parenthesis-depth aware: it only breaks on a
// top-level (depth-0) ";".
func normalizeRefs(raw string) []string {
	var out []string
	for _, part := range splitTopLevelSemicolons(raw) {
		part = strings.Join(strings.Fields(part), " ")
		part = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(part), "."))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// splitTopLevelSemicolons splits s on ";" characters that are not nested
// inside parentheses. Depth never goes negative (a stray ")" is treated as
// depth 0), so malformed input still splits sensibly rather than panicking.
func splitTopLevelSemicolons(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ';':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// scanRegistry walks registryDir (schema-registry/) and returns
// module -> revision-date -> sorted list of that snapshot's file paths
// (relative to registryDir). Every top-level entry that is a directory and is
// not "notices" is treated as a module directory; each of its date-named
// subdirectories is a revision snapshot. A top-level FILE (index.json itself,
// index.html) is skipped simply by not being a directory, so the index never
// indexes itself.
func scanRegistry(registryDir string) (map[string]map[string][]string, error) {
	registry := map[string]map[string][]string{}

	modEnts, err := os.ReadDir(registryDir)
	if err != nil {
		return nil, fmt.Errorf("build index: read registry dir %s: %w", registryDir, err)
	}
	for _, modEnt := range modEnts {
		if !modEnt.IsDir() || modEnt.Name() == "notices" {
			continue
		}
		module := modEnt.Name()

		revEnts, err := os.ReadDir(filepath.Join(registryDir, module))
		if err != nil {
			return nil, fmt.Errorf("build index: read module dir %s: %w", module, err)
		}
		revisions := map[string][]string{}
		for _, revEnt := range revEnts {
			if !revEnt.IsDir() {
				continue
			}
			revision := revEnt.Name()
			revDir := filepath.Join(registryDir, module, revision)
			fileEnts, err := os.ReadDir(revDir)
			if err != nil {
				return nil, fmt.Errorf("build index: read revision dir %s/%s: %w", module, revision, err)
			}
			var files []string
			for _, fe := range fileEnts {
				if fe.IsDir() {
					continue
				}
				// Paths are relative to registryDir (this index's own
				// location) so they resolve directly when served alongside it.
				files = append(files, module+"/"+revision+"/"+fe.Name())
			}
			sort.Strings(files)
			if len(files) > 0 {
				revisions[revision] = files
			}
		}
		if len(revisions) > 0 {
			registry[module] = revisions
		}
	}
	return registry, nil
}

// sortedKeys returns a set's keys as a sorted, non-nil slice.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// nonNil returns s, or an empty non-nil slice if s is nil, so it marshals as
// [] rather than null.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// MarshalIndex renders idx as deterministic, pretty-printed JSON with a
// trailing newline. encoding/json marshals struct fields in declaration order
// and map keys in sorted order, and every slice in idx is pre-sorted, so the
// output is byte-stable across runs.
func MarshalIndex(idx *Index) ([]byte, error) {
	body, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal index: %w", err)
	}
	return append(body, '\n'), nil
}

// GenerateIndex loads every YANG module under yangDir, derives the ce-type
// catalog (BuildCatalog), scans registryDir for the snapshot map, and writes
// the neutral self-index to registryDir/index.json. registryDir doubles as
// both the directory scanned for the registry map and the output location, so
// the index lands beside the registry it describes. Mirrors GenerateAsyncAPI's
// load-derive-write shape (asyncapi.go) — this is the function the -catalog
// CLI flag calls.
func GenerateIndex(yangDir, registryDir string) error {
	ms, mods, err := LoadModules(yangDir)
	if err != nil {
		return fmt.Errorf("generate index: load yang modules from %s: %w", yangDir, err)
	}

	cat, err := BuildCatalog(ms, mods)
	if err != nil {
		return fmt.Errorf("generate index: build catalog: %w", err)
	}

	idx, err := BuildIndex(mods, cat, registryDir)
	if err != nil {
		return fmt.Errorf("generate index: %w", err)
	}

	body, err := MarshalIndex(idx)
	if err != nil {
		return fmt.Errorf("generate index: %w", err)
	}

	if err := os.MkdirAll(registryDir, 0o755); err != nil {
		return fmt.Errorf("generate index: mkdir %s: %w", registryDir, err)
	}
	outPath := filepath.Join(registryDir, "index.json")
	if err := os.WriteFile(outPath, body, 0o644); err != nil {
		return fmt.Errorf("generate index: write %s: %w", outPath, err)
	}
	return nil
}
