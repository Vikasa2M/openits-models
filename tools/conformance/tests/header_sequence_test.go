package tests

import (
	"strings"
	"testing"

	commonv1 "github.com/Vikasa2M/openits-models/pkg/proto/openits/common/v1"
)

func fault(src, epoch string, seq uint64) EventEnvelope {
	return EventEnvelope{
		Subject: "openits.us-xx.agency.d01.dms." + src + ".fault-raised",
		CEType:  "openits.dms.fault-raised.v1",
		Data: &commonv1.FaultRaised{
			SourceDeviceId: src,
			SequenceEpoch:  epoch,
			Sequence:       seq,
		},
	}
}

func runSeq(events ...EventEnvelope) Result {
	return Run("seq", TestEvent_SequenceContiguousWithinEpoch, &Observation{Events: events})
}

func TestSequence_ContiguousPasses(t *testing.T) {
	if r := runSeq(fault("a", "e1", 5), fault("a", "e1", 6), fault("a", "e1", 7)); !r.Pass {
		t.Fatalf("contiguous stream failed: %s", r.Message)
	}
}

func TestSequence_GapFails(t *testing.T) {
	r := runSeq(fault("a", "e1", 5), fault("a", "e1", 7))
	if r.Pass || !strings.Contains(r.Message, "want 6") {
		t.Fatalf("gap not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

func TestSequence_EpochChangeIsNotAGap(t *testing.T) {
	if r := runSeq(fault("a", "e1", 41), fault("a", "e2", 1), fault("a", "e2", 2)); !r.Pass {
		t.Fatalf("new epoch treated as a gap: %s", r.Message)
	}
}

func TestSequence_InterleavedSourcesIndependent(t *testing.T) {
	if r := runSeq(fault("a", "e1", 1), fault("b", "e9", 100), fault("a", "e1", 2), fault("b", "e9", 101)); !r.Pass {
		t.Fatalf("interleaved sources read as gaps: %s", r.Message)
	}
}

func TestSequence_MissingEpochFails(t *testing.T) {
	r := runSeq(fault("a", "", 1))
	if r.Pass || !strings.Contains(r.Message, "no sequence-epoch") {
		t.Fatalf("missing epoch not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}

func TestSequence_NilDataSkipped(t *testing.T) {
	if r := runSeq(EventEnvelope{CEType: "openits.signal-control.operational-status.v1"}); !r.Pass {
		t.Fatalf("nil Data not skipped: %s", r.Message)
	}
}

func withID(e EventEnvelope, id string) EventEnvelope {
	e.CEID = id
	return e
}

func TestSequence_RedeliveryIsNotAGap(t *testing.T) {
	r := runSeq(
		withID(fault("a", "e1", 5), "id5"),
		withID(fault("a", "e1", 6), "id6"),
		withID(fault("a", "e1", 5), "id5"), // redelivered
		withID(fault("a", "e1", 7), "id7"),
	)
	if !r.Pass {
		t.Fatalf("redelivered event read as a gap: %s", r.Message)
	}
}

func TestSequence_MissingSourceFails(t *testing.T) {
	r := runSeq(fault("", "e1", 1))
	if r.Pass || !strings.Contains(r.Message, "no source-device-id") {
		t.Fatalf("missing source-device-id not reported: pass=%v msg=%q", r.Pass, r.Message)
	}
}
