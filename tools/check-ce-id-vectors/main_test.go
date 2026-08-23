package main

import (
	"encoding/hex"
	"strings"
	"testing"
)

// The published vectors, hardcoded. main.go checks that docs/ce-id-spec.md
// agrees with the algorithm; this file checks that the algorithm itself has
// not moved. Keeping both means an edit to the spec's numbers fails the
// gate, and an edit to the derivation fails the tests — neither can be
// changed unilaterally without something going red.
const (
	specPayloadHex = "1a0a76616c69646174696f6e"
	specCEType     = "openits.dms.message-activation-failed.v1"
	specCESource   = "urn:openits:sign:us-xx:example-agency:d01:demo-sign-1"
)

func TestCEIDPublishedVectors(t *testing.T) {
	payload, err := hex.DecodeString(specPayloadHex)
	if err != nil {
		t.Fatalf("decoding payload hex: %v", err)
	}

	tests := []struct {
		name       string
		source     string
		stableTime string
		want       string
	}{
		{
			name:       "vector 1, occurred-at equals ce-time",
			source:     specCESource,
			stableTime: "2026-07-22T12:00:00.000Z",
			want:       "01KY4V4VG09C9D44NNQCDWKJCN",
		},
		{
			name:       "vector 2, backfill where the times diverge",
			source:     specCESource,
			stableTime: "2026-07-22T11:59:00.000Z",
			want:       "01KY4V30X08F697N951X9F30AS",
		},
		{
			// The pre-0.4 vector, retained as a regression: it pins the
			// digest construction independently of the ce-source format
			// change that landed alongside the algorithm fix.
			name:       "retired v0.2 vector, region-first ce-source",
			source:     "urn:openits:us-xx:example-agency:d01:dms:demo-sign-1",
			stableTime: "2026-07-22T12:00:00.000Z",
			want:       "01KY4V4VG0ZNQNVEQB1WEBSX24",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ceID(tt.source, specCEType, tt.stableTime, payload)
			if err != nil {
				t.Fatalf("ceID() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ceID() = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestCEIDIsObserverInvariant is the property the algorithm change exists
// to establish: ce-time must not reach the id at all. Two observers of one
// occurrence, publishing at different wall-clock times, must agree.
func TestCEIDIsObserverInvariant(t *testing.T) {
	payload, err := hex.DecodeString(specPayloadHex)
	if err != nil {
		t.Fatalf("decoding payload hex: %v", err)
	}
	const stable = "2026-07-22T11:59:00.000Z"

	first, err := ceIDStampedWith(specCESource, specCEType, stable, stable, payload)
	if err != nil {
		t.Fatalf("ceIDStampedWith() error = %v", err)
	}
	// A second observer that saw the same occurrence six minutes later
	// still derives from stable-time, so it must land on the same id.
	second, err := ceID(specCESource, specCEType, stable, payload)
	if err != nil {
		t.Fatalf("ceID() error = %v", err)
	}
	if first != second {
		t.Errorf("two observers of one occurrence disagree: %s != %s", first, second)
	}

	// And the non-conformant reading — stamping from ce-time — must
	// produce a different id, or the vector could not catch the bug.
	wrong, err := ceIDStampedWith(specCESource, specCEType, stable, "2026-07-22T12:05:00.000Z", payload)
	if err != nil {
		t.Fatalf("ceIDStampedWith() error = %v", err)
	}
	if wrong == first {
		t.Fatal("stamping the ULID from ce-time produced the same id; the counter-example cannot fail")
	}
	if want := "01KY4VE0F08F697N951X9F30AS"; wrong != want {
		t.Errorf("counter-example = %s, want %s", wrong, want)
	}
	// Only the timestamp prefix may differ: the randomness is digest-derived
	// and the digest never sees ce-time.
	if wrong[10:] != first[10:] {
		t.Errorf("randomness differs between the two stampings: %s vs %s", wrong[10:], first[10:])
	}
}

func TestEncodeULID(t *testing.T) {
	tests := []struct {
		name string
		id   [16]byte
		want string
	}{
		{
			name: "zero value",
			id:   [16]byte{},
			want: "00000000000000000000000000",
		},
		{
			// All bits set: 128 ones read as 130 bits with two zero pads
			// puts 0b011 = 3 in the leading character and 0b11111 = Z in
			// the remaining 25.
			name: "all bits set",
			id:   [16]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			want: "7ZZZZZZZZZZZZZZZZZZZZZZZZZ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := encodeULID(tt.id); got != tt.want {
				t.Errorf("encodeULID() = %s, want %s", got, tt.want)
			}
		})
	}
}

const fixtureSpec = "" +
	"**Vector 1 — same times.**\n" +
	"\n" +
	"| Field | Value |\n" +
	"|---|---|\n" +
	"| `ce-source` | urn:example:a |\n" +
	"| `occurred-at` / `stable-time` | 2026-07-22T12:00:00.000Z |\n" +
	"| `ce-time` | 2026-07-22T12:00:00.000Z |\n" +
	"| **`ce-id`** | 01KY4V4VG09C9D44NNQCDWKJCN |\n" +
	"\n" +
	"Some prose in between.\n" +
	"\n" +
	"**Vector 2 — backfill.**\n" +
	"\n" +
	"| Field | Value |\n" +
	"|---|---|\n" +
	"| `ce-source` | urn:example:b |\n" +
	"| `occurred-at` / `stable-time` | 2026-07-22T11:59:00.000Z |\n" +
	"| `ce-time` | 2026-07-22T12:05:00.000Z |\n" +
	"| **`ce-id`** | 01KY4V30X08F697N951X9F30AS |\n"

func TestParseVectors(t *testing.T) {
	got, err := parseVectors(fixtureSpec)
	if err != nil {
		t.Fatalf("parseVectors() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("parseVectors() returned %d vectors, want 2", len(got))
	}

	if got[0].source != "urn:example:a" {
		t.Errorf("vector 1 source = %q, want urn:example:a", got[0].source)
	}
	if got[0].stableTime != "2026-07-22T12:00:00.000Z" {
		t.Errorf("vector 1 stable-time = %q", got[0].stableTime)
	}
	if got[0].wantID != "01KY4V4VG09C9D44NNQCDWKJCN" {
		t.Errorf("vector 1 ce-id = %q", got[0].wantID)
	}
	// The bold markers around **`ce-id`** must not survive into the label,
	// or the row would be silently skipped and the vector never checked.
	if got[1].wantID != "01KY4V30X08F697N951X9F30AS" {
		t.Errorf("vector 2 ce-id = %q, want the bolded cell to be parsed", got[1].wantID)
	}
	if got[1].ceTime != "2026-07-22T12:05:00.000Z" {
		t.Errorf("vector 2 ce-time = %q", got[1].ceTime)
	}
}

func TestParseVectorsRejectsIncompleteTable(t *testing.T) {
	const incomplete = "" +
		"**Vector 1 — missing its id.**\n" +
		"\n" +
		"| Field | Value |\n" +
		"|---|---|\n" +
		"| `ce-source` | urn:example:a |\n" +
		"| `occurred-at` / `stable-time` | 2026-07-22T12:00:00.000Z |\n"

	if _, err := parseVectors(incomplete); err == nil {
		t.Fatal("parseVectors() accepted a table with no ce-id row; want an error")
	}
}

// TestFindOneFailsLoudly guards the failure direction that matters: a
// reworded spec must break the build, not quietly check nothing.
func TestFindOneFailsLoudly(t *testing.T) {
	_, err := findOne(payloadRE, "a spec that no longer states the encoding", "payload hex", "docs/ce-id-spec.md")
	if err == nil {
		t.Fatal("findOne() returned no error for absent content; want a loud failure")
	}
	if !strings.Contains(err.Error(), "tools/check-ce-id-vectors") {
		t.Errorf("error should tell the maintainer where to fix the pattern, got: %v", err)
	}
}
