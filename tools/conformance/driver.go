package main

import (
	"context"
	"fmt"
	"time"

	yangpkg "github.com/openits/openits-models/pkg/yang/openits"
	"github.com/openits/openits-models/tools/conformance/tests"
)

// Driver is the minimal interface the harness needs to observe a device
// under test.  Real drivers (SNMP today, gNMI tomorrow) are expected to
// live in the SDK and satisfy this shape.
type Driver interface {
	// Collect returns the canonical YANG-modeled state of the device.
	Collect(ctx context.Context) (*yangpkg.Device, error)
	// Subscribe runs until ctx is done, delivering every CloudEvents
	// envelope the device emits during the observation window.
	Subscribe(ctx context.Context, out chan<- tests.EventEnvelope) error
}

// newDriver returns a driver for the given name.  Unknown names return
// an error rather than a nil driver so the caller fails loudly.
func newDriver(name, host, community, kind string, offlineWindow time.Duration) (Driver, error) {
	switch name {
	case "mock":
		return newMockDriver(kind, offlineWindow), nil
	case "snmp":
		return nil, fmt.Errorf("snmp driver not implemented in this milestone; use -driver mock")
	default:
		return nil, fmt.Errorf("unknown driver %q (valid: mock, snmp)", name)
	}
}
