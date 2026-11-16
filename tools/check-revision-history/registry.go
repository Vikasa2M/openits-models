package main

// Registry half of the gate: a schema-registry snapshot is the published
// contract of one module revision, so once it exists on the base branch its
// files are immutable, and every snapshot must carry the files its module's
// shape calls for.
//
// Two rules:
//
//  1. Immutable. Every file under schema-registry/<module>/<revision>/ on the
//     base must survive byte-for-byte, and no snapshot on the base may lose a
//     revision directory. A released snapshot may gain a file only when that
//     file is an artifact type introduced after the snapshot was taken (the
//     backfillable set below); it may never gain schema.proto or schema.json,
//     because those describe the wire and a revision that shipped without them
//     shipped without a pinned wire shape. That second half is the case where
//     openits-zone-occupancy-events 2026-08-15 sat with schema.yang alone for
//     two releases and gained schema.proto in a third.
//
//  2. Complete. Every snapshot carries schema.yang, MANIFEST.json and
//     README.md, and a snapshot whose module declares a live (not deprecated
//     or obsolete) top-level notification also carries schema.proto and
//     schema.json. Snapshots that shipped incomplete before this gate existed
//     are listed in grandfathered with the files they lack; the list may only
//     shrink.

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// backfillable lists artifact types introduced after some snapshots were
// already released. Adding one to a released snapshot records a fact about
// that revision without changing any file it shipped with.
var backfillable = map[string]bool{
	"schema.shape": true,
}

// grandfathered lists snapshots that shipped without a file rule 2 requires,
// keyed by "<module>/<revision>". They are immutable, so they cannot be
// completed; the list documents them and stops new ones joining.
var grandfathered = map[string][]string{
	// Taken in the initial import, before the snapshot script emitted
	// schema.proto or README.md for every events module.
	"openits-rsu-events/2026-07-21": {"README.md", "schema.proto"},
}

var snapshotFile = regexp.MustCompile(`^([^/]+)/(\d{4}-\d{2}-\d{2})/([^/]+)$`)

// registryEntry is one file in a snapshot, keyed by its path relative to the
// registry root, with its git blob id.
type registryEntry struct {
	module, revision, name string
	blob                   string
}

// checkRegistry returns one problem line per rule violation.
func checkRegistry(base, registry string) ([]string, int, error) {
	baseFiles, err := baseRegistryFiles(base, registry)
	if err != nil {
		return nil, 0, err
	}
	headFiles, err := headRegistryFiles(registry)
	if err != nil {
		return nil, 0, err
	}
	problems := immutabilityProblems(baseFiles, headFiles)
	problems = append(problems, completenessProblems(registry, headFiles)...)
	sort.Strings(problems)
	return problems, len(baseFiles), nil
}

// baseRegistryFiles lists every snapshot file on base with its blob id.
func baseRegistryFiles(base, registry string) (map[string]registryEntry, error) {
	out, err := exec.Command("git", "ls-tree", "-r", base, "--", registry+"/").Output()
	if err != nil {
		return nil, fmt.Errorf("listing %s at %s: %w", registry, base, err)
	}
	files := map[string]registryEntry{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		// "<mode> blob <sha>\t<path>"
		meta, path, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) != 3 || fields[1] != "blob" {
			continue
		}
		rel := strings.TrimPrefix(path, registry+"/")
		if e, ok := parseSnapshotPath(rel); ok {
			e.blob = fields[2]
			files[rel] = e
		}
	}
	return files, nil
}

// headRegistryFiles lists every snapshot file in the working tree with the
// blob id git would give it.
func headRegistryFiles(registry string) (map[string]registryEntry, error) {
	files := map[string]registryEntry{}
	paths, err := filepath.Glob(filepath.Join(registry, "*", "*", "*"))
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		rel, err := filepath.Rel(registry, p)
		if err != nil {
			return nil, err
		}
		e, ok := parseSnapshotPath(filepath.ToSlash(rel))
		if !ok {
			continue
		}
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		e.blob = blobID(data)
		files[filepath.ToSlash(rel)] = e
	}
	return files, nil
}

func parseSnapshotPath(rel string) (registryEntry, bool) {
	m := snapshotFile.FindStringSubmatch(rel)
	if m == nil {
		return registryEntry{}, false
	}
	return registryEntry{module: m[1], revision: m[2], name: m[3]}, true
}

