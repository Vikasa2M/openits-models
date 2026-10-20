# conformance

YANG- and subject-hierarchy-aware conformance harness for OpenITS
devices — nine kinds: signal controllers (`asc`), RSUs, DMS, ESS,
ramp meters, traffic sensors, reversible lanes, perception sensors
and CCTV cameras. Instantiates a driver, collects the device's
current YANG-modeled state, subscribes to a window of emitted
CloudEvents, and runs a suite of checks against the combined
observation.

## Usage

```
# Self-test against the built-in mock device (always passes).
go run ./tools/conformance -driver mock -kind asc

# Every kind (same as `make conformance`).
for k in asc rsu dms ess ramp-metering traffic-sensor \
    reversible-lane perception cctv; do
  go run ./tools/conformance -driver mock -kind "$k"
done
```

`mock` is the only working driver today; `-driver snmp` is reserved
and returns a "not implemented" error.

Exit status is 0 on full pass, 1 on any failure, 2 on harness errors
(bad flags, driver init, collect errors).

## Flags

| Flag | Default | Notes |
|------|---------|-------|
| `-driver`    | `mock`   | `mock` (offline); `snmp` reserved, not implemented |
| `-host`      | –        | `host[:port]`; reserved for `-driver snmp` |
| `-community` | `public` | SNMP community |
| `-kind`      | `asc`    | `asc`, `rsu`, `dms`, `ess`, `ramp-metering`, `traffic-sensor`, `reversible-lane`, `perception`, `cctv` |
| `-window`    | `5s`     | Observation window for the subscription phase |

## Check categories

`tools/conformance/tests/` holds the check functions, grouped by file.

Universal (every kind):

- `subject.go` — published events use the 7-token `openits.*` subject
  shape with lowercase-alnum-hyphen tokens
- `cetype.go` — `ce-type` is `openits.<service>.<event>.v<major>`;
  `ce-source` is a well-formed `urn:openits:<entity-kind>:…` URN (any
  kebab-case entity kind); `ce-id` is non-empty
- `header_sequence.go` — `sequence` is contiguous within each
  (`source-device-id`, `sequence-epoch`) stream
- `fault_kind.go` — a fault event's `kind` belongs to the emitting
  service
- `mode_kind.go` — a `mode-changed` value belongs to its kind's mode
  family (`enums.go` holds the shared identity helpers)
- `yang_validation.go` — collected device state validates against the
  YANG schema via `ygot.Validate`

Signal control (`asc`):

- `identity.go` — controller-id, firmware, make/model, location
- `phase_timing.go` — yellow-change ≥ 3.0s (MUTCD minimum), red-clear
  non-negative, min-green ≥ 4s (operational floor); sanity caps on
  max-green and ped-clear
- `coordination.go` — active plan present, splits within cycle, NEMA
  dual-ring, barrier assignment and alignment
- `sequences.go`, `overlaps.go`, `channels.go`, `detectors.go`,
  `timebase.go` — ring membership, overlap / channel / detector
  references resolve, time-base references resolve
- `conflict_monitor.go`, `monitor.go` — permissive table and monitor
  read status
- `operation.go`, `cabinet_output.go`, `health.go` — the `operation`
  container, `cabinet-output-changed` events, the
  `operational-status-report` heartbeat, and no cabinet flash
- `preemption.go` — rail track-clearance timing; preemption events
  fire with the expected shape
- `cabinet_power.go` — power source, battery charge, runtime on
  battery

Other kinds: one file per kind (`rsu.go`, `dms.go`, `ess.go`,
`ramp_metering.go`, `traffic_sensor.go`, `reversible_lane.go`,
`perception.go`, `cctv.go`), plus `zone_occupancy.go` for the
zone-occupancy capability.

## Extending

Add a new check by:

1. Writing a `func TestFoo_Bar(t *T, obs *Observation)` function in
   package `tests` (in the appropriate file, or a new one).
2. Registering it as a `TestCase{Name, Fn}` in `All(kind)` in
   `tests/tests.go`: in the `common` slice if it applies to every
   kind, otherwise in that kind's `case` of the switch.

The harness runs that list in order; no registration macros or
reflection.

## Writing a real driver

Implement `main.Driver` (see `tools/conformance/driver.go`) and wire
it into `newDriver`. The mock device for each kind lives in
`mock_driver.go` plus a per-kind `mock_*.go` (`mock_cctv.go`,
`mock_perception.go`, `mock_reversible_lane.go`,
`mock_traffic_sensor.go`; `mock_header.go` stamps the event header).
An SNMP driver is planned; until it lands, `-driver snmp` returns a
clear "not implemented" error.
