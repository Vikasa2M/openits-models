package main

import (
	"os"
	"path/filepath"
	"testing"
)

const refModule = `module openits-test-refs {
  namespace "urn:x";
  prefix x;
  revision 2026-10-13 {
    description "r";
    reference "docs/06-extension-model.md";
  }
  leaf a {
    type string;
    reference
      "RFC 7950; see docs/data-model.md.";
  }
  leaf b {
    type string;
    reference "NTCIP 1202:2019 §5.";
  }
}
`

func TestReferences_FlagsRepoPathsOutsideRevisions(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "openits-test-refs.yang"), []byte(refModule), 0o644)
	vs, err := ruleReferences(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 || vs[0].Path != "line 11" {
		t.Fatalf("got %+v, want one violation at line 11", vs)
	}
}

func TestReferences_IgnoresRevisionBlocks(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "openits-test-refs.yang"), []byte(refModule), 0o644)
	vs, _ := ruleReferences(dir)
	for _, v := range vs {
		if v.Path == "line 6" {
			t.Fatalf("revision reference flagged: %+v", v)
		}
	}
}
