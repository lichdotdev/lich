//go:build linux || darwin

package main

// #include <stdint.h>
import "C"

import (
	"slices"
	"time"
)

// seenWait bounds the wait for the page to get an input event: one may
// reach no DOM (a press on a scrollbar, a page still loading) and is then
// answered without.
const seenWait = 500 * time.Millisecond

// inOrder runs act after every input act queued before it called its next.
func (b *browser) inOrder(act func(next func())) {
	b.inputs = append(b.inputs, act)
	if len(b.inputs) == 1 {
		b.runInput()
	}
}

func (b *browser) runInput() {
	act := b.inputs[0]
	act(func() {
		b.inputs = b.inputs[1:]
		if len(b.inputs) > 0 {
			b.runInput()
		}
	})
}

// waiter is an input event of the wire's waiting for the page's DOM event.
type waiter struct {
	event string
	done  func(any, error)
}

// await answers done once the page's DOM gets event, or after seenWait.
func (p *page) await(event string, done func(any, error)) {
	w := &waiter{event: event, done: done}
	p.waiting = append(p.waiting, w)
	time.AfterFunc(seenWait, func() { onMain(func() { p.answer(w) }) })
}

func (p *page) answer(w *waiter) {
	if i := slices.Index(p.waiting, w); i >= 0 {
		p.waiting = slices.Delete(p.waiting, i, i+1)
		w.done(nil, nil)
	}
}

// lichInputSeen is the page getting a DOM input event, which answers every
// event of the wire's waiting for that kind: WebKit merges wheel events still
// queued into one.
//
//export lichInputSeen
func lichInputSeen(key C.uintptr_t, event *C.char) {
	p := the.byKey[uintptr(key)]
	if p == nil {
		return
	}
	kind := C.GoString(event)
	for _, w := range slices.Clone(p.waiting) {
		if w.event == kind {
			p.answer(w)
		}
	}
}
