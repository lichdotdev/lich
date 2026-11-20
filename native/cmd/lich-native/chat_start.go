package main

import (
	"fmt"
	"hash/fnv"
	"image"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const (
	// chatStartRise is how far up the start screen's block sits: the share of
	// the room it leaves that goes above it, under half so it stands a little
	// over the middle.
	chatStartRise = 0.42
	// chatStartRun is how long the composer takes to reach the foot once the
	// first item comes, and chatStartFade how long the logo and the question
	// take to go: the chat's own move and exit.
	chatStartRun  = 220 * time.Millisecond
	chatStartFade = 120 * time.Millisecond
	// chatStartQuestionSize is the question's size, off the type scale: the
	// centre-mix mockup's 17px, a step over the body so it reads on the field.
	chatStartQuestionSize unit.Sp = 17
)

// chatStartLift is what the question keeps over the composer on top of the
// composer's own chatComposerGap: 36dp between them, the centre-mix mockup's.
const chatStartLift unit.Dp = 12

// chatStartGap is the room between the logo and the question.
var chatStartGap = ui.Space(4)

// chatStartSep splits the catalog's list of questions, which each language
// writes at its own length.
const chatStartSep = "|"

// chatStart is the start screen of a chat with nothing in it: whether the
// conversation was empty the frame before, and when its first item came.
type chatStart struct {
	empty bool
	left  time.Time
	// mark is where the mark's slot lay the frame before, from the chat's
	// top-left, and markFade how far it showed: the backdrop paints the mark
	// there.
	mark     image.Point
	markFade float32
	// centre is the middle of the start screen's block, where the field
	// calms down, and quiet the composer and the line under it, where it
	// goes out so the project and the branch read on any part of it.
	centre image.Point
	quiet  image.Rectangle
}

// shows reports whether the start screen is drawn at now: while the
// conversation is empty, and over the run that takes it away once it is not.
func (s *chatStart) shows(now time.Time, empty bool) bool {
	if s.empty && !empty {
		s.left = now
	}
	s.empty = empty
	return empty || (!s.left.IsZero() && now.Sub(s.left) < chatStartRun)
}

// emptyNow is whether session sv's chat has nothing in it: read and empty,
// or never given a conversation at all, which a new session is before its
// chat even opens, so its start screen is up from the first frame. The
// conversation a new session's chat opens is bound before the window reads
// it, and is empty too: a start screen already up stays until the read.
func (s *chatStart) emptyNow(read bool, sv sessionView, items []backend.ChatItem) bool {
	if !read {
		return sv.ProviderSessionID == "" || s.empty
	}
	return len(items) == 0
}

// chatStartSlot is the room the mark takes over the question. The mark is
// not drawn here: the pane's backdrop paints it in its own dots, where the
// start screen says it lies (chatStart.mark).
func chatStartSlot(gtx C) D {
	return D{Size: chatStartMarkSize.Mul(max(1, gtx.Dp(backdropCell)))}
}

// chatStartQuestion is the question the start screen asks, picked by session
// from the language's own list, as the turn's verb is, around the project's
// name.
func chatStartQuestion(session string) (before, after string) {
	const marker = "\x00"
	lines := strings.Split(i18n.T("chat.start.lines", i18n.Args{"project": marker}), chatStartSep)
	h := fnv.New32a()
	fmt.Fprint(h, session)
	before, after, _ = strings.Cut(lines[int(h.Sum32()%uint32(len(lines)))], marker)
	return before, after
}

// chatStartHero is the logo over the question, the project's name in mono.
func chatStartHero(gtx C, th *ui.Theme, session, project string) D {
	before, after := chatStartQuestion(session)
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(chatStartSlot),
		layout.Rigid(layout.Spacer{Height: chatStartGap}.Layout),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(th.Text(chatStartQuestionSize, before).Weight(font.SemiBold).Layout),
				layout.Rigid(th.Text(chatStartQuestionSize, project).Mono().Weight(font.SemiBold).Layout),
				layout.Rigid(th.Text(chatStartQuestionSize, after).Weight(font.SemiBold).Layout),
			)
		}),
		layout.Rigid(layout.Spacer{Height: chatStartLift}.Layout),
	)
}

// chatStartScreen draws a chat with nothing in it: the logo and the question
// over the composer, the block a little above the middle of the pane. When
// the first item comes the logo and the question fade and the composer runs
// down to the foot while the conversation grows over it. A pane too low for
// the block keeps the composer at its foot, with neither.
func (u *window) chatStartScreen(gtx C, th *ui.Theme, p *chatPane, sv sessionView, list, composer layout.Widget) {
	w, h := gtx.Constraints.Max.X, gtx.Constraints.Max.Y
	free := gtx
	free.Constraints.Min = image.Point{}
	m := op.Record(gtx.Ops)
	cd := composer(free)
	composed := m.Stop()
	m = op.Record(gtx.Ops)
	hd := chatStartHero(free, th, sv.ID, u.model.projectName(sv.projectID))
	hero := m.Stop()
	room := h - hd.Size.Y - cd.Size.Y
	below := room - int(float32(room)*chatStartRise)
	shown := room > 0
	if !shown {
		below = 0
	}
	fade := float32(1)
	if !p.start.empty {
		below = int(float32(below) * (1 - chatPhase(gtx, p.start.left, 0, chatStartRun, paneEaseOut)))
		fade = 1 - chatPhase(gtx, p.start.left, 0, chatStartFade, paneEaseIn)
		lg := gtx
		lg.Constraints = layout.Constraints{Max: image.Pt(w, h-cd.Size.Y-below)}
		list(lg)
	}
	top := h - cd.Size.Y - below
	heroAt := image.Pt((w-hd.Size.X)/2, top-hd.Size.Y)
	p.start.mark = heroAt.Add(image.Pt((hd.Size.X-chatStartSlot(gtx).Size.X)/2, 0))
	p.start.centre = image.Pt(w/2, top-hd.Size.Y+(hd.Size.Y+cd.Size.Y)/2)
	column := max(min(w-2*gtx.Dp(chatPadX), gtx.Dp(chatBoxMax)), 0)
	p.start.quiet = image.Rect((w-column)/2, top, (w+column)/2, top+cd.Size.Y)
	p.start.markFade = 0
	if shown {
		p.start.markFade = fade
	}
	if shown && fade > 0 {
		off := op.Offset(heroAt).Push(gtx.Ops)
		chatStartFaded(gtx, hero, fade)
		off.Pop()
	}
	off := op.Offset(image.Pt(0, top)).Push(gtx.Ops)
	composed.Add(gtx.Ops)
	off.Pop()
}

// chatStartFaded adds call at opacity a, with no layer when it is whole.
func chatStartFaded(gtx C, call op.CallOp, a float32) {
	if a < 1 {
		defer paint.PushOpacity(gtx.Ops, a).Pop()
	}
	call.Add(gtx.Ops)
}
