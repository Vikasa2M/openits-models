// check-mode-kinds binds each family's mode-changed kind to that family's
// mode vocabulary. The shared openits-common-mode-events:mode-changed
// notification types previous-mode and mode as identityrefs based on the
// cross-service openits-types:operational-mode, so YANG accepts a DMS mode
// value under a signal-control kind. The openits-types:mode-value-base
// extension on each family's mode-event-kind names the identity those
// values must derive from; this gate enforces it:
//
//	(a) every identity derived from openits-types:mode-event-kind (other
//	    than the base itself and the allowlist below) carries exactly one
//	    mode-value-base, naming an identity strictly derived from
//	    openits-types:operational-mode; the extension appears on nothing
//	    else; and every allowlist entry is still needed.
//	(b) every <fixtures>/valid-*.json whose top-level element is a
//	    mode-changed carries a mode (and, if present, a previous-mode) that
//	    strictly derives from the base its kind names (the abstract base
//	    itself is not a value).
//	(c) every <invalid>/invalid-*.json is a mode-changed fixture that FAILS
//	    rule (b). These fixtures are schema-valid, so they live outside the
//	    directory yanglint scans and this gate is what rejects them.
//
// Wired to `make check-mode-kinds`.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

const (
	modeEventKind   = "openits-types:mode-event-kind"
	operationalMode = "openits-types:operational-mode"
	extModule       = "openits-types"
	extName         = "mode-value-base"
	extDisplay      = extModule + ":" + extName
)

// allowlist names the mode-event-kinds that may lack a mode-value-base,
// each with the reason. Entries are removed as the family cuts land; an
// entry that is no longer needed fails rule (a).
var allowlist = map[string]string{
	"openits-cctv-types:cctv-mode-event-kind": "CCTV mode values are not operational-mode identities; reconciled in the CCTV cut",
}

func main() {
	yangDir := flag.String("yang", "yang", "directory of YANG modules")
	fixtures := flag.String("fixtures", "yang/testdata", "directory whose valid-*.json fixtures are checked (rule b)")
	invalid := flag.String("invalid", "yang/testdata/mode-kinds", "directory of invalid-*.json mode-changed fixtures that must fail (rule c)")
	flag.Parse()
	if err := run([]string{*yangDir}, *fixtures, *invalid, allowlist, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "check-mode-kinds:", err)
		os.Exit(1)
	}
}

