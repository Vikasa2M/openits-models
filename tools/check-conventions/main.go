// check-conventions lints OpenITS YANG modules against the mechanical
// conventions of the pre-1.0 house contract (rule 6 in
// docs/reference/yang-reference-conventions.md) and compares the result
// with a checked-in, shrink-only exceptions file.
//
// Wired to `make check-conventions`.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	yangDir := flag.String("yang", "yang", "directory of YANG modules")
	excPath := flag.String("exceptions", "conventions-exceptions.yaml", "exceptions file")
	write := flag.Bool("write-exceptions", false,
		"rewrite the exceptions file to list the current violations (initial baseline, or after fixing entries)")
	flag.Parse()
	if err := run(*yangDir, *excPath, *write, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "check-conventions:", err)
		os.Exit(1)
	}
}

func run(yangDir, excPath string, write bool, w io.Writer) error {
	mods, err := loadModules(yangDir)
	if err != nil {
		return err
	}
	vs := checkSchema(mods)
	sortViolations(vs)
	if write {
		return writeExceptions(excPath, vs)
	}
	exc, err := readExceptions(excPath)
	if err != nil {
		return err
	}
	fresh, stale := compare(vs, exc)
	for _, v := range fresh {
		fmt.Fprintf(w, "NEW    [%s] %s %s: %s\n", v.Rule, v.Module, v.Path, v.Detail)
	}
	for _, x := range stale {
		fmt.Fprintf(w, "STALE  [%s] %s %s: fixed, delete it from %s\n", x.Rule, x.Module, x.Path, excPath)
	}
	if len(fresh)+len(stale) > 0 {
		return fmt.Errorf("%d new violation(s), %d stale exception(s)", len(fresh), len(stale))
	}
	fmt.Fprintf(w, "check-conventions: OK (%d known exceptions)\n", len(exc))
	return nil
}
