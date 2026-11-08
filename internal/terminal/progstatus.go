package terminal

import (
	"encoding/base64"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The Program Status Protocol, OSC 7501, revision 0.2 of 2026-10-06:
// https://www.superlogical.com/rex/docs/build/program-status. A program in a
// session's PTY writes OSC 7501 ; key=value:key=value ST to say what it is
// doing, and the session's card shows it when no hook speaks for the session.

// The limits of the specification's Limits section. A report that breaks one
// is discarded whole.
const (
	progMaxSequence     = 4096
	progMaxKey          = 16
	progMaxMsgEncoded   = 2732
	progMaxMsgDecoded   = 2048
	progMaxTitleEncoded = 256
	progMaxTitleDecoded = 192
	progMaxApp          = 32
	progMaxID           = 128
	progMaxSegment      = 32
	progMaxDepth        = 8
	progMaxRecords      = 256
)

const (
	progIdle    = "idle"
	progWorking = "working"
	progDone    = "done"
	progBlocked = "blocked"
	progError   = "error"
	progClear   = "clear"
)

const (
	progPrefix = "7501;"
	// progQueryReply is the only thing ever written back, without its
	// terminator: no record's id, title or message reaches the program.
	progQueryReply = "\x1b]7501;?"
)

// progReport is one OSC 7501 report that passed every check. kind and
// progress are not kept: neither can discard a report, and the card has no
// place for either.
type progReport struct {
	state, id, app, title, msg string
}

// progRawPairs is a report's values before they are checked, the last of a
// repeated key winning.
type progRawPairs struct {
	state, id, app, title, msg string
	hasID, hasTitle, hasMsg    bool
}

// parseProgReport reads a report's body, the bytes after "7501;", from a
// sequence seqLen bytes long, OSC through ST. False means the report is
// discarded or ignored, and nothing from it applies.
func parseProgReport(body string, seqLen int) (progReport, bool) {
	if seqLen > progMaxSequence {
		return progReport{}, false
	}
	raw, ok := progPairs(body)
	if !ok {
		return progReport{}, false
	}
	switch raw.state {
	case progIdle, progWorking, progDone, progBlocked, progError, progClear:
	default:
		return progReport{}, false
	}
	if raw.hasID && !progValidID(raw.id) {
		return progReport{}, false
	}
	if raw.state == progClear {
		return progReport{state: progClear, id: raw.id}, true
	}
	if len(raw.app) > progMaxApp {
		return progReport{}, false
	}
	r := progReport{state: raw.state, id: raw.id}
	if progValidSegment(raw.app, progMaxApp) {
		r.app = raw.app
	}
	if raw.hasTitle {
		if r.title, ok = progText(raw.title, progMaxTitleEncoded, progMaxTitleDecoded); !ok {
			return progReport{}, false
		}
	}
	if raw.hasMsg {
		if r.msg, ok = progText(raw.msg, progMaxMsgEncoded, progMaxMsgDecoded); !ok {
			return progReport{}, false
		}
	}
	return r, true
}

// progPairs splits a body into its known keys. A malformed pair is skipped,
// except an id: one outside the value set would otherwise land the report on
// the root record. An over-long key breaks a limit and discards the report.
func progPairs(body string) (progRawPairs, bool) {
	var raw progRawPairs
	for pair := range strings.SplitSeq(body, ":") {
		k, v, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		// Only ASCII space and tab: any other blank is a byte outside the
		// value set, which makes the pair malformed.
		k, v = strings.Trim(k, " \t"), strings.Trim(v, " \t")
		if len(k) > progMaxKey {
			return progRawPairs{}, false
		}
		if !progValidKey(k) {
			continue
		}
		if !progValidValue(v) {
			if k == "id" {
				return progRawPairs{}, false
			}
			continue
		}
		switch k {
		case "state":
			raw.state = v
		case "id":
			raw.id, raw.hasID = v, true
		case "app":
			raw.app = v
		case "title":
			raw.title, raw.hasTitle = v, true
		case "msg":
			raw.msg, raw.hasMsg = v, true
		}
	}
	return raw, true
}

// progValidKey is the key grammar, [a-z]+.
func progValidKey(k string) bool {
	if k == "" {
		return false
	}
	for i := range len(k) {
		if k[i] < 'a' || k[i] > 'z' {
			return false
		}
	}
	return true
}

// progValidValue is the value grammar, [A-Za-z0-9_.,+/=-]*.
func progValidValue(v string) bool {
	for i := range len(v) {
		if !progSegmentByte(v[i]) && v[i] != ',' && v[i] != '/' && v[i] != '=' {
			return false
		}
	}
	return true
}

// progSegmentByte is the set an id segment and app share, [A-Za-z0-9_.+-].
func progSegmentByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return c == '_' || c == '.' || c == '+' || c == '-'
}

func progValidSegment(s string, limit int) bool {
	if s == "" || len(s) > limit {
		return false
	}
	for i := range len(s) {
		if !progSegmentByte(s[i]) {
			return false
		}
	}
	return true
}

// progValidID is one to eight segments joined with "/", 128 bytes at most.
// The root record has no id, so "" is not one.
func progValidID(id string) bool {
	if id == "" || len(id) > progMaxID {
		return false
	}
	depth := 0
	for seg := range strings.SplitSeq(id, "/") {
		depth++
		if depth > progMaxDepth || !progValidSegment(seg, progMaxSegment) {
			return false
		}
	}
	return true
}

// progText decodes a base64 free-text value and makes it safe to show. It
// fails on bad base64, on text over its limit, on text that is not UTF-8 and
// on any control character. Padding is optional, but padding that is there
// must be right, and Strict refuses set bits in the unused tail, so each text
// has one encoding.
func progText(v string, maxEncoded, maxDecoded int) (string, bool) {
	if len(v) > maxEncoded {
		return "", false
	}
	if !strings.Contains(v, "=") {
		v += strings.Repeat("=", (4-len(v)%4)%4)
	}
	b, err := base64.StdEncoding.Strict().DecodeString(v)
	if err != nil || len(b) > maxDecoded || !utf8.Valid(b) {
		return "", false
	}
	for _, r := range string(b) {
		if r <= 0x1f || (r >= 0x7f && r <= 0x9f) {
			return "", false
		}
	}
	return stripInvisible(string(b)), true
}

// stripInvisible drops bidi overrides, zero-width and other format
// characters, and the line and paragraph separators, so a program cannot
// reorder or hide what the card says it came from.
func stripInvisible(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
			return -1
		}
		return r
	}, s)
}

// progQuery reports whether body, the bytes after "7501;", is the feature
// detection query.
func progQuery(body string) bool {
	return strings.Trim(body, " \t") == "?"
}

// progSeqLen is the length of a whole sequence, OSC through ST, around a
// payload ("7501;..." included) ended with BEL when bel is set.
func progSeqLen(payloadLen int, bel bool) int {
	if bel {
		return 2 + payloadLen + 1
	}
	return 2 + payloadLen + 2
}
