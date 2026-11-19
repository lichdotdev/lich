package main

import (
	"context"
	"log"
	"runtime"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	driver "github.com/lichdotdev/gio-driver"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icAttach   = icons.Lucide("paperclip")
	icDiff     = icons.Lucide("diff")
	icFileText = icons.Lucide("file-text")
	icFolder   = icons.Lucide("folder")
	icTimer    = icons.Lucide("timer")
)

// footer is what the user arranged in the footer editor, following the active
// session: its left side and its right side float as two balloons at the foot
// of the stage (footer_balloons.go).
type footer struct {
	// tour is the window's guided tour, which points at the footer's chips.
	tour       *driver.Tour
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
	// menus are the left and the right balloon's right-click menus.
	menus  [2]ui.MenuState
	editor footerEditor
	// shown is what the balloons read this frame, which the editor's chips
	// and preview read the model from.
	shown footerInput
	// customize opens Settings on the Footer row.
	customize func()
	// dock is what Review changes opens, on its tab.
	dock *dock
	git  *gitPanel
	// panel is what an extension's panel item opens.
	panel   *extensionPanel
	toaster *ui.Toaster
	// extensions is the store the extension items read.
	extensions *extensionStore
	// loaded is the saved layout read, before which no extension item is
	// adopted into it.
	loaded bool
}

