package main

import (
	"strings"

	"github.com/Vikasa2M/openits-models/tools/conformance/tests"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// mockRegistry is the schema registry the mock reads its ce-dataschema
// values from, as a producer built from this release would: each event names
// the current revision of the module that defines its ce-type.
var mockRegistry *tests.Registry

// stampHeaders sets each event's ce-dataschema from mockRegistry, and fills
// the event-header stream fields on each event payload
// the way a conforming producer would: one epoch for the whole mock run,
// and sequence numbered 1, 2, 3... per source device in emission order.
// The source device is the payload's own source-device-id when set,
// otherwise the controller-id token of the subject
// (openits.<region>.<agency>.<unit>.<service>.<controller-id>.<event>).
// Payloads without a header (nil Data, or no sequence field) are left
// alone.
func stampHeaders(events []tests.EventEnvelope, epoch string) {
	if mockRegistry != nil {
		for i := range events {
			if url, ok := mockRegistry.CurrentDataSchema(events[i].CEType); ok {
				events[i].CEDataSchema = url
			}
		}
	}
	next := map[string]uint64{}
	for _, e := range events {
		if e.Data == nil {
			continue
		}
		m := e.Data.ProtoReflect()
		fields := m.Descriptor().Fields()
		seqF := fields.ByName("sequence")
		epochF := fields.ByName("sequence_epoch")
		srcF := fields.ByName("source_device_id")
		if seqF == nil || epochF == nil || srcF == nil {
			continue
		}
		src := m.Get(srcF).String()
		if src == "" {
			if tok := strings.Split(e.Subject, "."); len(tok) >= 7 {
				src = tok[5]
			}
			m.Set(srcF, protoreflect.ValueOfString(src))
		}
		next[src]++
		m.Set(seqF, protoreflect.ValueOfUint64(next[src]))
		m.Set(epochF, protoreflect.ValueOfString(epoch))
	}
}
