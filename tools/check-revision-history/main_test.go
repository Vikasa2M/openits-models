package main

import (
	"reflect"
	"testing"
)

const baseModule = `module m {
  namespace "urn:x";
  prefix x;

  revision 2026-10-14 {
    description
      "Second. A brace in text { stays inside the string.";
    reference "RFC 7950 §11.";
  }
  revision 2026-07-01 {
    description "First.";
  }
  leaf a { type string; }
}
`

func TestRevisionBlocks(t *testing.T) {
	got := revisionBlocks(baseModule)
	if len(got) != 2 {
		t.Fatalf("got %d revisions, want 2: %v", len(got), got)
	}
	if _, ok := got["2026-10-14"]; !ok {
		t.Fatalf("2026-10-14 missing: %v", got)
	}
	want := "revision 2026-07-01 {\n    description \"First.\";\n  }"
	if got["2026-07-01"] != want {
		t.Fatalf("2026-07-01 block = %q, want %q", got["2026-07-01"], want)
	}
}

func TestRevisionBlocks_BodylessAndQuotedDate(t *testing.T) {
	got := revisionBlocks("module m {\n  revision \"2026-01-02\";\n  revision 2026-01-01 { }\n}\n")
	if got["2026-01-02"] != `revision "2026-01-02";` {
		t.Fatalf("bodyless revision = %q", got["2026-01-02"])
	}
	if got["2026-01-01"] != "revision 2026-01-01 { }" {
		t.Fatalf("empty-body revision = %q", got["2026-01-01"])
	}
}

func TestChangedRevisions(t *testing.T) {
	newRevision := `module m {
  namespace "urn:x";
  prefix x;

  revision 2026-10-15 {
    description "Third, new in this change.";
  }
  revision 2026-10-14 {
    description
      "Second. A brace in text { stays inside the string.";
    reference "RFC 7950 §11.";
  }
  revision 2026-07-01 {
    description "First.";
  }
  leaf a { type string; description "live text may change"; }
}
`
	editedHistory := `module m {
  revision 2026-10-14 {
    description
      "Second, reworded.";
    reference "RFC 7950 §11.";
  }
  revision 2026-07-01 {
    description "First.";
  }
}
`
	deletedHistory := `module m {
  revision 2026-10-14 {
    description
      "Second. A brace in text { stays inside the string.";
    reference "RFC 7950 §11.";
  }
}
`
	cases := []struct {
		name string
		head string
		want []string
	}{
		{"new revision and live edits pass", newRevision, nil},
		{"edited shipped revision fails", editedHistory, []string{"revision 2026-10-14 changed"}},
		{"deleted shipped revision fails", deletedHistory, []string{"revision 2026-07-01 removed"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := changedRevisions(baseModule, c.head)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("changedRevisions = %v, want %v", got, c.want)
			}
		})
	}
}