func newFooter(ctx context.Context, client *backend.Client, root *ui.Root, usage *footerUsage, quota *planQuota, invalidate func()) *footer {
	return &footer{
		tour: new(driver.Tour), ctx: ctx, client: client, root: root, invalidate: invalidate, usage: usage,
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
	f.layout, f.loaded = l, true
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

// holdsKeys reports whether a balloon's menu wants the keyboard.
func (f *footer) holdsKeys() bool { return f.menus[0].Opened() || f.menus[1].Opened() }

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
	f.readExtensions(&in)
	in.usage, in.hasUsage = f.usage.get(in.sessionID)
	f.follow(in)
	in.plans = f.quota.plans(in.sessionID)
	in.handsOn, in.handsOnErr = f.handsOn.read()
	return in
}

// sessionInput reads what the line under a chat's box shows of session sv, a
// pane the footer does not follow: its checkout, its branch and its changes,
// which are its own with or without the focus. The rest of the footer's data is
// the followed session's alone.
func (f *footer) sessionInput(m *model, sv sessionView, now time.Time) footerInput {
	in := footerInput{now: now, sessionID: sv.ID, provider: sv.agent, path: sv.shown, checkout: sv.path, host: sv.host, pr: sv.pr}
	if in.provider == "" {
		in.provider = sv.Kind
	}
	in.git, in.isGit = m.gitStatus(in.path)
	return in
}

// readExtensions reads the extensions' placed surfaces and their items' text
// for the active checkout, and adopts an item the footer never offered.
func (f *footer) readExtensions(in *footerInput) {
	if f.extensions == nil {
		return
	}
	in.placed = placedSurfaces(f.extensions.all(), f.extensions.surfaceImages())
	ids := make([]string, len(in.placed))
	for i, p := range in.placed {
		ids[i] = extensionItemID(p.key)
		in.extItems = append(in.extItems, footerExtItem{placedSurface: p, id: ids[i], text: f.extensions.footer(p.key, in.contextPath())})
	}
	f.mu.Lock()
	l, ready := f.layout, f.loaded && !f.saving
	f.mu.Unlock()
	if next, fresh := l.adopt(ids); fresh && ready {
		f.change(next)
	}
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

// frame runs the balloons' clicks and keeps what they read, before anything
// the clicks open is laid out. It runs every frame, balloons drawn or not: the
// footer editor reads what it keeps.
func (f *footer) frame(gtx C, in footerInput) {
	f.act(gtx, in)
	f.shown = in
	// The clock and every countdown read on the minute.
	gtx.Execute(op.InvalidateCmd{At: in.now.Add(untilNextMinute(in.now))})
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

// act carries out the clicks of the frame, before the balloons are laid out.
func (f *footer) act(gtx C, in footerInput) {
	if f.w("attach").click.Clicked(gtx) && in.sessionID != "" {
		go f.attach(in)
	}
	if f.w("changes").click.Clicked(gtx) {
		f.dock.show(dockReview)
	}
	for _, it := range in.extItems {
		if it.place == extension.PlaceDock && f.w(it.id).click.Clicked(gtx) {
			f.dock.showExtension(it.key)
		}
	}
	if f.w("pr").click.Clicked(gtx) && in.pr != nil && f.openPullRequest != nil {
		f.openPullRequest(prTarget(in.pr, in.path, in.checkout))
	}
	if f.w("delegates").click.Clicked(gtx) && in.delegates > 0 {
		f.showDelegates(in.sessionID)
	}
}

// press lights the items whose dock tab is up, read as the balloons are drawn,
// after every click of the frame moved them. The branch and a panel's item
// need no light: the balloons are not drawn while their panel is up.
func (f *footer) press(in footerInput) {
	f.w("changes").pressed = f.dock.showing(dockReview)
	for _, it := range in.extItems {
		if it.place == extension.PlaceDock {
			f.w(it.id).pressed = f.dock.showsExtension(it.key) && f.dock.visible()
		}
	}
}

// actGit runs the branch's click before the frame lays the Git panel out: the
// panel's row comes before the balloons, and a click read there would open it
// only on the next event.
func (f *footer) actGit(gtx C) {
	if f.w("checkout").click.Clicked(gtx) {
		f.git.open = !f.git.open
	}
	for _, it := range f.shown.extItems {
		if it.place == extension.PlacePanel && f.w(it.id).click.Clicked(gtx) {
			f.panel.toggle(it.key)
		}
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
	w := f.w(s.ID + s.Pane)
	if it, ok := in.extItem(s.ID); ok {
		if it.text == "" {
			return footerButton(gtx, th, f.root, w, s.Hint, false, false, iconOnly(it.glyph))
		}
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(checkoutMaxWidth))
		return footerButton(gtx, th, f.root, w, s.Hint, true, false, branchContent(it.glyph, it.text))
	}
	switch s.ID {
	case "attach":
		return f.attachButton(gtx, th, in, icAttach)
	case "changes":
		chip := func(gtx C) D {
			return footerButton(gtx, th, f.root, w, i18n.T("shell.footerBar.reviewChanges"), true, false, changesContent(in.git))
		}
		if s.Pane != "" {
			return chip(gtx)
		}
		return f.tour.Anchor(gtx, tourReviewChip, chip)
	case "pr":
		return footerButton(gtx, th, f.root, w, i18n.T("shell.footerBar.viewPullRequest"), true, false, glyphText(icPR, i18n.T("shell.footerBar.pullRequest", i18n.Args{"number": in.pr.Number})))
	case "checkout", "path":
		ic := icFolder
		if s.ID == "checkout" && in.git.Branch != "" {
			ic = icBranch
		}
		if s.ID == "checkout" && f.git != nil && in.inRepository() {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(checkoutMaxWidth))
			return f.tour.Anchor(gtx, worktreesBranch, func(gtx C) D {
				return footerButton(gtx, th, f.root, w, s.Hint, true, false, branchContent(ic, s.Text))
			})
		}
		glyph := func(gtx C) D { return ic.Layout(gtx, footerGlyph, th.MutedForeground) }
		return footerSpan(gtx, th, f.root, &w.tip, s.Hint, checkoutMaxWidth, glyph, s.Text)
	case "model":
		glyph := func(gtx C) D { return providerIcon(in.provider).Layout(gtx, 14, th.MutedForeground) }
		return footerSpan(gtx, th, f.root, &w.tip, s.Hint, modelMaxWidth, glyph, s.Text)
	case "delegates":
		return f.tour.Anchor(gtx, tourDelegates, func(gtx C) D {
			return footerButton(gtx, th, f.root, w, i18n.T("layouts.delegates.hint"), true, false, glyphText(icDelegates, s.Text))
		})
	case "clock":
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, th.Text(ui.TextXS, s.Text).Muted().Layout)
	}
	return f.readout(gtx, th, w, s, in)
}

// attachButton is the attach item drawn with glyph.
func (f *footer) attachButton(gtx C, th *ui.Theme, in footerInput, glyph icons.Glyph) D {
	return footerButton(gtx, th, f.root, f.w("attach"), i18n.T("shell.footerBar.attachFile"), false, in.sessionID == "", iconOnly(glyph))
}

// readout draws the readings that carry a tooltip.
func (f *footer) readout(gtx C, th *ui.Theme, w *footerWidgets, s footerSlot, in footerInput) D {
	r := s.Readout
	pop := th.On(th.Popover)
	switch s.ID {
	case "context":
		return footerReadoutFace(gtx, th, f.root, w, r, contextTip(pop, in.usage), ringContent(in.usage.Percent, r.Text))
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
