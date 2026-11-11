package extensions

import (
	"encoding/json"
	"sync"
	"time"
)

const (
	// maxRendersPerSecond bounds what one surface sends the window. Every
	// extension's messages share the one /events socket, and a write stalled
	// 5 seconds drops it for every session, so an extension rendering in a
	// loop must cost the window a frame rate, not the connection. 30 a second
	// is smooth for an interface that changes on events.
	maxRendersPerSecond = 30
	renderInterval      = time.Second / maxRendersPerSecond

	// undeclaredSurface is the shared gate of every surface the manifest does
	// not list: a render names its surface freely, and a gate per name would
	// let an extension dodge the limit with a new name each time. Surface ids
	// cannot be empty, so no declared one collides with it.
	undeclaredSurface = ""
)

// renderGate coalesces an extension's renders, per surface: the first goes out
// at once, and while the interval has not passed only the newest is kept and
// sent when it does. Nothing older waits behind it.
type renderGate struct {
	mu    sync.Mutex
	slots map[string]*renderSlot
}

type renderSlot struct {
	last    time.Time
	pending json.RawMessage
	timer   *time.Timer
}

// submit sends msg now or keeps it as the surface's pending render. send runs
// under the gate's lock, which keeps a surface's renders in order.
func (g *renderGate) submit(key string, msg json.RawMessage, send func(json.RawMessage)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.slots == nil {
		g.slots = make(map[string]*renderSlot)
	}
	slot := g.slots[key]
	if slot == nil {
		slot = &renderSlot{}
		g.slots[key] = slot
	}
	wait := renderInterval - time.Since(slot.last)
	if slot.timer == nil && wait <= 0 {
		slot.last = time.Now()
		send(msg)
		return
	}
	slot.pending = msg
	if slot.timer == nil {
		slot.timer = time.AfterFunc(wait, func() { g.flush(slot, send) })
	}
}

func (g *renderGate) flush(slot *renderSlot, send func(json.RawMessage)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if slot.timer == nil {
		return
	}
	msg := slot.pending
	slot.pending, slot.timer = nil, nil
	slot.last = time.Now()
	send(msg)
}

// reset drops what is pending, for a process that is gone: its last render
// must not land after its successor's.
func (g *renderGate) reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, slot := range g.slots {
		if slot.timer != nil {
			slot.timer.Stop()
		}
		slot.pending, slot.timer = nil, nil
	}
}

// throttleKey is the gate a render of surface goes through.
func (x *installed) throttleKey(surface string) string {
	for _, declared := range x.manifest.Surfaces {
		if declared.ID == surface {
			return surface
		}
	}
	return undeclaredSurface
}
