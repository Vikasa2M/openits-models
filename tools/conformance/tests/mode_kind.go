package tests

import (
	"strings"

	commonv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/common/v1"
)

// modeFamily is a service's mode-changed kind and the generated enum of
// the identity its openits-types:mode-value-base names. An empty enum
// means the kind carries no mode-value-base yet (CCTV: its mode values are
// not operational-mode identities; reconciled in the CCTV cut), so only
// the kind is checked.
type modeFamily struct {
	kind string
	enum string
}

// modeFamilyByService maps a subject's service token to its mode family.
// The kind is matched exactly: a sub-kind (an identity derived from a
// family's mode-event-kind) is not supported by this table and would need
// enum-membership matching. It mirrors the mode-value-base statements in the -types modules, which
// tools/check-mode-kinds enforces on the YANG side.
var modeFamilyByService = map[string]modeFamily{
	"signal-control": {"openits-signal-control-types:sc-mode-event-kind", "E_OpenitsSignalControlTypes_ControllerMode"},
	"dms":            {"openits-dms-types:dms-mode-event-kind", "E_OpenitsDmsTypes_SignMode"},
	"ramp-metering":  {"openits-ramp-metering-types:ramp-meter-mode-event-kind", "E_OpenitsRampMeteringTypes_MeterMode"},
	"rsu":            {"openits-rsu-types:rsu-mode-event-kind", "E_OpenitsRsuTypes_RsuMode"},
	"cctv":           {"openits-cctv-types:cctv-mode-event-kind", ""},
}

// TestMode_ValueMatchesKind checks a mode-changed event against its
// subject's service: the kind must be that service's mode kind, and mode
// and previous-mode must be members of the family enum its
// mode-value-base names (identity name and defining module). The
// notification types both values to the cross-service operational-mode,
// so YANG cannot stop a DMS value under a signal-control kind; this is
// where it is enforced. A vendor-module kind or value is skipped, as in
// TestFault_KindMatchesService; an unknown openits module fails.
func TestMode_ValueMatchesKind(t *T, obs *Observation) {
	for _, e := range obs.Events {
		parts := strings.Split(e.Subject, ".")
		if len(parts) != 7 || parts[6] != "mode-changed" {
			continue
		}
		mc, ok := e.Data.(*commonv1.ModeChanged)
		if !ok {
			continue
		}
		service := parts[4]
		fam, ok := modeFamilyByService[service]
		if !ok {
			continue
		}
		module, _, st := classifyIdentity(mc.GetKind())
		switch st {
		case identityEmpty:
			t.Errorf("mode-changed on a %s subject has no kind", service)
			continue
		case identityNoPrefix:
			t.Errorf("mode-changed on a %s subject has kind %q without a module prefix", service, mc.GetKind())
			continue
		case identitySkipped:
			continue
		case identityUnknownModule:
			t.Errorf("mode-changed on a %s subject has kind %q from unknown module %q", service, mc.GetKind(), module)
			continue
		}
		if mc.GetKind() != fam.kind {
			t.Errorf("mode-changed on a %s subject has kind %q, which is not the %s mode kind %s",
				service, mc.GetKind(), service, fam.kind)
			continue
		}
		if fam.enum == "" {
			continue
		}
		checkValue := func(leaf, v string) {
			module, name, st := classifyIdentity(v)
			switch st {
			case identityEmpty:
				t.Errorf("mode-changed on a %s subject has no %s", service, leaf)
			case identityNoPrefix:
				t.Errorf("mode-changed on a %s subject has %s %q without a module prefix", service, leaf, v)
			case identityUnknownModule:
				t.Errorf("mode-changed on a %s subject has %s %q from unknown module %q", service, leaf, v, module)
			case identityChecked:
				if !enumHas(fam.enum, module, name) {
					t.Errorf("mode-changed on a %s subject has %s %q, which does not derive from the %s mode base (%s)",
						service, leaf, v, service, fam.enum)
				}
			}
		}
		checkValue("mode", mc.GetMode())
		if mc.PreviousMode != nil {
			checkValue("previous-mode", mc.GetPreviousMode())
		}
	}
}
