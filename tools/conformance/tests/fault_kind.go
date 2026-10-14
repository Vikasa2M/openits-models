package tests

import (
	"strings"
)

// faultEnumByService maps a subject's service token to the generated enum
// type name of that service's fault-event-kind base. The enum lists every
// identity derived from the base, including the service's own form of a
// platform fault (for example sc-fault-cabinet-on-battery).
var faultEnumByService = map[string]string{
	"signal-control":  "E_OpenitsSignalControlTypes_ScFaultEventKind",
	"dms":             "E_OpenitsDmsTypes_DmsFaultEventKind",
	"rsu":             "E_OpenitsRsuTypes_RsuFaultEventKind",
	"ess":             "E_OpenitsEssTypes_EssFaultEventKind",
	"ramp-metering":   "E_OpenitsRampMeteringTypes_RampMeterFaultEventKind",
	"traffic-sensor":  "E_OpenitsTrafficSensorTypes_TrafficSensorFaultEventKind",
	"reversible-lane": "E_OpenitsReversibleLaneTypes_ReversibleLaneFaultEventKind",
	"perception":      "E_OpenitsPerceptionTypes_PerceptionFaultEventKind",
	"cctv":            "E_OpenitsCctvTypes_CctvFaultEventKind",
}

// TestFault_KindMatchesService checks that the kind carried by a
// fault-raised or fault-cleared event derives from the fault base of the
// service named in the event's subject. The notifications are shared across
// services, so YANG cannot type kind per service; this is where it is
// enforced. A kind from a module the generated schema does not know
// (a vendor extension) is skipped. A kind from a known module that is not
// in the subject service's fault enum fails, which covers another
// service's kind and a bare platform identity.
func TestFault_KindMatchesService(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if e.Data == nil {
			continue
		}
		parts := strings.Split(e.Subject, ".")
		if len(parts) != 7 {
			continue
		}
		last := parts[len(parts)-1]
		if last != "fault-raised" && last != "fault-cleared" {
			continue
		}
		m := e.Data.ProtoReflect()
		kf := m.Descriptor().Fields().ByName("kind")
		if kf == nil {
			continue
		}
		service := parts[4]
		enum, ok := faultEnumByService[service]
		if !ok {
			continue
		}
		kind := m.Get(kf).String()
		module, name, st := classifyIdentity(kind)
		switch st {
		case identityEmpty:
			t.Errorf("%s on a %s subject has no kind", last, service)
		case identityNoPrefix:
			t.Errorf("%s on a %s subject has kind %q without a module prefix", last, service, kind)
		case identityUnknownModule:
			t.Errorf("%s on a %s subject has kind %q from unknown module %q", last, service, kind, module)
		case identityChecked:
			if !enumHas(enum, module, name) {
				t.Errorf("%s on a %s subject has kind %q, which is not a %s fault", last, service, kind, service)
			}
		}
	}
}
