package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGenerate_protocGenGoCompiles is the Go-package-level buildability gate.
// TestGenerate_protocCompiles (protoc_test.go) already runs protoc with `-o
// descriptor.pb`, which fully parses, resolves imports, and type-checks
// every generated .proto file — but that output mode never invokes
// protoc-gen-go, so it cannot see the failure mode ClaimedNames
// (ProtoFile.ClaimedNames, see emit.go) exists to prevent: two files that
// share a go_package (e.g. one service's events.proto + state.proto) still
// flatten into one Go package, and a top-level enum name that's perfectly
// valid to repeat across distinct proto *packages* becomes a Go symbol
// collision the moment both land in that one shared package.
//
// Note protoc-gen-go itself does NOT catch this: it generates each .proto
// file's Go bindings independently and has no notion that a different input
// file shares its `go_package` — running `protoc --go_out` alone exits 0
// even when two output files both declare `enum Severity`. The collision
// only becomes visible once the generated .go files are compiled together as
// the Go package they share on disk, where it surfaces as an ordinary Go
// "redeclared in this block" error. So this test doesn't stop at protoc's
// exit code: it also runs `go build` over protoc-gen-go's output.
//
// Per-service packaging means the generated output is now MANY Go packages
// (one per go_package/service) with real cross-package imports between them
// (e.g. openits/common/v1 imports openits/types/v1 for the shared
// WireSource message) — not one flat package. To compile that as a whole
// and have every absolute `github.com/openits/openits-models/pkg/proto/...`
// import resolve to what Generate JUST produced (not whatever happens to be
// committed on disk in the real pkg/proto tree, which this test must catch a
// regression in even before `make gen` has been re-run for real), the test
// lays the generated .go files out under a synthetic module root shaped
// exactly like the real repo (pkg/proto/... under module
// github.com/openits/openits-models) and copies this repo's own go.mod/
// go.sum into it verbatim, so `go build ./...` resolves both the protobuf
// runtime dependency and every cross-generated-package import against the
// files this test just generated, using the already-warm local module cache
// (no network access required).
func TestGenerate_protocGenGoCompiles(t *testing.T) {
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not installed")
	}
	if _, err := exec.LookPath("protoc-gen-go"); err != nil {
		t.Skip("protoc-gen-go not installed")
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

	goMod := t.TempDir()
	protoGoOut := filepath.Join(goMod, "pkg", "proto")
	if err := os.MkdirAll(protoGoOut, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", protoGoOut, err)
	}
	args := append([]string{
		"--proto_path=" + out,
		"--go_out=" + protoGoOut,
		"--go_opt=paths=source_relative",
	}, protoFiles...)

	protocCmd := exec.Command("protoc", args...)
	if output, err := protocCmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc --go_out failed to run protoc-gen-go over the generated .proto files:\n%v\n%s", err, output)
	}

	if len(globRecursive(t, filepath.Join(protoGoOut, "openits"), ".go")) == 0 {
		t.Fatal("protoc --go_out produced no openits/**/*.go files to compile")
	}

	for _, f := range []string{"go.mod", "go.sum"} {
		src := filepath.Join("..", "..", f)
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read %s: %v", src, err)
		}
		if err := os.WriteFile(filepath.Join(goMod, f), b, 0o644); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}

	// Compile every generated package the way `go build ./...` would across
	// the real repo: this is the step that catches a same-go_package
	// cross-file Go symbol collision, which surfaces as an ordinary Go
	// "redeclared in this block" error, as well as a broken cross-package
	// import between two generated go_packages.
	buildCmd := exec.Command("go", "build", "./...")
	buildCmd.Dir = goMod
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed to compile protoc-gen-go's generated Go packages:\n%v\n%s", err, output)
	}
}
