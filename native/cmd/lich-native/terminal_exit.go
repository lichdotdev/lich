package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

var icExitRestart = icons.Lucide("rotate-cw")

// sessionExit is internal/terminal's exitEvent: the status the PTY child left,
// nil when it left none (killed by a signal), which is not a clean exit.
type sessionExit struct {
	Code *int `json:"code,omitempty"`
}

func readSessionExit(data json.RawMessage) sessionExit {
	var e sessionExit
	if json.Unmarshal(data, &e) != nil {
		return sessionExit{}
	}
	return e
}

// exitNotice is session-exit.ts exitNotice: a failing code is named, for it
// tells a crash from a session the user ended.
func exitNotice(e sessionExit) string {
	if e.Code == nil || *e.Code == 0 {
		return i18n.T("terminal.sessionExit.ended")
	}
	return i18n.T("terminal.sessionExit.endedWithCode", i18n.Args{"code": *e.Code})
}

// exitMarker is session-exit.ts exitMarker: the scrollback line where the
// process died, which outlives the band after a restart.
func exitMarker(e sessionExit) string {
	if e.Code == nil {
		return "\r\n[process exited]\r\n"
	}
	return fmt.Sprintf("\r\n[process exited: %d]\r\n", *e.Code)
}

// exited marks session id's terminal as holding a process that is gone. A
// session never shown here has no terminal: showing it starts a new process.
func (ts *terminals) exited(id string, e sessionExit) {
	ts.mu.Lock()
	t := ts.byID[id]
	if t != nil {
		t.exit = &e
	}
	ts.mu.Unlock()
	if t == nil {
		return
	}
	t.vt.Write([]byte(exitMarker(e)))
	ts.invalidate()
}

// exitOf is the exit session id's terminal is showing, if any.
func (ts *terminals) exitOf(id string) (sessionExit, bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	t := ts.byID[id]
	if t == nil || t.exit == nil {
		return sessionExit{}, false
	}
	return *t.exit, true
}

// restart is TerminalView.tsx restart: the same kind in the same directory,
// as a conversation of its own, into the same terminal and its scrollback.
func (ts *terminals) restart(ctx context.Context, s sessionView) {
	ts.mu.Lock()
	t := ts.byID[s.ID]
	ts.mu.Unlock()
	if t == nil {
		return
	}
	cols, rows := t.vt.Size()
	if err := ts.client.Start(ctx, s.ID, s.projectID, s.path, s.Kind, "", cols, rows); err != nil {
		ts.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("terminal.view.restartFailed", i18n.Args{"error": lichclient.ErrorText(err)})})
		return
	}
	ts.mu.Lock()
	t.exit = nil
	ts.mu.Unlock()
	ts.invalidate()
}

// exitBand is TerminalExitBanner: anchored to the bottom of the terminal
// rather than raised as a dialog, so the scrollback that says why the process
// is gone stays readable.
type exitBand struct {
	restart, close widget.Clickable
}

// layout draws the band at the bottom of gtx's area and reports which button
// was clicked this frame.
func (b *exitBand) layout(gtx C, th *ui.Theme, e sessionExit) (restart, closing bool) {
	restart, closing = b.restart.Clicked(gtx), b.close.Clicked(gtx)
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, 0)
	dims := b.content(gtx, th, e)
	call := macro.Stop()
	defer op.Offset(image.Pt(0, gtx.Constraints.Max.Y-dims.Size.Y)).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, th.Card, clip.Rect{Max: dims.Size}.Op())
	call.Add(gtx.Ops)
	th.Separator(gtx)
	return restart, closing
}

func (b *exitBand) content(gtx C, th *ui.Theme, e sessionExit) D {
	on := th.On(th.Card)
	return layout.Inset{Top: ui.Space(2), Bottom: ui.Space(2), Left: ui.Space(3), Right: ui.Space(3)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(on.Text(ui.TextXS, exitNotice(e)).Muted().Layout),
			layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
			layout.Rigid(func(gtx C) D {
				btn := on.Button(&b.restart, i18n.T("terminal.exitBanner.restart"))
				btn.Icon, btn.Variant, btn.Size = icExitRestart, ui.VariantGhost, ui.SizeXS
				return btn.Layout(gtx)
			}),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D {
				btn := on.Button(&b.close, i18n.T("terminal.exitBanner.close"))
				btn.Variant, btn.Size = ui.VariantDestructive, ui.SizeXS
				return btn.Layout(gtx)
			}),
		)
	})
}
