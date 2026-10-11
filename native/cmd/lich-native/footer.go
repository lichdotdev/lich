package main

import (
	"context"
	"image"
	"log"
	"runtime"
	"strconv"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n.

var (
	icAttach   = icons.Lucide("paperclip")
	icCode     = icons.Lucide("code")
	icDiff     = icons.Lucide("diff")
	icFileText = icons.Lucide("file-text")
	icFolder   = icons.Lucide("folder")
	icTimer    = icons.Lucide("timer")
)

// footer is FooterBar: the status strip under the stage, following the
// active session, laid out as the user arranged it.
type footer struct {
	ctx        context.Context
	client     *lichclient.Client
	root       *ui.Root
	invalidate func()
	usage      *footerUsage
	quota      *planQuota
	handsOn    *handsOn
	cost       *costGate

	mu     sync.Mutex
	layout footerLayout
	saving bool // a layout change is being written

	watching string // the session whose plan is polled
	unwatch  func()
	widgets  map[string]*footerWidgets
	menu     ui.MenuState
	editor   footerEditor
	// reviewing is Review changes pressed: the stage shows the checkout's
	// diff instead of the terminal, where the web opens its dock's Review tab.
	reviewing bool
	toaster   *ui.Toaster
}

func newFooter(ctx context.Context, client *lichclient.Client, root *ui.Root, usage *footerUsage, invalidate func()) *footer {
	return &footer{
		ctx: ctx, client: client, root: root, invalidate: invalidate, usage: usage,
		quota:   newPlanQuota(func(id string) ([]lichclient.QuotaPlan, error) { return client.QuotaPlans(ctx, id) }, invalidate),
		handsOn: newHandsOn(func(id string) (int64, error) { return client.HandsOn(ctx, id) }, invalidate),
		cost:    &costGate{get: client.GetSetting, set: client.SetSetting, onChange: invalidate},
		layout:  defaultFooterLayout(),
		widgets: map[string]*footerWidgets{},
	}
}

// start loads the cost flag, then the layout resolved against it, and starts
// the hands-on clock. A failed flag load leaves the readout off, as the web's
// gate does; a failed layout load is a backend that cannot answer at all.
func (f *footer) start() error {
	if err := f.cost.load(f.ctx); err != nil {
		log.Printf("load %s: %v", costReadoutKey, err)
	}
	l, err := loadFooterLayout(f.ctx, f.client, f.cost.on())
	if err != nil {
		return err
	}
	f.mu.Lock()
	f.layout = l
	f.mu.Unlock()
	go f.handsOn.run(f.ctx)
	return nil
}

func (f *footer) snapshot() (footerLayout, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.layout, f.saving
}

// holdsKeys reports whether the menu or the editor wants the keyboard.
func (f *footer) holdsKeys() bool { return f.menu.Opened() || f.editor.open }

// input reads what the strip shows: the active session, else the project's
// own checkout. It points the polls at what the layout draws.
func (f *footer) input(m *model, now time.Time) footerInput {
	in := footerInput{now: now, cost: f.cost.on()}
	if sv, ok := m.activeSession(); ok {
		in.sessionID, in.provider, in.path, in.host, in.pr = sv.ID, sv.agent, sv.shown, sv.host, sv.pr
		in.checkout, in.sandboxed = sv.path, sv.sandbox.Confined
		if in.provider == "" {
			in.provider = sv.Kind
		}
	} else if p, ok := m.project(); ok {
		in.path = p.Path
	}
	in.git, in.isGit = m.gitStatus(in.path)
	in.usage, in.hasUsage = f.usage.get(in.sessionID)
	f.follow(in)
	in.plans = f.quota.plans(in.sessionID)
	in.handsOn, in.handsOnErr = f.handsOn.read()
	return in
}

// follow polls plan and hands-on time only for a session whose footer shows
// them, as each web readout subscribes only while it is mounted.
func (f *footer) follow(in footerInput) {
	l, _ := f.snapshot()
	handsOnID := ""
	if l.has("handsOn") {
		handsOnID = in.sessionID
	}
	f.handsOn.follow(handsOnID)
	plan := ""
	if l.has("plan") && in.provider != "" {
		plan = in.sessionID
	}
	if plan == f.watching {
		return
	}
	if f.unwatch != nil {
		f.unwatch()
		f.unwatch = nil
	}
	f.watching = plan
	if plan != "" {
		f.unwatch = f.quota.watch(plan)
	}
}

// layoutBar draws the strip: flex-wrap justify-between gap-x-4 gap-y-1,
// min-h-11 py-1.5, the right side ml-auto justify-end. A right press on it
// opens the menu holding the layout editor.
func (f *footer) layoutBar(gtx C, th *ui.Theme, in footerInput) D {
	f.act(gtx, in)
	l, _ := f.snapshot()
	// The clock and every countdown read on the minute.
	gtx.Execute(op.InvalidateCmd{At: in.now.Add(untilNextMinute(in.now))})
	entries := []ui.MenuEntry{{
		Label: "Customize footer…",
		// The web edits the layout in Settings › Footer; this dialog stands
		// in for it until the native window has a Settings screen.
		OnClick: func() { f.editor.open = true },
	}}
	return th.ContextMenu(f.root, &f.menu, entries).Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, gtx.Dp(ui.Space(11)))
		return layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
			return ui.Wrap{GapX: ui.Space(4), GapY: ui.Space(1), Justify: ui.JustifyBetween}.Layout(gtx,
				ui.WrapItem(f.side(th, footerSlots(l.Left, in), in, ui.JustifyStart)),
				ui.WrapMLAuto(f.side(th, footerSlots(l.Right, in), in, ui.JustifyEnd)),
			)
		})
	})
}

