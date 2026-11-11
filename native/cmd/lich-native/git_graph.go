package main

import (
	"image"
	"image/color"
	"math"
	"slices"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

var (
	gitRowHeight  = ui.Space(6.5)
	gitLaneWidth  = unit.Dp(14)
	gitGraphInset = ui.Space(3)
	gitNodeRadius = unit.Dp(3)
)

// gitMaxLanes is how many lanes the graph column grows to: past it the
// message would have no room, so the outer lanes are clipped.
const gitMaxLanes = 12

// gitLink is a line of one row, at a lane and in a color index.
type gitLink struct{ lane, color int }

// gitGraphRow is what one commit's row draws: the lines coming down into its
// node, those leaving it toward its parents, and those passing straight by.
type gitGraphRow struct {
	lane, color       int
	up, down, through []gitLink
}

type gitLaneSlot struct {
	want  string // the commit this lane is waiting for, "" for a free lane
	color int
}

// layoutGitGraph assigns each commit a lane, children before parents as the
// log lists them. A first parent keeps its child's lane and color, so a branch
// reads as one line; two children of one commit hold a lane each until they
// meet at it. A parent not loaded yet just keeps its lane to the bottom edge.
func layoutGitGraph(commits []backend.GitCommit) ([]gitGraphRow, int) {
	var slots []gitLaneSlot
	nextColor, width := 0, 0
	freeLane := func() int {
		if i := slices.IndexFunc(slots, func(s gitLaneSlot) bool { return s.want == "" }); i >= 0 {
			return i
		}
		slots = append(slots, gitLaneSlot{})
		return len(slots) - 1
	}
	rows := make([]gitGraphRow, len(commits))
	for i, c := range commits {
		row := gitGraphRow{lane: -1}
		for l, s := range slots {
			switch s.want {
			case "":
			case c.Hash:
				if row.lane < 0 {
					row.lane, row.color = l, s.color
				}
				row.up = append(row.up, gitLink{l, s.color})
				slots[l].want = ""
			default:
				row.through = append(row.through, gitLink{l, s.color})
			}
		}
		if row.lane < 0 {
			row.lane, row.color = freeLane(), nextColor
			nextColor++
		}
		for p, parent := range c.Parents {
			link := gitLink{row.lane, row.color}
			if p > 0 {
				if l := slices.IndexFunc(slots, func(s gitLaneSlot) bool { return s.want == parent }); l >= 0 {
					link = gitLink{l, slots[l].color}
				} else {
					link = gitLink{freeLane(), nextColor}
					nextColor++
				}
			}
			slots[link.lane] = gitLaneSlot{parent, link.color}
			row.down = append(row.down, link)
		}
		width = max(width, len(slots))
		rows[i] = row
	}
	return rows, width
}

func gitGraphWidth(gtx C, lanes int) int {
	if lanes == 0 {
		return 0
	}
	return gtx.Dp(gitGraphInset) + min(lanes, gitMaxLanes)*gtx.Dp(gitLaneWidth)
}

// drawGitRow draws one row of the graph in a box of its column's width and the
// row's height.
func drawGitRow(gtx C, th *ui.Theme, r gitGraphRow, size image.Point) {
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	h := float32(size.Y)
	inset, lane := float32(gtx.Dp(gitGraphInset)), float32(gtx.Dp(gitLaneWidth))
	x := func(l int) float32 { return inset + float32(l)*lane }
	node := f32.Pt(x(r.lane), h/2)
	width := float32(gtx.Dp(sessionTreeStroke))
	tone := func(c int) color.NRGBA { return th.GraphLanes[c%len(th.GraphLanes)] }
	for _, l := range r.through {
		strokeGitLink(gtx, tone(l.color), f32.Pt(x(l.lane), 0), f32.Pt(x(l.lane), h), width)
	}
	for _, l := range r.up {
		strokeGitLink(gtx, tone(l.color), f32.Pt(x(l.lane), 0), node, width)
	}
	for _, l := range r.down {
		strokeGitLink(gtx, tone(l.color), node, f32.Pt(x(l.lane), h), width)
	}
	radius := float32(gtx.Dp(gitNodeRadius))
	ui.StrokeArc(gtx, node, radius/2, radius, 0, 2*math.Pi, tone(r.color))
}

func strokeGitLink(gtx C, c color.NRGBA, from, to f32.Point, width float32) {
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(from)
	if from.X == to.X {
		p.LineTo(to)
	} else {
		mid := (from.Y + to.Y) / 2
		p.CubeTo(f32.Pt(from.X, mid), f32.Pt(to.X, mid), to)
	}
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: width}.Op())
}

// gitRefLabel is the one ref a row names, how many more the commit carries,
// and the local branch a session could be on.
type gitRefLabel struct {
	text, branch string
	kind         string
	head         bool
	extra        int
}

// gitRefLabelOf picks what a row says of a commit's refs: the branch HEAD is
// on, else a local branch, a remote one, a tag. A local branch and its remote
// at the same commit read as one, the rest wait in the commit's detail.
func gitRefLabelOf(refs []backend.GitRef) (gitRefLabel, bool) {
	if len(refs) == 0 {
		return gitRefLabel{}, false
	}
	rank := func(r backend.GitRef) int {
		switch {
		case r.Head:
			return 0
		case r.Kind == backend.GitRefLocal:
			return 1
		case r.Kind == backend.GitRefRemote:
			return 2
		}
		return 3
	}
	best := refs[0]
	for _, r := range refs[1:] {
		if rank(r) < rank(best) {
			best = r
		}
	}
	label := gitRefLabel{text: best.Name, kind: best.Kind, head: best.Head, extra: len(refs) - 1}
	if best.Kind != backend.GitRefLocal {
		return label, true
	}
	label.branch = best.Name
	for _, r := range refs {
		if remote, ok := strings.CutSuffix(r.Name, "/"+best.Name); ok && r.Kind == backend.GitRefRemote {
			label.text, label.extra = best.Name+" & "+remote, label.extra-1
			break
		}
	}
	return label, true
}

// gitWhen is a commit's date as short as the row can tell it from now.
func gitWhen(unix int64, now time.Time) string {
	t := time.Unix(unix, 0).In(now.Location())
	switch {
	case t.Year() != now.Year():
		return t.Format("Jan 2, 2006")
	case t.YearDay() != now.YearDay():
		return t.Format("Jan 2")
	}
	return t.Format("15:04")
}
