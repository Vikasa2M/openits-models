# conformance

YANG- and subject-hierarchy-aware conformance harness for OpenITS
signal controllers. Instantiates a driver, collects the device's
current YANG-modeled state, subscribes to a short window of emitted
CloudEvents, and runs a suite of checks against the combined
observation.

## Usage

```
# Self-test against the built-in mock device (always passes).
go run ./tools/conformance -driver mock -kind asc

# Real device.
go run ./tools/conformance -driver snmp -host 192.168.1.10 \
    -community public -kind asc -window 30s
```

Exit status is 0 on full pass, 1 on any failure, 2 on harness errors
(bad flags, driver init, collect errors).

## Flags

| Flag | Default | Notes |
|------|---------|-------|
| `-driver`    | `mock`   | `mock` (offline), `snmp` (live device) |
| `-host`      | –        | `host[:port]`; required for `-driver snmp` |
| `-community` | `public` | SNMP community |
| `-kind`      | `asc`    | `asc` or `rsu` |
| `-window`    | `5s`     | Observation window for the subscription phase |

## Check categories

`tools/conformance/tests/` holds the check functions, grouped by file:

- `identity.go` — controller-id, firmware, make/model, location
- `phase_timing.go` — MUTCD minimums: yellow-change ≥ 3.0s, red-clear ≥
  1.0s, min-green ≥ 4s; sanity caps on max-green and ped-clear
- `coordination.go` — active plan present, NEMA dual-ring, barrier
  assignment
- `preemption.go` — `preemption-activated` notification fires during
  the observation window with the correct `ce-type`
- `health.go` — operational-status heartbeat arrives, not flashing
- `yang_validation.go` — emitted device state validates against the
  YANG schema via `ygot.Validate`
- `subject.go` — published events use the 7-token `openits.*` subject
  shape with lowercase-alnum-hyphen tokens
- `cetype.go` — `ce-type` is `openits.<service>.<event>.v<major>`;
  `ce-source` is a well-formed `urn:openits:controller:…` URN;
  `ce-id` is non-empty

## Extending

Add a new check by:

1. Writing a `func TestFoo_Bar(t *tests.T, obs *tests.Observation)`
   function in the appropriate file (or a new file).
2. Appending it to the slice returned by `tests.All()` in
   `tests/tests.go`.

The harness compiles and runs that list in order; no registration
macros or reflection.

## Writing a real driver

Implement `main.Driver` (see `tools/conformance/driver.go`) and wire
it into `newDriver`. The SDK's SNMP adapter will land here in the
follow-up milestone; until then, `-driver snmp` returns a clear "not
implemented" error.
