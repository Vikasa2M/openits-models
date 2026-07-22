// Command arcit-coverage scans openits YANG modules for `arc-it-flow`
// annotations and emits a Markdown report diffing them against an
// ARC-IT inventory.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

type inventoryFlow struct {
	Flow string `json:"flow"`
}

type servicePackage struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Flows []inventoryFlow `json:"flows"`
}

type inventory struct {
	Generated       string           `json:"generated"`
	Source          string           `json:"source"`
	ServicePackages []servicePackage `json:"service_packages"`
}

type annotation struct {
	Flow   string
	Path   string
	Module string
}

func main() {
	inv := flag.String("inventory", "arcit_inventory.json", "ARC-IT inventory JSON")
	yangDir := flag.String("yang-dir", "../../yang", "YANG modules directory")
	flag.Parse()

	inv2, err := loadInventory(*inv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load inventory: %v\n", err)
		os.Exit(1)
	}
	anns, modules, err := scanYANG(*yangDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan yang: %v\n", err)
		os.Exit(1)
	}
	if err := writeReport(os.Stdout, inv2, anns, modules); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(1)
	}
}

func loadInventory(path string) (*inventory, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var inv inventory
	if err := json.Unmarshal(b, &inv); err != nil {
		return nil, err
	}
	return &inv, nil
}

// scanYANG loads every *.yang file under dir and its augments/ subtree,
// and returns the arc-it-flow annotations found on their schema entries,
// plus a sorted list of `module@revision` identifiers. Vendor augment
// modules carry their own arc-it-flow annotations (e.g. the trafficvision
// camera augment), so they must be scanned for coverage too.
func scanYANG(dir string) ([]annotation, []string, error) {
	ms := yang.NewModules()
	ms.AddPath(dir)
	ms.AddPath(filepath.Join(dir, "ietf"))
	ms.AddPath(filepath.Join(dir, "augments"))

	entries, err := filepath.Glob(filepath.Join(dir, "*.yang"))
	if err != nil {
		return nil, nil, err
	}
	augEntries, err := filepath.Glob(filepath.Join(dir, "augments", "*.yang"))
	if err != nil {
		return nil, nil, err
	}
	entries = append(entries, augEntries...)
	var moduleNames []string
	for _, f := range entries {
		name := strings.TrimSuffix(filepath.Base(f), ".yang")
		if err := ms.Read(name); err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", name, err)
		}
		moduleNames = append(moduleNames, name)
	}
	if errs := ms.Process(); len(errs) > 0 {
		return nil, nil, fmt.Errorf("yang process: %v", errs)
	}

	moduleIDs := make([]string, 0, len(moduleNames))
	for _, n := range moduleNames {
		m := ms.Modules[n]
		if m == nil {
			continue
		}
		rev := ""
		if len(m.Revision) > 0 {
			rev = m.Revision[0].Name
		}
		moduleIDs = append(moduleIDs, fmt.Sprintf("%s@%s", n, rev))
	}
	sort.Strings(moduleIDs)

	var anns []annotation
	for _, n := range moduleNames {
		mod, ok := ms.Modules[n]
		if !ok {
			continue
		}
		entry := yang.ToEntry(mod)
		walk(entry, n, &anns)
	}
	sort.SliceStable(anns, func(i, j int) bool {
		if anns[i].Flow != anns[j].Flow {
			return anns[i].Flow < anns[j].Flow
		}
		return anns[i].Path < anns[j].Path
	})
	return anns, moduleIDs, nil
}

func walk(e *yang.Entry, module string, out *[]annotation) {
	if e == nil {
		return
	}
	for _, ext := range e.Exts {
		if !strings.HasSuffix(ext.Keyword, ":arc-it-flow") && ext.Keyword != "arc-it-flow" {
			continue
		}
		*out = append(*out, annotation{
			Flow:   strings.TrimSpace(ext.Argument),
			Path:   e.Path(),
			Module: module,
		})
	}
	for _, child := range e.Dir {
		walk(child, module, out)
	}
}

func writeReport(w io.Writer, inv *inventory, anns []annotation, modules []string) error {
	byFlow := make(map[string][]annotation, len(anns))
	for _, a := range anns {
		byFlow[a.Flow] = append(byFlow[a.Flow], a)
	}

	fmt.Fprintln(w, "# ARC-IT Coverage Report — OpenITS")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Inventory generated: %s\n\n", inv.Generated)
	fmt.Fprintf(w, "YANG modules scanned: %s\n\n", strings.Join(modules, ", "))

	inventoryFlows := make(map[string]struct{})

	for _, sp := range inv.ServicePackages {
		total := len(sp.Flows)
		covered := 0
		for _, f := range sp.Flows {
			if _, ok := byFlow[f.Flow]; ok {
				covered++
			}
		}
		pct := 0
		if total > 0 {
			pct = (covered * 100) / total
		}
		fmt.Fprintf(w, "## Service Package %s (%s)\n\n", sp.ID, sp.Name)
		fmt.Fprintf(w, "Coverage: %d / %d flows (%d%%)\n\n", covered, total, pct)
		fmt.Fprintln(w, "| Flow | Annotated | Node |")
		fmt.Fprintln(w, "|------|-----------|------|")
		for _, f := range sp.Flows {
			inventoryFlows[f.Flow] = struct{}{}
			matches, ok := byFlow[f.Flow]
			if !ok {
				fmt.Fprintf(w, "| %s | ❌ | — |\n", f.Flow)
				continue
			}
			nodes := make([]string, 0, len(matches))
			for _, m := range matches {
				nodes = append(nodes, m.Path)
			}
			fmt.Fprintf(w, "| %s | ✅ | %s |\n", f.Flow, strings.Join(nodes, "<br>"))
		}
		fmt.Fprintln(w)
	}

	// Any annotations not in the inventory — flag as "orphan" so the
	// inventory can be extended or the annotation fixed.
	var orphans []annotation
	for _, a := range anns {
		if _, ok := inventoryFlows[a.Flow]; !ok {
			orphans = append(orphans, a)
		}
	}
	if len(orphans) > 0 {
		fmt.Fprintln(w, "## Orphan Annotations")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Flow annotations present in YANG but not listed in the inventory.")
		fmt.Fprintln(w, "Either extend the inventory or correct the annotation.")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| Flow | Node |")
		fmt.Fprintln(w, "|------|------|")
		for _, o := range orphans {
			fmt.Fprintf(w, "| %s | %s |\n", o.Flow, o.Path)
		}
		fmt.Fprintln(w)
	}
	return nil
}
