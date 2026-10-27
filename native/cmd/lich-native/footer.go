package main

import (
	"context"
	"log"
	"runtime"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icAttach   = icons.Lucide("paperclip")
	icCode     = icons.Lucide("code")
	icDiff     = icons.Lucide("diff")
	icFileText = icons.Lucide("file-text")
	icFolder   = icons.Lucide("folder")
	icTimer    = icons.Lucide("timer")
)

// footer is the status strip under the stage, following the active session,
// laid out as the user arranged it.
type footer struct {
	ctx        context.Context
	client     *backend.Client
	root       *ui.Root
	invalidate func()
	// openPullRequest opens a pull request where prTarget says.
	openPullRequest func(number int, url string)
	// showDelegates lays a session's delegates out beside it.
	showDelegates func(id string)
	usage         *footerUsage
	quota         *planQuota
	handsOn       *handsOn
	cost          *costGate
	// writePaste is terminals.writePaste, which unfolds a long paste.
	writePaste func(ctx context.Context, id, block string) error

	mu     sync.Mutex
	layout footerLayout
	budget float64 // the spend ceiling the cost readout colours against
	saving bool    // a layout change is being written

	watching string // the session whose plan is polled
	unwatch  func()
	widgets  map[string]*footerWidgets
	menu     ui.MenuState
	editor   footerEditor
	// shown is what the strip read last frame, which the editor's chips and
	// preview read the model from.
	shown footerInput
	// customize opens Settings on the Footer row.
	customize func()
	// dock is what Browse code and Review changes open, on their tab.
	dock    *dock
	toaster *ui.Toaster
}

func newFooter(ctx context.Context, client *backend.Client, root *ui.Root, usage *footerUsage, quota *planQuota, invalidate func()) *footer {
	return &footer{
		ctx: ctx, client: client, root: root, invalidate: invalidate, usage: usage,
		quota:   quota,
		handsOn: newHandsOn(func(id string) (int64, error) { return client.HandsOn(ctx, id) }, invalidate),
		cost:    &costGate{get: client.GetSetting, set: client.SetSetting, onChange: invalidate},
		layout:  defaultFooterLayout(),
		widgets: map[string]*footerWidgets{},
	}
}

// start loads the cost flag, then the layout resolved against it, and starts
// the hands-on clock. A failed flag load leaves the readout off; a failed
// layout load is a backend that cannot answer at all.
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

func (f *footer) setBudget(budget float64) {
	f.mu.Lock()
	f.budget = budget
	f.mu.Unlock()
}

// holdsKeys reports whether the menu wants the keyboard.
func (f *footer) holdsKeys() bool { return f.menu.Opened() }