func run(yangDirs []string, fixturesDir, invalidDir string, allow map[string]string, w io.Writer) error {
	s, err := LoadSchema(yangDirs...)
	if err != nil {
		return err
	}
	var problems []string
	problems = append(problems, s.CheckKinds(allow)...)

	valid, err := filepath.Glob(filepath.Join(fixturesDir, "valid-*.json"))
	if err != nil {
		return err
	}
	sort.Strings(valid)
	checked := 0
	for _, f := range valid {
		applies, vs, err := s.CheckFixture(f)
		if err != nil {
			return err
		}
		if applies {
			checked++
		}
		for _, v := range vs {
			problems = append(problems, fmt.Sprintf("%s: %s", f, v))
		}
	}

	invalid, err := filepath.Glob(filepath.Join(invalidDir, "invalid-*.json"))
	if err != nil {
		return err
	}
	sort.Strings(invalid)
	if len(invalid) == 0 {
		problems = append(problems, fmt.Sprintf("%s: no invalid-mode-changed-*.json fixtures (rule c has nothing to check)", invalidDir))
	}
	for _, f := range invalid {
		applies, vs, err := s.CheckFixture(f)
		if err != nil {
			return err
		}
		switch {
		case !applies:
			problems = append(problems, fmt.Sprintf("%s is not a mode-changed fixture", f))
		case len(vs) == 0:
			problems = append(problems, fmt.Sprintf("%s passes the mode-value-base check but is an invalid fixture", f))
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("%d violation(s):\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
	fmt.Fprintf(w, "check-mode-kinds: OK — %d mode kind(s) (%d allowlisted), %d valid mode-changed fixture(s), %d invalid fixture(s) rejected\n",
		len(s.modeKinds()), len(allow), checked, len(invalid))
	return nil
}

// Ident is one identity, keyed "<module>:<name>".
type Ident struct {
	Key        string
	Bases      []string // resolved keys; unresolvable bases are dropped
	ValueBases []string // mode-value-base arguments that resolved
	Unresolved []string // mode-value-base arguments that did not resolve
}

// Schema is the identity graph of a set of YANG modules.
type Schema struct {
	ids map[string]*Ident
}

// LoadSchema parses every .yang file directly in dirs. It does not run
// goyang's Process: only identity statements and module prefixes are
// needed, so modules this gate does not load (ietf imports) are no
// obstacle.
func LoadSchema(dirs ...string) (*Schema, error) {
	ms := yang.NewModules()
	for _, d := range dirs {
		files, err := filepath.Glob(filepath.Join(d, "*.yang"))
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("no .yang files in %s", d)
		}
		sort.Strings(files)
		for _, f := range files {
			if err := ms.Read(f); err != nil {
				return nil, fmt.Errorf("reading %s: %w", f, err)
			}
		}
	}
	s := &Schema{ids: map[string]*Ident{}}
	names := make([]string, 0, len(ms.Modules))
	for name := range ms.Modules {
		if !strings.Contains(name, "@") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		m := ms.Modules[name]
		prefixes := map[string]string{}
		if m.Prefix != nil {
			prefixes[m.Prefix.Name] = m.Name
		}
		for _, imp := range m.Import {
			if imp.Prefix != nil {
				prefixes[imp.Prefix.Name] = imp.Name
			}
		}
		resolve := func(ref string) (string, bool) {
			pfx, local, ok := strings.Cut(ref, ":")
			if !ok {
				return m.Name + ":" + ref, true
			}
			mod, ok := prefixes[pfx]
			if !ok {
				return "", false
			}
			return mod + ":" + local, true
		}
		for _, id := range m.Identity {
			ident := &Ident{Key: m.Name + ":" + id.Name}
			for _, b := range id.Base {
				if k, ok := resolve(b.Name); ok {
					ident.Bases = append(ident.Bases, k)
				}
			}
			for _, ext := range id.Extensions {
				pfx, kw, ok := strings.Cut(ext.Keyword, ":")
				if !ok || kw != extName || prefixes[pfx] != extModule {
					continue
				}
				if k, ok := resolve(ext.Argument); ok {
					ident.ValueBases = append(ident.ValueBases, k)
				} else {
					ident.Unresolved = append(ident.Unresolved, ext.Argument)
				}
			}
			s.ids[ident.Key] = ident
		}
	}
	return s, nil
}

// DerivedFromOrSelf reports whether identity id is anc or transitively
// derives from it.
func (s *Schema) DerivedFromOrSelf(id, anc string) bool {
	seen := map[string]bool{}
	var walk func(string) bool
	walk = func(k string) bool {
		if k == anc {
			return true
		}
		if seen[k] {
			return false
		}
		seen[k] = true
		if i := s.ids[k]; i != nil {
			for _, b := range i.Bases {
				if walk(b) {
					return true
				}
			}
		}
		return false
	}
	return walk(id)
}

// modeKinds returns every identity strictly derived from mode-event-kind.
func (s *Schema) modeKinds() []string {
	var out []string
	for k := range s.ids {
		if k != modeEventKind && s.DerivedFromOrSelf(k, modeEventKind) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// CheckKinds applies rule (a).
func (s *Schema) CheckKinds(allow map[string]string) []string {
	var vs []string
	kinds := map[string]bool{}
	for _, k := range s.modeKinds() {
		kinds[k] = true
		id := s.ids[k]
		n := len(id.ValueBases) + len(id.Unresolved)
		if _, ok := allow[k]; ok {
			if n > 0 {
				vs = append(vs, fmt.Sprintf("%s is allowlisted but declares %s; remove the allowlist entry", k, extDisplay))
			}
			continue
		}
		switch {
		case n == 0:
			vs = append(vs, fmt.Sprintf("%s has no %s naming its family's mode base", k, extDisplay))
			continue
		case n > 1:
			vs = append(vs, fmt.Sprintf("%s has %d %s statements; exactly one is allowed", k, n, extDisplay))
			continue
		}
		if len(id.Unresolved) == 1 {
			vs = append(vs, fmt.Sprintf("%s: %s %q does not resolve to an imported module prefix", k, extDisplay, id.Unresolved[0]))
			continue
		}
		base := id.ValueBases[0]
		switch {
		case s.ids[base] == nil:
			vs = append(vs, fmt.Sprintf("%s: %s %s is not a defined identity", k, extDisplay, base))
		case base == operationalMode || !s.DerivedFromOrSelf(base, operationalMode):
			vs = append(vs, fmt.Sprintf("%s: %s %s is not derived from %s", k, extDisplay, base, operationalMode))
		}
	}
	for _, k := range sortedKeys(s.ids) {
		id := s.ids[k]
		if !kinds[k] && len(id.ValueBases)+len(id.Unresolved) > 0 {
			vs = append(vs, fmt.Sprintf("%s is not a mode-event-kind but carries %s", k, extDisplay))
		}
	}
	for _, k := range sortedKeys(allow) {
		if !kinds[k] {
			vs = append(vs, fmt.Sprintf("%s is allowlisted but is not a mode-event-kind; remove the allowlist entry", k))
		}
	}
	return vs
}

// valueBase returns the single resolved mode-value-base of kind, or "".
func (s *Schema) valueBase(kind string) string {
	id := s.ids[kind]
	if id == nil || len(id.ValueBases) != 1 || len(id.Unresolved) != 0 {
		return ""
	}
	return id.ValueBases[0]
}

// CheckFixture applies rule (b) to one JSON fixture. applies is false when
// the fixture's top-level element is not a mode-changed notification.
func (s *Schema) CheckFixture(path string) (applies bool, vs []string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, nil, err
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return false, nil, nil // a top-level array or scalar: not a notification
		}
		return false, nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(top) != 1 {
		return false, nil, nil
	}
	var inst map[string]any
	for k, v := range top {
		if !strings.HasSuffix(k, ":mode-changed") {
			return false, nil, nil
		}
		if err := json.Unmarshal(v, &inst); err != nil {
			return true, []string{"mode-changed is not an object"}, nil
		}
	}
	leaf := func(name string) (string, bool) {
		for k, v := range inst {
			if k == name || strings.HasSuffix(k, ":"+name) {
				str, _ := v.(string)
				return str, true
			}
		}
		return "", false
	}
	kind, _ := leaf("kind")
	if kind == "" {
		return true, []string{"mode-changed has no kind"}, nil
	}
	if s.ids[kind] == nil {
		return true, []string{fmt.Sprintf("kind %s is an unknown identity", kind)}, nil
	}
	base := s.valueBase(kind)
	if base == "" {
		return true, []string{fmt.Sprintf("kind %s names no %s, so no mode value can be checked against it", kind, extDisplay)}, nil
	}
	check := func(name, val string) {
		switch {
		case !strings.Contains(val, ":"):
			vs = append(vs, fmt.Sprintf("%s %q has no module prefix", name, val))
		case s.ids[val] == nil:
			vs = append(vs, fmt.Sprintf("%s %s is an unknown identity", name, val))
		case val == base:
			vs = append(vs, fmt.Sprintf("%s %s is the abstract %s of kind %s, not a mode value derived from it", name, val, extDisplay, kind))
		case !s.DerivedFromOrSelf(val, base):
			vs = append(vs, fmt.Sprintf("%s %s does not derive from %s (the %s of kind %s)", name, val, base, extDisplay, kind))
		}
	}
	if mode, ok := leaf("mode"); !ok || mode == "" {
		vs = append(vs, "mode-changed has no mode")
	} else {
		check("mode", mode)
	}
	if prev, ok := leaf("previous-mode"); ok {
		check("previous-mode", prev)
	}
	return true, vs, nil
}
