package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var repoPathRE = regexp.MustCompile(`docs/[A-Za-z0-9_./-]+|\b[A-Za-z0-9_-]+\.md\b`)

// ruleReferences flags reference statements that cite a repository path
// instead of a standard. It reads the module text (goyang does not keep
// reference statements on entries) and skips revision blocks, whose text
// is immutable once published.
func ruleReferences(dir string) ([]Violation, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yang"))
	if err != nil {
		return nil, err
	}
	var out []Violation
	for _, f := range files {
		module := strings.TrimSuffix(filepath.Base(f), ".yang")
		if !strings.HasPrefix(module, "openits-") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		depth, revDepth := 0, -1
		inRef := false
		for i, line := range strings.Split(string(b), "\n") {
			trimmed := strings.TrimSpace(line)
			if revDepth < 0 && strings.HasPrefix(trimmed, "revision ") {
				revDepth = depth
			}
			if revDepth < 0 {
				starts := strings.HasPrefix(trimmed, "reference ") || trimmed == "reference"
				if starts || inRef {
					if repoPathRE.MatchString(line) {
						out = append(out, Violation{"references", module, fmt.Sprintf("line %d", i+1),
							"reference cites a repository path; cite the standard or RFC instead"})
					}
					inRef = !strings.HasSuffix(trimmed, "\";")
				}
			}
			depth += strings.Count(line, "{") - strings.Count(line, "}")
			if revDepth >= 0 && depth <= revDepth {
				revDepth = -1
			}
		}
	}
	return out, nil
}
