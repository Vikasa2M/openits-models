package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestHasLiveNotification(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"live", `module m { notification n { leaf a { type string; } } }`, true},
		{"deprecated", `module m { notification n { status deprecated; leaf a { type string; } } }`, false},
		{"obsolete quoted", `module m { notification n { status "obsolete"; } }`, false},
		{"nested leaf status does not count", `module m {
  notification n {
    leaf a { status deprecated; type string; }
  }
}`, true},
		{"word in description is not a statement", `module m {
  description "Emits a notification { when } it changes.";
  leaf a { type string; }
}`, false},
		{"comment is not a statement", `module m {
  // notification n { }
  /* notification m { } */
  leaf a { type string; }
}`, false},
		{"one live among deprecated", `module m {
  notification old { status deprecated; }
  notification new { leaf a { type string; } }
}`, true},
		{"nested notification is not top-level", `module m {
  container c { notification n { } }
}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasLiveNotification([]byte(c.text)); got != c.want {
				t.Fatalf("hasLiveNotification = %v, want %v", got, c.want)
			}
		})
	}
}

func entry(module, rev, name, blob string) registryEntry {
	return registryEntry{module: module, revision: rev, name: name, blob: blob}
}

func TestImmutabilityProblems(t *testing.T) {
	base := map[string]registryEntry{
		"m/2026-01-01/schema.yang":  entry("m", "2026-01-01", "schema.yang", "a"),
		"m/2026-01-01/schema.proto": entry("m", "2026-01-01", "schema.proto", "b"),
		"n/2026-01-01/schema.yang":  entry("n", "2026-01-01", "schema.yang", "c"),
	}
	cases := []struct {
		name string
		head map[string]registryEntry
		want []string
	}{
		{"unchanged plus a new revision passes", map[string]registryEntry{
			"m/2026-01-01/schema.yang":  entry("m", "2026-01-01", "schema.yang", "a"),
			"m/2026-01-01/schema.proto": entry("m", "2026-01-01", "schema.proto", "b"),
			"n/2026-01-01/schema.yang":  entry("n", "2026-01-01", "schema.yang", "c"),
			"m/2026-02-01/schema.yang":  entry("m", "2026-02-01", "schema.yang", "d"),
		}, nil},
		{"backfilling schema.shape passes", map[string]registryEntry{
			"m/2026-01-01/schema.yang":  entry("m", "2026-01-01", "schema.yang", "a"),
			"m/2026-01-01/schema.proto": entry("m", "2026-01-01", "schema.proto", "b"),
			"m/2026-01-01/schema.shape": entry("m", "2026-01-01", "schema.shape", "e"),
			"n/2026-01-01/schema.yang":  entry("n", "2026-01-01", "schema.yang", "c"),
		}, nil},
		{"edit, removal and wire backfill fail", map[string]registryEntry{
			"m/2026-01-01/schema.yang":  entry("m", "2026-01-01", "schema.yang", "x"),
			"n/2026-01-01/schema.yang":  entry("n", "2026-01-01", "schema.yang", "c"),
			"n/2026-01-01/schema.proto": entry("n", "2026-01-01", "schema.proto", "f"),
		}, []string{
			"m/2026-01-01/schema.proto: removed from a released snapshot",
			"m/2026-01-01/schema.yang: changed in a released snapshot",
			"n/2026-01-01/schema.proto: added to a released snapshot (only schema.shape may be backfilled)",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := immutabilityProblems(base, c.head)
			sort.Strings(got)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("immutabilityProblems =\n%q\nwant\n%q", got, c.want)
			}
		})
	}
}

func TestCompletenessProblems(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	events := "module e { notification n { leaf a { type string; } } }"
	core := "module c { leaf a { type string; } }"
	for _, f := range []string{"MANIFEST.json", "README.md"} {
		write("e/2026-01-01/"+f, "{}")
		write("c/2026-01-01/"+f, "{}")
	}
	write("e/2026-01-01/schema.yang", events)
	write("e/2026-01-01/schema.proto", "")
	write("c/2026-01-01/schema.yang", core)
	head, err := headRegistryFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := completenessProblems(dir, head)
	want := []string{"e/2026-01-01: missing schema.json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completenessProblems = %q, want %q", got, want)
	}

	grandfathered["e/2026-01-01"] = []string{"schema.json"}
	defer delete(grandfathered, "e/2026-01-01")
	if got := completenessProblems(dir, head); len(got) != 0 {
		t.Fatalf("grandfathered snapshot still reported: %q", got)
	}
}

func TestBlobIDMatchesGit(t *testing.T) {
	// `printf 'hello\n' | git hash-object --stdin`
	if got, want := blobID([]byte("hello\n")), "ce013625030ba8dba906f756967f9e9ca394464a"; got != want {
		t.Fatalf("blobID = %s, want %s", got, want)
	}
}
