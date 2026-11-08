package terminal

import "sync"

// replayCapBytes caps a session's server-side replay tail. Per running
// session, in memory —
// a personal harness runs a handful; a disk-backed store (waveterm's filestore)
// is the upgrade path if session count or size ever makes that cost matter.
const replayCapBytes = 2 << 20

// replayBuffer keeps a capped tail of a session's raw PTY output so a
// window attaching to a running session can reseed its scrollback: the PTY
// (and this tail) live on in the backend while no window holds the terminal.
// One buffer, grown as output arrives up to the cap and then overwritten from
// its oldest byte, so a read costs a copy and no allocation. The bytes it
// overwrites feed modes, so a snapshot opens with the terminal state they set
// and never with half a sequence. Safe for concurrent append (the stream
// goroutine) and snapshot (an RPC call).
type replayBuffer struct {
	mu  sync.Mutex
	buf []byte
	// oldest is where the tail starts in buf once buf holds capBytes.
	oldest   int
	capBytes int
	modes    *modeState
}

func newReplayBuffer(capBytes int) *replayBuffer {
	return &replayBuffer{capBytes: capBytes, modes: newModeState()}
}

// append copies data (the caller reuses its read buffer) into the tail,
// overwriting the oldest bytes once the tail holds capBytes. A write larger
// than the cap keeps its last capBytes.
func (b *replayBuffer) append(data []byte) {
	if len(data) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(data) > b.capBytes {
		b.modes.feed(b.buf[b.oldest:])
		b.modes.feed(b.buf[:b.oldest])
		b.buf, b.oldest = b.buf[:0], 0
		b.modes.feed(data[:len(data)-b.capBytes])
		data = data[len(data)-b.capBytes:]
	}
	if room := b.capBytes - len(b.buf); room > 0 {
		n := min(room, len(data))
		b.buf = append(b.buf, data[:n]...)
		data = data[n:]
	}
	for len(data) > 0 {
		n := min(len(b.buf)-b.oldest, len(data))
		b.modes.feed(b.buf[b.oldest : b.oldest+n])
		copy(b.buf[b.oldest:], data[:n])
		b.oldest = (b.oldest + n) % b.capBytes
		data = data[n:]
	}
}

// snapshot is what rebuilds a terminal: the preamble that restores what the
// evicted bytes set, then the tail in arrival order.
func (b *replayBuffer) snapshot() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	pre := b.modes.preamble()
	out := b.joined(pre)
	if skip := b.modes.skip(out[len(pre):]); skip > 0 {
		out = append(out[:len(pre)], out[len(pre)+skip:]...)
	}
	return out
}

// text is the tail for a reader of the output itself, not a terminal: no
// preamble, and it starts past the sequence the ring head cut in two.
func (b *replayBuffer) text() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := b.joined(nil)
	return out[b.modes.toGround(out):]
}

// joined copies the tail after head, as one contiguous slice.
func (b *replayBuffer) joined(head []byte) []byte {
	out := make([]byte, 0, len(head)+len(b.buf))
	out = append(out, head...)
	out = append(out, b.buf[b.oldest:]...)
	return append(out, b.buf[:b.oldest]...)
}
