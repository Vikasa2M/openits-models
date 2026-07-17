# YANG Testdata

Golden IETF-JSON instance documents used by `make validate-yang` to
exercise the `must`-constraints in the openits YANG modules via
`yanglint` (libyang).

| File | Expectation |
|------|-------------|
| `valid-signal-controller.json` | Validates clean. |
| `invalid-yellow-below-mutcd.json` | Fails: yellow-change < 3.0. |
| `invalid-min-green-below-four.json` | Fails: min-green < 4. |
| `invalid-red-clear-below-one.json` | Fails: red-clear < 1.0. |
| `invalid-walk-without-ped-clear.json` | Fails: walk > 0, ped-clear = 0. |
| `invalid-max-green-less-than-min.json` | Fails: max-green < min-green. |

`make validate-yang` runs yanglint inside a Docker container
(`sysrepo/sysrepo-netopeer2`) and asserts each file produces the
expected outcome.

(This table predates several fixtures added since; see the files on
disk for the current full set.)

## Deviation-tier fixtures

Files named `invalid-<x>-under-<deviation-name>.json` prove that a
`yang/deviations/*.yang` module *tightens* the base contract: the same
instance data is VALID against base alone and INVALID once the named
deviation is also loaded. `make validate-yang`'s base-only schema set
skips this family (see the `invalid-*-under-*.json` case in
`scripts/validate-yang.sh`); `make check-deviations` runs the
base+deviation yanglint pass that actually proves the rejection.

| File | Expectation |
|------|-------------|
| `valid-yellow-3.5-base.json` | Validates clean against base alone (yellow-change 3.5 s satisfies the base 3.0-6.0 s range). |
| `invalid-yellow-3.5-under-mutcd-strict.json` | Same instance; rejected once `openits-signal-control-mutcd-strict` (yellow-change >= 4.0 s) is also loaded. |
