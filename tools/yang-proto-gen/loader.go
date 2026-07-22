// Command yang-proto-gen generates protobuf definitions from the openits
// YANG modules using goyang.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

// LoadModules reads every .yang file under yangDir (and yangDir/ietf),
// processes them, and returns the processed module set plus one Entry per
// module. A non-empty Process() error slice is returned as an error, and so
// is a yangDir that can't be read (e.g. a bad -yang CLI flag) — the
// optional yangDir/ietf subdirectory (vendored ietf-*.yang imports; not
// every yangDir has one, e.g. this package's own test fixtures) is the only
// path component allowed to be silently absent.
func LoadModules(yangDir string) (*yang.Modules, []*yang.Entry, error) {
	ms := yang.NewModules()
	var files []string

	rootEnts, err := os.ReadDir(yangDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read yang dir %s: %w", yangDir, err)
	}
	for _, f := range rootEnts {
		if strings.HasSuffix(f.Name(), ".yang") {
			files = append(files, filepath.Join(yangDir, f.Name()))
		}
	}
	if ietfEnts, err := os.ReadDir(filepath.Join(yangDir, "ietf")); err == nil {
		for _, f := range ietfEnts {
			if strings.HasSuffix(f.Name(), ".yang") {
				files = append(files, filepath.Join(yangDir, "ietf", f.Name()))
			}
		}
	}
	for _, f := range files {
		if err := ms.Read(f); err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", f, err)
		}
	}
	if errs := ms.Process(); len(errs) > 0 {
		return nil, nil, fmt.Errorf("process: %v", errs)
	}
	var mods []*yang.Entry
	for name, m := range ms.Modules {
		// ms.Modules is keyed by both the module name and
		// "name@revision"; skip the revisioned alias so each module is
		// only visited once by its bare name. Submodules never appear in
		// ms.Modules (goyang files them under ms.SubModules), but guard
		// against BelongsTo anyway in case that changes upstream.
		if name != m.Name || m.BelongsTo != nil {
			continue
		}
		e, errs := ms.GetModule(name)
		if len(errs) == 0 && e != nil {
			mods = append(mods, e)
		}
	}
	return ms, dedupeEntries(mods), nil
}

func dedupeEntries(in []*yang.Entry) []*yang.Entry {
	seen := map[string]bool{}
	var out []*yang.Entry
	for _, e := range in {
		if !seen[e.Name] {
			seen[e.Name] = true
			out = append(out, e)
		}
	}
	return out
}

// entryModule is a thin helper over a module Entry for test/assertion use.
type entryModule struct{ e *yang.Entry }

func (m entryModule) notificationCount() int {
	n := 0
	for _, c := range m.e.Dir {
		if c.Kind == yang.NotificationEntry {
			n++
		}
	}
	return n
}
