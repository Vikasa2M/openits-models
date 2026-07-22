// check-graduation reports per-augment NoI counts, independent-org counts,
// operator presence, and graduation eligibility.
//
// An augment is graduation-eligible when:
//   - ≥ 3 distinct canonical organisations have filed NoIs against it
//   - at least one of those NoIs is implementer_type: operator
//   - all NoIs reference the same revision
//
// "Canonical organisation" collapses NoI implementer ids via
// schema-registry/notices/organizations.yaml. Two NoIs from different
// aliases of the same parent count once.
//
// Wired to `make check-graduation`.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// revisionRe matches a YANG `revision YYYY-MM-DD` statement.
var revisionRe = regexp.MustCompile(`(?m)^\s*revision\s+(\d{4}-\d{2}-\d{2})`)

// augmentRevisions returns the set of revision dates the contribution's YANG
// module declares. A Tier-2 contribution is either an augment module under
// yang/augments/ or an identity-only vendor module under yang/ (see
// docs/06-extension-model.md); try both. ok is false when neither exists.
func augmentRevisions(name string) (revs map[string]bool, ok bool) {
	for _, p := range []string{
		filepath.Join("yang", "augments", name+".yang"),
		filepath.Join("yang", name+".yang"),
	} {
		body, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		revs = make(map[string]bool)
		for _, m := range revisionRe.FindAllStringSubmatch(string(body), -1) {
			revs[m[1]] = true
		}
		return revs, true
	}
	return nil, false
}

type noi struct {
	Augment         string `yaml:"augment"`
	Revision        string `yaml:"revision"`
	Implementer     string `yaml:"implementer"`
	ImplementerType string `yaml:"implementer_type"`
}

type orgRegistry struct {
	Organizations []orgEntry `yaml:"organizations"`
}

type orgEntry struct {
	ID      string   `yaml:"id"`
	Aliases []string `yaml:"aliases"`
}

func (r *orgRegistry) canonical(implementer string) string {
	for _, o := range r.Organizations {
		if o.ID == implementer {
			return o.ID
		}
		for _, a := range o.Aliases {
			if a == implementer {
				return o.ID
			}
		}
	}
	// Implementer not in registry — treat its id as canonical.
	return implementer
}

type augmentSummary struct {
	Augment      string
	Revisions    map[string]bool
	Orgs         map[string]bool // canonical org ids
	Operators    map[string]bool // canonical org ids that are operators
	Total        int
	MissingFile  bool     // no yang/augments/<augment>.yang on disk
	BadRevisions []string // NoI revisions absent from the augment file
}

func main() {
	root := "schema-registry/notices"

	registry, err := loadOrgRegistry(filepath.Join(root, "organizations.yaml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "check-graduation: %v\n", err)
		os.Exit(2)
	}

	files, err := findNoIFiles(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "check-graduation: %v\n", err)
		os.Exit(2)
	}

	by := make(map[string]*augmentSummary)
	for _, f := range files {
		var n noi
		body, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", f, err)
			continue
		}
		if err := yaml.Unmarshal(body, &n); err != nil {
			fmt.Fprintf(os.Stderr, "parse %s: %v\n", f, err)
			continue
		}
		s, ok := by[n.Augment]
		if !ok {
			s = &augmentSummary{
				Augment:   n.Augment,
				Revisions: make(map[string]bool),
				Orgs:      make(map[string]bool),
				Operators: make(map[string]bool),
			}
			by[n.Augment] = s
		}
		canonical := registry.canonical(n.Implementer)
		s.Orgs[canonical] = true
		s.Revisions[n.Revision] = true
		s.Total++
		if n.ImplementerType == "operator" {
			s.Operators[canonical] = true
		}
	}

	// Verify each NoI's declared revision actually exists in the augment's
	// YANG file — an NoI pinning a nonexistent (e.g. superseded) revision
	// must not be counted toward graduation.
	for name, s := range by {
		revs, ok := augmentRevisions(name)
		if !ok {
			s.MissingFile = true
			continue
		}
		for r := range s.Revisions {
			if !revs[r] {
				s.BadRevisions = append(s.BadRevisions, r)
			}
		}
		sort.Strings(s.BadRevisions)
	}

	report(by)
}

func report(by map[string]*augmentSummary) {
	if len(by) == 0 {
		fmt.Println("check-graduation: no NoIs filed yet")
		return
	}

	// Sort augments alphabetically for stable output.
	names := make([]string, 0, len(by))
	for n := range by {
		names = append(names, n)
	}
	sort.Strings(names)

	const minOrgs = 3
	fmt.Printf("%-50s %5s %6s %8s %s\n", "Augment", "NoIs", "Orgs", "Operator", "Eligible")
	fmt.Println("---------------------------------------------------------------------------------------------")
	for _, n := range names {
		s := by[n]
		ops := "no"
		if len(s.Operators) > 0 {
			ops = "yes"
		}
		eligible := "NO"
		var reason string
		switch {
		case s.MissingFile:
			reason = "augment .yang not found under yang/augments/"
		case len(s.BadRevisions) > 0:
			reason = "NoI revision(s) not in the augment: " + strings.Join(s.BadRevisions, ", ")
		case len(s.Orgs) < minOrgs:
			reason = fmt.Sprintf("need %d more org(s)", minOrgs-len(s.Orgs))
		case len(s.Operators) == 0:
			reason = "need 1 operator NoI"
		case len(s.Revisions) > 1:
			reason = fmt.Sprintf("NoIs reference %d revisions; need single", len(s.Revisions))
		default:
			eligible = "YES"
		}
		summary := eligible
		if reason != "" {
			summary = eligible + " (" + reason + ")"
		}
		fmt.Printf("%-50s %5d %6d %8s %s\n", n, s.Total, len(s.Orgs), ops, summary)
	}
}

func loadOrgRegistry(path string) (*orgRegistry, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		// Missing organizations.yaml is non-fatal; treat every implementer
		// id as its own canonical org.
		if os.IsNotExist(err) {
			return &orgRegistry{}, nil
		}
		return nil, err
	}
	var r orgRegistry
	if err := yaml.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func findNoIFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "_schema" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(d.Name()) != ".yaml" {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if filepath.Dir(rel) == "." {
			return nil
		}
		out = append(out, path)
		return nil
	})
	sort.Strings(out)
	return out, err
}
