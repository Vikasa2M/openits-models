package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
)

// realAugmentsDir points at the repo's real yang/augments tree. `go test`
// runs with the package directory as its working directory, mirroring the
// convention used by tools/check-deviations's tests.
func realAugmentsDir() string { return filepath.Join("..", "..", "yang", "augments") }

// writeAugment writes a minimal but well-formed augment module to dir,
// importing openits-dms under the given prefix and augmenting the given
// target path (which should reference that prefix, e.g. "/dms:sign").
func writeAugment(t *testing.T, dir, fileBase, moduleName, importPrefix, augmentPath string) {
	t.Helper()
	src := `module ` + moduleName + ` {
  yang-version 1.1;
  namespace "urn:test:yang:` + moduleName + `";
  prefix ` + moduleName + `;

  import openits-dms {
    prefix ` + importPrefix + `;
  }

  organization "test";
  contact "test-org";
  description "Synthetic augment for check-augment-collisions tests.";

  revision 2026-07-10 {
    description "Test fixture.";
  }

  augment "` + augmentPath + `" {
    description "Test fixture augment.";
    leaf probe {
      type string;
      description "Test fixture leaf.";
    }
  }
}
`
	path := filepath.Join(dir, fileBase+".yang")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// Two augments that both target the SAME core node (openits-dms's "sign")
// but declare DIFFERENT local prefixes for the imported openits-dms
// module ("dms" vs "d") must be detected as a collision: the raw
// declared strings "/dms:sign" and "/d:sign" differ textually, but both
// resolve, through each module's own import-prefix map, to the same
// (openits-dms, sign) node identity.
func TestFindCollisions_prefixAliasCollision_isDetected(t *testing.T) {
	dir := t.TempDir()
	writeAugment(t, dir, "vendor-a-dms-probe", "vendor-a-dms-probe", "dms", "/dms:sign")
	writeAugment(t, dir, "vendor-b-dms-probe", "vendor-b-dms-probe", "d", "/d:sign")

	_, collisions, err := FindCollisions(dir)
	if err != nil {
		t.Fatalf("FindCollisions: %v", err)
	}

	if len(collisions) != 1 {
		t.Fatalf("expected exactly 1 collision (alias-blind same node), got %d: %+v", len(collisions), collisions)
	}
	c := collisions[0]
	if c.Normalized != "/openits-dms:sign" {
		t.Errorf("collision normalized path = %q, want %q", c.Normalized, "/openits-dms:sign")
	}
	if len(c.Files) != 2 {
		t.Fatalf("expected 2 contributing files in the collision, got %d: %v", len(c.Files), c.Files)
	}
	want := map[string]bool{"vendor-a-dms-probe.yang": true, "vendor-b-dms-probe.yang": true}
	for _, f := range c.Files {
		if !want[f] {
			t.Errorf("unexpected file %q in collision (want %v)", f, want)
		}
	}
}

// Two augments targeting genuinely DIFFERENT nodes of the same imported
// module — even with differing local prefixes — must NOT collide.
func TestFindCollisions_differentNodes_noCollision(t *testing.T) {
	dir := t.TempDir()
	writeAugment(t, dir, "vendor-a-dms-probe", "vendor-a-dms-probe", "dms", "/dms:sign")
	writeAugment(t, dir, "vendor-c-dms-probe", "vendor-c-dms-probe", "d", "/d:sign-group")

	_, collisions, err := FindCollisions(dir)
	if err != nil {
		t.Fatalf("FindCollisions: %v", err)
	}
	if len(collisions) != 0 {
		t.Errorf("expected 0 collisions for genuinely different target nodes, got %d: %+v", len(collisions), collisions)
	}
}

// run() is what main() delegates to; it must return a non-zero exit code
// when a collision is found (the pre-rewrite tool always returned 0).
func TestRun_exitsNonZero_onCollision(t *testing.T) {
	dir := t.TempDir()
	writeAugment(t, dir, "vendor-a-dms-probe", "vendor-a-dms-probe", "dms", "/dms:sign")
	writeAugment(t, dir, "vendor-b-dms-probe", "vendor-b-dms-probe", "d", "/d:sign")

	var out bytes.Buffer
	code := run(dir, &out)
	if code == 0 {
		t.Errorf("run() returned 0 (success) for a directory with a real collision; want non-zero")
	}
	if !bytes.Contains(out.Bytes(), []byte("/openits-dms:sign")) {
		t.Errorf("run() output does not mention the colliding normalized path; got:\n%s", out.String())
	}
}

// run() must return 0 when augments don't collide.
func TestRun_exitsZero_noCollision(t *testing.T) {
	dir := t.TempDir()
	writeAugment(t, dir, "vendor-a-dms-probe", "vendor-a-dms-probe", "dms", "/dms:sign")
	writeAugment(t, dir, "vendor-c-dms-probe", "vendor-c-dms-probe", "d", "/d:sign-group")

	var out bytes.Buffer
	code := run(dir, &out)
	if code != 0 {
		t.Errorf("run() returned %d for non-colliding augments; want 0.\noutput:\n%s", code, out.String())
	}
}

// The 4 real augments under yang/augments/ target distinct core nodes and
// must not collide (this is the no-regression guard for `make
// check-augment-collisions` against real content).
func TestFindCollisions_realAugments_noCollisions(t *testing.T) {
	targets, collisions, err := FindCollisions(realAugmentsDir())
	if err != nil {
		t.Fatalf("FindCollisions: %v", err)
	}
	if len(targets) == 0 {
		t.Fatal("expected at least one augment target from the real yang/augments tree, got none")
	}
	if len(collisions) != 0 {
		t.Errorf("expected 0 collisions among the real augments, got %d: %+v", len(collisions), collisions)
	}
}

// importPrefixMap must let a module's own declared prefix resolve to itself
// even if (invalidly per RFC 7950) an import reuses that same prefix alias.
// Before the import-precedence fix the import entry, applied after the self-mapping, clobbered it.
func TestImportPrefixMap_selfPrefixWinsOverImport(t *testing.T) {
	m := &yang.Module{Name: "self-mod"}
	m.Prefix = &yang.Value{Name: "sp"}
	m.Import = []*yang.Import{{Name: "openits-dms", Prefix: &yang.Value{Name: "sp"}}}

	got := importPrefixMap(m)
	if got["sp"] != "self-mod" {
		t.Errorf("self-prefix \"sp\" resolved to %q, want %q (import must not override self)", got["sp"], "self-mod")
	}
	if got[""] != "self-mod" {
		t.Errorf("empty prefix resolved to %q, want %q", got[""], "self-mod")
	}
}
