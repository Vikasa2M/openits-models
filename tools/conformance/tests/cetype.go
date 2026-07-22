package tests

import (
	"regexp"
	"strings"
)

var ceTypeRE = regexp.MustCompile(`^openits\.[a-z0-9-]+\.[a-z0-9-]+\.v\d+$`)

// urnRE matches the per-service ce-source URN form
// `urn:openits:<entity-kind>:<region>:<agency>:<unit>:<id>`. Entity kinds
// are per-service (controller, sign, station, rsu, ramp-meter, poller);
// the regex accepts any well-formed kebab-case kind so new services don't
// require updating this regex.
var urnRE = regexp.MustCompile(`^urn:openits:[a-z][a-z0-9-]*:[a-z0-9-]+:[a-z0-9-]+:[a-z0-9-]+:[A-Za-z0-9_-]+$`)

func TestCEType_OpenitsFormat(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !ceTypeRE.MatchString(e.CEType) {
			t.Errorf("ce-type %q does not match openits.<service>.<event>.v<major>", e.CEType)
		}
	}
}

func TestCESource_URN(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !urnRE.MatchString(e.CESource) {
			t.Errorf("ce-source %q is not a well-formed openits URN (urn:openits:<entity-kind>:<region>:<agency>:<unit>:<id>)", e.CESource)
		}
	}
}

func TestCEID_Present(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if strings.TrimSpace(e.CEID) == "" {
			t.Errorf("ce-id empty on event %q (subject %q); breaks idempotent replay", e.CEType, e.Subject)
		}
	}
}
