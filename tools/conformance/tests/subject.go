package tests

import (
	"regexp"
	"strings"
)

// openits.{region}.{agency}.{agency-unit}.{service}.{controller-id}.{event} → 7 tokens.
var tokenRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func TestSubject_SevenTokenShape(t *T, obs *Observation) {
	for _, e := range obs.Events {
		parts := strings.Split(e.Subject, ".")
		if len(parts) != 7 {
			t.Errorf("subject %q has %d tokens; want 7 (openits.region.agency.agency-unit.service.controller-id.event)", e.Subject, len(parts))
			continue
		}
		for _, p := range parts {
			if !tokenRE.MatchString(p) {
				t.Errorf("subject %q token %q is not lowercase-alnum-hyphen", e.Subject, p)
			}
		}
	}
}

func TestSubject_OpenitsPrefix(t *T, obs *Observation) {
	for _, e := range obs.Events {
		if !strings.HasPrefix(e.Subject, "openits.") {
			t.Errorf("subject %q does not start with the authority prefix openits.", e.Subject)
		}
	}
}
