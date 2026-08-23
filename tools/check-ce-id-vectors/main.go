// check-ce-id-vectors re-derives every published ce-id test vector in
// docs/ce-id-spec.md straight from the algorithm that document specifies,
// and fails if any of them disagrees.
//
// The spec is normative for the wire contract but ships no implementation:
// "the reference implementation lives in the collector, not in this repo."
// That left the vectors as prose — numbers no build step ever evaluated.
// A vector nobody recomputes is a vector that silently rots the first time
// the algorithm text is edited without re-running the arithmetic by hand,
// and every downstream implementer inherits the rot.
//
// So this checker is deliberately NOT a reusable ce-id library: it is a
// gate. It parses the vector tables out of the markdown, recomputes each
// id, and compares. Changing the algorithm now forces the vectors to be
// regenerated (the checker fails until they are), and editing a vector
// without changing the algorithm fails too. Both directions are covered,
// which is the property the spec's own "a vector that cannot fail is not a
// vector" note is asking for.
//
// It also verifies the counter-example: the id the spec says an
// implementation produces if it wrongly stamps the ULID timestamp from
// ce-time instead of stable-time. That number is the whole reason the
// backfill vector exists, so it is checked, not trusted.
//
// Wired to `make check-ce-id-vectors`.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// crockford is the Crockford base32 alphabet ULID uses: the digits and
// uppercase letters with I, L, O and U removed.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// unitSep is the 0x1f byte the spec places between the four digest fields
// (three separators, none trailing).
const unitSep = 0x1f

// vector is one row-set of the "Test vectors" section.
type vector struct {
	name       string
	source     string
	stableTime string // as written in the doc: RFC 3339, ms precision, UTC Z
	ceTime     string
	wantID     string
}

var (
	// The payload and ce-type are stated once in the prose preceding the
	// vector tables and shared by every vector.
	payloadRE = regexp.MustCompile("deterministic proto3 encoding is\\s+`([0-9a-fA-F]+)`")
	ctypeRE   = regexp.MustCompile("the\\s+`ce-type`\\s+`([A-Za-z0-9.\\-]+)`")

	// "**Vector 2 — backfill, where ...**" opens each vector's table.
	vectorHeadRE = regexp.MustCompile(`^\*\*(Vector [^*]+?)\.?\*\*`)

	// A two-column markdown table row.
	rowRE = regexp.MustCompile(`^\|(.+?)\|(.+?)\|\s*$`)

	// The ce-time counter-example, stated in prose after the last vector.
	counterRE = regexp.MustCompile("produces\\s+`([0-9A-HJKMNP-TV-Z]{26})`")
)

func main() {
	const specPath = "docs/ce-id-spec.md"

	raw, err := os.ReadFile(specPath)
	if err != nil {
		fatalf("reading %s: %v", specPath, err)
	}
	src := string(raw)

	payloadHex, err := findOne(payloadRE, src, "payload hex", specPath)
	if err != nil {
		fatalf("%v", err)
	}
	payload, err := hex.DecodeString(payloadHex)
	if err != nil {
		fatalf("%s: payload %q is not valid hex: %v", specPath, payloadHex, err)
	}
	ceType, err := findOne(ctypeRE, src, "ce-type", specPath)
	if err != nil {
		fatalf("%v", err)
	}

	vectors, err := parseVectors(src)
	if err != nil {
		fatalf("%s: %v", specPath, err)
	}
	// The spec's own argument for vector 2 is that vector 1 cannot pin the
	// ULID timestamp, because its two times coincide. Fewer than two
	// vectors means that argument no longer holds.
	if len(vectors) < 2 {
		fatalf("%s: found %d vector(s), want at least 2 — vector 1 alone "+
			"cannot distinguish a ULID stamped from stable-time from one "+
			"stamped from ce-time", specPath, len(vectors))
	}

	failed := 0
	for _, v := range vectors {
		got, err := ceID(v.source, ceType, v.stableTime, payload)
		if err != nil {
			fatalf("%s: %s: %v", specPath, v.name, err)
		}
		if got != v.wantID {
			failed++
			fmt.Printf("FAIL  %s\n      ce-source   %s\n      stable-time %s\n"+
				"      want ce-id  %s\n      got  ce-id  %s\n",
				v.name, v.source, v.stableTime, v.wantID, got)
			continue
		}
		fmt.Printf("OK    %s  %s\n", v.name, got)
	}

	// The counter-example belongs to the last vector — the one whose
	// occurred-at and ce-time diverge.
	last := vectors[len(vectors)-1]
	if last.ceTime == "" || last.ceTime == last.stableTime {
		fatalf("%s: the final vector must be one where ce-time diverges from "+
			"stable-time; got ce-time %q and stable-time %q", specPath,
			last.ceTime, last.stableTime)
	}
	wantCounter, err := findOne(counterRE, src, "ce-time counter-example id", specPath)
	if err != nil {
		fatalf("%v", err)
	}
	// Same digest as the real vector — only the ULID timestamp is wrong.
	// That is exactly the bug the vector exists to catch, so the id must
	// share its randomness and differ only in the leading timestamp.
	gotCounter, err := ceIDStampedWith(last.source, ceType, last.stableTime, last.ceTime, payload)
	if err != nil {
		fatalf("%s: counter-example: %v", specPath, err)
	}
	if gotCounter != wantCounter {
		failed++
		fmt.Printf("FAIL  ce-time counter-example\n      want ce-id  %s\n      got  ce-id  %s\n",
			wantCounter, gotCounter)
	} else {
		fmt.Printf("OK    ce-time counter-example  %s\n", gotCounter)
	}

	if failed > 0 {
		fmt.Fprintf(os.Stderr, "\ncheck-ce-id-vectors: %d vector(s) do not match the "+
			"algorithm in %s.\nEither the algorithm text changed and the vectors were not "+
			"regenerated, or a vector was edited by hand.\n", failed, specPath)
		os.Exit(1)
	}
	fmt.Printf("\ncheck-ce-id-vectors: %d vector(s) + counter-example reproduce exactly.\n",
		len(vectors))
}

