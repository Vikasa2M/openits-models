package main

// Version half of the gate: openits-version is the module's compatibility
// signal, the way openconfig-version is in OpenConfig, so a stamp that exists
// but never has to move tells a consumer nothing. Against the base branch:
//
//  1. A module whose newest revision changed must raise its openits-version.
//     OpenConfig's models-ci enforces the same rule for openconfig-version.
//  2. The raise must be at least as large as the wire change calls for. The
//     module's wire shape (api/shapes/<module>.shape) is diffed against the
//     base: a removed or altered field or enum value is breaking (MAJOR from
//     1.0 on, MINOR before it, where semver lets 0.x break on MINOR); a new
//     field, message or enum value is additive (at least MINOR); no wire
//     change allows a PATCH. This is the counterpart of OpenConfig's ocdiff
//     report, mechanized for the wire. YANG-only changes that alter meaning
//     without altering the wire (a tighter range or must) are left to review.
//  3. MAJOR tracks the ce-type major of every event the module defines
//     (schema-registry/index.json): a 0.x or 1.x module emits .v1, a 2.x
//     module .v2, and so on.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var versionStamp = regexp.MustCompile(`openits-version\s+"([^"]*)"`)
var semverRE = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)
var ceMajorRE = regexp.MustCompile(`\.v(\d+)$`)

type semver struct{ major, minor, patch int }

func (v semver) String() string { return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch) }

