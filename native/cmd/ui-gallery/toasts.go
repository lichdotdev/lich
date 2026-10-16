package main

import (
	"flag"
	"fmt"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

var (
	flagToasts         = flag.Bool("toasts", false, "raise the demo toasts at start, for -shot")
	flagToastsExpanded = flag.Bool("toasts-expanded", false, "draw the toast stack expanded, as a hover would, for -shot")
)

// toastDemoCount is len(toastDemos), which an array length cannot read.
const toastDemoCount = 9

// toastDemos are the gallery's toast buttons: one per variant, then the
// buttons and description, then an update in place.
var toastDemos = []struct {
	label string
	toast ui.Toast
}{
	{"Default", ui.Toast{Title: "Session closed"}},
	{"Success", ui.Toast{Variant: ui.ToastSuccess, Title: "Branch pushed"}},
	{"Info", ui.Toast{Variant: ui.ToastInfo, Title: "No room to show it beside"}},
	{"Warning", ui.Toast{Variant: ui.ToastWarning, Title: "lich quit unexpectedly last time",
		Description: "Sessions that were running were restored.", Duration: ui.ToastForever,
		Cancel: &ui.ToastButton{Label: "Dismiss"}, Action: &ui.ToastButton{Label: "Report"}}},
	{"Error", ui.Toast{Variant: ui.ToastError, Title: "Could not open the folder: no file manager found"}},
	{"With action", ui.Toast{Title: "Session closed: Claude 3", Action: &ui.ToastButton{Label: "Undo"}}},
	{"Progress", ui.Toast{Title: "Downloading lich 1.4.0", Description: "30% · 12.3 MB of 41.0 MB",
		Progress: &ui.ToastProgress{Percent: 30}, Duration: ui.ToastForever}},
	{"Spinner", ui.Toast{Title: "Installing lich 1.4.0…", Description: "lich closes and reopens on its own.",
		Icon: icons.Lucide("loader-circle"), Spin: true, Duration: ui.ToastForever}},
	{"Same id", ui.Toast{ID: "gallery-progress", Variant: ui.ToastInfo, Title: "Replaced in place"}},
}

// toastsSection raises toasts on the gallery's Toaster.
func (g *gallery) toastsSection(gtx C, th *ui.Theme) D {
	var buttons []layout.Widget
	for i, demo := range toastDemos {
		if g.toastButtons[i].Clicked(gtx) {
			g.raise(demo.toast)
		}
		b := th.Button(&g.toastButtons[i], demo.label)
		b.Variant = ui.VariantOutline
		buttons = append(buttons, b.Layout)
	}
	return stack(4,
		caption(th, "Toaster: hover the stack to expand it, drag a toast right or up to dismiss it"),
		row(2, buttons[:4]...),
		row(2, buttons[4:]...),
	)(gtx)
}

// raise shows t, numbering repeats so a stack of the same demo reads apart.
func (g *gallery) raise(t ui.Toast) {
	g.toastCount++
	if t.ID == "" {
		t.Title = fmt.Sprintf("%s (%d)", t.Title, g.toastCount)
	}
	if t.Action != nil {
		t.Action = &ui.ToastButton{Label: t.Action.Label, OnClick: func() { g.lastAction = t.Action.Label }}
	}
	g.toaster.Show(t)
}

// raiseDemoToasts is -toasts: every demo but the update, held up for the
// shot.
func (g *gallery) raiseDemoToasts() {
	for _, demo := range toastDemos[:len(toastDemos)-1] {
		t := demo.toast
		t.Duration = ui.ToastForever
		g.raise(t)
	}
}

// toastState is the gallery's share of the toast demo.
type toastState struct {
	toaster      ui.Toaster
	toastButtons [toastDemoCount]widget.Clickable
	toastCount   int
}
