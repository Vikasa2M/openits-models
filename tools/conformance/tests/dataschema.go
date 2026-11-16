package tests

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"

	"github.com/Vikasa2M/openits-models/internal/wireshape"
)

// Snapshot is what the schema registry's index.json records about one
// module revision (see tools/yang-proto-gen/index.go, SnapshotIndex).
type Snapshot struct {
	Current     bool              `json:"current"`
	CeTypes     []string          `json:"ceTypes"`
	ShapeDigest string            `json:"shapeDigest"`
	Messages    map[string]string `json:"messages"`
}

// Registry is the schema registry a run checks events against: the
// published snapshots, by module and revision.
type Registry struct {
	Snapshots map[string]map[string]Snapshot `json:"snapshots"`
}

// LoadRegistry reads dir/index.json.
func LoadRegistry(dir string) (*Registry, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		return nil, err
	}
	var r Registry
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("parse %s/index.json: %w", dir, err)
	}
	return &r, nil
}

// DataSchemaBase is the origin every ce-dataschema URL is published under.
const DataSchemaBase = "https://schemas.open-its.org/"

// CurrentDataSchema returns the ce-dataschema a producer built from this
// release sends with ceType: the current revision of the module that defines
// its payload. ok is false when no current revision lists ceType.
func (r *Registry) CurrentDataSchema(ceType string) (url string, ok bool) {
	modules := make([]string, 0, len(r.Snapshots))
	for m := range r.Snapshots {
		modules = append(modules, m)
	}
	sort.Strings(modules)
	for _, m := range modules {
		for rev, s := range r.Snapshots[m] {
			if s.Current && slices.Contains(s.CeTypes, ceType) {
				return DataSchemaBase + m + "/" + rev + "/", true
			}
		}
	}
	return "", false
}

var dataSchemaRE = regexp.MustCompile(`^https://schemas\.open-its\.org/([a-z0-9-]+)/(\d{4}-\d{2}-\d{2})/$`)

// snapshotFor resolves an event's ce-dataschema, reporting why it cannot.
func snapshotFor(t *T, obs *Observation, e EventEnvelope) (module, rev string, s Snapshot, ok bool) {
	if obs.Registry == nil {
		t.Fatalf("no schema registry loaded (pass -registry)")
	}
	m := dataSchemaRE.FindStringSubmatch(e.CEDataSchema)
	if m == nil {
		t.Errorf("ce-dataschema %q on %s is not %s<module>/<revision>/", e.CEDataSchema, e.CEType, DataSchemaBase)
		return "", "", Snapshot{}, false
	}
	s, ok = obs.Registry.Snapshots[m[1]][m[2]]
	if !ok {
		t.Errorf("ce-dataschema on %s names %s@%s, which the schema registry has no snapshot of", e.CEType, m[1], m[2])
	}
	return m[1], m[2], s, ok
}

// TestCEDataSchema_ResolvesToSnapshot: every event names, in ce-dataschema,
// a published snapshot of the module that defines its ce-type.
func TestCEDataSchema_ResolvesToSnapshot(t *T, obs *Observation) {
	for _, e := range obs.Events {
		module, rev, s, ok := snapshotFor(t, obs, e)
		if !ok {
			continue
		}
		// ce-types are listed for current revisions; an older revision's
		// were whatever the release that shipped it said.
		if s.Current && !slices.Contains(s.CeTypes, e.CEType) {
			t.Errorf("ce-dataschema on %s names %s@%s, which does not define that ce-type", e.CEType, module, rev)
		}
	}
}

// TestCEDataSchema_PayloadMatchesSnapshot: the payload message the producer
// compiled has exactly the wire shape the named snapshot pins. This is what
// makes ce-dataschema a true statement: a producer built from a release whose
// protos drifted from the snapshot fails here, whatever its YANG says.
func TestCEDataSchema_PayloadMatchesSnapshot(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if e.Data == nil {
			continue
		}
		module, rev, s, ok := snapshotFor(t, obs, e)
		if !ok {
			continue
		}
		md := e.Data.ProtoReflect().Descriptor()
		if s.ShapeDigest == "" {
			t.Errorf("%s@%s predates schema.shape, so the %s payload's wire shape cannot be verified", module, rev, e.CEType)
			continue
		}
		want, ok := s.Messages[string(md.FullName())]
		if !ok {
			t.Errorf("%s payload is %s, which is not a root message of %s@%s", e.CEType, md.FullName(), module, rev)
			continue
		}
		if got := wireshape.Digest(md); got != want {
			t.Errorf("%s payload %s was compiled with wire shape %s, but %s@%s pins %s", e.CEType, md.FullName(), got, module, rev, want)
		}
	}
}
