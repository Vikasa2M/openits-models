package main

import (
	"bytes"
	"flag"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The wire-mapping test vectors keep docs/wire-mapping-spec.md honest the
// way tools/check-ce-id-vectors keeps docs/ce-id-spec.md honest: the spec
// states the YANG-to-wire mapping as rules, and each rule cites a vector
// under testdata/wire-mapping/ whose expected output this test reproduces
// byte for byte by running the real generator (Generate) over the vector's
// YANG.
//
// Layout of one vector, testdata/wire-mapping/<name>/:
//
//	input/*.yang               the vector's YANG modules (ietf-yang-types and
//	                           ietf-inet-types from yang/ietf are always
//	                           available to import)
//	input/field-numbers.yaml   optional: the field-number lock as it stands
//	                           before generation
//	expected/                  the complete output tree Generate writes (every
//	                           .proto and .schema.json), plus
//	                           expected/field-numbers.yaml, the lock as
//	                           Generate saves it
//	expected-error.txt         instead of expected/: a substring the error
//	                           Generate returns must contain
//
// A vector that changes is a mapping change, and a mapping change is a spec
// change: regenerate with
//
//	go test ./tools/yang-proto-gen -run TestWireMappingVectors -update-wire-vectors
//
// then review the diff, update the spec text and its changelog, and bump the
// revision of every module whose generated output the change alters.
var updateWireVectors = flag.Bool("update-wire-vectors", false,
	"rewrite testdata/wire-mapping/*/expected from the current generator instead of comparing")

const (
	wireVectorsDir  = "testdata/wire-mapping"
	wireMappingSpec = "../../docs/wire-mapping-spec.md"
	vectorLockName  = "field-numbers.yaml"
)

func TestWireMappingVectors(t *testing.T) {
	names := wireVectorNames(t)
	if len(names) == 0 {
		t.Fatalf("no vectors under %s", wireVectorsDir)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) { runWireVector(t, name) })
	}
}

// TestWireMappingSpecCitesVectors fails when the spec's vector index and the
// vector directories disagree, in either direction: a rule cannot cite a
// vector that does not exist, and a vector nobody cites is not part of the
// specification. It also ties the vector set to the spec version it was
// generated for.
func TestWireMappingSpecCitesVectors(t *testing.T) {
	raw, err := os.ReadFile(wireMappingSpec)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	spec := strings.ReplaceAll(string(raw), "\r\n", "\n")

	cited := specVectorIndex(t, spec)
	onDisk := map[string]bool{}
	for _, n := range wireVectorNames(t) {
		onDisk[n] = true
		if !cited[n] {
			t.Errorf("vector %q is not listed in the spec's vector index", n)
		}
	}
	for n := range cited {
		if !onDisk[n] {
			t.Errorf("spec's vector index lists %q, but %s/%s does not exist", n, wireVectorsDir, n)
		}
	}

	// Every indexed vector must also be cited from the body of the spec,
	// next to the rule it pins.
	body := spec
	if i := strings.Index(spec, "\n## Test vectors"); i >= 0 {
		body = spec[:i]
	}
	for n := range cited {
		if !strings.Contains(body, "`"+n+"`") {
			t.Errorf("vector %q is indexed but no rule in the spec cites it", n)
		}
	}

	wantVer, err := os.ReadFile(filepath.Join(wireVectorsDir, "SPEC_VERSION"))
	if err != nil {
		t.Fatalf("read SPEC_VERSION: %v", err)
	}
	m := regexp.MustCompile("(?m)^\\*\\*Version:\\*\\* `([^`]+)`").FindStringSubmatch(spec)
	if m == nil {
		t.Fatalf("spec does not declare **Version:** `x.y.z`")
	}
	if got, want := m[1], strings.TrimSpace(string(wantVer)); got != want {
		t.Errorf("spec declares version %s, vectors were generated for %s (testdata/wire-mapping/SPEC_VERSION)", got, want)
	}
}

// specVectorIndex returns the vector names listed in the first column of the
// table under the spec's "## Test vectors" heading.
func specVectorIndex(t *testing.T, spec string) map[string]bool {
	t.Helper()
	i := strings.Index(spec, "\n## Test vectors")
	if i < 0 {
		t.Fatalf("spec has no \"## Test vectors\" section")
	}
	section := spec[i+1:]
	if j := strings.Index(section[1:], "\n## "); j >= 0 {
		section = section[:j+1]
	}
	row := regexp.MustCompile("(?m)^\\|\\s*`([a-z0-9-]+)`\\s*\\|")
	out := map[string]bool{}
	for _, m := range row.FindAllStringSubmatch(section, -1) {
		out[m[1]] = true
	}
	if len(out) == 0 {
		t.Fatalf("spec's \"## Test vectors\" section lists no vectors")
	}
	return out
}

