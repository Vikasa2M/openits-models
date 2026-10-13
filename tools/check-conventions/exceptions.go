package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Exception is one known violation, listed with the family cut expected to
// remove it. Cut "permanent" is for a name a cited standard fixes, and
// needs a reason.
type Exception struct {
	Rule   string `yaml:"rule"`
	Module string `yaml:"module"`
	Path   string `yaml:"path"`
	Cut    string `yaml:"cut"`
	Reason string `yaml:"reason,omitempty"`
}

func (x Exception) key() string { return x.Rule + "\x00" + x.Module + "\x00" + x.Path }

type exceptionsFile struct {
	Exceptions []Exception `yaml:"exceptions"`
}

const exceptionsHeader = `# Known violations of the house-contract conventions, checked by
# tools/check-conventions (make check-conventions). This list only shrinks:
# the gate fails on any violation not listed here and on any entry that no
# longer violates. Each family cut deletes its own entries. "cut: permanent"
# is for a name a cited standard fixes and must carry a reason.
`

// readExceptions loads the exceptions file. A missing file is an empty list.
func readExceptions(path string) ([]Exception, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var f exceptionsFile
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	dup := map[string]bool{}
	for _, x := range f.Exceptions {
		if dup[x.key()] {
			return nil, fmt.Errorf("%s: duplicate exception %s %s %s", path, x.Rule, x.Module, x.Path)
		}
		dup[x.key()] = true
		if x.Cut == "" {
			return nil, fmt.Errorf("%s: %s %s %s has no cut", path, x.Rule, x.Module, x.Path)
		}
		if x.Cut == "permanent" && strings.TrimSpace(x.Reason) == "" {
			return nil, fmt.Errorf("%s: permanent exception %s %s %s needs a reason", path, x.Rule, x.Module, x.Path)
		}
	}
	return f.Exceptions, nil
}

// writeExceptions rewrites the file to list exactly vs, keeping the cut and
// reason of entries already present.
func writeExceptions(path string, vs []Violation) error {
	old, err := readExceptions(path)
	if err != nil {
		return err
	}
	kept := map[string]Exception{}
	for _, x := range old {
		kept[x.key()] = x
	}
	var f exceptionsFile
	for _, v := range vs {
		x, ok := kept[v.key()]
		if !ok {
			cut := cutFor(v.Module)
			if cut == "" {
				return fmt.Errorf("no family cut known for module %s; add it to cutFor", v.Module)
			}
			x = Exception{Rule: v.Rule, Module: v.Module, Path: v.Path, Cut: cut}
		}
		f.Exceptions = append(f.Exceptions, x)
	}
	b, err := yaml.Marshal(&f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append([]byte(exceptionsHeader), b...), 0o644)
}

// compare splits the current violations into ones not listed (fresh) and
// lists the exceptions that no longer match any violation (stale).
func compare(vs []Violation, exc []Exception) (fresh []Violation, stale []Exception) {
	listed := map[string]bool{}
	for _, x := range exc {
		listed[x.key()] = true
	}
	found := map[string]bool{}
	for _, v := range vs {
		found[v.key()] = true
		if !listed[v.key()] {
			fresh = append(fresh, v)
		}
	}
	for _, x := range exc {
		if !found[x.key()] {
			stale = append(stale, x)
		}
	}
	return fresh, stale
}

// checkGrowth enforces shrink-only: it fails when the exceptions file
// lists an entry the base branch's file does not.
func checkGrowth(cur, base []Exception) error {
	inBase := map[string]bool{}
	for _, x := range base {
		inBase[x.key()] = true
	}
	var grown []string
	for _, x := range cur {
		if !inBase[x.key()] {
			grown = append(grown, fmt.Sprintf("%s %s %s", x.Rule, x.Module, x.Path))
		}
	}
	if len(grown) > 0 {
		return fmt.Errorf("exceptions file grew: %s is not in the base", strings.Join(grown, "; "))
	}
	return nil
}

// cutFor names the family cut expected to remove a violation in module, or
// "" when the module maps to no cut.
func cutFor(module string) string {
	m := strings.TrimPrefix(module, "openits-")
	if rest, ok := strings.CutPrefix(m, "vendor-"); ok {
		// vendor-<vendor>-<service...>: the cut is the service's.
		if i := strings.Index(rest, "-"); i >= 0 {
			return cutFor("openits-" + rest[i+1:])
		}
		return ""
	}
	m = strings.TrimSuffix(m, "-types")
	m = strings.TrimSuffix(m, "-events")
	switch {
	case m == "types", strings.HasPrefix(m, "common-"), m == "device-diagnostics",
		m == "cabinet-power", m == "schedule", m == "nema-common":
		return "foundation"
	case strings.HasPrefix(m, "v2x-"), m == "scms", m == "rsu", m == "vehicle-detection":
		return "rsu"
	case m == "traffic-sensor", m == "perception", m == "zone-occupancy":
		return "sensing"
	case m == "signal-control", m == "reversible-lane", m == "work-zone", m == "dms",
		m == "cctv", m == "ramp-metering", m == "ess":
		return m
	}
	return ""
}

func sortViolations(vs []Violation) {
	sort.Slice(vs, func(i, j int) bool { return vs[i].key() < vs[j].key() })
}
