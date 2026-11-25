package main

import (
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// chatQuiz is the window's side of a question item while it waits: the ask on
// show, the option the keys stand on, and what the person answered so far. The
// quiz itself is the tag the card's keys go to.
type chatQuiz struct {
	// step is the ask on show.
	step int
	// hot is the row the keys stand on: an option, then "Other…" at
	// len(options), then Continue past it on an ask that takes several.
	hot     int
	answers []api.ChatAskAnswer
	done    []bool
	marks   [][]ui.Bool
	rows    [][]ui.Clickable
	others  []widget.Editor
	steps   []ui.Clickable
	send    ui.Clickable
	motion  panelMotion
	decline ui.Clickable
	// moved asks for the focus to follow the step.
	moved bool
	// sent holds the card back from a second reply while the first is on its
	// way.
	sent atomic.Bool
	// proposal is a card that asks to start on a plan, whose field keeps the
	// agent planning.
	proposal bool
}

// quiz is question item id's state, made for its asks.
func (v *chatView) quiz(id string, asks []api.ChatAsk) *chatQuiz {
	if v.quizzes == nil {
		v.quizzes = map[string]*chatQuiz{}
	}
	q, ok := v.quizzes[id]
	if !ok || !q.fits(asks) {
		q = &chatQuiz{answers: make([]api.ChatAskAnswer, len(asks)), done: make([]bool, len(asks)),
			marks: make([][]ui.Bool, len(asks)), rows: make([][]ui.Clickable, len(asks)),
			others: make([]widget.Editor, len(asks)), steps: make([]ui.Clickable, len(asks))}
		for i, a := range asks {
			q.marks[i] = make([]ui.Bool, len(a.Options))
			q.rows[i] = make([]ui.Clickable, len(a.Options))
			q.others[i].Submit = true
			q.others[i].SingleLine = len(a.Options) > 0
		}
		v.quizzes[id] = q
	}
	return q
}

// fits is whether q was made for asks: an id the backend numbers per process
// can come back for another question.
func (q *chatQuiz) fits(asks []api.ChatAsk) bool {
	if len(q.rows) != len(asks) {
		return false
	}
	for i, a := range asks {
		if len(q.rows[i]) != len(a.Options) {
			return false
		}
	}
	return true
}

// dropQuizzes forgets every card but the waiting one's, so an id that comes
// back for a new question starts afresh and takes the keys again.
func (v *chatView) dropQuizzes(asking *backend.ChatItem) {
	for id := range v.quizzes {
		if asking == nil || id != asking.ID {
			delete(v.quizzes, id)
		}
	}
	if asking == nil {
		v.asked = ""
	}
}

// questionWaiting is the question item still unanswered, the last one, nil
// for none.
func questionWaiting(items []backend.ChatItem) *backend.ChatItem {
	for _, it := range slices.Backward(items) {
		if it.Kind == api.ItemQuestion && it.Question != nil && it.Question.Reply == nil {
			return &it
		}
	}
	return nil
}

// target is what takes the keys on the step on show: the field of a question
// with no options, the card for the rest.
func (q *chatQuiz) target(asks []api.ChatAsk) event.Tag {
	if q.step < len(asks) && len(asks[q.step].Options) == 0 {
		return &q.others[q.step]
	}
	return q
}

// holds is whether the card or one of its fields has the keys.
func (q *chatQuiz) holds(gtx C) bool {
	if gtx.Focused(q) {
		return true
	}
	for i := range q.others {
		if gtx.Focused(&q.others[i]) {
			return true
		}
	}
	return false
}

// focusQuestion hands the keys to a waiting question once when it comes and
// again when its step moves, and back to it when nothing of the chat holds
// them; a click on the composer keeps them there.
func (p *chatPane) focusQuestion(gtx C, it *backend.ChatItem, q *chatQuiz) {
	asks := it.Question.Questions
	if p.asked != it.ID || q.moved || (!gtx.Focused(&p.editor) && !q.holds(gtx)) {
		p.asked, q.moved = it.ID, false
		gtx.Execute(key.FocusCmd{Tag: q.target(asks)})
	}
}

// questionActs runs a waiting question's keys and clicks.
func (u *window) questionActs(gtx C, p *chatPane, sv sessionView, it *backend.ChatItem) {
	asks := it.Question.Questions
	q := p.quiz(it.ID, asks)
	event.Op(gtx.Ops, q)
	for {
		if _, ok := gtx.Event(key.FocusFilter{Target: q}); !ok {
			break
		}
	}
	if q.sent.Load() {
		return
	}
	reply := func(r api.ChatQReplied) { u.replyChat(sv.ID, it.ID, q, r) }
	if q.decline.Clicked(gtx) {
		reply(api.ChatQReplied{Declined: true})
		return
	}
	for i := range asks {
		u.questionFieldActs(gtx, q, asks, i, reply)
	}
	for i := range q.steps {
		if q.steps[i].Clicked(gtx) && (q.done[i] || i == q.step) {
			q.step, q.hot, q.moved = i, 0, true
		}
	}
	// An answer moves the card on mid-frame: the rest of the frame's clicks
	// and keys belong to the ask that was on show, and are dropped with it.
	step := q.step
	for j := range q.rows[step] {
		if q.rows[step][j].Clicked(gtx) {
			q.hot = j
			q.pick(asks, j, reply)
			break
		}
	}
	if q.step != step {
		return
	}
	if q.send.Clicked(gtx) {
		q.submit(asks, reply)
		return
	}
	q.keys(gtx, asks, reply)
}

// questionFieldActs runs ask i's own field: Enter answers with it, Esc
// declines the whole question.
func (u *window) questionFieldActs(gtx C, q *chatQuiz, asks []api.ChatAsk, i int, reply func(api.ChatQReplied)) {
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &q.others[i], Name: key.NameEscape})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			reply(api.ChatQReplied{Declined: true})
		}
	}
	for {
		ev, ok := q.others[i].Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok && i == q.step {
			q.submit(asks, reply)
		}
	}
}