// ceID computes the deterministic ce-id: a ULID whose timestamp is
// stable-time and whose 80 bits of randomness are the leading bytes of a
// SHA-256 over the identity fields.
func ceID(source, ceType, stableTime string, payload []byte) (string, error) {
	return ceIDStampedWith(source, ceType, stableTime, stableTime, payload)
}

// ceIDStampedWith is ceID with the ULID timestamp taken from stampTime
// rather than stableTime. Passing anything but stableTime is non-conformant
// — it exists only so the checker can reproduce the spec's counter-example.
func ceIDStampedWith(source, ceType, stableTime, stampTime string, payload []byte) (string, error) {
	stamp, err := time.Parse(time.RFC3339, stampTime)
	if err != nil {
		return "", fmt.Errorf("parsing time %q: %w", stampTime, err)
	}

	sum := digest(source, ceType, stableTime, payload)

	var id [16]byte
	ms := uint64(stamp.UnixMilli())
	for i := range 6 {
		id[i] = byte(ms >> (40 - 8*i))
	}
	copy(id[6:], sum[:10])
	return encodeULID(id), nil
}

// digest is SHA-256 over the four identity fields joined by 0x1f. The
// payload arrives already stripped of the producer-assigned leaves; the
// vectors' payload carries none of them, so no clearing step is needed
// here.
func digest(source, ceType, stableTime string, payload []byte) [32]byte {
	h := sha256.New()
	h.Write([]byte(source))
	h.Write([]byte{unitSep})
	h.Write([]byte(ceType))
	h.Write([]byte{unitSep})
	h.Write([]byte(stableTime))
	h.Write([]byte{unitSep})
	h.Write(payload)
	return [32]byte(h.Sum(nil))
}

// encodeULID renders 128 bits as the 26 Crockford base32 characters of a
// ULID. 26 characters hold 130 bits, so the value is read as if
// left-padded with two zero bits — which is why the first character only
// ever carries the top two bits of the timestamp.
func encodeULID(id [16]byte) string {
	var out [26]byte
	for i := range out {
		// Bit offset, within the 128-bit value, of this character's
		// most-significant bit. Negative offsets are the two pad bits.
		hi := 5*i - 2
		var v byte
		for j := range 5 {
			pos := hi + j
			var bit byte
			if pos >= 0 {
				bit = (id[pos/8] >> (7 - pos%8)) & 1
			}
			v = v<<1 | bit
		}
		out[i] = crockford[v]
	}
	return string(out[:])
}

// parseVectors walks the document, collecting each "**Vector N — ...**"
// heading together with the two-column table that follows it.
func parseVectors(src string) ([]vector, error) {
	var (
		vectors []vector
		cur     *vector
	)
	flush := func() error {
		if cur == nil {
			return nil
		}
		if cur.source == "" || cur.stableTime == "" || cur.wantID == "" {
			return fmt.Errorf("%s: table is missing one of ce-source / "+
				"stable-time / ce-id", cur.name)
		}
		vectors = append(vectors, *cur)
		cur = nil
		return nil
	}

	for line := range strings.SplitSeq(src, "\n") {
		if m := vectorHeadRE.FindStringSubmatch(line); m != nil {
			if err := flush(); err != nil {
				return nil, err
			}
			cur = &vector{name: strings.TrimSpace(m[1])}
			continue
		}
		if cur == nil {
			continue
		}
		m := rowRE.FindStringSubmatch(line)
		if m == nil {
			// Blank line or prose after the table ends the vector; the
			// table's own header/separator rows match rowRE and fall
			// through the label switch harmlessly.
			if strings.TrimSpace(line) == "" && (cur.source != "" || cur.wantID != "") {
				if err := flush(); err != nil {
					return nil, err
				}
			}
			continue
		}
		label, value := cleanCell(m[1]), cleanCell(m[2])
		switch label {
		case "ce-source":
			cur.source = value
		case "occurred-at / stable-time", "stable-time":
			cur.stableTime = value
		case "ce-time":
			cur.ceTime = value
		case "ce-id":
			cur.wantID = value
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return vectors, nil
}

// cleanCell strips a markdown table cell down to its literal value,
// removing the backticks and bold markers the spec uses for emphasis.
func cleanCell(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "*", "")
	return strings.TrimSpace(s)
}

// findOne requires exactly one capture, so a reworded spec fails loudly
// here rather than silently checking nothing.
func findOne(re *regexp.Regexp, src, what, path string) (string, error) {
	m := re.FindStringSubmatch(src)
	if m == nil {
		return "", fmt.Errorf("%s: could not locate the %s. If the spec was "+
			"reworded, update the pattern in tools/check-ce-id-vectors rather "+
			"than dropping the check", path, what)
	}
	return m[1], nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "check-ce-id-vectors: "+format+"\n", args...)
	os.Exit(1)
}
