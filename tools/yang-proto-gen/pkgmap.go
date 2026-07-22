package main

import "strings"

// typesPackage/typesFilePath are the proto package and output-relative path
// every shared grouping (see grouping.go's SharedGroupings) is centralized
// into, instead of living in whichever service file happens to reference it
// first. A dedicated file/package keeps the packaging independent of
// module-processing order: which grouping ends up "shared" depends only on
// the YANG `uses` graph, never on which service module Generate happens to
// visit first. Like every other output file, this one lands in its own
// per-package Go directory (pkg/proto/openits/types/v1) rather than the
// single shared openitspb package every file used to flatten into.
const (
	typesPackage  = "openits.types.v1"
	typesFilePath = "openits/types/v1/types.proto"
)

// serviceRoute maps a YANG module-name prefix to the proto package and
// output file (relative to Generate's outDir) every matching module's
// notifications are collected into.
type serviceRoute struct {
	prefix string
	pkg    string
	file   string
}

// serviceRoutes maps every service's YANG module prefix to its proto package
// and per-service output file. The file path always mirrors the proto
// package: strings.ReplaceAll(pkg, ".", "/") + "/events.proto" — e.g.
// "openits.signal_control.v1" -> "openits/signal_control/v1/events.proto".
// Each service therefore gets its own directory (and, via goPackageFor, its
// own Go package) instead of every service sharing one flat
// openits/v1/*_events.proto layout — this is what lets two services declare
// the same bare message name (e.g. "Detector") without colliding: they land
// in different Go packages. Every prefix here is a distinct service token —
// no two routes ever match the same module name — so match order is not
// significant.
var serviceRoutes = []serviceRoute{
	{"openits-common-", "openits.common.v1", "openits/common/v1/events.proto"},
	{"openits-signal-control", "openits.signal_control.v1", "openits/signal_control/v1/events.proto"},
	{"openits-dms", "openits.dms.v1", "openits/dms/v1/events.proto"},
	{"openits-ess", "openits.ess.v1", "openits/ess/v1/events.proto"},
	{"openits-rsu", "openits.rsu.v1", "openits/rsu/v1/events.proto"},
	{"openits-ramp-metering", "openits.ramp_metering.v1", "openits/ramp_metering/v1/events.proto"},
	{"openits-perception", "openits.perception.v1", "openits/perception/v1/events.proto"},
	{"openits-traffic-sensor", "openits.traffic_sensor.v1", "openits/traffic_sensor/v1/events.proto"},
	{"openits-reversible-lane", "openits.reversible_lane.v1", "openits/reversible_lane/v1/events.proto"},
	{"openits-cctv", "openits.cctv.v1", "openits/cctv/v1/events.proto"},
}

// pkgFor returns the proto package and output file moduleName's
// notifications belong in. ok is false for modules with no service mapping
// — typedef/grouping-only modules (openits-types, openits-nema-common,
// vendor -types modules) and imported ietf-* modules never declare
// notifications in the current yang/ corpus, so Generate silently skips
// them; a module that matches no route but does declare a live
// notification is a Generate error (see the corpus-drift guard in
// generate()), not a silent drop.
func pkgFor(moduleName string) (pkg, file string, ok bool) {
	for _, r := range serviceRoutes {
		if strings.HasPrefix(moduleName, r.prefix) {
			return r.pkg, r.file, true
		}
	}
	return "", "", false
}

// configStateRoutes is the opt-in allowlist of modules whose config/state
// tree (and actions) the generator emits, keyed by exact module name. The
// route's file is a NEW per-service file (state.proto) distinct from the
// service's events.proto, sharing the service's proto package/go_package —
// so a service's events and config/state messages share one Go package
// (and must not collide with each other) while staying independent of every
// other service's package. Each service's config/state proto turns on only
// when that service's module is restructured to the config/state idiom and
// added here, so `make gen` stays byte-identical for every not-yet-converted
// module.
var configStateRoutes = map[string]serviceRoute{
	"openits-ess":             {pkg: "openits.ess.v1", file: "openits/ess/v1/state.proto"},
	"openits-ramp-metering":   {pkg: "openits.ramp_metering.v1", file: "openits/ramp_metering/v1/state.proto"},
	"openits-traffic-sensor":  {pkg: "openits.traffic_sensor.v1", file: "openits/traffic_sensor/v1/state.proto"},
	"openits-perception":      {pkg: "openits.perception.v1", file: "openits/perception/v1/state.proto"},
	"openits-dms":             {pkg: "openits.dms.v1", file: "openits/dms/v1/state.proto"},
	"openits-reversible-lane": {pkg: "openits.reversible_lane.v1", file: "openits/reversible_lane/v1/state.proto"},
	"openits-signal-control":  {pkg: "openits.signal_control.v1", file: "openits/signal_control/v1/state.proto"},
	"openits-rsu":             {pkg: "openits.rsu.v1", file: "openits/rsu/v1/state.proto"},
	"openits-cctv":            {pkg: "openits.cctv.v1", file: "openits/cctv/v1/state.proto"},
}

// configStateFor returns the route for a module opted into config/state
// generation, and ok=false for every module not on the allowlist.
func configStateFor(moduleName string) (r serviceRoute, ok bool) {
	r, ok = configStateRoutes[moduleName]
	return r, ok
}
