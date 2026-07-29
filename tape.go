// Package uyaml builds a structural index (a "tape") over Unity-serialized
// YAML using SIMD, in the spirit of simdjson's stage 1.
//
// Full YAML is context-sensitive (significant indentation, unbounded
// lookahead for "key:") and widely considered infeasible to structure-parse
// with SIMD. The Unity serialization dialect is the rare exception: it is
// strictly line-based, documents begin with a fixed anchor header
// ("--- !u!<classID> &<fileID>"), and indentation follows rigid two-space
// rules. That regularity is what this package exploits.
//
// Stage 1 (this package, v0.1) vectorizes the dominant scan — newline
// positions — and derives line starts and document headers from it. Later
// stages (key-colon positions, field extraction) build on the same tape.
//
// Honesty contract: this package never guesses. A line that begins with the
// document-header prefix but fails to parse is reported as a Doc with
// Malformed set — counted, never dropped, never "fixed up".
package uyaml

import (
	"errors"
	"math/bits"
)

// MaxInput is the largest input Index accepts. Offsets are stored as uint32
// to keep the tape compact; larger inputs are rejected explicitly rather
// than silently truncated.
const MaxInput = 1<<32 - 1

// ErrTooLarge is returned when the input exceeds MaxInput.
var ErrTooLarge = errors.New("uyaml: input exceeds 4 GiB tape offset limit")

// Doc is one Unity YAML document header ("--- !u!<classID> &<fileID>").
type Doc struct {
	FileID   int64  // anchor after '&'; Unity fileIDs may be negative
	ClassID  int32  // Unity class ID after "!u!"
	Line     uint32 // index into Tape.LineStarts of the header line
	Stripped bool   // header carries the "stripped" suffix
	// Malformed marks a line that begins with "--- !u!" but whose remainder
	// did not parse. ClassID/FileID are zero and MUST NOT be trusted; the
	// line is reported (never dropped) so counts stay honest.
	Malformed bool
}

// Tape is the structural index of one Unity YAML buffer.
type Tape struct {
	// LineStarts holds the byte offset of every line start, in order.
	// LineStarts[0] is always 0 for non-empty input. A trailing newline does
	// not open a final empty line.
	LineStarts []uint32
	// Docs lists document headers in file order, including malformed ones.
	Docs []Doc
	// N is the input length in bytes.
	N int
}

// LineEnd returns the byte offset one past the content of line i (before its
// newline). The final line ends at N.
func (t *Tape) LineEnd(i int) int {
	if i+1 < len(t.LineStarts) {
		return int(t.LineStarts[i+1]) - 1
	}
	return t.N
}

var docPrefix = []byte("--- !u!")

// Index builds the tape for data. The newline scan runs on the best
// available kernel (NEON on arm64, SWAR elsewhere); everything derived from
// it is bit-identical to the scalar reference implementation.
func Index(data []byte) (*Tape, error) {
	if len(data) > MaxInput {
		return nil, ErrTooLarge
	}
	t := &Tape{N: len(data)}
	if len(data) == 0 {
		return t, nil
	}

	masks := make([]uint64, (len(data)+63)/64)
	newlineMasks(data, masks)

	// Newline density in Unity YAML is roughly one per 30-60 bytes.
	t.LineStarts = make([]uint32, 0, len(data)/32+1)
	t.LineStarts = append(t.LineStarts, 0)
	for i, m := range masks {
		base := i * 64
		for m != 0 {
			pos := base + bits.TrailingZeros64(m)
			m &= m - 1
			if pos+1 < len(data) {
				t.LineStarts = append(t.LineStarts, uint32(pos+1))
			}
		}
	}

	for i, start := range t.LineStarts {
		line := data[start:t.LineEnd(i)]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if d, ok := parseDocHeader(line, uint32(i)); ok {
			t.Docs = append(t.Docs, d)
		}
	}
	return t, nil
}

// parseDocHeader parses "--- !u!<classID> &<fileID>[ stripped]". ok is false
// when the line does not start with the prefix at all. A prefixed line that
// fails to parse returns a Malformed doc (ok true): reported, not guessed.
func parseDocHeader(line []byte, lineIdx uint32) (Doc, bool) {
	if len(line) < len(docPrefix) || string(line[:len(docPrefix)]) != string(docPrefix) {
		return Doc{}, false
	}
	malformed := Doc{Line: lineIdx, Malformed: true}
	rest := line[len(docPrefix):]

	classID, n := parseUint(rest, 1<<31-1)
	if n == 0 || n >= len(rest) || rest[n] != ' ' {
		return malformed, true
	}
	rest = rest[n+1:]
	if len(rest) == 0 || rest[0] != '&' {
		return malformed, true
	}
	rest = rest[1:]
	neg := false
	if len(rest) > 0 && rest[0] == '-' {
		neg = true
		rest = rest[1:]
	}
	fileID, n := parseUint(rest, 1<<63-1)
	if n == 0 {
		return malformed, true
	}
	rest = rest[n:]
	stripped := false
	switch {
	case len(rest) == 0:
	case string(rest) == " stripped":
		stripped = true
	default:
		return malformed, true
	}
	id := int64(fileID)
	if neg {
		id = -id
	}
	return Doc{ClassID: int32(classID), FileID: id, Line: lineIdx, Stripped: stripped}, true
}

// parseUint reads leading ASCII digits, returning the value and the number
// of digits consumed. n is 0 when there are no digits or the value would
// exceed max (overflow is a parse failure, never a wrapped value).
func parseUint(b []byte, max uint64) (v uint64, n int) {
	for n < len(b) && b[n] >= '0' && b[n] <= '9' {
		d := uint64(b[n] - '0')
		if v > (max-d)/10 {
			return 0, 0
		}
		v = v*10 + d
		n++
	}
	return v, n
}
