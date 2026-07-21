# Changelog

All notable changes to this repository are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
for the Go module. See [`docs/versioning.md`](docs/versioning.md) for how the
Go module version, per-module YANG revision dates, and protobuf wire
compatibility relate.

## [0.1.0](https://github.com/Vikasa2M/openits-models/compare/v0.1.0...v0.1.0) (2026-07-21)


### ⚠ BREAKING CHANGES

* pre-v1 wire changes — signal cycle-state enum trim, overlap numeric re-key + shared id typedefs, nema-common min/max-green to decimal64 tenths, preemption-type to identityref, TIM end-time removal, message-stats counter collapse, backhaul-cellular rename, DMS MULTI-tag identityref conversion, mode trigger identityref, ramp command-source convergence, control-mode identity prefixing, vendor module namespace moves.

### Features

* full model review remediation across all nine device families ([8c37bb4](https://github.com/Vikasa2M/openits-models/commit/8c37bb43274ff3704c5d31359a25236bd45aa16c))


### Bug Fixes

* **registry:** record contact normalization as a revision on the example augment ([edfff30](https://github.com/Vikasa2M/openits-models/commit/edfff30c8cae6bf795b4916ef7e5f25e84955280))


### Miscellaneous Chores

* pin first release version ([c184e99](https://github.com/Vikasa2M/openits-models/commit/c184e99eb4a22881c085ea923c0f5adbc4836469))

## [Unreleased]

## [0.1.0] - 2026-07-17

Initial public release of the OpenITS data-model layer as a standalone,
importable Go module (`github.com/openits/openits-models`).

### Added

- **YANG source modules** (`yang/`) — the vendor-neutral source of truth for
  the model: signal control, DMS, ESS, RSU/V2X, ramp metering, traffic sensor,
  reversible lane, perception, CCTV/PTZ, cabinet power, plus shared type and
  event modules and the common/NEMA groupings.
- **Generated protobuf** (`api/proto/`, `pkg/proto/openits/v1/`) — per-service
  event and state messages generated from YANG, alongside the hand-authored
  `command.proto` / `device.proto` core, with a stable field-number lock
  (`field-numbers.yaml`).
- **Generated ygot Go types** (`pkg/yang/openits/`).
- **AsyncAPI 3.0 document** (`asyncapi.yaml`) generated from the YANG-derived
  ce-type catalog, and the per-service `schema-registry/` snapshots.
- **Generation, validation, and lint tooling** (`scripts/`, `tools/`): the
  YANG→proto generator, revision/naming/deviation/augment-collision/
  events-layering guards, NoI validator, graduation report, and the
  conformance harness covering nine device kinds.
- **Documentation** (`docs/`) — design decisions, standards alignment, the
  extension/graduation model, capability architecture, and conformance.

[Unreleased]: https://github.com/openits/openits-models/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/openits/openits-models/releases/tag/v0.1.0
