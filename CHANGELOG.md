# Changelog

All notable changes to this repository are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
for the Go module. See [`docs/versioning.md`](docs/versioning.md) for how the
Go module version, per-module YANG revision dates, and protobuf wire
compatibility relate.

## [0.3.0](https://github.com/Vikasa2M/openits-models/compare/v0.2.2...v0.3.0) (2026-08-16)


### ⚠ BREAKING CHANGES

* identityref values move namespace, from 'openits-perception-types:object-*' to 'openits-types:object-*'. Proto field tags are unaffected — identityrefs render as proto strings, so no tag moves and no wire layout changes; only the string values do. Generated Go constants rename from OpenitsPerceptionTypes_ObjectClass_* to OpenitsTypes_ObjectClass_*.
* pan-degrees ranges tighten to 0.0..359.9 (360.0 was previously accepted); and generated-name churn in openits.cctv.v1 — the new notification's absolute/velocity containers collide with the ptz/config message names, so the generator renames the config messages to PtzConfigAbsolute/PtzConfigVelocity and the ptz-move-mode enum to OpenitsCctvPtzMoveMode (defining-module naming once two modules reference it). Field numbers, tags, and JSON field names are unchanged, so binary and RFC 7951 wire forms are unaffected; descriptor names and generated Go types move. Pre-1.0.
* signal-controller cabinet-power leaves move one level deeper (cabinet-power/X -> cabinet-power/{config,state}/X) and the on-battery-policy mode enum re-bases around an unspecified=0 member (its wire path is new in the same change). traffic-sensor and perception associations change from config-false to config-true (paths unchanged; semantic change only). Pre-1.0.

### Features

* add the zone-occupancy capability, hoist object-class to the foundation layer ([#13](https://github.com/Vikasa2M/openits-models/issues/13)) ([398b49a](https://github.com/Vikasa2M/openits-models/commit/398b49aeac33f70b78196498d2ce25fcde11b4be))
* carry wire-source provenance on wire-decoded event notifications ([195fac9](https://github.com/Vikasa2M/openits-models/commit/195fac9f850da2f49dad2623e00ea50c077baff0))
* harden signal-control safety and integrity surface ([a0c128e](https://github.com/Vikasa2M/openits-models/commit/a0c128e0df706ad5d7c53d4ea0fd1c81a6444db7))
* service-family corrections + ptz-move-commanded audit event ([342123e](https://github.com/Vikasa2M/openits-models/commit/342123e122127a78cf790030627ffa2b8a33bce3))
* share schedule primitives, make associations provisioned, split cabinet-power policy into config/state ([b1b45e2](https://github.com/Vikasa2M/openits-models/commit/b1b45e218234061a33b098ef42d05a6ace39ebcb))


### Bug Fixes

* collapse duplicate same-date revision statements in the MUTCD deviation modules ([28c7e0a](https://github.com/Vikasa2M/openits-models/commit/28c7e0aaed9ee412b7d48fbcba21fde2c58b5849))
* correct MAP PSID citations and bound TIM duration to J2735 MinutesDuration ([b42ad4e](https://github.com/Vikasa2M/openits-models/commit/b42ad4e0b5d5b3b25599b42d3ce8bb85963a7a73))
* **deps:** bump github.com/openconfig/ygot in the go-dependencies group ([#12](https://github.com/Vikasa2M/openits-models/issues/12)) ([e611930](https://github.com/Vikasa2M/openits-models/commit/e61193031000536b3d21c5bc763a6fe12ccb657c))
* stamp explicit value statements on every remaining enum member ([9ba1efc](https://github.com/Vikasa2M/openits-models/commit/9ba1efc08622e9afa914ba80150c41db61f1a810))
* vendor/platform consistency — Ledstar namespace repatriation, version stamps, plan-id prose ([8437281](https://github.com/Vikasa2M/openits-models/commit/8437281ed9c63926aeeef3344fc15427375d4740))

## [0.2.2](https://github.com/Vikasa2M/openits-models/compare/v0.2.1...v0.2.2) (2026-07-22)


### Bug Fixes

* **deps:** bump golang.org/x/net from 0.53.0 to 0.55.0 ([#7](https://github.com/Vikasa2M/openits-models/issues/7)) ([9d767f9](https://github.com/Vikasa2M/openits-models/commit/9d767f96e16e7847c52a501805fae4cc7179432c))

## [0.2.1](https://github.com/Vikasa2M/openits-models/releases/tag/v0.2.1) (2026-07-21)

Initial public baseline. The repository history was consolidated to a single
root commit ahead of the public launch, and the earlier v0.1.0–v0.2.0 releases
were retired along with it. The model surface (YANG modules, protobuf,
AsyncAPI bindings, schema registry) is unchanged from the final
pre-consolidation state; this release re-establishes the version line.
