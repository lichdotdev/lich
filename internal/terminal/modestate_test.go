package terminal

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func kittyPushes(top int) string {
	return strings.Repeat("\x1b[>0u", kittyStackLen-1) + "\x1b[>" + string(rune('0'+top)) + "u"
}

func TestModeStatePreamble(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, in, want string
	}{
		{"mouse any-event", "\x1b[?1003h", "\x1b[?1003h"},
		{"mouse last set wins", "\x1b[?1000h\x1b[?1003h\x1b[?1000h", "\x1b[?1003h\x1b[?1000h"},
		{"mouse encoding", "\x1b[?1006h", "\x1b[?1006h"},
		{"several in one", "\x1b[?1;25l", "\x1b[?1l\x1b[?25l"},
		{"alt screen", "\x1b[?1049h", "\x1b[?1049h"},
		{"bracketed paste", "\x1b[?2004h", "\x1b[?2004h"},
		{"focus events", "\x1b[?1004h", "\x1b[?1004h"},
		{"grapheme clusters", "\x1b[?2027h", "\x1b[?2027h"},
		{"keypad application", "\x1b=", "\x1b="},
		{"keypad back to numeric", "\x1b=\x1b>", ""},
		{"kitty push", "\x1b[>1u", kittyPushes(1)},
		{"kitty push pop balance", "\x1b[>1u\x1b[>3u\x1b[<2u", ""},
		{"kitty set or", "\x1b[>1u\x1b[=4;2u", kittyPushes(5)},
		{"kitty set not", "\x1b[>5u\x1b[=4;3u", kittyPushes(1)},
		{"kitty set replace", "\x1b[=2u", kittyPushes(2)},
		{"kitty pop past the stack", "\x1b[>1u\x1b[>1u\x1b[<9u", ""},
		{"kitty flags out of range", "\x1b[>40u", ""},
		{"kitty query", "\x1b[?u", ""},
		{"kitty on the alternate screen", "\x1b[?1049h\x1b[>3u", "\x1b[?1047h" + kittyPushes(3) + "\x1b[?1047l\x1b[?1049h"},
		{"modify other keys", "\x1b[>4;2m", "\x1b[>4;2m"},
		{"modify other keys reset", "\x1b[>4;2m\x1b[>4m", ""},
		{"modify keys reset all", "\x1b[>4;2m\x1b[>m", ""},
		{"modify keys bad resource", "\x1b[>9;1m", ""},
		{"cursor style", "\x1b[5 q", "\x1b[5 q"},
		{"scroll margins", "\x1b[2;20r", "\x1b[2;20r"},
		{"scroll margins reset", "\x1b[2;20r\x1b[r", ""},
		{"palette", "\x1b]4;1;rgb:ff/00/00;2;#00ff00\x07", "\x1b]4;1;rgb:ff/00/00\x1b\\\x1b]4;2;#00ff00\x1b\\"},
		{"palette reset one", "\x1b]4;1;red;2;blue\x07\x1b]104;1\x07", "\x1b]4;2;blue\x1b\\"},
		{"palette reset all", "\x1b]4;1;red\x1b\\\x1b]104\x07", ""},
		{"background", "\x1b]11;rgb:1e/1e/2e\x1b\\", "\x1b]11;rgb:1e/1e/2e\x1b\\"},
		{"dynamic colors chained", "\x1b]10;a;b\x07", "\x1b]10;a\x1b\\\x1b]11;b\x1b\\"},
		{"dynamic color reset", "\x1b]12;red\x07\x1b]112\x07", ""},
		{"color query", "\x1b]11;?\x07", ""},
		{"charset and shift out", "\x1b)0\x0e", "\x1b)0\x0e"},
		{"shift in", "\x1b)0\x0e\x0f", "\x1b)0"},
		{"shift out inside a CSI", "\x1b[1\x0em", "\x0e"},
		{"RIS resets everything", "\x1b[?1003h\x1b[>1u\x1b]11;red\x07\x1bc", ""},
		{"CAN aborts", "\x1b[?1003\x18h", ""},
		{"ESC restarts", "\x1b[?10\x1b[?2004h", "\x1b[?2004h"},
		{"string aborted by ESC", "\x1bPq\x1b[?1003h", "\x1b[?1003h"},
		{"string ended", "\x1b_Gi=1\x1b\\\x1b[?1003h", "\x1b[?1003h"},
		{"partial ESC", "\x1b", "\x1b"},
		{"partial ESC intermediate", "\x1b(", "\x1b("},
		{"partial CSI", "\x1b[?10", "\x1b[?10"},
		{"partial OSC", "\x1b]11;rgb", "\x1b]11;rgb"},
		{"partial OSC at ST", "\x1b]11;x\x1b", "\x1b]11;x\x1b"},
		{"partial string not replayed", "\x1bPq", ""},
		{"partial string at its ESC", "\x1bPq\x1b", "\x1b"},
		{"bad mode number", "\x1b[?h\x1b]x;1\x07", ""},
		{"too many modes keep the latest", manyModes(modesCap + 1), manyModesWant(modesCap + 1)},
		{"C0 inside an escape", "\x1b\x0e[?1003h", "\x1b[?1003h\x0e"},
		{"ESC restarts an escape", "\x1b\x1b[?1003h", "\x1b[?1003h"},
		{"OSC aborted by ESC", "\x1b]11;red\x1b[?1003h", "\x1b[?1003h"},
		{"OSC skips C0", "\x1b]11;r\x0ed\x07", "\x1b]11;rd\x1b\\"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newModeState()
			s.feed([]byte(c.in))
			if got := string(s.preamble()); got != c.want {
				t.Errorf("preamble = %q, want %q", got, c.want)
			}
		})
	}
}

func manyModes(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "\x1b[?%dh", i+2000)
	}
	return b.String()
}

