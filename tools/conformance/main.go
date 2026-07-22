// Command conformance runs the OpenITS conformance suite against a
// device under test.
//
// Usage:
//
//	conformance -driver mock -kind asc
//	conformance -driver snmp -host 192.168.1.10 -community public -kind asc
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Vikasa2M/openits-models/tools/conformance/tests"
)

func main() {
	driver := flag.String("driver", "mock", "driver name (mock, snmp)")
	host := flag.String("host", "", "device host[:port] (required for snmp)")
	community := flag.String("community", "public", "SNMP community")
	kind := flag.String("kind", "asc", "device kind (asc, rsu, dms, ess, ramp-metering, traffic-sensor, reversible-lane, perception, cctv)")
	window := flag.Duration("window", 5*time.Second, "subscription observation window")
	flag.Parse()

	d, err := newDriver(*driver, *host, *community, *kind, *window)
	if err != nil {
		fmt.Fprintf(os.Stderr, "driver: %v\n", err)
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *window+30*time.Second)
	defer cancel()

	collected, err := d.Collect(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "collect: %v\n", err)
		os.Exit(2)
	}

	evCh := make(chan tests.EventEnvelope, 64)
	subCtx, subCancel := context.WithTimeout(ctx, *window)
	subDone := make(chan error, 1)
	go func() {
		subDone <- d.Subscribe(subCtx, evCh)
		close(evCh)
	}()

	var events []tests.EventEnvelope
	for e := range evCh {
		events = append(events, e)
	}
	subCancel()
	if err := <-subDone; err != nil {
		fmt.Fprintf(os.Stderr, "subscribe: %v\n", err)
	}

	obs := &tests.Observation{
		Device: collected,
		Events: events,
		Window: *window,
	}

	pass, fail := 0, 0
	fmt.Printf("OpenITS conformance suite — driver=%s kind=%s window=%s\n\n", *driver, *kind, *window)
	for _, tc := range tests.All(*kind) {
		r := tests.Run(tc.Name, tc.Fn, obs)
		if r.Pass {
			pass++
			fmt.Printf("  PASS  %s\n", r.Name)
		} else {
			fail++
			fmt.Printf("  FAIL  %s\n        %s\n", r.Name, r.Message)
		}
	}
	fmt.Printf("\n%d passed, %d failed (%d total)\n", pass, fail, pass+fail)
	if fail > 0 {
		os.Exit(1)
	}
}
