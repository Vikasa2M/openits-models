package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

// loadModules parses every .yang file in dir and in dir/ietf (when it
// exists) and returns the processed OpenITS module entries, sorted by name.
// Other modules are loaded so imports resolve, but the conventions apply
// only to modules named openits-*.
func loadModules(dir string) ([]*yang.Entry, error) {
	ms := yang.NewModules()
	var files []string
	for _, d := range []string{dir, filepath.Join(dir, "ietf")} {
		ents, err := os.ReadDir(d)
		if err != nil {
			if d != dir && os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", d, err)
		}
		for _, f := range ents {
			if strings.HasSuffix(f.Name(), ".yang") {
				files = append(files, filepath.Join(d, f.Name()))
			}
		}
	}
	for _, f := range files {
		if err := ms.Read(f); err != nil {
			return nil, fmt.Errorf("reading %s: %w", f, err)
		}
	}
	if errs := ms.Process(); len(errs) > 0 {
		return nil, fmt.Errorf("processing modules: %v", errs)
	}
	var mods []*yang.Entry
	for name, m := range ms.Modules {
		// ms.Modules is keyed by both name and name@revision.
		if strings.Contains(name, "@") || !strings.HasPrefix(name, "openits-") {
			continue
		}
		mods = append(mods, yang.ToEntry(m))
	}
	sort.Slice(mods, func(i, j int) bool { return mods[i].Name < mods[j].Name })
	return mods, nil
}
