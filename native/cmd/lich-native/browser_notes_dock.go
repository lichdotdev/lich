package main

import (
	"context"
	"image"
	"image/color"
	"log"
	"strconv"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The notes bar: Select and Draw, floating over the page's bottom where the
// person looks, each with its name. Alone it is a pill at the center; with a
// note or a sketch waiting it is the head of the tray (browser_notes_view.go).
// Its arrow folds it into a tab on the page's edge, for a page it covers.

var (
	icNotesFold   = icons.Lucide("chevron-down")
	icNotesUnfold = icons.Lucide("chevron-up")
)

var (
	notesPillPad = ui.Space(1)
	// notesTab is the folded bar: wide enough to hit, low enough to cover
	// nothing of the page.
	notesTabWidth  = unit.Dp(44)
	notesTabHeight = unit.Dp(16)
	// inkDot is a pencil color in the bar, inkCell the room it is clicked in,
	// and inkOutline the ring of the chosen one and its distance from the dot.
	inkDot     = ui.Space(3)
	inkCell    = ui.Space(5)
	inkOutline = unit.Dp(2)
)

// notesFoldedKey is whether the bar is folded, a global setting.
const notesFoldedKey = "lich.browser.notesFolded"

// notesBarPrefs is what the bar keeps across pages: folded, stored, and the
// pencil's color, for as long as the window lives.
type notesBarPrefs struct {
	folded bool
	ink    string
}

func (b notesBarPrefs) inkName() string {
	if b.ink == "" {
		return defaultInk
	}
	return b.ink
}

func loadNotesFolded(ctx context.Context, c *backend.Client) (bool, error) {
	raw, err := c.GetSetting(ctx, notesFoldedKey, "")
	return raw == "true", err
}

func (u *window) setNotesFolded(folded bool) {
	u.notesBar.folded = folded
	go func() {
		if err := u.client.SetSetting(u.ctx, notesFoldedKey, "", strconv.FormatBool(folded)); err != nil {
			log.Printf("browser: save the notes bar: %v", err)
		}
	}()
}

// notesDock draws the bar in the shape the moment asks for: the tray while
// something waits, the pill while a tool is on or the bar is unfolded, and
// the tab otherwise.
func (u *window) notesDock(gtx C, p *browserPaneUI, session string) {
	n := &p.notes
	u.notesBarClicks(gtx, p)
	switch {
	case n.pending():
		u.notesTray(gtx, p, session)
	case n.picking || n.drawing || !u.notesBar.folded:
		n.trayShown = time.Time{}
		n.docked = u.notesChip(gtx, n, gtx.Dp(gutter), paneRadius, func(gtx C) D { return u.notesPill(gtx, n) })
	default:
		n.trayShown, n.docked = time.Time{}, 0
		u.notesChip(gtx, n, 0, ui.RadiusLG, func(gtx C) D {
			return u.spot(gtx, tourBrowserPick, func(gtx C) D { return u.notesTab(gtx, n) })
		})
	}
}

// notesBarClicks takes the bar's clicks before it is drawn: a tool turns on,
// or off when it is the one on, and the arrow folds the bar.
func (u *window) notesBarClicks(gtx C, p *browserPaneUI) {
	n := &p.notes
	toggled := false
	if n.selectTool.Clicked(gtx) {
		n.setPicking(!n.picking)
		toggled = true
	}
	if n.drawTool.Clicked(gtx) {
		n.setDrawing(!n.drawing)
		toggled = true
	}
	for i := range n.inks {
		if n.inks[i].Clicked(gtx) {
			u.notesBar.ink, n.erasing = cardColorNames[i], false
		}
	}
	if n.eraser.Click.Clicked(gtx) {
		n.erasing = !n.erasing
	}
	if n.fold.Click.Clicked(gtx) {
		n.setPicking(false)
		n.setDrawing(false)
		u.setNotesFolded(true)
	}
	if n.unfold.Clicked(gtx) {
		u.setNotesFolded(false)
	}
	if toggled {
		// The tool's keys (Esc, Ctrl+Z) are the page's.
		gtx.Execute(key.FocusCmd{Tag: &p.tag})
	}
}

// notesChip draws body on the bar's surface, centered at the page's bottom
// and lift above it, and takes the pointer from the page under it; with no
// lift it stands on the page's edge, square at the bottom. It reports how much
// of the page's bottom it covers.
func (u *window) notesChip(gtx C, n *browserNotes, lift int, radius unit.Dp, body layout.Widget) int {
	th := u.theme
	page := gtx.Constraints.Max
	macro := op.Record(gtx.Ops)
	inner := gtx
	inner.Constraints.Min = image.Point{}
	d := body(inner)
	call := macro.Stop()
	defer op.Offset(image.Pt((page.X-d.Size.X)/2, page.Y-d.Size.Y-lift)).Push(gtx.Ops).Pop()
	r := gtx.Dp(radius)
	shape := clip.RRect{Rect: image.Rectangle{Max: d.Size}, NW: r, NE: r, SE: r, SW: r}
	if lift == 0 {
		shape.SE, shape.SW = 0, 0
	}
	paint.FillShape(gtx.Ops, th.Background, shape.Op(gtx.Ops))
	area := shape.Push(gtx.Ops)
	event.Op(gtx.Ops, &n.dock)
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &n.dock, Kinds: pointer.Press | pointer.Release | pointer.Move | pointer.Drag | pointer.Scroll, ScrollY: noteScrollRange}); !ok {
			break
		}
	}
	call.Add(gtx.Ops)
	area.Pop()
	paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
	return d.Size.Y + lift
}