func wireVectorNames(t *testing.T) []string {
	t.Helper()
	ents, err := os.ReadDir(wireVectorsDir)
	if err != nil {
		t.Fatalf("read %s: %v", wireVectorsDir, err)
	}
	var names []string
	for _, e := range ents {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

func runWireVector(t *testing.T, name string) {
	dir := filepath.Join(wireVectorsDir, name)
	work := t.TempDir()
	yangDir := filepath.Join(work, "yang")
	outDir := filepath.Join(work, "out")
	lockPath := filepath.Join(work, vectorLockName)

	// The vector's own modules, plus the vendored ietf modules every real
	// generation run can import.
	inputs, err := filepath.Glob(filepath.Join(dir, "input", "*.yang"))
	if err != nil || len(inputs) == 0 {
		t.Fatalf("vector %s has no input/*.yang (err %v)", name, err)
	}
	for _, f := range inputs {
		wvCopyFile(t, f, filepath.Join(yangDir, filepath.Base(f)))
	}
	ietf, _ := filepath.Glob(filepath.Join("..", "..", "yang", "ietf", "*.yang"))
	for _, f := range ietf {
		wvCopyFile(t, f, filepath.Join(yangDir, "ietf", filepath.Base(f)))
	}
	if src := filepath.Join(dir, "input", vectorLockName); wvFileExists(src) {
		wvCopyFile(t, src, lockPath)
	}

	genErr := Generate(yangDir, outDir, lockPath)

	errFile := filepath.Join(dir, "expected-error.txt")
	if wvFileExists(errFile) {
		want := strings.TrimSpace(wvReadNormalized(t, errFile))
		if genErr == nil {
			t.Fatalf("Generate succeeded; vector expects an error containing %q", want)
		}
		if !strings.Contains(genErr.Error(), want) {
			t.Fatalf("Generate error\n  %v\ndoes not contain\n  %s", genErr, want)
		}
		return
	}
	if genErr != nil {
		t.Fatalf("Generate: %v", genErr)
	}

	// A vector can never pin output protoc would reject, whether it is
	// being compared or rewritten.
	wvProtocCheck(t, outDir)

	got := wvTreeFiles(t, outDir)
	got[vectorLockName] = wvReadNormalized(t, lockPath)

	expDir := filepath.Join(dir, "expected")
	if *updateWireVectors {
		if err := os.RemoveAll(expDir); err != nil {
			t.Fatal(err)
		}
		for rel, content := range got {
			wvWriteFile(t, filepath.Join(expDir, rel), content)
		}
		return
	}

	want := wvTreeFiles(t, expDir)
	for _, rel := range wvSortedKeys(want) {
		g, ok := got[rel]
		if !ok {
			t.Errorf("expected %s, but Generate did not write it", rel)
			continue
		}
		if g != want[rel] {
			t.Errorf("%s differs from the vector\n--- got ---\n%s\n--- want ---\n%s", rel, g, want[rel])
		}
	}
	for _, rel := range wvSortedKeys(got) {
		if _, ok := want[rel]; !ok {
			t.Errorf("Generate wrote %s, which the vector does not expect", rel)
		}
	}
}

// wvProtocCheck compiles the vector's generated .proto files. Skipped when
// protoc is not installed; the byte comparison still runs.
func wvProtocCheck(t *testing.T, outDir string) {
	t.Helper()
	if _, err := exec.LookPath("protoc"); err != nil {
		return
	}
	var protos []string
	for rel := range wvTreeFiles(t, outDir) {
		if strings.HasSuffix(rel, ".proto") {
			protos = append(protos, rel)
		}
	}
	if len(protos) == 0 {
		return
	}
	sort.Strings(protos)
	args := append([]string{"--proto_path=" + outDir, "-o" + filepath.Join(t.TempDir(), "d.pb")}, protos...)
	cmd := exec.Command("protoc", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Errorf("protoc rejects the vector's output: %v\n%s", err, stderr.String())
	}
}

// wvTreeFiles returns every regular file under root, keyed by its slash-separated
// path relative to root, with line endings normalized.
func wvTreeFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if !wvFileExists(root) {
		return out
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = wvReadNormalized(t, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

func wvReadNormalized(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

func wvCopyFile(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	wvWriteFile(t, dst, string(b))
}

func wvWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func wvFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func wvSortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
