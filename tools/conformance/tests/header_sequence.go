package tests

// TestEvent_SequenceContiguousWithinEpoch checks the event-header stream
// contract. Every notification carries sequence-epoch, and within one
// (source-device-id, sequence-epoch) stream sequence increases by exactly 1
// in arrival order. The harness receives the device's full stream, so a gap
// here is loss or a numbering bug, not subscription filtering. A new epoch
// starts a fresh stream. Events without a header (nil Data, or a payload
// with no sequence field) are skipped, and so is an event whose ce-id was
// already seen: at-least-once delivery may redeliver it, which is not a
// gap. An event with no source-device-id is an error, since it names no
// stream.
func TestEvent_SequenceContiguousWithinEpoch(t *T, obs *Observation) {
	type stream struct{ source, epoch string }
	last := map[stream]uint64{}
	seenID := map[string]bool{}
	for _, e := range obs.Events {
		if e.Data == nil {
			continue
		}
		if e.CEID != "" {
			if seenID[e.CEID] {
				continue
			}
			seenID[e.CEID] = true
		}
		m := e.Data.ProtoReflect()
		fields := m.Descriptor().Fields()
		seqF := fields.ByName("sequence")
		epochF := fields.ByName("sequence_epoch")
		srcF := fields.ByName("source_device_id")
		if seqF == nil || epochF == nil || srcF == nil {
			continue
		}
		epoch := m.Get(epochF).String()
		if epoch == "" {
			t.Errorf("%s (subject %q) has no sequence-epoch", e.CEType, e.Subject)
			continue
		}
		src := m.Get(srcF).String()
		if src == "" {
			t.Errorf("%s (subject %q) has no source-device-id", e.CEType, e.Subject)
			continue
		}
		k := stream{src, epoch}
		seq := m.Get(seqF).Uint()
		if prev, ok := last[k]; ok && seq != prev+1 {
			t.Errorf("%s (subject %q): sequence %d follows %d in epoch %q; want %d",
				e.CEType, e.Subject, seq, prev, epoch, prev+1)
		}
		last[k] = seq
	}
}
