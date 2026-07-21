package main

import (
	"os"
	"path/filepath"
	"testing"
)

// isAllowedImport / disallowedImports is the core judgment: given the list
// of modules one *-events.yang file imports, which of them violate the
// layering rule (allow openits-types, ietf-yang-types,
// openits-nema-common, and anything ending in "-types"; flag everything
// else, e.g. a bare service core like openits-dms)?
func TestDisallowedImports_matrix(t *testing.T) {
	tests := []struct {
		name    string
		imports []string
		want    []string // nil means "no violations"
	}{
		{
			name:    "shared types + service types + ietf are all allowed",
			imports: []string{"openits-types", "openits-dms-types", "ietf-yang-types"},
			want:    nil,
		},
		{
			name:    "openits-nema-common is allowed",
			imports: []string{"openits-types", "openits-nema-common"},
			want:    nil,
		},
		{
			name:    "a bare service core is a violation",
			imports: []string{"openits-types", "openits-dms-types", "openits-dms"},
			want:    []string{"openits-dms"},
		},
		{
			name:    "another events module is a violation",
			imports: []string{"openits-types", "openits-signal-control-events"},
			want:    []string{"openits-signal-control-events"},
		},
		{
			name:    "multiple violations are all reported, in import order",
			imports: []string{"openits-dms", "openits-types", "openits-v2x-radio"},
			want:    []string{"openits-dms", "openits-v2x-radio"},
		},
		{
			name:    "no imports at all is not a violation",
			imports: nil,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := disallowedImports(tt.imports)
			if len(got) != len(tt.want) {
				t.Fatalf("disallowedImports(%v) = %v, want %v", tt.imports, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("disallowedImports(%v)[%d] = %q, want %q", tt.imports, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// isAllowedImport exercises the single-name predicate directly, including
// the "-types" suffix rule's edge cases.
func TestIsAllowedImport(t *testing.T) {
	tests := []struct {
		name string
		mod  string
		want bool
	}{
		{"exact openits-types", "openits-types", true},
		{"exact ietf-yang-types", "ietf-yang-types", true},
		{"exact openits-nema-common", "openits-nema-common", true},
		{"suffix -types on a service module", "openits-signal-control-types", true},
		{"suffix -types alone is degenerate but still allowed", "-types", true},
		{"service core is not allowed", "openits-dms", false},
		{"another events module is not allowed", "openits-rsu-events", false},
		{"unrelated ietf module is not allowed", "ietf-inet-types", true}, // ends in -types
		{"v2x capability module is not allowed", "openits-v2x-messaging", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAllowedImport(tt.mod); got != tt.want {
				t.Errorf("isAllowedImport(%q) = %v, want %v", tt.mod, got, tt.want)
			}
		})
	}
}

// extractImports scans a real (if minimal) YANG file's `import <name> {`
// statements textually, in file order, ignoring unrelated lines (revision,
// description prose that happens to contain the word "import", etc.).
func TestExtractImports(t *testing.T) {
	dir := t.TempDir()
	src := `module openits-example-events {
  yang-version 1.1;
  namespace "urn:test:yang:example-events";
  prefix openits-example-events;

  import openits-types {
    prefix openits-types;
  }
  import openits-example-types {
    prefix openits-example-types;
  }

  organization "test";
  contact "test@vikasa.io";
  description
    "Prose that mentions the word import but is not a statement.";

  revision 2026-07-14 {
    description "Test fixture.";
  }
}
`
	path := filepath.Join(dir, "openits-example-events.yang")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	got, err := extractImports(path)
	if err != nil {
		t.Fatalf("extractImports: %v", err)
	}
	want := []string{"openits-types", "openits-example-types"}
	if len(got) != len(want) {
		t.Fatalf("extractImports = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("extractImports[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// CheckDir ties extraction + the allow-rule together over a directory: a
// compliant *-events.yang produces no violations, and a module importing a
// bare service core is reported with the right module name and import
// name. Only *-events.yang files are considered — a co-located *-types.yang
// or core module in the same dir must not be scanned.
func TestCheckDir_compliantAndViolating(t *testing.T) {
	dir := t.TempDir()

	compliant := `module openits-good-events {
  yang-version 1.1;
  namespace "urn:test:yang:good-events";
  prefix openits-good-events;

  import openits-types { prefix openits-types; }
  import openits-good-types { prefix openits-good-types; }

  organization "test";
  contact "test@vikasa.io";
  description "Compliant fixture.";

  revision 2026-07-14 {
    description "Test fixture.";
  }
}
`
	violating := `module openits-bad-events {
  yang-version 1.1;
  namespace "urn:test:yang:bad-events";
  prefix openits-bad-events;

  import openits-types { prefix openits-types; }
  import openits-bad { prefix openits-bad; }

  organization "test";
  contact "test@vikasa.io";
  description "Violating fixture: imports its service core.";

  revision 2026-07-14 {
    description "Test fixture.";
  }
}
`
	notEvents := `module openits-bad-types {
  yang-version 1.1;
  namespace "urn:test:yang:bad-types";
  prefix openits-bad-types;

  import openits-bad { prefix openits-bad; }

  organization "test";
  contact "test@vikasa.io";
  description "Not an events module: must be ignored even though it also imports a core.";

  revision 2026-07-14 {
    description "Test fixture.";
  }
}
`
	writeFixture(t, dir, "openits-good-events.yang", compliant)
	writeFixture(t, dir, "openits-bad-events.yang", violating)
	writeFixture(t, dir, "openits-bad-types.yang", notEvents)

	violations, fileCount, err := CheckDir(dir)
	if err != nil {
		t.Fatalf("CheckDir: %v", err)
	}
	if fileCount != 2 {
		t.Errorf("fileCount = %d, want 2 (only *-events.yang files)", fileCount)
	}
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want exactly 1", violations)
	}
	v := violations[0]
	if v.Module != "openits-bad-events" {
		t.Errorf("violation.Module = %q, want %q", v.Module, "openits-bad-events")
	}
	if v.Import != "openits-bad" {
		t.Errorf("violation.Import = %q, want %q", v.Import, "openits-bad")
	}
}

// A directory with no *-events.yang files at all is a legitimate empty
// state (mirrors check-deviations's missing-dir behavior), not an error.
func TestCheckDir_noEventsFiles_isEmptyNotError(t *testing.T) {
	dir := t.TempDir()
	violations, fileCount, err := CheckDir(dir)
	if err != nil {
		t.Fatalf("CheckDir: %v", err)
	}
	if len(violations) != 0 || fileCount != 0 {
		t.Errorf("CheckDir(empty dir) = (%v, %d), want (nil, 0)", violations, fileCount)
	}
}

// The openits-rsu-events grandfather exception has been removed now that
// openits-rsu-events imports only "-types" modules (the identities it used
// to reach into openits-v2x-messaging/openits-v2x-radio for moved into
// openits-v2x-messaging-types/openits-v2x-radio-types). This test proves
// there is no residual special-casing left for that module name: importing
// the bare v2x capability cores directly is a plain violation for
// openits-rsu-events exactly as it is for any other module, with no notes
// and no exceptions.
func TestCheckDir_rsuEventsGrandfatherRemoved(t *testing.T) {
	dir := t.TempDir()

	// openits-rsu-events importing the two formerly-grandfathered bare cores
	// plus a bare service core: all three are now violations.
	rsuEvents := `module openits-rsu-events {
  yang-version 1.1;
  namespace "urn:test:yang:rsu-events";
  prefix openits-rsu-events;

  import openits-types { prefix openits-types; }
  import openits-rsu-types { prefix openits-rsu-types; }
  import openits-v2x-messaging { prefix openits-v2x-messaging; }
  import openits-v2x-radio { prefix openits-v2x-radio; }
  import openits-rsu { prefix openits-rsu; }

  organization "test";
  contact "test@vikasa.io";
  description "Fixture: the two formerly-grandfathered imports plus a bare service core.";

  revision 2026-07-14 {
    description "Test fixture.";
  }
}
`
	// A different module importing openits-v2x-radio: proves every module is
	// judged by the same rule now that there is no per-module exception.
	otherEvents := `module openits-other-events {
  yang-version 1.1;
  namespace "urn:test:yang:other-events";
  prefix openits-other-events;

  import openits-types { prefix openits-types; }
  import openits-v2x-radio { prefix openits-v2x-radio; }

  organization "test";
  contact "test@vikasa.io";
  description "Fixture: same import, different module.";

  revision 2026-07-14 {
    description "Test fixture.";
  }
}
`
	writeFixture(t, dir, "openits-rsu-events.yang", rsuEvents)
	writeFixture(t, dir, "openits-other-events.yang", otherEvents)

	violations, fileCount, err := CheckDir(dir)
	if err != nil {
		t.Fatalf("CheckDir: %v", err)
	}
	if fileCount != 2 {
		t.Errorf("fileCount = %d, want 2", fileCount)
	}

	wantViolations := map[string][]string{
		"openits-rsu-events":   {"openits-v2x-messaging", "openits-v2x-radio", "openits-rsu"},
		"openits-other-events": {"openits-v2x-radio"},
	}
	wantTotal := 0
	for _, imps := range wantViolations {
		wantTotal += len(imps)
	}
	if len(violations) != wantTotal {
		t.Fatalf("violations = %v, want %d total", violations, wantTotal)
	}

	got := map[string][]string{}
	for _, v := range violations {
		got[v.Module] = append(got[v.Module], v.Import)
	}
	for mod, want := range wantViolations {
		gotImports := got[mod]
		if len(gotImports) != len(want) {
			t.Fatalf("violations for %q = %v, want %v", mod, gotImports, want)
		}
		for i := range want {
			if gotImports[i] != want[i] {
				t.Errorf("violations for %q[%d] = %q, want %q", mod, i, gotImports[i], want[i])
			}
		}
	}
}

func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
