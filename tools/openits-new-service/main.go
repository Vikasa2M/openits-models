// openits-new-service scaffolds the boilerplate-heavy files for a new
// service package. It generates skeletons; the contributor fills in
// substance (real OID mappings, real protobuf payloads, real conformance
// tests).
//
// Reduces a ~16-file-touch contribution to "run the tool, then edit five
// substantive files." See docs/tutorial-add-a-service.md for the
// 30-minute walkthrough.
//
// Usage:
//
//	go run ./tools/openits-new-service \
//	    --service parking-sensor \
//	    --description "On-street and off-street parking-availability sensors" \
//	    --events occupancy-changed,zone-interval-report \
//	    [--reference "NTCIP 1208 (functional)"] \
//	    [--dry-run]
//
// --events is for genuinely service-specific domain events only. Fault and
// mode events are NOT scaffolded per service: every service emits the
// common openits-common-fault-events:{fault-raised,fault-cleared} and
// openits-common-mode-events:mode-changed notifications, discriminated by
// a service-derived `kind` identityref. See docs/data-model.md ("The event
// taxonomy") and the header comment of the generated events module.
//
// Generated files:
//
//	yang/openits-<service>.yang                              — base module skeleton
//	yang/openits-<service>-events.yang                       — service-specific domain-event skeleton (-events companion naming), one notification per --event; does NOT include fault/mode
//	api/proto/openits/<service_snake>/v1/events.proto        — proto skeleton with one message per --event, in the service's own go_package
//
// Followed by a "Next steps" printout listing the five remaining edits
// (envelope.go service spec, publisher methods, translator package,
// projector, conformance tests).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type service struct {
	Service     string // "parking-sensor"
	Camel       string // "ParkingSensor"
	Snake       string // "parking_sensor"
	Description string
	Reference   string
	Events      []eventTpl // hyphen-form
	Revision    string     // YYYY-MM-DD
	Year        int
}

type eventTpl struct {
	Hyphen string // "occupancy-changed"
	Camel  string // "OccupancyChanged"
	Snake  string // "occupancy_changed"
}

