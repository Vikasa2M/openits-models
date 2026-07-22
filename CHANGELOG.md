# Changelog

All notable changes to this repository are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
for the Go module. See [`docs/versioning.md`](docs/versioning.md) for how the
Go module version, per-module YANG revision dates, and protobuf wire
compatibility relate.

## [0.2.2](https://github.com/Vikasa2M/openits-models/compare/v0.2.1...v0.2.2) (2026-07-22)


### Bug Fixes

* **deps:** bump golang.org/x/net from 0.53.0 to 0.55.0 ([#7](https://github.com/Vikasa2M/openits-models/issues/7)) ([9d767f9](https://github.com/Vikasa2M/openits-models/commit/9d767f96e16e7847c52a501805fae4cc7179432c))

## [0.2.1](https://github.com/Vikasa2M/openits-models/releases/tag/v0.2.1) (2026-07-21)

Initial public baseline. The repository history was consolidated to a single
root commit ahead of the public launch, and the earlier v0.1.0–v0.2.0 releases
were retired along with it. The model surface (YANG modules, protobuf,
AsyncAPI bindings, schema registry) is unchanged from the final
pre-consolidation state; this release re-establishes the version line.
