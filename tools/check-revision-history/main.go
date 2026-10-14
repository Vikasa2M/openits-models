// check-revision-history fails when a change edits or deletes a YANG
// revision statement that already exists on the base branch.
//
// A revision describes what was true when it shipped, and its
// schema-registry snapshot is immutable, so every revision present on the
// base must survive byte-for-byte. New revisions are unrestricted, and so is
// the module's live text. Description sweeps have repeatedly reworded old
// revision text by accident; reviews caught it each time, this gate makes it
// mechanical.
//
// It compares the working tree (so it works before committing) with the
// given git ref, for every .yang file under the yang directory that exists
// on that ref. A file removed in the working tree is not this gate's
// concern. Wired to `make check-revision-history` and to CI on pull
// requests.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

func main() {
	base := flag.String("base", "origin/main", "git ref whose revisions must be preserved")
	dir := flag.String("yang", "yang", "directory of YANG modules (searched recursively)")
	flag.Parse()
	if err := run(*base, *dir); err != nil {
		fmt.Fprintln(os.Stderr, "check-revision-history:", err)
		os.Exit(1)
	}
}

func run(base, dir string) error {
	out, err := exec.Command("git", "ls-tree", "-r", "--name-only", base, "--", dir).Output()
	if err != nil {
		return fmt.Errorf("listing %s at %s: %w", dir, base, err)
	}
	var problems []string
	checked := 0
	for _, path := range strings.Fields(string(out)) {
		if !strings.HasSuffix(path, ".yang") {
			continue
		}
		head, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		old, err := exec.Command("git", "show", base+":"+path).Output()
		if err != nil {
			return fmt.Errorf("reading %s at %s: %w", path, base, err)
		}
		checked++
		for _, p := range changedRevisions(string(old), string(head)) {
			problems = append(problems, path+": "+p)
		}
	}
	for _, p := range problems {
		fmt.Println("CHANGED  " + p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d revision(s) that exist on %s were edited or removed; restore them verbatim and record new changes in a new revision", len(problems), base)
	}
	fmt.Printf("check-revision-history: OK (%d modules, no revision on %s changed)\n", checked, base)
	return nil
}

// changedRevisions lists the revisions of base that head no longer carries
// byte-for-byte.
func changedRevisions(base, head string) []string {
	before := revisionBlocks(base)
	after := revisionBlocks(head)
	dates := make([]string, 0, len(before))
	for d := range before {
		dates = append(dates, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	var out []string
	for _, d := range dates {
		got, ok := after[d]
		switch {
		case !ok:
			out = append(out, "revision "+d+" removed")
		case got != before[d]:
			out = append(out, "revision "+d+" changed")
		}
	}
	return out
}

var revisionStart = regexp.MustCompile(`(?m)^[ \t]*revision[ \t]+"?(\d{4}-\d{2}-\d{2})"?[ \t]*([{;])`)

// revisionBlocks maps each revision date in a module to the full text of its
// revision statement, from the keyword through the closing brace (or the
// semicolon of a body-less revision). Braces inside quoted strings are
// skipped, so description text cannot end a block early.
func revisionBlocks(text string) map[string]string {
	blocks := map[string]string{}
	for _, m := range revisionStart.FindAllStringSubmatchIndex(text, -1) {
		start := m[0] + strings.Index(text[m[0]:m[1]], "revision")
		date := text[m[2]:m[3]]
		opener := m[4]
		if text[opener] == ';' {
			blocks[date] = text[start : opener+1]
			continue
		}
		if end := matchBrace(text, opener); end >= 0 {
			blocks[date] = text[start : end+1]
		}
	}
	return blocks
}

// matchBrace returns the index of the brace closing the one at open, or -1.
func matchBrace(text string, open int) int {
	depth := 0
	for i := open; i < len(text); i++ {
		switch c := text[i]; c {
		case '"', '\'':
			i = skipString(text, i)
			if i < 0 {
				return -1
			}
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// skipString returns the index of the quote closing the string that starts
// at i. Double-quoted strings honour backslash escapes; single-quoted ones
// do not (RFC 7950 §6.1.3).
func skipString(text string, i int) int {
	q := text[i]
	for j := i + 1; j < len(text); j++ {
		if q == '"' && text[j] == '\\' {
			j++
			continue
		}
		if text[j] == q {
			return j
		}
	}
	return -1
}
