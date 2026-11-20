package main

import (
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// chatStartLogo is lich's mark in pixel art, the still frame of the Welcome
// screen's animated one: 48x68, row 0 on top, 1 the darkest tone to 5 the
// lightest, . nothing.
var chatStartLogo = [...]string{
	"................................................",
	"................................................",
	"................................................",
	"....................55..........................",
	"...................5555.........................",
	"...................5554.........................",
	"..................55555.........................",
	"..................45554.........................",
	".................455555.........................",
	".................555554.........................",
	".................4555554........................",
	".................45555544.......................",
	".................45555554.......................",
	".................455555544......................",
	".................4555555544.....................",
	".................45555554544....................",
	".................454555555544...................",
	".................4445555554444..................",
	"..................4445555554444.................",
	"..................44445555554444................",
	"...................4445555555444................",
	"....................4445555555444...............",
	"....................43445455544443..............",
	".....................3344445544443..............",
	"....55................3344545554433.......55....",
	"...5555................334444554443......5554...",
	"...5555.................334445544433.....5554...",
	"..4555...................33444554443......5544..",
	"..4555.........33.........33444544433.....5543..",
	".33455.......3333333......33344544343.....44433.",
	".34444......334333433......3334444433.....44433.",
	".33444.....33334443333......3344444433....34433.",
	".33444....333344433333......3334444343....33433.",
	"333433....33344433233.......3334444433....334323",
	"333433....334443323..........234444443....334333",
	"233433....33343322...........234444433....233333",
	"334333....3344433............233444443....334332",
	"233333....3334332...........2233443433....233332",
	"2333323...3343332...........2333444333...2233332",
	"2333232...2333332...........2333443323...2333332",
	"2333332...32333322.........2233443333....2333332",
	"2223332....23333222.......22233433332...22233322",
	".2333222...222333222.....222233332322...2233322.",
	".2233222....222332222222222233333222....2233322.",
	".22233222....2223322222222233333322....22233222.",
	".22233222....2222333333333333332222....22233222.",
	"..22233222....22223333333333332222....22233222..",
	"..222332222....222223333333332222....222233222..",
	"...222332222....2222233333322222....222233222...",
	"...2122332221.....1222233222121....2122333221...",
	"....2223322221.....2212222222.....2222333222....",
	"....12222322221......212221......21223232212....",
	".....12222322221.......121......21223232212.....",
	"......112222222111............111222232211......",
	".......112222222212..........122223222211.......",
	"........1112222222111......1112222222111........",
	"..........112222222211....112222222211..........",
	"...........111222222211..112222222111...........",
	".............1112222221..1222222111.............",
	"...............111222211112222111...............",
	".................11222222222211.................",
	"..................111212221111..................",
	"....................11222111....................",
	".....................112111.....................",
	"......................1111......................",
	".......................11.......................",
	"................................................",
	"................................................",
}

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
	// chatStartMid is how far the logo's middle tone goes from the muted
	// foreground toward the foreground, and chatStartShade and chatStartDeep
	// how far its two dark ones go toward the background.
	chatStartMid   = 0.5
	chatStartShade = 0.35
	chatStartDeep  = 0.65
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

// chatStartColor is the color of a logo symbol, from the theme's own tokens
// so it follows a light theme and a dark one; false for nothing.
func chatStartColor(th *ui.Theme, symbol byte) (color.NRGBA, bool) {
	switch symbol {
	case '5':
		return th.Foreground, true
	case '4':
		return ui.MixOKLab(th.MutedForeground, th.Foreground, chatStartMid), true
	case '3':
		return th.MutedForeground, true
	case '2':
		return ui.MixOKLab(th.MutedForeground, th.Background, chatStartShade), true
	case '1':
		return ui.MixOKLab(th.MutedForeground, th.Background, chatStartDeep), true
	}
	return color.NRGBA{}, false
}

// chatStartMark draws the logo, a run of one tone at a time.
func chatStartMark(gtx C, th *ui.Theme) D {
	px := max(gtx.Dp(chatStartPixel), 1)
	for y, row := range chatStartLogo {
		for x := 0; x < len(row); {
			run := 1
			for x+run < len(row) && row[x+run] == row[x] {
				run++
			}
			if c, ok := chatStartColor(th, row[x]); ok {
				paint.FillShape(gtx.Ops, c, clip.Rect(image.Rect(x*px, y*px, (x+run)*px, (y+1)*px)).Op())
			}
			x += run
		}
	}
	return D{Size: image.Pt(len(chatStartLogo[0])*px, len(chatStartLogo)*px)}
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