func main() {
	var (
		svcName   = flag.String("service", "", "Service name (hyphen-form, e.g., parking-sensor)")
		desc      = flag.String("description", "", "One-sentence service description")
		ref       = flag.String("reference", "", "Optional reference standard (e.g., NTCIP 1208)")
		eventsCSV = flag.String("events", "", "Comma-separated SERVICE-SPECIFIC domain event names (hyphen-form, e.g., occupancy-changed,zone-interval-report). Do not list fault/mode events here; those are emitted via the common openits-common-fault-events / openits-common-mode-events modules with a kind identityref — see the generated events module's header comment.")
		dryRun    = flag.Bool("dry-run", false, "Print files that would be generated; do not write")
		outRoot   = flag.String("root", ".", "Repository root (defaults to cwd)")
	)
	flag.Parse()

	if *svcName == "" || *desc == "" || *eventsCSV == "" {
		fmt.Fprintln(os.Stderr, "openits-new-service: --service, --description, and --events are required")
		flag.Usage()
		os.Exit(2)
	}

	if !validServiceName(*svcName) {
		fmt.Fprintf(os.Stderr, "openits-new-service: invalid service name %q (must match [a-z][a-z0-9-]+)\n", *svcName)
		os.Exit(2)
	}

	events := parseEvents(*eventsCSV)
	if len(events) == 0 {
		fmt.Fprintln(os.Stderr, "openits-new-service: --events must list at least one event")
		os.Exit(2)
	}

	if bad := reservedFaultModeNames(events); len(bad) > 0 {
		fmt.Fprintf(os.Stderr, "openits-new-service: --events must not include fault/mode notifications (%s) — these are emitted via openits-common-fault-events / openits-common-mode-events with a kind identityref, not scaffolded per service. Derive %s-fault-event-kind / %s-mode-event-kind in openits-%s-types.yang instead. See docs/data-model.md (\"The event taxonomy\").\n", strings.Join(bad, ", "), *svcName, *svcName, *svcName)
		os.Exit(2)
	}

	svc := service{
		Service:     *svcName,
		Camel:       toCamel(*svcName),
		Snake:       toSnake(*svcName),
		Description: *desc,
		Reference:   *ref,
		Events:      events,
		Revision:    time.Now().UTC().Format("2006-01-02"),
		Year:        time.Now().UTC().Year(),
	}

	files := []struct {
		path     string
		template func(service) string
	}{
		{filepath.Join(*outRoot, "yang", "openits-"+svc.Service+".yang"), renderBaseYANG},
		{filepath.Join(*outRoot, "yang", "openits-"+svc.Service+"-events.yang"), renderEventsYANG},
		{filepath.Join(*outRoot, "api", "proto", "openits", svc.Snake, "v1", "events.proto"), renderEventsProto},
	}

	for _, f := range files {
		body := f.template(svc)
		if *dryRun {
			fmt.Printf("--- %s ---\n%s\n", f.path, body)
			continue
		}
		if err := writeIfAbsent(f.path, body); err != nil {
			fmt.Fprintf(os.Stderr, "openits-new-service: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[ok] wrote %s\n", f.path)
	}

	if *dryRun {
		return
	}
	printNextSteps(svc)
}

func validServiceName(s string) bool {
	if len(s) == 0 || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for _, c := range s {
		if !(c == '-' || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

func parseEvents(csv string) []eventTpl {
	out := []eventTpl{}
	for _, raw := range strings.Split(csv, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		out = append(out, eventTpl{
			Hyphen: name,
			Camel:  toCamel(name),
			Snake:  toSnake(name),
		})
	}
	return out
}

// reservedFaultModeNames returns the hyphen-form names in events that
// collide with the common fault/mode notification pattern
// (openits-common-fault-events:{fault-raised,fault-cleared} and
// openits-common-mode-events:mode-changed). --events is for
// service-specific domain events only; scaffolding a bespoke
// "<svc>-fault-raised" or "<svc>-mode-changed" notification is exactly
// the per-service fault/mode anti-pattern that was removed in favour of the
// common notifications, so the scaffold refuses to
// regenerate it. The comparison is case-insensitive so "Fault-Raised" or
// "MODE-CHANGED" can't bypass the guard; the original casing is preserved
// in the returned names (used verbatim in the error message).
func reservedFaultModeNames(events []eventTpl) []string {
	var bad []string
	for _, e := range events {
		h := e.Hyphen
		lower := strings.ToLower(h)
		switch {
		case lower == "fault-raised", lower == "fault-cleared", lower == "mode-changed":
			bad = append(bad, h)
		case strings.HasSuffix(lower, "-fault-raised"), strings.HasSuffix(lower, "-fault-cleared"), strings.HasSuffix(lower, "-mode-changed"):
			bad = append(bad, h)
		}
	}
	return bad
}

func toCamel(hyphen string) string {
	parts := strings.Split(hyphen, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}

func toSnake(hyphen string) string { return strings.ReplaceAll(hyphen, "-", "_") }

// goPkgName derives a service's bare Go package name from its snake-case
// name, mirroring tools/yang-proto-gen's goPkgName: drop "_" separators, keep
// the version suffix — e.g. "parking_sensor" -> "parkingsensorv1". Every
// generated proto now gets its own go_package/directory
// (pkg/proto/openits/<snake>/v1), not the single shared openitspb package.
func goPkgName(snake string) string {
	return strings.ReplaceAll(snake, "_", "") + "v1"
}

func writeIfAbsent(path, body string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists; refusing to overwrite", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

// --- templates ---

func renderBaseYANG(s service) string {
	var b strings.Builder
	fmt.Fprintf(&b, "module openits-%s {\n", s.Service)
	fmt.Fprintf(&b, "  yang-version 1.1;\n")
	fmt.Fprintf(&b, "  namespace \"urn:openits:yang:%s\";\n", s.Service)
	fmt.Fprintf(&b, "  prefix openits-%s;\n", s.Service)
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "  organization\n")
	fmt.Fprintf(&b, "    \"OpenITS Working Group (proposal)\";\n")
	fmt.Fprintf(&b, "  contact\n")
	fmt.Fprintf(&b, "    \"TODO: maintainer email\";\n")
	fmt.Fprintf(&b, "  description\n")
	fmt.Fprintf(&b, "    \"%s\";\n", s.Description)
	fmt.Fprintf(&b, "\n")
	if s.Reference != "" {
		fmt.Fprintf(&b, "  reference\n    \"%s\";\n\n", s.Reference)
	}
	fmt.Fprintf(&b, "  revision %s {\n", s.Revision)
	fmt.Fprintf(&b, "    description \"Initial revision (scaffold).\";\n")
	fmt.Fprintf(&b, "  }\n")
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "  // TODO: replace this skeleton with the real data model.\n")
	fmt.Fprintf(&b, "  // Define the top-level container, identity / state / config\n")
	fmt.Fprintf(&b, "  // sub-containers, must-constraints, and any list/grouping shapes\n")
	fmt.Fprintf(&b, "  // your service needs.\n")
	fmt.Fprintf(&b, "  container %s {\n", s.Service)
	fmt.Fprintf(&b, "    description \"Top-level container for %s state.\";\n", s.Service)
	fmt.Fprintf(&b, "  }\n")
	fmt.Fprintf(&b, "}\n")
	return b.String()
}

// renderEventsYANG renders the service's events companion module
// (-events companion naming: openits-<svc>-events). It only scaffolds the
// service-specific domain events passed via --events — fault and mode
// events are deliberately NOT scaffolded here. See reservedFaultModeNames
// for the guard that rejects fault/mode-shaped --events names, and the
// header comment this function emits for the author-facing explanation.
func renderEventsYANG(s service) string {
	var b strings.Builder
	fmt.Fprintf(&b, "module openits-%s-events {\n", s.Service)
	fmt.Fprintf(&b, "  yang-version 1.1;\n")
	fmt.Fprintf(&b, "  namespace \"urn:openits:yang:%s-events\";\n", s.Service)
	fmt.Fprintf(&b, "  prefix openits-%s-events;\n", s.Service)
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "  organization\n")
	fmt.Fprintf(&b, "    \"OpenITS Working Group (proposal)\";\n")
	fmt.Fprintf(&b, "  contact\n")
	fmt.Fprintf(&b, "    \"TODO: maintainer email\";\n")
	fmt.Fprintf(&b, "  description\n")
	fmt.Fprintf(&b, "    \"Service-specific domain events for openits-%s. Split into\n", s.Service)
	fmt.Fprintf(&b, "     a companion module so ygot's proto backend can generate\n")
	fmt.Fprintf(&b, "     protobuf from the base module.\n")
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "     Fault and mode events are emitted via\n")
	fmt.Fprintf(&b, "     openits-common-fault-events / openits-common-mode-events;\n")
	fmt.Fprintf(&b, "     derive %s-fault-event-kind / %s-mode-event-kind in\n", s.Service, s.Service)
	fmt.Fprintf(&b, "     openits-%s-types.yang and emit the common notifications\n", s.Service)
	fmt.Fprintf(&b, "     with kind set accordingly — do not declare per-service\n")
	fmt.Fprintf(&b, "     fault-raised / fault-cleared / mode-changed notifications\n")
	fmt.Fprintf(&b, "     in this module.\";\n")
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "  revision %s {\n", s.Revision)
	fmt.Fprintf(&b, "    description \"Initial revision (scaffold).\";\n")
	fmt.Fprintf(&b, "  }\n")
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "  /* --------------------------------------------------------------\n")
	fmt.Fprintf(&b, "   * NOTE: fault and mode events are NOT declared in this module.\n")
	fmt.Fprintf(&b, "   *\n")
	fmt.Fprintf(&b, "   * Fault and mode events are emitted via openits-common-fault-events\n")
	fmt.Fprintf(&b, "   * / openits-common-mode-events; derive %s-fault-event-kind /\n", s.Service)
	fmt.Fprintf(&b, "   * %s-mode-event-kind in openits-%s-types.yang and emit the common\n", s.Service, s.Service)
	fmt.Fprintf(&b, "   * notifications (fault-raised, fault-cleared, mode-changed) with\n")
	fmt.Fprintf(&b, "   * kind set accordingly. See docs/data-model.md (\"The event\n")
	fmt.Fprintf(&b, "   * taxonomy\") for the pattern and a worked example. Only\n")
	fmt.Fprintf(&b, "   * genuinely service-specific domain events belong below.\n")
	fmt.Fprintf(&b, "   * -------------------------------------------------------------- */\n")
	fmt.Fprintf(&b, "\n")
	for _, e := range s.Events {
		fmt.Fprintf(&b, "  notification %s {\n", e.Hyphen)
		fmt.Fprintf(&b, "    description \"TODO: describe when %s fires.\";\n", e.Hyphen)
		fmt.Fprintf(&b, "    // TODO: add typed leaves describing the event payload.\n")
		fmt.Fprintf(&b, "  }\n")
		fmt.Fprintf(&b, "\n")
	}
	fmt.Fprintf(&b, "}\n")
	return b.String()
}