// blobID is git's object id for a blob with this content.
func blobID(data []byte) string {
	h := sha1.New()
	h.Write([]byte("blob " + strconv.Itoa(len(data)) + "\x00"))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func immutabilityProblems(base, head map[string]registryEntry) []string {
	var problems []string
	baseDirs := map[string]bool{}
	for rel, b := range base {
		baseDirs[b.module+"/"+b.revision] = true
		h, ok := head[rel]
		switch {
		case !ok:
			problems = append(problems, rel+": removed from a released snapshot")
		case h.blob != b.blob:
			problems = append(problems, rel+": changed in a released snapshot")
		}
	}
	for rel, h := range head {
		if _, ok := base[rel]; ok || !baseDirs[h.module+"/"+h.revision] {
			continue
		}
		if !backfillable[h.name] {
			problems = append(problems, rel+": added to a released snapshot (only "+strings.Join(sortedNames(backfillable), ", ")+" may be backfilled)")
		}
	}
	return problems
}

func completenessProblems(registry string, head map[string]registryEntry) []string {
	dirs := map[string]map[string]bool{}
	for _, h := range head {
		key := h.module + "/" + h.revision
		if dirs[key] == nil {
			dirs[key] = map[string]bool{}
		}
		dirs[key][h.name] = true
	}
	var problems []string
	for key, names := range dirs {
		required := []string{"MANIFEST.json", "README.md", "schema.yang"}
		if names["schema.yang"] {
			text, err := os.ReadFile(filepath.Join(registry, key, "schema.yang"))
			if err != nil {
				problems = append(problems, key+": "+err.Error())
				continue
			}
			if hasLiveNotification(text) {
				required = append(required, "schema.json", "schema.proto")
			}
		}
		excused := map[string]bool{}
		for _, n := range grandfathered[key] {
			excused[n] = true
		}
		for _, n := range required {
			if !names[n] && !excused[n] {
				problems = append(problems, key+": missing "+n)
			}
		}
	}
	return problems
}

func sortedNames(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var notificationStart = regexp.MustCompile(`^notification\s+[A-Za-z_][\w.-]*\s*\{`)
var liveStatus = regexp.MustCompile(`(?m)^\s*status\s+"?(deprecated|obsolete)"?\s*;`)

// hasLiveNotification reports whether a module declares a top-level
// notification whose own status is neither deprecated nor obsolete. Only the
// module body's direct children count, matching the generator, and only a
// notification's own status statement counts, not one on a nested leaf.
func hasLiveNotification(text []byte) bool {
	s := string(text)
	open := strings.IndexByte(s, '{')
	if open < 0 {
		return false
	}
	end := matchBrace(s, open)
	if end < 0 {
		return false
	}
	for _, stmt := range childStatements(s, open, end) {
		if !notificationStart.MatchString(stmt) {
			continue
		}
		if !liveStatus.MatchString(directText(stmt)) {
			return true
		}
	}
	return false
}

// childStatements splits the body between the braces at open and end into
// its direct child statements, each from its keyword through its closing
// brace or semicolon. Comments and quoted strings are skipped.
func childStatements(s string, open, end int) []string {
	var out []string
	start := -1
	for i := open + 1; i < end; i++ {
		c := s[i]
		switch {
		case c == '/' && i+1 < end && s[i+1] == '/':
			if nl := strings.IndexByte(s[i:end], '\n'); nl >= 0 {
				i += nl
			} else {
				i = end
			}
			continue
		case c == '/' && i+1 < end && s[i+1] == '*':
			if j := strings.Index(s[i+2:end], "*/"); j >= 0 {
				i += j + 3
			} else {
				i = end
			}
			continue
		}
		if start < 0 {
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
				continue
			}
			start = i
		}
		switch c {
		case '"', '\'':
			if j := skipString(s, i); j >= 0 {
				i = j
			}
		case ';':
			out = append(out, s[start:i+1])
			start = -1
		case '{':
			if j := matchBrace(s, i); j >= 0 {
				out = append(out, s[start:j+1])
				i = j
				start = -1
			}
		}
	}
	return out
}

// directText returns a block statement's text with every nested block
// removed, so only its own substatements remain.
func directText(stmt string) string {
	open := strings.IndexByte(stmt, '{')
	if open < 0 {
		return stmt
	}
	var b bytes.Buffer
	for _, child := range childStatements(stmt, open, len(stmt)-1) {
		if i := strings.IndexByte(child, '{'); i >= 0 {
			if j := strings.IndexByte(child, '"'); j < 0 || i < j {
				continue
			}
		}
		b.WriteString(child)
		b.WriteByte('\n')
	}
	return b.String()
}
