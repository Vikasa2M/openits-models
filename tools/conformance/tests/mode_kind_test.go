package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	commonv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/common/v1"
)

func modeEv(service, kind, prev, mode string) EventEnvelope {
	mc := &commonv1.ModeChanged{Kind: kind, Mode: mode}
	if prev != "" {
		mc.PreviousMode = proto.String(prev)
	}
	return EventEnvelope{
		Subject: "openits.us-xx.agency.d01." + service + ".dev-1.mode-changed",
		CEType:  "openits." + service + ".mode-changed.v1",
		CEID:    service + kind + mode,
		Data:    mc,
	}
}

func runMode(events ...EventEnvelope) Result {
	return Run("mode", TestMode_ValueMatchesKind, &Observation{Events: events})
}

func TestModeKind_OwnFamilyPasses(t *testing.T) {
	cases := []EventEnvelope{
		modeEv("signal-control", "openits-signal-control-types:sc-mode-event-kind",
			"openits-signal-control-types:mode-free", "openits-signal-control-types:mode-coordinated"),
		modeEv("dms", "openits-dms-types:dms-mode-event-kind", "openits-dms-types:mode-blank", "openits-dms-types:mode-normal"),
		modeEv("ramp-metering", "openits-ramp-metering-types:ramp-meter-mode-event-kind",
			"openits-ramp-metering-types:mode-on-standby", "openits-ramp-metering-types:mode-active"),
		modeEv("rsu", "openits-rsu-types:rsu-mode-event-kind", "openits-rsu-types:mode-standby", "openits-rsu-types:mode-operate"),
	}
	for _, ev := range cases {
		if r := runMode(ev); !r.Pass {
			t.Errorf("own-family mode-changed failed: %s", r.Message)
		}
	}
}

// The DMS unknown dual-bases on the canonical openits-types:mode-unknown;
// it stays a sign-mode value, so it is accepted under the DMS kind.
func TestModeKind_DualBasedUnknownPassesForDMS(t *testing.T) {
	if r := runMode(modeEv("dms", "openits-dms-types:dms-mode-event-kind", "openits-dms-types:mode-unknown", "openits-dms-types:mode-normal")); !r.Pass {
		t.Fatalf("DMS mode-unknown rejected: %s", r.Message)
	}
}

