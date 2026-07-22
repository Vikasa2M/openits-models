package main

import (
	"bytes"
	"io/fs"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// globRecursive returns every file under root whose name has the given
// suffix (e.g. ".proto" or ".go"), sorted. Generate's per-service output now
// spans many nested go_package directories instead of one flat openits/v1/,
// so tests that used to filepath.Glob a single directory need to walk the
// whole tree instead.
func globRecursive(t *testing.T, root, suffix string) []string {
	t.Helper()
	var out []string
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, suffix) {
			out = append(out, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Strings(out)
	return out
}

// TestGenerate_protocCompiles is the real buildability gate: it generates
// the full openits YANG corpus (the same input `go run ./tools/yang-proto-gen`
// uses by default) into a temp dir and then hands every generated
// openits/**/*.proto file to the actual protoc compiler. Generate's own
// validation (validateUniqueMessages/validateUniqueFieldTags, see main.go)
// only catches problems it knows to check for; this test catches everything
// else protoc itself would reject — like the enum double-definition bug
// fixed alongside this test (see TestEmitEnum_unspecifiedMemberNoDuplicate)
// — so that class of bug can't recur silently again.
func TestGenerate_protocCompiles(t *testing.T) {
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not installed")
	}

	out := t.TempDir()
	lock := filepath.Join(out, "field-numbers.yaml")
	if err := Generate(filepath.Join("..", "..", "yang"), out, lock); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	protoFiles := globRecursive(t, filepath.Join(out, "openits"), ".proto")
	if len(protoFiles) == 0 {
		t.Fatal("Generate produced no openits/**/*.proto files to compile")
	}

	// protoc refuses to run at all without some output directive (it exits
	// with "Missing output directives" otherwise); -o a scratch descriptor
	// set is the standard way to make it fully parse, resolve imports, and
	// type-check every input file — exactly the "does this compile"
	// validation this test wants — without producing any language bindings.
	descriptorOut := filepath.Join(out, "descriptor.pb")
	args := append([]string{
		"--proto_path=" + out,
		"-o" + descriptorOut,
	}, protoFiles...)

	cmd := exec.Command("protoc", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("protoc failed to compile generated .proto files: %v\nstderr:\n%s", err, stderr.String())
	}
}
