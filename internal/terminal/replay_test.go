package terminal

import (
	"bytes"
	"github.com/lichdotdev/lich/api"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestReplayBufferKeepsOutputInOrder(t *testing.T) {
	t.Parallel()
	b := newReplayBuffer(1024)
	b.append([]byte("hello "))
	b.append([]byte("world"))
	if got := string(b.snapshot()); got != "hello world" {
		t.Fatalf("snapshot = %q, want %q", got, "hello world")
	}
}

func TestReplayBufferIgnoresEmptyAppend(t *testing.T) {
	t.Parallel()
	b := newReplayBuffer(1024)
	b.append(nil)
	b.append([]byte{})
	if got := b.snapshot(); len(got) != 0 {
		t.Fatalf("snapshot = %q, want empty", got)
	}
}

func TestReplayBufferCopiesInput(t *testing.T) {
	t.Parallel()
	b := newReplayBuffer(1024)
	buf := []byte("abc")
	b.append(buf)
	copy(buf, "XYZ") // the caller reuses its read buffer for the next PTY read
	if got := string(b.snapshot()); got != "abc" {
		t.Fatalf("snapshot = %q, want %q — append must copy, not alias", got, "abc")
	}
}

func TestReplayBufferDropsOldestOnOverflow(t *testing.T) {
	t.Parallel()
	b := newReplayBuffer(10)
	b.append([]byte("aaaaa")) // 5
	b.append([]byte("bbbbb")) // 10, still within cap
	b.append([]byte("ccccc")) // 15 → drops the first chunk
	got := string(b.snapshot())
	if strings.Contains(got, "a") {
		t.Fatalf("snapshot = %q, still holds the dropped chunk", got)
	}
	if got != "bbbbbccccc" {
		t.Fatalf("snapshot = %q, want %q", got, "bbbbbccccc")
	}
}

func TestReplayBufferKeepsOneChunkLargerThanCap(t *testing.T) {
	t.Parallel()
	b := newReplayBuffer(4)
	big := bytes.Repeat([]byte("x"), 100)
	b.append(big)
	if got := b.snapshot(); len(got) != 100 {
		t.Fatalf("snapshot len = %d, want 100 — a lone oversized chunk must survive", len(got))
	}
}

// A bare Service with a hand-built session exercises Replay without a PTY, so
// this stays OS-agnostic (no !windows build tag). The replay frame lands
// between the output already emitted and the output after it, and what the
// coalescer was holding goes out once, inside the tail, never again after it.
func TestServiceReplayQueuesTheTailBetweenOutputFrames(t *testing.T) {
	t.Parallel()
	var got []string
	box := newOutbox(
		func(data []byte) { got = append(got, "out:"+string(data)) },
		func(tail []byte) { got = append(got, "replay:"+string(tail)) },
		outboxDepth)
	sess := &session{replay: newReplayBuffer(replayCapBytes), outbox: box, out: newCoalescer(box.push, time.Hour, time.Hour)}
	read := func(data string) {
		sess.replay.append([]byte(data))
		sess.out.Write([]byte(data))
	}
	svc := &Service{sessions: map[string]*session{"s1": sess}}

	read("a")
	read("b")
	queued, err := svc.Replay(t.Context(), api.SessionIDOptions{ID: "s1"})
	if err != nil || !queued {
		t.Fatalf("Replay = %v, %v; want a replay queued", queued, err)
	}
	read("c")
	sess.out.Close()
	box.close()

	if want := []string{"out:a", "replay:ab", "out:c"}; !slices.Equal(got, want) {
		t.Fatalf("delivered %q, want %q", got, want)
	}
}

func TestServiceReplayUnknownSessionQueuesNothing(t *testing.T) {
	t.Parallel()
	svc := &Service{sessions: map[string]*session{}}
	queued, err := svc.Replay(t.Context(), api.SessionIDOptions{ID: "nope"})
	if err != nil {
		t.Fatalf("Replay = %v, want nil", err)
	}
	if queued {
		t.Fatal("Replay of an unknown session reported a replay coming")
	}
}
