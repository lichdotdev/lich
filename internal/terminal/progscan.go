package terminal

import "bytes"

// progSignal is one thing a session's output says that program status reads:
// an OSC 7501 payload ("7501;..."), the shell starting a prompt (OSC 133 A),
// or a full reset (RIS, ESC c).
type progSignal struct {
	payload string
	bel     bool
	prompt  bool
	reset   bool
}

const (
	scanGround = iota
	scanEsc
	scanOsc
	scanOscEsc
)

const (
	asciiBEL = 0x07
	asciiCAN = 0x18
	asciiSUB = 0x1a
	asciiESC = 0x1b
)

// progScanner finds program status signals in a session's output. It keeps
// its state between reads, so a sequence split across two PTY reads is read
// whole. Only the stream goroutine of its session uses it.
type progScanner struct {
	state int
	buf   []byte
	// over is set once an OSC string outgrows the longest report: it can no
	// longer be one, and is skipped to its end.
	over bool
}

// scan reads one chunk of output and returns the signals completed in it.
// The OSC string rules are the DEC parser's, as the window's emulator reads
// them: a C0 control inside the string other than BEL and ESC is ignored, CAN
// and SUB cancel it, and an ESC that does not start ST ends it there.
func (s *progScanner) scan(p []byte) []progSignal {
	var out []progSignal
	for i := 0; i < len(p); i++ {
		b := p[i]
		switch s.state {
		case scanGround:
			j := bytes.IndexByte(p[i:], asciiESC)
			if j < 0 {
				return out
			}
			i += j
			s.state = scanEsc
		case scanEsc:
			out = s.afterEsc(b, out)
		case scanOsc:
			out = s.inOsc(b, out)
		case scanOscEsc:
			if b == '\\' {
				out = s.endOsc(false, out)
				continue
			}
			// The string ends at the ESC, which then starts the next sequence.
			out = s.endOsc(false, out)
			s.state = scanEsc
			i--
		}
	}
	return out
}

func (s *progScanner) afterEsc(b byte, out []progSignal) []progSignal {
	switch b {
	case ']':
		s.buf, s.over = s.buf[:0], false
		s.state = scanOsc
	case 'c':
		s.state = scanGround
		return append(out, progSignal{reset: true})
	case asciiESC:
	default:
		s.state = scanGround
	}
	return out
}

func (s *progScanner) inOsc(b byte, out []progSignal) []progSignal {
	switch {
	case b == asciiBEL:
		return s.endOsc(true, out)
	case b == asciiESC:
		s.state = scanOscEsc
	case b == asciiCAN || b == asciiSUB:
		s.state = scanGround
	case b < 0x20:
	case len(s.buf) < progMaxSequence:
		s.buf = append(s.buf, b)
	default:
		s.over = true
	}
	return out
}

func (s *progScanner) endOsc(bel bool, out []progSignal) []progSignal {
	s.state = scanGround
	if s.over {
		return out
	}
	switch {
	case bytes.HasPrefix(s.buf, []byte(progPrefix)):
		return append(out, progSignal{payload: string(s.buf), bel: bel})
	case isPromptStart(s.buf):
		return append(out, progSignal{prompt: true})
	}
	return out
}

func isPromptStart(payload []byte) bool {
	rest, ok := bytes.CutPrefix(payload, []byte("133;A"))
	return ok && (len(rest) == 0 || rest[0] == ';')
}