func parseSemver(s string) (semver, error) {
	m := semverRE.FindStringSubmatch(s)
	if m == nil {
		return semver{}, fmt.Errorf("openits-version %q is not MAJOR.MINOR.PATCH", s)
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	c, _ := strconv.Atoi(m[3])
	return semver{a, b, c}, nil
}

func (v semver) less(w semver) bool {
	if v.major != w.major {
		return v.major < w.major
	}
	if v.minor != w.minor {
		return v.minor < w.minor
	}
	return v.patch < w.patch
}

// wireChange classifies a shape diff.
type wireChange int

const (
	wireNone wireChange = iota
	wireAdditive
	wireBreaking
)

func (c wireChange) String() string {
	return [...]string{"no wire change", "an additive wire change", "a breaking wire change"}[c]
}

// requiredBump reports whether going from old to new is a large enough raise
// for change, and if not, what the smallest acceptable version is.
func requiredBump(old, new semver, change wireChange) (ok bool, min semver) {
	switch {
	case change == wireBreaking && old.major >= 1:
		min = semver{old.major + 1, 0, 0}
	case change >= wireAdditive:
		min = semver{old.major, old.minor + 1, 0}
	default:
		min = semver{old.major, old.minor, old.patch + 1}
	}
	return !new.less(min), min
}

// checkVersions applies rules 1-3 to every module present on both base and
// the working tree.
func checkVersions(base, yangDir, shapesDir, indexPath string) ([]string, error) {
	out, err := exec.Command("git", "ls-tree", "-r", "--name-only", base, "--", yangDir).Output()
	if err != nil {
		return nil, fmt.Errorf("listing %s at %s: %w", yangDir, base, err)
	}
	var problems []string
	for path := range strings.FieldsSeq(string(out)) {
		if !strings.HasSuffix(path, ".yang") {
			continue
		}
		head, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		old, err := exec.Command("git", "show", base+":"+path).Output()
		if err != nil {
			return nil, fmt.Errorf("reading %s at %s: %w", path, base, err)
		}
		oldStamp, newStamp := stamp(old), stamp(head)
		if newRev, oldRev := newestRevision(head), newestRevision(old); newRev == oldRev || newStamp == "" {
			continue
		}
		module := strings.TrimSuffix(filepath.Base(path), ".yang")
		nv, err := parseSemver(newStamp)
		if err != nil {
			problems = append(problems, path+": "+err.Error())
			continue
		}
		if oldStamp == "" {
			continue
		}
		ov, err := parseSemver(oldStamp)
		if err != nil {
			continue // a malformed base stamp was base's problem
		}
		change := wireNone
		shapePath := filepath.Join(shapesDir, module+".shape")
		if headShape, err := os.ReadFile(shapePath); err == nil {
			if baseShape, err := exec.Command("git", "show", base+":"+filepath.ToSlash(shapePath)).Output(); err == nil {
				change = classifyShapeChange(string(baseShape), string(headShape))
			}
		}
		if !ov.less(nv) {
			problems = append(problems, fmt.Sprintf("%s: new revision %s but openits-version %s was not raised (base %s)", path, newestRevision(head), nv, ov))
			continue
		}
		if ok, min := requiredBump(ov, nv, change); !ok {
			problems = append(problems, fmt.Sprintf("%s: openits-version %s -> %s is too small for %s in api/shapes/%s.shape; need at least %s", path, ov, nv, change, module, min))
		}
	}
	more, err := ceMajorProblems(yangDir, indexPath)
	if err != nil {
		return nil, err
	}
	problems = append(problems, more...)
	sort.Strings(problems)
	return problems, nil
}

func stamp(text []byte) string {
	if m := versionStamp.FindSubmatch(text); m != nil {
		return string(m[1])
	}
	return ""
}

func newestRevision(text []byte) string {
	newest := ""
	for d := range revisionBlocks(string(text)) {
		if d > newest {
			newest = d
		}
	}
	return newest
}

// ceMajorProblems applies rule 3 to every module in the working tree whose
// current revision the index lists ce-types for.
func ceMajorProblems(yangDir, indexPath string) ([]string, error) {
	raw, err := os.ReadFile(indexPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var idx struct {
		Snapshots map[string]map[string]struct {
			Current bool     `json:"current"`
			CeTypes []string `json:"ceTypes"`
		} `json:"snapshots"`
	}
	if err := json.Unmarshal(raw, &idx); err != nil {
		return nil, fmt.Errorf("reading %s: %w", indexPath, err)
	}
	var problems []string
	for module, revs := range idx.Snapshots {
		for _, s := range revs {
			if !s.Current || len(s.CeTypes) == 0 {
				continue
			}
			path := filepath.Join(yangDir, module+".yang")
			text, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			v, err := parseSemver(stamp(text))
			if err != nil {
				continue // rule 1 reports malformed stamps on changed modules
			}
			want := max(v.major, 1)
			for _, t := range s.CeTypes {
				m := ceMajorRE.FindStringSubmatch(t)
				if m == nil {
					continue
				}
				if got, _ := strconv.Atoi(m[1]); got != want {
					problems = append(problems, fmt.Sprintf("%s: openits-version %s means ce-type major v%d, but %s", path, v, want, t))
				}
			}
		}
	}
	return problems, nil
}

// classifyShapeChange compares two schema.shape bodies declaration by
// declaration. Fields are matched by number and enum values by name; a
// declaration present on only one side is not judged itself, because the
// field that reaches it is.
func classifyShapeChange(base, head string) wireChange {
	b, h := shapeDecls(base), shapeDecls(head)
	change := wireNone
	for name, bm := range b {
		hm, ok := h[name]
		if !ok {
			continue
		}
		for key, line := range bm {
			got, ok := hm[key]
			if !ok || got != line {
				return wireBreaking
			}
		}
		for key := range hm {
			if _, ok := bm[key]; !ok {
				change = wireAdditive
			}
		}
	}
	for name := range h {
		if _, ok := b[name]; !ok {
			change = max(change, wireAdditive)
		}
	}
	return change
}

var (
	declStart = regexp.MustCompile(`^(message|enum) (\S+) \{$`)
	oneofLine = regexp.MustCompile(`^\s+oneof (\S+) \{$`)
	fieldNum  = regexp.MustCompile(`= (\d+)(?: \[.*\])?;$`)
	enumValue = regexp.MustCompile(`^\s+(\S+) = (-?\d+);$`)
)

// shapeDecls parses a shape body into declaration -> member key -> canonical
// member text. Message members are keyed by field number, with any oneof
// recorded in the text; enum members by value name.
func shapeDecls(shape string) map[string]map[string]string {
	decls := map[string]map[string]string{}
	var cur map[string]string
	kind, oneof := "", ""
	for line := range strings.SplitSeq(shape, "\n") {
		if m := declStart.FindStringSubmatch(line); m != nil {
			kind = m[1]
			cur = map[string]string{}
			decls[m[1]+" "+m[2]] = cur
			continue
		}
		if cur == nil {
			continue
		}
		switch {
		case line == "}":
			cur = nil
		case oneofLine.MatchString(line):
			oneof = oneofLine.FindStringSubmatch(line)[1]
		case strings.TrimSpace(line) == "}":
			oneof = ""
		case kind == "enum":
			if m := enumValue.FindStringSubmatch(line); m != nil {
				cur[m[1]] = m[2]
			}
		default:
			if m := fieldNum.FindStringSubmatch(line); m != nil {
				cur[m[1]] = "oneof=" + oneof + " " + strings.TrimSpace(line)
			}
		}
	}
	return decls
}
