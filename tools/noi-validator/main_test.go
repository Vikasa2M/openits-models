package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// realSchema loads the repo's actual noi-schema.yaml so the tests validate
// against the same single source of truth `make validate-noi` uses.
func realSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	s, err := loadYAMLSchema(filepath.Join("..", "..", "schema-registry", "notices", "_schema", "noi-schema.yaml"))
	if err != nil {
		t.Fatalf("loadYAMLSchema: %v", err)
	}
	return s
}

// writeNoI writes body to <dir>/<augment>/<implementer>.yaml (the on-disk
// convention pathChecks enforces) and returns the file path.
func writeNoI(t *testing.T, dir, augment, implementer, body string) string {
	t.Helper()
	sub := filepath.Join(dir, augment)
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(sub, implementer+".yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

const validBody = `augment: test-augment
revision: 2026-07-08
implementer: ref-org
implementer_contact: https://github.com/ref-org
implementer_type: conformance-reference
first_observed: 2026-07-08
`

func TestValidateFile_valid(t *testing.T) {
	schema := realSchema(t)
	p := writeNoI(t, t.TempDir(), "test-augment", "ref-org", validBody)
	if errs := validateFile(schema, p); len(errs) != 0 {
		t.Errorf("valid NoI reported errors: %v", errs)
	}
}

// The whole point of strict field validation: a typo'd / unknown field must now be rejected via
// the schema's additionalProperties:false, which the old struct-based validator
// silently ignored.
func TestValidateFile_unknownFieldRejected(t *testing.T) {
	schema := realSchema(t)
	body := validBody + "implementer_contct: https://github.com/oops\n" // typo'd key
	p := writeNoI(t, t.TempDir(), "test-augment", "ref-org", body)
	errs := validateFile(schema, p)
	if len(errs) == 0 {
		t.Fatal("expected an additionalProperties error for the typo'd field, got none")
	}
	joined := strings.Join(errs, "; ")
	if !strings.Contains(joined, "additionalProperties") && !strings.Contains(joined, "implementer_contct") {
		t.Errorf("errors do not mention the unexpected property: %v", errs)
	}
}

// Unquoted YAML dates (revision, first_observed) must validate: they are kept as
// strings, not resolved to !!timestamp / "…T00:00:00Z" which would fail the
// schema's date/pattern rules.
func TestValidateFile_unquotedDatesStayStrings(t *testing.T) {
	schema := realSchema(t)
	// validBody already uses unquoted dates; assert explicitly it passes and
	// that a bad date is still caught.
	good := writeNoI(t, t.TempDir(), "test-augment", "ref-org", validBody)
	if errs := validateFile(schema, good); len(errs) != 0 {
		t.Errorf("unquoted-date NoI reported errors: %v", errs)
	}
	badBody := strings.Replace(validBody, "revision: 2026-07-08", "revision: 2026-7-8", 1)
	bad := writeNoI(t, t.TempDir(), "test-augment", "ref-org", badBody)
	if errs := validateFile(schema, bad); len(errs) == 0 {
		t.Error("expected a pattern error for malformed revision 2026-7-8, got none")
	}
}

func TestValidateFile_missingRequiredRejected(t *testing.T) {
	schema := realSchema(t)
	body := strings.Replace(validBody, "implementer_type: conformance-reference\n", "", 1)
	p := writeNoI(t, t.TempDir(), "test-augment", "ref-org", body)
	if errs := validateFile(schema, p); len(errs) == 0 {
		t.Error("expected a required-field error for missing implementer_type, got none")
	}
}

func TestValidateFile_pathMismatchRejected(t *testing.T) {
	schema := realSchema(t)
	// File placed under the wrong augment directory.
	p := writeNoI(t, t.TempDir(), "wrong-augment", "ref-org", validBody)
	errs := validateFile(schema, p)
	if len(errs) == 0 {
		t.Fatal("expected a path-convention error (dir != augment), got none")
	}
	if !strings.Contains(strings.Join(errs, "; "), "does not match augment") {
		t.Errorf("errors do not mention the augment/dir mismatch: %v", errs)
	}
}
