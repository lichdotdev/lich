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
// its oldest byte, so a read costs a copy and no allocation. After the first
// overwrite the head may be a partial ANSI or UTF-8 sequence, an accepted
// artifact (TUIs repaint, shells scroll it away). Safe for concurrent append
// (the stream goroutine) and snapshot (an RPC call).
type replayBuffer struct {
	mu  sync.Mutex
	buf []byte
	// oldest is where the tail starts in buf once buf holds capBytes.
	oldest   int
	capBytes int
}

func newReplayBuffer(capBytes int) *replayBuffer {
	return &replayBuffer{capBytes: capBytes}
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
		data = data[len(data)-b.capBytes:]
	}
	if room := b.capBytes - len(b.buf); room > 0 {
		n := min(room, len(data))
		b.buf = append(b.buf, data[:n]...)
		data = data[n:]
	}
	for len(data) > 0 {
		n := copy(b.buf[b.oldest:], data)
		b.oldest = (b.oldest + n) % b.capBytes
		data = data[n:]
	}
}

// snapshot returns the tail as one contiguous slice in arrival order.
func (b *replayBuffer) snapshot() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]byte, len(b.buf))
	n := copy(out, b.buf[b.oldest:])
	copy(out[n:], b.buf[:b.oldest])
	return out
}
