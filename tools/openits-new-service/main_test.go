package main

import (
	"testing"
)

// TestReservedFaultModeNames exercises the guard that stops
// openits-new-service from scaffolding per-service fault/mode
// notifications (the per-service fault/mode anti-pattern that was removed). It calls the actual
// guard function, reservedFaultModeNames, via parseEvents — the same path
// main() uses to turn --events into []eventTpl before checking it.
func TestReservedFaultModeNames(t *testing.T) {
	tests := []struct {
		name      string
		eventsCSV string
		wantBad   []string
	}{
		// Exact reserved names.
		{"exact fault-raised", "fault-raised", []string{"fault-raised"}},
		{"exact fault-cleared", "fault-cleared", []string{"fault-cleared"}},
		{"exact mode-changed", "mode-changed", []string{"mode-changed"}},

		// Suffix forms.
		{"suffix sign-fault-raised", "sign-fault-raised", []string{"sign-fault-raised"}},
		{"suffix ramp-meter-mode-changed", "ramp-meter-mode-changed", []string{"ramp-meter-mode-changed"}},

		// Case variants — exercise the case-insensitive normalization.
		{"case Fault-Raised", "Fault-Raised", []string{"Fault-Raised"}},
		{"case MODE-CHANGED", "MODE-CHANGED", []string{"MODE-CHANGED"}},
		{"case suffix -Fault-Cleared", "sign-Fault-Cleared", []string{"sign-Fault-Cleared"}},

		// Legitimate names that merely contain the reserved substrings —
		// must be allowed (not flagged).
		{"allowed detector-fault-other", "detector-fault-other", nil},
		{"allowed vehicle-fault-code-report", "vehicle-fault-code-report", nil},
		{"allowed occupancy-changed", "occupancy-changed", nil},
		{"allowed zone-interval-report", "zone-interval-report", nil},

		// Mixed list: only the reserved one should be flagged.
		{"mixed list", "occupancy-changed,Fault-Raised,zone-interval-report", []string{"Fault-Raised"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := parseEvents(tt.eventsCSV)
			got := reservedFaultModeNames(events)

			if len(got) != len(tt.wantBad) {
				t.Fatalf("reservedFaultModeNames(%q) = %v, want %v", tt.eventsCSV, got, tt.wantBad)
			}
			for i, name := range got {
				if name != tt.wantBad[i] {
					t.Fatalf("reservedFaultModeNames(%q) = %v, want %v", tt.eventsCSV, got, tt.wantBad)
				}
			}
		})
	}
}