// input reads what the strip shows: the active session, else the project's
// own checkout. It points the polls at what the layout draws.
func (f *footer) input(m *model, now time.Time) footerInput {
	f.mu.Lock()
	budget := f.budget
	f.mu.Unlock()
	in := footerInput{now: now, cost: f.cost.on(), budget: budget}
	if sv, ok := m.activeSession(); ok {
		in.sessionID, in.provider, in.path, in.host, in.pr = sv.ID, sv.agent, sv.shown, sv.host, sv.pr
		in.checkout, in.sandboxed = sv.path, sv.sandbox.Confined
		if in.provider == "" {
			in.provider = sv.Kind
		}
		if m.layoutOf(sv.ID) == "" {
			in.delegates = len(m.delegatesOf(sv.ID))
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
// them.
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

// layoutBar draws the strip: wrapping items spread between the two sides, the
// right one pushed to the end. A right press on it opens the menu that leads
// to Settings › Appearance › Footer.
func (f *footer) layoutBar(gtx C, th *ui.Theme, in footerInput) D {
	f.act(gtx, in)
	f.shown = in
	l, _ := f.snapshot()
	// The clock and every countdown read on the minute.
	gtx.Execute(op.InvalidateCmd{At: in.now.Add(untilNextMinute(in.now))})
	entries := []ui.MenuEntry{{
		Label:   i18n.T("footer.footerMenu.customize"),
		OnClick: func() { f.customize() },
	}}
	return th.ContextMenu(f.root, &f.menu, entries).Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		pad := ui.Space(1.5)
		return layout.Inset{Top: pad, Bottom: pad}.Layout(gtx, func(gtx C) D {
			// The minimum height counts the padding, and Inset leaves Min as it finds it.
			gtx.Constraints.Min.Y = gtx.Dp(ui.Space(11)) - 2*gtx.Dp(pad)
			return ui.Wrap{GapX: ui.Space(4), GapY: ui.Space(1), Justify: ui.JustifyBetween}.Layout(gtx,
				ui.WrapItem(f.side(th, withDelegates(footerSlots(l.Left, in), in), in, ui.JustifyStart)),
				ui.WrapMLAuto(f.side(th, footerSlots(l.Right, in), in, ui.JustifyEnd)),
			)
		})
	})
}

// withDelegates ends the left side with the delegates item while the
// active session has delegates to lay out. It is no item of the user's
// layout: it shows only when there is something to offer.
func withDelegates(left []footerSlot, in footerInput) []footerSlot {
	if in.delegates == 0 {
		return left
	}
	return append(left, footerSlot{ID: "delegates", Text: i18n.T("layouts.delegates.showBeside", i18n.Args{"count": in.delegates})})
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
	if f.w("files").click.Clicked(gtx) {
		f.dock.tab = f.dock.tab.toggled(dockCode)
	}
	if f.w("changes").click.Clicked(gtx) {
		f.dock.tab = f.dock.tab.toggled(dockReview)
	}
	f.w("files").pressed = f.dock.tab == dockCode
	f.w("changes").pressed = f.dock.tab == dockReview
	if f.w("pr").click.Clicked(gtx) && in.pr != nil && f.openPullRequest != nil {
		f.openPullRequest(prTarget(in.pr, in.path, in.checkout))
	}
	if f.w("delegates").click.Clicked(gtx) && in.delegates > 0 {
		f.showDelegates(in.sessionID)
	}
}

// attach picks a file for the session and pastes its path at the prompt.
// A failure is toasted in the backend's own words, which name the ceiling a
// file was refused for.
func (f *footer) attach(in footerInput) {
	if err := attachFile(f.ctx, f.client, f.writePaste, in.sessionID, in.checkout, in.sandboxed, runtime.GOOS == "windows"); err != nil {
		f.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: backend.ErrorText(err)})
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

// slot draws one item, a control or a reading.
func (f *footer) slot(gtx C, th *ui.Theme, s footerSlot, in footerInput) D {
	w := f.w(s.ID)
	switch s.ID {
	case "attach":
		return footerButton(gtx, th, f.root, w, i18n.T("shell.footerBar.attachFile"), false, in.sessionID == "", iconOnly(icAttach))
	case "files":
		return footerButton(gtx, th, f.root, w, i18n.T("shell.footerBar.browseCode"), false, false, iconOnly(icCode))
	case "changes":
		return footerButton(gtx, th, f.root, w, i18n.T("shell.footerBar.reviewChanges"), true, false, changesContent(in.git))
	case "pr":
		return footerButton(gtx, th, f.root, w, i18n.T("shell.footerBar.viewPullRequest"), true, false, glyphText(icPR, i18n.T("shell.footerBar.pullRequest", i18n.Args{"number": in.pr.Number})))
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
	case "delegates":
		return footerButton(gtx, th, f.root, w, i18n.T("layouts.delegates.hint"), true, false, glyphText(icDelegates, s.Text))
	case "clock":
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, th.Text(ui.TextXS, s.Text).Muted().Layout)
	}
	return f.readout(gtx, th, w, s, in)
}

// readout draws the readings that carry a tooltip.
func (f *footer) readout(gtx C, th *ui.Theme, w *footerWidgets, s footerSlot, in footerInput) D {
	r := s.Readout
	pop := th.On(th.Popover)
	switch s.ID {
	case "context":
		detail := para(pop.Text(ui.TextXS, r.Detail[0]).Muted().Mono()).Layout
		if hit, ok := cacheHit(in.usage.Cache); ok {
			detail = cacheDetail(pop, detail, *in.usage.Cache, hit)
		}
		return footerReadoutView(gtx, th, f.root, w, r, detail, ringContent(in.usage.Percent, r.Text))
	case "plan":
		return footerReadoutView(gtx, th, f.root, w, r, planDetails(pop, s.Plan, in.now), glyphText(icGauge, r.Text))
	case "cost":
		// A provider with no window has no context tooltip, so its cache
		// reading rides on the cost one.
		hit, ok := cacheHit(in.usage.Cache)
		if !ok || in.usage.Window > 0 {
			break
		}
		detail := cacheDetail(pop, readoutDetail(pop, r.Detail), *in.usage.Cache, hit)
		return footerReadoutView(gtx, th, f.root, w, r, detail, footerText(r.Text))
	case "handsOn":
		content := glyphText(icTimer, r.Text)
		if in.handsOnErr != nil {
			content = footerText(r.Text)
		}
		return footerReadoutView(gtx, th, f.root, w, r, nil, content)
	}
	return footerReadoutView(gtx, th, f.root, w, r, nil, footerText(r.Text))
}