// chatQuizDigits is how many options a digit key reaches: 1 to 9.
const chatQuizDigits = 9

// keys runs the card's own keys on ask a: a digit picks its option, the
// arrows walk the rows, Space marks one of several, Enter takes the row the
// keys stand on, Esc declines.
func (q *chatQuiz) keys(gtx C, asks []api.ChatAsk, reply func(api.ChatQReplied)) {
	step := q.step
	a := asks[step]
	filters := []event.Filter{
		key.Filter{Focus: q, Name: key.NameUpArrow}, key.Filter{Focus: q, Name: key.NameDownArrow},
		key.Filter{Focus: q, Name: key.NameReturn}, key.Filter{Focus: q, Name: key.NameEnter},
		key.Filter{Focus: q, Name: key.NameSpace}, key.Filter{Focus: q, Name: key.NameEscape},
	}
	for d := 1; d <= min(len(a.Options)+1, chatQuizDigits); d++ {
		filters = append(filters, key.Filter{Focus: q, Name: key.Name(strconv.Itoa(d))})
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
		q.key(gtx, e.Name, a, asks, reply)
		if q.step != step || q.sent.Load() {
			return
		}
	}
}

func (q *chatQuiz) key(gtx C, name key.Name, a api.ChatAsk, asks []api.ChatAsk, reply func(api.ChatQReplied)) {
	n, rows := len(a.Options), quizRows(a)
	switch name {
	case key.NameUpArrow:
		q.hot = (q.hot + rows - 1) % rows
	case key.NameDownArrow:
		q.hot = (q.hot + 1) % rows
	case key.NameEscape:
		reply(api.ChatQReplied{Declined: true})
	case key.NameSpace:
		if a.Multi && q.hot < n {
			q.marks[q.step][q.hot].Value = !q.marks[q.step][q.hot].Value
		}
	case key.NameReturn, key.NameEnter:
		switch {
		case q.hot < n:
			q.pick(asks, q.hot, reply)
		case q.hot == n:
			gtx.Execute(key.FocusCmd{Tag: &q.others[q.step]})
		default:
			q.submit(asks, reply)
		}
	default:
		d, _ := strconv.Atoi(string(name))
		q.hot = d - 1
		if q.hot == n {
			gtx.Execute(key.FocusCmd{Tag: &q.others[q.step]})
			return
		}
		q.pick(asks, q.hot, reply)
	}
}

// quizRows is how many rows the keys walk on ask a: its options, "Other…",
// and Continue on one that takes several.
func quizRows(a api.ChatAsk) int {
	if a.Multi {
		return len(a.Options) + 2
	}
	return len(a.Options) + 1
}

// pick takes option j of the ask on show: one of several is marked, a single
// choice answers it.
func (q *chatQuiz) pick(asks []api.ChatAsk, j int, reply func(api.ChatQReplied)) {
	a := asks[q.step]
	if a.Multi {
		q.marks[q.step][j].Value = !q.marks[q.step][j].Value
		return
	}
	q.answer(asks, api.ChatAskAnswer{Picked: []string{a.Options[j].Label}}, reply)
}

// submit answers the ask on show with its marks and its field, when either
// holds something.
func (q *chatQuiz) submit(asks []api.ChatAsk, reply func(api.ChatQReplied)) {
	if ans, ok := q.draft(asks[q.step]); ok {
		q.answer(asks, ans, reply)
	}
}

// draft is what the ask on show holds now: its marks and its field's text.
func (q *chatQuiz) draft(a api.ChatAsk) (api.ChatAskAnswer, bool) {
	var ans api.ChatAskAnswer
	for j, m := range q.marks[q.step] {
		if m.Value {
			ans.Picked = append(ans.Picked, a.Options[j].Label)
		}
	}
	ans.Other = strings.TrimSpace(q.others[q.step].Text())
	return ans, len(ans.Picked) > 0 || ans.Other != ""
}

// answer records the ask on show and moves on to the first one left; the
// last one left sends them all.
func (q *chatQuiz) answer(asks []api.ChatAsk, ans api.ChatAskAnswer, reply func(api.ChatQReplied)) {
	q.answers[q.step], q.done[q.step] = ans, true
	next := slices.Index(q.done, false)
	if next < 0 {
		reply(api.ChatQReplied{Answers: q.answers})
		return
	}
	q.step, q.hot, q.moved = next, 0, true
}

// replyChat sends a question's reply; a refused one leaves the card as it was.
func (u *window) replyChat(id, item string, q *chatQuiz, r api.ChatQReplied) {
	if !q.sent.CompareAndSwap(false, true) {
		return
	}
	go func() {
		err := u.client.ChatReply(u.ctx, id, item, r)
		if err != nil {
			q.sent.Store(false)
		}
		u.chatFailed(err)
	}()
}