func TestModeKind_CrossFamilyModeFails(t *testing.T) {
	r := runMode(modeEv("signal-control", "openits-signal-control-types:sc-mode-event-kind",
		"openits-signal-control-types:mode-free", "openits-dms-types:mode-blank"))
	if r.Pass || !strings.Contains(r.Message, "openits-dms-types:mode-blank") {
		t.Fatalf("cross-family mode not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

func TestModeKind_CrossFamilyPreviousModeFails(t *testing.T) {
	r := runMode(modeEv("signal-control", "openits-signal-control-types:sc-mode-event-kind",
		"openits-dms-types:mode-blank", "openits-signal-control-types:mode-free"))
	if r.Pass || !strings.Contains(r.Message, "previous-mode") {
		t.Fatalf("cross-family previous-mode not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

// The canonical identity itself does not derive from any family base; a
// family reports its own unknown (dual-based on the canonical) instead.
func TestModeKind_CanonicalUnknownOutsideFamilyFails(t *testing.T) {
	r := runMode(modeEv("signal-control", "openits-signal-control-types:sc-mode-event-kind", "", "openits-types:mode-unknown"))
	if r.Pass {
		t.Fatalf("canonical mode-unknown accepted under the signal-control kind")
	}
}

func TestModeKind_KindFromOtherServiceFails(t *testing.T) {
	r := runMode(modeEv("signal-control", "openits-dms-types:dms-mode-event-kind", "", "openits-dms-types:mode-normal"))
	if r.Pass || !strings.Contains(r.Message, "not the signal-control mode kind") {
		t.Fatalf("kind from another service not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

func TestModeKind_AbstractKindFails(t *testing.T) {
	if r := runMode(modeEv("dms", "openits-types:mode-event-kind", "", "openits-dms-types:mode-normal")); r.Pass {
		t.Fatalf("abstract mode-event-kind accepted")
	}
}

func TestModeKind_BareValueFails(t *testing.T) {
	r := runMode(modeEv("dms", "openits-dms-types:dms-mode-event-kind", "blank", "openits-dms-types:mode-normal"))
	if r.Pass || !strings.Contains(r.Message, "module prefix") {
		t.Fatalf("bare value not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

func TestModeKind_EmptyModeFails(t *testing.T) {
	r := runMode(modeEv("dms", "openits-dms-types:dms-mode-event-kind", "", ""))
	if r.Pass || !strings.Contains(r.Message, "no mode") {
		t.Fatalf("empty mode not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

func TestModeKind_EmptyKindFails(t *testing.T) {
	r := runMode(modeEv("dms", "", "", "openits-dms-types:mode-normal"))
	if r.Pass || !strings.Contains(r.Message, "no kind") {
		t.Fatalf("empty kind not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

func TestModeKind_MisspelledValueModuleFails(t *testing.T) {
	r := runMode(modeEv("dms", "openits-dms-types:dms-mode-event-kind", "", "openits-dms-type:mode-normal"))
	if r.Pass || !strings.Contains(r.Message, "unknown module") {
		t.Fatalf("misspelled value module not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

// Vendor modules are not generator inputs; a vendor-derived value is
// skipped, as TestFault_KindMatchesService skips vendor kinds.
func TestModeKind_VendorValueSkipped(t *testing.T) {
	if r := runMode(modeEv("signal-control", "openits-signal-control-types:sc-mode-event-kind", "",
		"openits-vendor-acme-signal-control-types:acme-mode-x")); !r.Pass {
		t.Fatalf("vendor value not skipped: %s", r.Message)
	}
}

func TestModeKind_VendorKindSkipped(t *testing.T) {
	if r := runMode(modeEv("signal-control", "openits-vendor-acme-signal-control-types:acme-mode-kind", "",
		"openits-dms-types:mode-normal")); !r.Pass {
		t.Fatalf("vendor kind not skipped: %s", r.Message)
	}
}

// CCTV mode values are not operational-mode identities yet; the kind is
// checked against the subject, the values are not.
func TestModeKind_CCTVValuesSkipped(t *testing.T) {
	if r := runMode(modeEv("cctv", "openits-cctv-types:cctv-mode-event-kind", "", "openits-cctv-types:cctv-control-local")); !r.Pass {
		t.Fatalf("CCTV values not skipped: %s", r.Message)
	}
}

// A service with no mode kind in the table (ess, reversible-lane,
// perception, traffic-sensor) must not emit mode-changed at all.
func TestModeKind_ServiceWithoutModeKindFails(t *testing.T) {
	for _, svc := range []string{"ess", "reversible-lane", "perception", "traffic-sensor", "widget"} {
		r := runMode(modeEv(svc, "openits-dms-types:dms-mode-event-kind", "", "openits-dms-types:mode-normal"))
		if r.Pass || !strings.Contains(r.Message, "service has no mode kind") {
			t.Errorf("%s: mode-changed not reported: pass=%v msg=%q", svc, r.Pass, r.Message)
		}
	}
}

func TestModeKind_ServiceWithoutModeKindVendorKindSkipped(t *testing.T) {
	if r := runMode(modeEv("ess", "openits-vendor-acme-ess-types:acme-mode-kind", "", "openits-vendor-acme-ess-types:acme-x")); !r.Pass {
		t.Fatalf("vendor kind not skipped: %s", r.Message)
	}
}

func TestModeKind_OtherEventSkipped(t *testing.T) {
	ev := faultEv("dms", "openits-signal-control-types:sc-fault-power")
	if r := runMode(ev); !r.Pass {
		t.Fatalf("non-mode event not checked as mode-changed: %s", r.Message)
	}
}

// Every family enum named by the table exists in the generated schema.
func TestModeKind_TableEnumsExist(t *testing.T) {
	for svc, f := range modeFamilyByService {
		if f.enum == "" {
			continue
		}
		if len(enumMembers(f.enum)) == 0 {
			t.Errorf("%s: enum %s missing from the generated schema", svc, f.enum)
		}
	}
}

// modeKindSkip lists mode-event-kinds deliberately absent from
// modeFamilyByService, each with the reason.
var modeKindSkip = map[string]string{}

var (
	identityRe     = regexp.MustCompile(`(?m)^\s*identity\s+([A-Za-z0-9._-]+)\s*\{`)
	baseStmtRe     = regexp.MustCompile(`\bbase\s+"?([A-Za-z0-9._:-]+)"?\s*;`)
	valueBaseRe    = regexp.MustCompile(`mode-value-base\s+"([^"]+)"`)
	modulePrefixRe = regexp.MustCompile(`(?m)^\s*prefix\s+"?([A-Za-z0-9._-]+)"?\s*;`)
	importRe       = regexp.MustCompile(`import\s+([A-Za-z0-9._-]+)\s*\{[^}]*?prefix\s+"?([A-Za-z0-9._-]+)"?\s*;`)
)

// yangIdentity is an identity read straight from the YANG source.
type yangIdentity struct {
	key       string   // module:name
	bases     []string // module-qualified
	valueBase string   // module-qualified mode-value-base, "" if none
}

// readYangIdentities parses every identity in yang/*.yang: its qualified
// bases and its mode-value-base, resolving prefixes through each module's
// own prefix and imports.
func readYangIdentities(t *testing.T) map[string]*yangIdentity {
	t.Helper()
	files, err := filepath.Glob("../../../yang/*.yang")
	if err != nil || len(files) == 0 {
		t.Fatalf("no YANG modules found: %v", err)
	}
	out := map[string]*yangIdentity{}
	for _, f := range files {
		mod := strings.TrimSuffix(filepath.Base(f), ".yang")
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(raw)
		prefixes := map[string]string{}
		if m := modulePrefixRe.FindStringSubmatch(src); m != nil {
			prefixes[m[1]] = mod
		}
		for _, m := range importRe.FindAllStringSubmatch(src, -1) {
			prefixes[m[2]] = m[1]
		}
		qualify := func(ref string) string {
			if p, l, ok := strings.Cut(ref, ":"); ok {
				if m, ok := prefixes[p]; ok {
					return m + ":" + l
				}
				return ref
			}
			return mod + ":" + ref
		}
		locs := identityRe.FindAllStringSubmatchIndex(src, -1)
		for n, loc := range locs {
			end := len(src)
			if n+1 < len(locs) {
				end = locs[n+1][0]
			}
			body := src[loc[1]:end]
			id := &yangIdentity{key: mod + ":" + src[loc[2]:loc[3]]}
			for _, b := range baseStmtRe.FindAllStringSubmatch(body, -1) {
				id.bases = append(id.bases, qualify(b[1]))
			}
			if m := valueBaseRe.FindStringSubmatch(body); m != nil {
				id.valueBase = qualify(m[1])
			}
			out[id.key] = id
		}
	}
	return out
}

// yangModeKinds returns every identity strictly derived from
// openits-types:mode-event-kind.
func yangModeKinds(ids map[string]*yangIdentity) []string {
	const root = "openits-types:mode-event-kind"
	var derives func(k string, seen map[string]bool) bool
	derives = func(k string, seen map[string]bool) bool {
		if seen[k] {
			return false
		}
		seen[k] = true
		if id := ids[k]; id != nil {
			for _, b := range id.bases {
				if b == root || derives(b, seen) {
					return true
				}
			}
		}
		return false
	}
	var kinds []string
	for k := range ids {
		if derives(k, map[string]bool{}) {
			kinds = append(kinds, k)
		}
	}
	sort.Strings(kinds)
	return kinds
}

// TestModeFamilyTable_CoversEveryModeEventKind fails when an identity
// derived from openits-types:mode-event-kind (however it is named) is
// neither in modeFamilyByService nor modeKindSkip, so a new family cannot
// slip past TestMode_ValueMatchesKind.
func TestModeFamilyTable_CoversEveryModeEventKind(t *testing.T) {
	kinds := yangModeKinds(readYangIdentities(t))
	covered := map[string]bool{}
	for _, f := range modeFamilyByService {
		covered[f.kind] = true
	}
	if len(kinds) < len(modeFamilyByService) {
		t.Fatalf("found %d mode-event-kind identities, fewer than the %d table entries", len(kinds), len(modeFamilyByService))
	}
	for _, k := range kinds {
		if !covered[k] && modeKindSkip[k] == "" {
			t.Errorf("%s is in neither modeFamilyByService nor modeKindSkip", k)
		}
	}
	for k := range modeKindSkip {
		if covered[k] {
			t.Errorf("%s is in both modeFamilyByService and modeKindSkip", k)
		}
	}
}

// TestModeFamilyTable_EnumModuleMatchesBase checks each table enum is the
// enum of the module that defines the kind's mode-value-base: every member
// of the enum records that module as its defining module.
func TestModeFamilyTable_EnumModuleMatchesBase(t *testing.T) {
	ids := readYangIdentities(t)
	for svc, f := range modeFamilyByService {
		if f.enum == "" {
			continue
		}
		kind := ids[f.kind]
		if kind == nil || kind.valueBase == "" {
			t.Errorf("%s: kind %s has no mode-value-base in the YANG", svc, f.kind)
			continue
		}
		baseModule, _, _ := strings.Cut(kind.valueBase, ":")
		members := enumMembers(f.enum)
		if len(members) == 0 {
			t.Errorf("%s: enum %s has no members", svc, f.enum)
			continue
		}
		for _, d := range members {
			if d.DefiningModule != "" && d.DefiningModule != baseModule {
				t.Errorf("%s: enum %s member %q is defined in %q, but the mode-value-base %s is in %q",
					svc, f.enum, d.Name, d.DefiningModule, kind.valueBase, baseModule)
			}
		}
	}
}
