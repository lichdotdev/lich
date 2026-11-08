package terminal

import (
	"bytes"
	"testing"
	"time"
)

// BenchmarkHiddenFlood feeds PTY reads through a hidden session's output path
// the way stream does (replay tail, coalescer, outbox, the /ws frame) and
// reports what that path allocates per byte of output.
func BenchmarkHiddenFlood(b *testing.B) {
	read := make([]byte, readBufSize)
	for i := range read {
		read[i] = 'x'
	}
	replay := newReplayBuffer(replayCapBytes)
	header, err := encodeFrame("session-id", nil)
	if err != nil {
		b.Fatal(err)
	}
	box := newOutbox(header, func([]byte) {}, func([]byte) {}, outboxDepth)
	out := newCoalescer(box.push, visibleFlushInterval, time.Hour)
	out.SetVisible(false)
	b.SetBytes(int64(len(read)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		replay.append(read)
		out.Write(read)
	}
	out.Close()
	box.close()
}

// BenchmarkReplayStringPayload evicts a kitty image's APC chunks through the
// replay ring: megabytes of payload that the scanner only has to step over.
func BenchmarkReplayStringPayload(b *testing.B) {
	chunk := append(append([]byte("\x1b_Gm=1;"), bytes.Repeat([]byte("QUJD"), readBufSize/4)...), "\x1b\\"...)
	r := newReplayBuffer(replayCapBytes)
	for range replayCapBytes/len(chunk) + 1 {
		r.append(chunk)
	}
	b.SetBytes(int64(len(chunk)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		r.append(chunk)
	}
}
