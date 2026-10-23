package terminal

// outboxDepth is how many flushed frames may wait on a stalled window before
// the session's own PTY read blocks. A frame is one coalescer flush — 8ms of
// output while visible, capped at maxPendingBytes — so this bounds both the
// memory one session may hold and how far ahead of the window it may run.
const outboxDepth = 32

// outbox carries one session's coalesced output to the window on a goroutine
// of its own.
//
// Delivery blocks: one WebSocket carries every session (transport.send) and
// the /events fallback is one socket too, so a window that stops reading backs
// pressure up into whoever is writing. Doing that on the PTY read path stalls the read,
// the PTY buffer fills, and the child stops producing; because the socket is
// shared, that reaches every session, not just the slow one. The queue is what
// decouples them: a stalled window backs up one session's frames and, only
// once the queue is full, that session's PTY. The backpressure that remains is
// honest — it reaches the session that is producing, never its neighbours.
type outbox struct {
	frames chan outFrame
	done   chan struct{}
	// prefix heads every output frame: the /ws frame header of its session.
	prefix []byte
}

type outFrame struct {
	data   []byte
	replay bool
}

// newOutbox starts the delivery goroutine. deliver is called with each output
// frame, the pushed output behind prefix, and replay with each replay frame's
// tail, one at a time and in the order they were queued; both own the slice
// they are handed.
func newOutbox(prefix []byte, deliver, replay func(data []byte), depth int) *outbox {
	o := &outbox{frames: make(chan outFrame, depth), done: make(chan struct{}), prefix: prefix}
	go func() {
		defer close(o.done)
		for frame := range o.frames {
			if frame.replay {
				replay(frame.data)
				continue
			}
			deliver(frame.data)
		}
	}()
	return o
}

// push queues data behind the prefix for delivery, blocking only while this
// session's queue is full. data is copied into the frame, which is the one
// copy output takes on its way to the socket: the coalescer reuses the buffer
// it passes.
func (o *outbox) push(data []byte) {
	frame := make([]byte, len(o.prefix)+len(data))
	copy(frame, o.prefix)
	copy(frame[len(o.prefix):], data)
	o.frames <- outFrame{data: frame}
}

// pushReplay queues the session's replayed tail behind every frame already
// queued, so whoever reads it knows what came before the tail and what after.
func (o *outbox) pushReplay(tail []byte) {
	o.frames <- outFrame{data: tail, replay: true}
}

// close stops the queue and waits for the frames still in it to be delivered,
// so the caller can order a later event (the exit banner) after the last byte.
func (o *outbox) close() {
	close(o.frames)
	<-o.done
}
