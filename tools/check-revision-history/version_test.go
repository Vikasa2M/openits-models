package main

import "testing"

const shapeBase = `// header
// root openits.x.v1.A sha256:aa

message openits.x.v1.A {
  optional uint32 count = 1;
  string kind = 99;
  oneof choice {
    string a = 2;
    uint32 b = 3;
  }
}

enum openits.x.v1.E {
  E_UNSPECIFIED = 0;
  E_ONE = 1;
}
`

func TestClassifyShapeChange(t *testing.T) {
	cases := []struct {
		name string
		head string
		want wireChange
	}{
		{"identical", shapeBase, wireNone},
		{"header only", "// other header\n" + shapeBase[len("// header\n"):], wireNone},
		{"presence dropped", replace(shapeBase, "optional uint32 count = 1;", "uint32 count = 1;"), wireBreaking},
		{"field removed", replace(shapeBase, "  string kind = 99;\n", ""), wireBreaking},
		{"field renamed", replace(shapeBase, "string kind = 99;", "string type = 99;"), wireBreaking},
		{"left oneof", replace(replace(shapeBase, "    uint32 b = 3;\n", ""), "  string kind = 99;\n", "  string kind = 99;\n  uint32 b = 3;\n"), wireBreaking},
		{"enum value renumbered", replace(shapeBase, "E_ONE = 1;", "E_ONE = 2;"), wireBreaking},
		{"enum value removed", replace(shapeBase, "  E_ONE = 1;\n", ""), wireBreaking},
		{"field added", replace(shapeBase, "  string kind = 99;\n", "  string kind = 99;\n  string note = 4;\n"), wireAdditive},
		{"enum value added", replace(shapeBase, "  E_ONE = 1;\n", "  E_ONE = 1;\n  E_TWO = 2;\n"), wireAdditive},
		{"message added", shapeBase + "\nmessage openits.x.v1.B {\n  string s = 1;\n}\n", wireAdditive},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classifyShapeChange(shapeBase, c.head); got != c.want {
				t.Fatalf("classifyShapeChange = %v, want %v", got, c.want)
			}
		})
	}
}

func replace(s, old, new string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	panic("replace: " + old + " not found")
}

func TestRequiredBump(t *testing.T) {
	v := func(s string) semver {
		x, err := parseSemver(s)
		if err != nil {
			t.Fatal(err)
		}
		return x
	}
	cases := []struct {
		old, new string
		change   wireChange
		ok       bool
	}{
		{"0.4.0", "0.4.1", wireNone, true},
		{"0.4.0", "0.4.1", wireAdditive, false},
		{"0.4.0", "0.5.0", wireAdditive, true},
		{"0.4.0", "0.5.0", wireBreaking, true}, // pre-1.0 breaks on MINOR
		{"0.4.0", "0.4.1", wireBreaking, false},
		{"1.2.3", "1.3.0", wireBreaking, false},
		{"1.2.3", "2.0.0", wireBreaking, true},
		{"1.2.3", "1.3.0", wireAdditive, true},
		{"1.2.3", "1.2.4", wireNone, true},
	}
	for _, c := range cases {
		if ok, min := requiredBump(v(c.old), v(c.new), c.change); ok != c.ok {
			t.Errorf("%s -> %s for %v: ok = %v (min %s), want %v", c.old, c.new, c.change, ok, min, c.ok)
		}
	}
}

func TestParseSemver(t *testing.T) {
	for _, bad := range []string{"", "1.0", "v1.0.0", "1.0.0-rc1", "01.a.0"} {
		if _, err := parseSemver(bad); err == nil {
			t.Errorf("parseSemver(%q) accepted", bad)
		}
	}
}