// notesPill is the bar with nothing waiting: the tools, what the one that is
// on asks for, and the arrow that folds it.
func (u *window) notesPill(gtx C, n *browserNotes) D {
	th := u.theme
	return layout.UniformInset(notesPillPad).Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return u.noteTools(gtx, n) }),
			layout.Rigid(func(gtx C) D {
				if !n.picking {
					return D{}
				}
				return layout.Inset{Left: ui.Space(2.5), Right: ui.Space(1.5)}.Layout(gtx, th.Text(ui.TextXS, i18n.T("layouts.browser.pickHint")).In(notePick).Layout)
			}),
			ui.Gap(1),
			layout.Rigid(th.IconAction(&u.root, &n.fold, icNotesFold, i18n.T("layouts.browser.foldNotes"), false).Layout),
		)
	})
}

// noteTools is Select and Draw, the one that is on filled, and the pencil's
// colors beside them while it draws.
func (u *window) noteTools(gtx C, n *browserNotes) D {
	tool := func(click *widget.Clickable, ic *icons.Icon, label string, on bool) layout.Widget {
		b := u.theme.Button(click, label)
		b.Icon, b.Size, b.Variant = ic, ui.SizeSM, ui.VariantGhost
		if on {
			b.Variant = ui.VariantSecondary
		}
		return b.Layout
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return u.spot(gtx, tourBrowserPick, tool(&n.selectTool, icBrowserPick, i18n.T("layouts.browser.toolSelect"), n.picking))
		}),
		ui.Gap(0.5),
		layout.Rigid(tool(&n.drawTool, icBrowserDraw, i18n.T("layouts.browser.toolDraw"), n.drawing)),
		layout.Rigid(func(gtx C) D {
			if !n.drawing {
				return D{}
			}
			return layout.Inset{Left: ui.Space(1.5)}.Layout(gtx, func(gtx C) D { return u.inkSwatches(gtx, n) })
		}),
	)
}

// inkSwatches are the pencil's colors, the card palette, the chosen one
// ringed, and the eraser after them, which takes the ring's place while on.
func (u *window) inkSwatches(gtx C, n *browserNotes) D {
	cells := make([]layout.FlexChild, len(cardColorNames), len(cardColorNames)+2)
	for i, name := range cardColorNames {
		cells[i] = layout.Rigid(func(gtx C) D {
			return n.inks[i].Layout(gtx, func(gtx C) D {
				return inkSwatch(gtx, u.theme, cardColors[name], !n.erasing && name == u.notesBar.inkName())
			})
		})
	}
	cells = append(cells, ui.Gap(0.5),
		layout.Rigid(u.theme.IconAction(&u.root, &n.eraser, icBrowserErase, i18n.T("layouts.browser.eraser"), n.erasing).Layout))
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, cells...)
}

func inkSwatch(gtx C, th *ui.Theme, c color.NRGBA, chosen bool) D {
	side := gtx.Dp(inkCell)
	dot, center := gtx.Dp(inkDot), image.Pt(side/2, side/2)
	circle := func(radius int) clip.Ellipse {
		return clip.Ellipse{Min: center.Sub(image.Pt(radius, radius)), Max: center.Add(image.Pt(radius, radius))}
	}
	paint.FillShape(gtx.Ops, c, circle(dot/2).Op(gtx.Ops))
	if chosen {
		gap := gtx.Dp(inkOutline)
		ring := circle(dot/2 + gap + gap/2)
		paint.FillShape(gtx.Ops, th.Foreground, clip.Stroke{Path: ring.Path(gtx.Ops), Width: float32(gap)}.Op())
	}
	return D{Size: image.Pt(side, side)}
}

// notesTab is the folded bar: a click unfolds it.
func (u *window) notesTab(gtx C, n *browserNotes) D {
	return n.unfold.Layout(gtx, func(gtx C) D {
		size := image.Pt(gtx.Dp(notesTabWidth), gtx.Dp(notesTabHeight))
		gtx.Constraints = layout.Exact(size)
		fg := u.theme.MutedForeground
		if n.unfold.Hovered() {
			fg = u.theme.Foreground
		}
		return ui.CenteredIcon(gtx, icNotesUnfold, ui.Space(3.5), fg)
	})
}
