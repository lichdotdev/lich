package main

import (
	"strconv"

	"gioui.org/io/event"
	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// chatPermit is the keys' target while a permission waits; shown is the last
// one they were handed to.
type chatPermit struct {
	shown string
}

// focusPermit hands the keys to a waiting permission when it comes, unless a
// message is being typed, and back to it when nothing of the chat holds them;
// a click on the composer keeps them there.
func (p *chatPane) focusPermit(gtx C, it *backend.ChatItem) {
	came := p.permit.shown != it.ID
	p.permit.shown = it.ID
	switch {
	case gtx.Focused(&p.permit):
	case came && p.editor.Len() == 0, !gtx.Focused(&p.editor):
		gtx.Execute(key.FocusCmd{Tag: &p.permit})
	}
}

// permitActs runs a waiting permission's keys: a digit answers with the
// provider's option of its number, Esc with its first one-time reject. Enter
// answers nothing.
func (u *window) permitActs(gtx C, p *chatPane, id string, it *backend.ChatItem) {
	event.Op(gtx.Ops, &p.permit)
	opts := it.Permission.Options
	filters := []event.Filter{key.FocusFilter{Target: &p.permit}, key.Filter{Focus: &p.permit, Name: key.NameEscape}}
	for d := 1; d <= permitDigits(it.Permission); d++ {
		filters = append(filters, key.Filter{Focus: &p.permit, Name: key.Name(strconv.Itoa(d))})
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		if e.Name == key.NameEscape {
			u.answerChat(id, it.ID, permitOption(it.Permission, api.PermitRejectOnce))
			continue
		}
		d, _ := strconv.Atoi(string(e.Name))
		u.answerChat(id, it.ID, opts[d-1].ID)
	}
}

// permitSuggested is the answer a permission's box fills, never an always:
// its first one-time allow, or its first one-time reject on a default-no ask.
func permitSuggested(p *backend.ChatPermission) string {
	if p.DefaultNo {
		return permitOption(p, api.PermitRejectOnce)
	}
	return permitOption(p, api.PermitAllowOnce)
}

// permitDigits is how many of p's answers a digit key reaches: none on a
// default-no ask, which no single key may allow.
func permitDigits(p *backend.ChatPermission) int {
	if p.DefaultNo {
		return 0
	}
	return min(len(p.Options), chatQuizDigits)
}
