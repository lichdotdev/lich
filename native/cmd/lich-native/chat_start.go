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

	"github.com/lichdotdev/lich/native/grimoire"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const (
	// chatStartPixel is one logo pixel's side, a whole number of device pixels
	// at the scales a window runs at, as the flame's is.
	chatStartPixel unit.Dp = 1.5
	// chatStartRise is how far up the start screen's block sits: the share of
	// the room it leaves that goes above it, under half so it stands a little
	// over the middle.
	chatStartRise = 0.42
	// chatStartRun is how long the composer takes to reach the foot once the
	// first item comes, and chatStartFade how long the logo and the question
	// take to go: the chat's own move and exit.
	chatStartRun  = 220 * time.Millisecond
	chatStartFade = 120 * time.Millisecond
)

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

// chatStartMark draws the logo's flames at the frame of gtx.Now, the
// Welcome's, and asks for the next one.
func chatStartMark(gtx C, th *ui.Theme) D {
	frame, next := grimoire.LogoFrame(gtx.Now)
	// A window without focus keeps the frame it has and looks in less often.
	if th.Still {
		next = gtx.Now.Add(ui.StillFrame)
	}
	gtx.Execute(op.InvalidateCmd{At: next})
	px := max(gtx.Dp(chatStartPixel), 1)
	paintGrid(gtx, th, grimoire.Logo[frame], px)
	return D{Size: image.Pt(grimoire.LogoCols*px, grimoire.LogoRows*px)}
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
		layout.Rigid(func(gtx C) D { return chatStartMark(gtx, th) }),
		layout.Rigid(layout.Spacer{Height: chatStartGap}.Layout),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextBase, before).Weight(font.Medium).Layout),
				layout.Rigid(th.Text(ui.TextBase, project).Mono().Weight(font.Medium).Layout),
				layout.Rigid(th.Text(ui.TextBase, after).Weight(font.Medium).Layout),
			)
		}),
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
	if shown && fade > 0 {
		off := op.Offset(image.Pt((w-hd.Size.X)/2, top-hd.Size.Y)).Push(gtx.Ops)
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