func renderEventsProto(s service) string {
	var b strings.Builder
	fmt.Fprintf(&b, "syntax = \"proto3\";\n\n")
	fmt.Fprintf(&b, "package openits.%s.v1;\n\n", s.Snake)
	fmt.Fprintf(&b, "option go_package = \"github.com/Vikasa2M/openits-models/pkg/proto/openits/%s/v1;%sv1\";\n\n", s.Snake, goPkgName(s.Snake))
	fmt.Fprintf(&b, "import \"google/protobuf/timestamp.proto\";\n\n")
	fmt.Fprintf(&b, "// %s — %s\n", s.Service, s.Description)
	if s.Reference != "" {
		fmt.Fprintf(&b, "// Reference: %s\n", s.Reference)
	}
	fmt.Fprintf(&b, "\n")
	for _, e := range s.Events {
		fmt.Fprintf(&b, "// %s — TODO describe.\n", e.Camel)
		fmt.Fprintf(&b, "message %s {\n", e.Camel)
		fmt.Fprintf(&b, "  // Required by every per-event message; populate at the publisher.\n")
		fmt.Fprintf(&b, "  string controller_id = 1;\n")
		fmt.Fprintf(&b, "  google.protobuf.Timestamp observed_at = 2;\n")
		fmt.Fprintf(&b, "  // TODO: add typed fields specific to %s.\n", e.Hyphen)
		fmt.Fprintf(&b, "}\n\n")
	}
	return b.String()
}

