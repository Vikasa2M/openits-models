// noi-validator walks schema-registry/notices/ and validates every NoI YAML
// against schema-registry/notices/_schema/noi-schema.yaml — the single source
// of truth. Field rules (required, patterns, enums, additionalProperties:false,
// formats) live in that schema, not in this program, so the two cannot drift.
// This validator only adds the filesystem-convention checks that a per-file
// JSON Schema cannot express (directory == augment, filename == implementer).
//
// Wired to `make validate-noi`. Exits non-zero if any NoI is malformed.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

func main() {
	root := "schema-registry/notices"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	os.Exit(run(root, os.Stdout, os.Stderr))
}

// run validates every NoI under root against the schema and returns a process
// exit code (0 ok, 1 validation failures, 2 setup error). Split out from main
// so tests can drive it.
func run(root string, stdout, stderr *os.File) int {
	schemaPath := filepath.Join(root, "_schema", "noi-schema.yaml")
	schema, err := loadYAMLSchema(schemaPath)
	if err != nil {
		fmt.Fprintf(stderr, "noi-validator: load schema %s: %v\n", schemaPath, err)
		return 2
	}

	files, err := findNoIFiles(root)
	if err != nil {
		fmt.Fprintf(stderr, "noi-validator: %v\n", err)
		return 2
	}
	if len(files) == 0 {
		fmt.Fprintln(stdout, "noi-validator: no NoI files found")
		return 0
	}

	fail, pass := 0, 0
	for _, f := range files {
		errs := validateFile(schema, f)
		if len(errs) == 0 {
			pass++
			continue
		}
		fail++
		fmt.Fprintf(stderr, "FAIL  %s\n", f)
		for _, e := range errs {
			fmt.Fprintf(stderr, "      - %s\n", e)
		}
	}

	fmt.Fprintf(stdout, "noi-validator: %d passed, %d failed (%d total)\n", pass, fail, len(files))
	if fail > 0 {
		return 1
	}
	return 0
}

// loadYAMLSchema compiles the YAML-authored JSON Schema. jsonschema reads JSON,
// so the schema is round-tripped yaml -> json first. AssertFormat is enabled so
// the schema's `format: email` / `format: date` keywords are enforced (they are
// annotation-only by default in draft-07).
func loadYAMLSchema(path string) (*jsonschema.Schema, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	jsonBytes, err := yamlToJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("convert schema yaml->json: %w", err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat = true
	const id = "noi-schema.json"
	if err := c.AddResource(id, bytes.NewReader(jsonBytes)); err != nil {
		return nil, err
	}
	return c.Compile(id)
}

// yamlToJSON parses YAML into a generic JSON-compatible value and encodes it as
// JSON. It walks the yaml.Node tree rather than decoding straight into
// interface{} so that YAML's automatic !!timestamp resolution does not fire:
// an unquoted `2026-07-08` must stay the string "2026-07-08" (JSON has no date
// type), otherwise it would round-trip to "2026-07-08T00:00:00Z" and fail the
// schema's `format: date` / `^\d{4}-\d{2}-\d{2}$` rules.
func yamlToJSON(raw []byte) ([]byte, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil, err
	}
	v, err := nodeToValue(&node)
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

func nodeToValue(n *yaml.Node) (interface{}, error) {
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		return nodeToValue(n.Content[0])
	case yaml.MappingNode:
		m := make(map[string]interface{}, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			val, err := nodeToValue(n.Content[i+1])
			if err != nil {
				return nil, err
			}
			m[n.Content[i].Value] = val
		}
		return m, nil
	case yaml.SequenceNode:
		s := make([]interface{}, 0, len(n.Content))
		for _, c := range n.Content {
			v, err := nodeToValue(c)
			if err != nil {
				return nil, err
			}
			s = append(s, v)
		}
		return s, nil
	case yaml.ScalarNode:
		return scalarToValue(n), nil
	case yaml.AliasNode:
		return nodeToValue(n.Alias)
	default:
		return nil, fmt.Errorf("unsupported yaml node kind %d", n.Kind)
	}
}

// scalarToValue maps a YAML scalar to a JSON-native value by its resolved tag.
// Anything that is not a JSON primitive (notably !!timestamp) is kept as its
// raw string text.
func scalarToValue(n *yaml.Node) interface{} {
	switch n.Tag {
	case "!!null":
		return nil
	case "!!bool":
		if b, err := strconv.ParseBool(n.Value); err == nil {
			return b
		}
	case "!!int":
		if i, err := strconv.ParseInt(n.Value, 0, 64); err == nil {
			return i
		}
	case "!!float":
		if f, err := strconv.ParseFloat(n.Value, 64); err == nil {
			return f
		}
	}
	return n.Value
}

func findNoIFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip the schema directory itself.
			if d.Name() == "_schema" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(d.Name()) != ".yaml" {
			return nil
		}
		// Skip top-level non-NoI files (organizations.yaml, README, schema).
		rel, _ := filepath.Rel(root, path)
		if filepath.Dir(rel) == "." {
			return nil
		}
		out = append(out, path)
		return nil
	})
	sort.Strings(out)
	return out, err
}

func validateFile(schema *jsonschema.Schema, path string) []string {
	body, err := os.ReadFile(path)
	if err != nil {
		return []string{fmt.Sprintf("read: %v", err)}
	}
	jsonBytes, err := yamlToJSON(body)
	if err != nil {
		return []string{fmt.Sprintf("yaml parse: %v", err)}
	}
	var inst interface{}
	if err := json.Unmarshal(jsonBytes, &inst); err != nil {
		return []string{fmt.Sprintf("decode: %v", err)}
	}

	var errs []string
	if err := schema.Validate(inst); err != nil {
		errs = append(errs, flattenValidationErr(err)...)
	}
	// Filesystem convention, not expressible in the per-file JSON Schema.
	errs = append(errs, pathChecks(inst, path)...)
	return errs
}

// flattenValidationErr turns a jsonschema validation error tree into one string
// per leaf failure ("<instance-location>: <message>").
func flattenValidationErr(err error) []string {
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return []string{err.Error()}
	}
	var out []string
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			loc := e.InstanceLocation
			if loc == "" {
				loc = "/"
			}
			out = append(out, fmt.Sprintf("%s: %s", loc, e.Message))
			return
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	return out
}

// pathChecks enforces the on-disk convention that a NoI lives at
// notices/<augment>/<implementer>.yaml. These are filesystem facts, not
// document fields, so they are not in the JSON Schema.
func pathChecks(inst interface{}, path string) []string {
	m, ok := inst.(map[string]interface{})
	if !ok {
		return nil
	}
	var errs []string
	augment, _ := m["augment"].(string)
	implementer, _ := m["implementer"].(string)
	if augment != "" {
		if dir := filepath.Base(filepath.Dir(path)); dir != augment {
			errs = append(errs, fmt.Sprintf("directory %q does not match augment %q", dir, augment))
		}
	}
	if implementer != "" {
		want := implementer + ".yaml"
		if base := filepath.Base(path); base != want {
			errs = append(errs, fmt.Sprintf("filename %q does not match implementer %q (want %s)", base, implementer, want))
		}
	}
	return errs
}