func (f *footer) side(th *ui.Theme, slots []footerSlot, in footerInput, justify ui.Justify) layout.Widget {
	return func(gtx C) D {
		kids := make([]ui.WrapChild, len(slots))
		for i, s := range slots {
			kids[i] = ui.WrapItem(func(gtx C) D { return f.slot(gtx, th, s, in) })
		}
		return ui.Wrap{GapX: ui.Space(1), GapY: ui.Space(1), Justify: justify}.Layout(gtx, kids...)
	}
}

// act carries out the clicks of the frame, before the strip is laid out.
func (f *footer) act(gtx C, in footerInput) {
	if f.w("attach").click.Clicked(gtx) && in.sessionID != "" {
		go f.attach(in)
	}
	// ponytail: Browse code toggles the web's right dock, which this window
	// does not have yet; the dock is what gives it a target and its pressed
	// fill.
	f.w("files").click.Clicked(gtx)
	if f.w("changes").click.Clicked(gtx) {
		f.reviewing = !f.reviewing
	}
	f.w("changes").pressed = f.reviewing
	if f.w("pr").click.Clicked(gtx) && in.pr != nil {
		// The web opens its Pulls screen; this window has none, so the PR
		// opens in the browser as the card's PR chip does.
		url := in.pr.URL
		go func() {
			if err := f.client.OpenExternal(f.ctx, url); err != nil {
				log.Printf("open pull request: %v", err)
			}
		}()
	}
}

// attach picks a file for the session and pastes its path at the prompt.
// A failure is toasted in the backend's own words, which name the ceiling a
// file was refused for.
func (f *footer) attach(in footerInput) {
	if err := attachFile(f.ctx, f.client, in.sessionID, in.checkout, in.sandboxed, runtime.GOOS == "windows"); err != nil {
		f.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: lichclient.ErrorText(err)})
	}
}

func (f *footer) w(id string) *footerWidgets {
	w, ok := f.widgets[id]
	if !ok {
		w = new(footerWidgets)
		f.widgets[id] = w
	}
	return w
}

// slot draws one item, FooterBar's controls and FooterSession's readings.
func (f *footer) slot(gtx C, th *ui.Theme, s footerSlot, in footerInput) D {
	w := f.w(s.ID)
	switch s.ID {
	case "attach":
		return footerButton(gtx, th, f.root, w, "Attach file", false, in.sessionID == "", iconOnly(icAttach))
	case "files":
		return footerButton(gtx, th, f.root, w, "Browse code", false, false, iconOnly(icCode))
	case "changes":
		return footerButton(gtx, th, f.root, w, "Review changes", true, false, changesContent(in.git))
	case "pr":
		return footerButton(gtx, th, f.root, w, "View pull request", true, false, glyphText(icPR, "PR #"+strconv.Itoa(in.pr.Number)))
	case "checkout", "path":
		ic := icFolder
		if s.ID == "checkout" && in.git.Branch != "" {
			ic = icBranch
		}
		glyph := func(gtx C) D { return ic.Layout(gtx, footerGlyph, th.MutedForeground) }
		return footerSpan(gtx, th, f.root, &w.tip, s.Hint, checkoutMaxWidth, glyph, s.Text)
	case "model":
		glyph := func(gtx C) D { return providerIcon(in.provider).Layout(gtx, 14, th.MutedForeground) }
		return footerSpan(gtx, th, f.root, &w.tip, s.Hint, modelMaxWidth, glyph, s.Text)
	case "clock":
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, th.Text(ui.TextXS, s.Text).Muted().Layout)
	}
	return f.readout(gtx, th, w, s, in)
}

// readout draws FooterSession's tooltip readings.
func (f *footer) readout(gtx C, th *ui.Theme, w *footerWidgets, s footerSlot, in footerInput) D {
	r := s.Readout
	pop := th.On(th.Popover)
	switch s.ID {
	case "context":
		detail := para(pop.Text(ui.TextXS, r.Detail[0]).Muted().Mono()).Layout
		return footerReadoutView(gtx, th, f.root, w, r, detail, ringContent(in.usage.Percent, r.Text))
	case "plan":
		return footerReadoutView(gtx, th, f.root, w, r, planDetails(pop, s.Plan, in.now), glyphText(icGauge, r.Text))
	case "handsOn":
		content := glyphText(icTimer, r.Text)
		if in.handsOnErr != nil {
			content = footerText(r.Text)
		}
		return footerReadoutView(gtx, th, f.root, w, r, nil, content)
	}
	return footerReadoutView(gtx, th, f.root, w, r, nil, footerText(r.Text))
}