func manyModesWant(n int) string {
	var b strings.Builder
	for i := n - modesCap; i < n; i++ {
		fmt.Fprintf(&b, "\x1b[?%dh", i+2000)
	}
	return b.String()
}

// ringReplay runs data through a ring of capBytes in chunks of size n.
func ringReplay(capBytes, n int, data string) string {
	b := newReplayBuffer(capBytes)
	for len(data) > 0 {
		k := min(n, len(data))
		b.append([]byte(data[:k]))
		data = data[k:]
	}
	return string(b.snapshot())
}

func TestReplayRebuildsTheHead(t *testing.T) {
	t.Parallel()
	osc52 := "\x1b]52;c;" + strings.Repeat("A", rawCap+1000) + "\x07"
	cases := []struct {
		name, in, want string
		capBytes       int
	}{
		{"sequence cut by the eviction edge", "\x1b[?1049hAB" + "CDE", "\x1b[?1049hABCDE", 10},
		{"mode reset inside the tail", "\x1b[?2004h" + "0123456789" + "\x1b[?2004l", "\x1b[?2004h" + "3456789\x1b[?2004l", 15},
		{"RIS in the evicted part", "\x1b[?1003h\x1bc" + "0123456789", "0123456789", 10},
		{"head inside an OSC too long to keep", osc52 + "hello", "hello", 1000},
		{"head inside an OSC at its ST", osc52[:len(osc52)-1] + "\x1b\\hi", "hi", 100},
		{"head inside an overflowed CSI", "\x1b[" + strings.Repeat("1", rawCap+100) + "mok", "ok", 20},
		{"overflowed CSI aborted by ESC", "\x1b[" + strings.Repeat("1", rawCap+10) + "\x1b[?2004h", "\x1b[?2004h", 9},
		{"head inside a DCS", "\x1bPqqqqqqqq\x1b\\hi", "hi", 6},
		{"head inside a DCS after its ESC, a lone ST", "\x1bPqqqq\x1b\\hi", "\x1b\\hi", 3},
		{"head inside an OSC too long, aborted by ESC", osc52[:len(osc52)-1] + "\x1b[?2004h", "\x1b[?2004h", 100},
		{"head inside an OSC too long, ended after its ESC", osc52[:len(osc52)-1] + "\x1b\\hi", "\x1b\\hi", 3},
		{"head inside an endless string", "\x1b_G" + strings.Repeat("A", 50), "", 10},
		{"DCS aborted right after the evicted ESC", "\x1bPqqqq\x1b[?2004h", "\x1b[?2004h", 7},
		{"head inside a UTF-8 character", "xé" + "abc", "abc", 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, n := range []int{1, 3, len(c.in)} {
				if got := ringReplay(c.capBytes, n, c.in); got != c.want {
					t.Errorf("chunks of %d: snapshot = %q, want %q", n, got, c.want)
				}
			}
		})
	}
}

func TestReplayTextStartsOutsideAnySequence(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, in, want string
		capBytes       int
	}{
		{"nothing evicted", "\x1b[?2004hab", "\x1b[?2004hab", 64},
		{"modes evicted, no preamble", "\x1b[?2004h" + "0123456789", "0123456789", 10},
		{"CSI cut by the head", "\x1b[?1049hABCDE", "ABCDE", 10},
		{"OSC cut by the head", "\x1b]11;rgb:1e/1e/2e\x07hello", "hello", 8},
		{"OSC cut before its ST", "\x1b]11;red\x1b\\hi", "hi", 3},
		{"whole sequence evicted", "\x1b[?2004hab", "ab", 2},
		{"DCS cut by the head", "\x1bPqqqq\x1b\\hi", "hi", 5},
		{"UTF-8 character cut by the head", "xé" + "abc", "abc", 4},
		{"head never reaches ground", "\x1b]2;" + strings.Repeat("t", 40), "", 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := newReplayBuffer(c.capBytes)
			b.append([]byte(c.in))
			if got := string(b.text()); got != c.want {
				t.Errorf("text = %q, want %q", got, c.want)
			}
		})
	}
}

// However the output was cut into reads, the ring and its preamble rebuild
// the same terminal.
func TestReplaySameAcrossChunking(t *testing.T) {
	t.Parallel()
	in := strings.Repeat("\x1b[?1049h\x1b[>1u\x1b]4;3;#123456\x07text é\x1b[?1049l\x1b[<u\x1b[2;9r\x1b(0", 50)
	want := ringReplay(64, len(in), in)
	for n := 1; n < 40; n++ {
		if got := ringReplay(64, n, in); got != want {
			t.Fatalf("chunks of %d: snapshot = %q, want %q", n, got, want)
		}
	}
}

// A TUI sets its modes once at startup and then only repaints; once the
// repaint has pushed those bytes out of the ring, the rebuilt terminal must
// still get them.
func TestReplayKeepsModesSetBeforeTheRing(t *testing.T) {
	t.Parallel()
	modes := []string{"\x1b[?1049h", "\x1b[?1003h", "\x1b[?1006h", "\x1b[?2004h", "\x1b[?1004h", "\x1b[>1u", "\x1b]11;rgb:1e/1e/2e\x1b\\"}
	b := newReplayBuffer(replayCapBytes)
	b.append([]byte(strings.Join(modes, "")))
	frame := []byte("\x1b[H\x1b[2J\x1b[38;2;200;100;50mredraw line of a TUI frame\x1b[0m\r\n")
	for written := 0; written < replayCapBytes+len(frame); written += len(frame) {
		b.append(frame)
	}
	snap := b.snapshot()
	for _, m := range modes {
		if !bytes.Contains(snap, []byte(m)) {
			t.Errorf("rebuilt terminal misses %q", m)
		}
	}
}