// printNextSteps tells the contributor what to edit by hand. Intentionally
// terse so it fits the "30-minute walkthrough" promise.
func printNextSteps(s service) {
	fmt.Println()
	fmt.Println("Next steps (manual edits — see docs/tutorial-add-a-service.md):")
	fmt.Println()
	fmt.Printf("  1. Edit yang/openits-%s.yang — design the real data model.\n", s.Service)
	fmt.Printf("  2. Edit yang/openits-%s-events.yang — type the event leaves (service-specific\n", s.Service)
	fmt.Println("     domain events only).")
	fmt.Printf("  3. Edit api/proto/openits/%s/v1/events.proto — fill in proto fields.\n", s.Snake)
	fmt.Println()
	fmt.Printf("  Fault/mode: do NOT add fault-raised/fault-cleared/mode-changed\n")
	fmt.Printf("  notifications to openits-%s-events.yang. Instead, in\n", s.Service)
	fmt.Printf("  openits-%s-types.yang, derive %s-fault-event-kind (base\n", s.Service, s.Service)
	fmt.Printf("  openits-types:fault-event-kind) and, if applicable,\n")
	fmt.Printf("  %s-mode-event-kind (base openits-types:mode-event-kind); emit\n", s.Service)
	fmt.Println("  openits-common-fault-events:{fault-raised,fault-cleared} and")
	fmt.Println("  openits-common-mode-events:mode-changed with kind set to those")
	fmt.Println("  identities. See docs/data-model.md (\"The event taxonomy\").")
	fmt.Println()
	fmt.Println("  4. Add to internal/cloudevents/envelope.go:")
	fmt.Printf("       svc%s = serviceSpec{%q, \"openits-%s\", %q}\n", s.Camel, s.Service, s.Service, s.Revision)
	for _, e := range s.Events {
		fmt.Printf("       Type%s%s = \"openits.%s.%s.v1\"\n", s.Camel, e.Camel, s.Service, e.Hyphen)
	}
	fmt.Printf("       func New%sEventEnvelope(...) *Envelope { return newEnvelope(svc%s, ...) }\n", s.Camel, s.Camel)
	fmt.Println()
	fmt.Println("  5. Add to internal/cloudevents/registry.go AllSpecs() / AllEvents()")
	fmt.Println("     so the asyncapi-gen tool picks up your service.")
	fmt.Println()
	fmt.Println("  6. (Optional) Add a translator package at")
	fmt.Printf("       internal/translator/%s/translator.go\n", strings.ReplaceAll(s.Service, "-", ""))
	fmt.Println("     plus a register.go that side-effect-registers it.")
	fmt.Println()
	fmt.Println("  7. (Optional) Add a command-handler package at")
	fmt.Printf("       internal/command/%s/handler.go\n", strings.ReplaceAll(s.Service, "-", ""))
	fmt.Println("     and register it in cmd/poller/main.go.")
	fmt.Println()
	fmt.Println("  8. Run:")
	fmt.Println("       make yang && make proto && make build && make test")
	fmt.Println("       make validate-yang && make check-revisions")
	fmt.Println("       make asyncapi   # regenerate the AsyncAPI spec")
	fmt.Println()
}
