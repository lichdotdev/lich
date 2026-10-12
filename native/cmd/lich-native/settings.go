package main

import (
	"context"
	"image"
	"image/color"
	"log"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n.
const (
	settingsNotYet       = "Not here yet"
	settingsProjectDesc  = "What this project changes from Global. A row without a change follows Global."
	settingsCloseHint    = "Close settings"
	settingsSectionKey   = "lich.settings.section"
	settingsLitFor       = 2 * time.Second
	settingsLitFadingFor = 700 * time.Millisecond // SettingBlock's duration-700
	// settingsDescCh is the description's max-w-[62ch], a ch taken as 0.6em.
	settingsDescCh = 62
)

// The two islands' geometry, from the approved mockup.
var (
	settingsNavWidth = unit.Dp(236)
	settingsPanePadX = ui.Space(10)
	settingsPanePadY = ui.Space(8)
	settingsTitle    = unit.Sp(24)
	settingsDescSize = unit.Sp(13)
	icSettingsClose  = icons.Lucide("x")
)

// settingsScreen is the Settings screen: the nav island and the pane island
// over the stage, the sessions sidebar left beside it. It owns its backend
// reads and writes, as the footer does.
type settingsScreen struct {
	ctx        context.Context
	client     *lichclient.Client
	root       *ui.Root
	invalidate func()
	// applyEditor hands the review panel the Editor preference at key that
	// just changed.
	applyEditor func(key string, prefs editorPrefs)

	open     bool
	section  string
	scope    string // a project id, "" for Global
	projects []settingsProject

	mu     sync.Mutex
	values scopedValues

	nav    settingsNav
	pane   widget.List
	close  widget.Clickable
	resets map[string]*widget.Clickable
	// lit is the row the search sent the user to, lit until litUntil.
	lit      string
	litUntil time.Time
	scrollTo int // the pane's row to scroll in next frame, -1 for none

	editor editorSettings
	theme  *themePicker
	footer *footer
	look   appearanceControls
	// revealing is the row reveal asked for, lit on the next frame.
	revealing *settingHit

	general generalSwitches
	notify  *notifier
	sandbox sandboxSettings
	vcs     vcsSettings
	help    helpSettings
	hotkeys hotkeyEditor
	updates updatesSettings
}

func newSettingsScreen(ctx context.Context, c *lichclient.Client, root *ui.Root, invalidate func()) *settingsScreen {
	s := &settingsScreen{ctx: ctx, client: c, root: root, invalidate: invalidate, section: defaultSection,
		resets: map[string]*widget.Clickable{}, scrollTo: -1}
	s.pane.Axis = layout.Vertical
	s.nav.init()
	return s
}

// load reads the remembered section before the first open.
func (s *settingsScreen) load() error {
	raw, err := s.client.GetSetting(s.ctx, settingsSectionKey, "")
	s.section = parseSettingsSection(raw)
	return err
}

// show opens the screen in scope ("" for Global; a project that is not open
// falls back to it) and rereads what the open projects set.
func (s *settingsScreen) show(scope string, projects []settingsProject) {
	s.open, s.projects = true, projects
	s.scope = resolveScope(scope, projects)
	s.vcs.revalidate()
	s.updates.revalidate()
	s.refresh()
}

func (s *settingsScreen) hide() { s.open = false }

// showRemembered opens the screen on the scope it was left in.
func (s *settingsScreen) showRemembered(projects []settingsProject) { s.show(s.scope, projects) }

func (s *settingsScreen) refresh() {
	projects := s.projects
	s.loadSandbox()
	go func() {
		values, err := loadScopedValues(s.ctx, s.client, projects)
		if err != nil {
			log.Printf("settings: %v", err)
			return
		}
		s.mu.Lock()
		s.values = values
		s.mu.Unlock()
		s.invalidate()
	}()
}

func (s *settingsScreen) snapshot() scopedValues {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values
}

func (s *settingsScreen) openSection(id string) {
	s.section = id
	go func() {
		if err := s.client.SetSetting(s.ctx, settingsSectionKey, "", id); err != nil {
			log.Printf("settings: save section: %v", err)
		}
	}()
}

// choose opens a search result's section and lights its row.
func (s *settingsScreen) choose(gtx C, h settingHit) {
	s.openSection(h.section)
	s.lit, s.litUntil, s.scrollTo = h.title, gtx.Now.Add(settingsLitFor), -1
	for i, e := range sectionByID(h.section).rows {
		if e.title == h.title {
			s.scrollTo = i
		}
	}
}

// useGlobal deletes a project's value: the backend reads "" as unset.
func (s *settingsScreen) useGlobal(key, projectID string) {
	s.mu.Lock()
	delete(s.values[key], projectID)
	s.mu.Unlock()
	go func() {
		if err := s.client.SetSetting(s.ctx, key, projectID, ""); err != nil {
			log.Printf("settings: use global: %v", err)
		}
	}()
}

// layout draws the two islands across gtx; th is on the window's canvas.
func (s *settingsScreen) layout(gtx C, th *ui.Theme, projects []settingsProject) {
	s.projects = projects
	s.scope = resolveScope(s.scope, projects)
	if s.close.Clicked(gtx) {
		s.open = false
	}
	if s.revealing != nil {
		s.choose(gtx, *s.revealing)
		s.revealing = nil
	}
	values := s.snapshot()
	gtx.Constraints.Min = gtx.Constraints.Max
	layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(settingsNavWidth), gtx.Constraints.Max.Y))
			return s.island(gtx, th, th.Sidebar, func(gtx C, th *ui.Theme) D { return s.navLayout(gtx, th, values) })
		}),
		layout.Rigid(layout.Spacer{Width: gutter}.Layout),
		layout.Flexed(1, func(gtx C) D {
			return s.island(gtx, th, th.Background, func(gtx C, th *ui.Theme) D { return s.paneLayout(gtx, th, values) })
		}),
	)
}

// island is a rounded-lg box of bg the size of gtx.
func (s *settingsScreen) island(gtx C, th *ui.Theme, bg color.NRGBA, w func(gtx C, th *ui.Theme) D) D {
	gtx.Constraints.Min = gtx.Constraints.Max
	ui.Fill(gtx, bg, ui.RadiusLG)
	return w(gtx, th.On(bg))
}

// paneLayout is the open section: the project pill in a project's scope, the
// title, the description, and the rows, the × in the top-right corner.
func (s *settingsScreen) paneLayout(gtx C, th *ui.Theme, values scopedValues) D {
	rows := s.paneRows(sectionByID(s.section), th, values)
	d := layout.Inset{Left: settingsPanePadX, Right: settingsPanePadX, Top: settingsPanePadY, Bottom: settingsPanePadY}.Layout(gtx, func(gtx C) D {
		return th.ScrollArea(&s.pane).Layout(gtx, len(rows), func(gtx C, i int) D { return rows[i](gtx) })
	})
	inset := gtx.Dp(ui.Space(3.5))
	size := gtx.Dp(ui.Space(7))
	defer op.Offset(image.Pt(gtx.Constraints.Max.X-inset-size, inset)).Push(gtx.Ops).Pop()
	tip := th.Tooltip(s.root, &s.nav.closeTip, settingsCloseHint)
	tip.Layout(gtx, func(gtx C) D { return barIconButton(gtx, th, &s.close, icSettingsClose, false) })
	return d
}

// paneRows are the pane's list children: the header lines, then each row.
func (s *settingsScreen) paneRows(sec settingsSection, th *ui.Theme, values scopedValues) []layout.Widget {
	var rows []layout.Widget
	if s.scope != "" {
		rows = append(rows, func(gtx C) D {
			return layout.Inset{Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D { return s.scopePill(gtx, th) })
		})
	}
	rows = append(rows, func(gtx C) D {
		return layout.Inset{Bottom: ui.Space(4)}.Layout(gtx, th.Text(settingsTitle, sec.label).Weight(font.SemiBold).Leading(32).Layout)
	})
	if desc := s.paneDescription(sec); desc != "" {
		rows = append(rows, func(gtx C) D {
			return layout.Inset{Top: -ui.Space(2), Bottom: ui.Space(4)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Sp(settingsDescSize)*settingsDescCh*6/10)
				l := th.Text(settingsDescSize, desc).Muted().Leading(20)
				l.MaxLines = 0
				return l.Layout(gtx)
			})
		})
	}
	if sec.id == sectionHotkeys {
		return append(rows, s.hotkeyPaneRows(th)...)
	}
	if len(sec.rows) == 0 {
		return append(rows, th.Text(settingsDescSize, settingsNotYet).Muted().Layout)
	}
	shown, group := 0, ""
	for i, e := range sec.rows {
		r := s.rowFor(e, values)
		if r.kind == rowHidden || !s.rowApplies(e) {
			continue
		}
		if e.group != group {
			rows = append(rows, groupHeading(th, e.group, shown == 0))
			group, shown = e.group, 0
		}
		if i == s.scrollTo {
			s.pane.ScrollTo(len(rows))
		}
		first := shown == 0
		shown++
		rows = append(rows, func(gtx C) D { return s.drawRow(gtx, th, e, r, first) })
	}
	s.scrollTo = -1
	return rows
}

func (s *settingsScreen) paneDescription(sec settingsSection) string {
	if s.scope != "" && sec.projectScoped() {
		return settingsProjectDesc
	}
	return ""
}

// rowFor is e's SettingRow without its control.
func (s *settingsScreen) rowFor(e settingEntry, values scopedValues) settingRow {
	r := settingRow{title: e.title, kind: rowKindOf(e, s.scope, s.projects, values)}
	switch r.kind {
	case rowOverridden:
		r.overriddenIn = overriders(e, s.projects, values)
	case rowChanged:
		r.reset = clickableOf(s.resets, e.key)
	}
	return r
}

// drawRow lays one row out with its control, lit while the search's
// highlight on it lasts.
func (s *settingsScreen) drawRow(gtx C, th *ui.Theme, e settingEntry, r settingRow, first bool) D {
	if r.reset != nil && r.reset.Clicked(gtx) {
		s.useGlobal(e.key, s.scope)
		s.refresh()
	}
	if s.lit == e.title && gtx.Now.Before(s.litUntil) {
		r.lit = min(1, float32(s.litUntil.Sub(gtx.Now))/float32(settingsLitFadingFor))
		gtx.Execute(op.InvalidateCmd{})
	}
	switch e.key {
	case themeSettingKey:
		r.description = themeDesc
		return s.theme.layoutRow(gtx, th, r, first)
	case footerLayoutKey:
		return s.footerRow(gtx, th, r, first)
	case sandboxLadderKey:
		return s.sandboxLadder(gtx, th)
	case sshAgentKey, ghTokenKey:
		return s.grantRow(gtx, th, e, r, first)
	}
	if d, ok := s.appearanceRow(gtx, th, e, r, first); ok {
		return d
	}
	if d, ok := s.vcsRow(gtx, th, e, r, first); ok {
		return d
	}
	if d, ok := s.helpRow(gtx, th, e, r, first); ok {
		return d
	}
	if d, ok := s.updatesRow(gtx, th, e, r, first); ok {
		return d
	}
	if desc, control, ok := s.notifyRow(gtx, e.key); ok {
		r.description = desc
		return r.layout(gtx, th, first, control)
	}
	desc, control := s.editor.row(gtx, e.key, s)
	r.description = desc
	return r.layout(gtx, th, first, control)
}

// scopePill is the pane's "● project" pill in a project's scope.
func (s *settingsScreen) scopePill(gtx C, th *ui.Theme) D {
	name := ""
	for _, p := range s.projects {
		if p.id == s.scope {
			name = p.name
		}
	}
	return pill(gtx, th, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return swatch(gtx, projectSwatch(s.scope)) }),
			ui.Gap(2),
			layout.Rigid(th.Text(ui.TextXS, name).Muted().Weight(font.Medium).Layout),
		)
	}, ui.Space(1.75), ui.Space(2.25))
}

// pill is a rounded-full box with an inset 1px border around w.
func pill(gtx C, th *ui.Theme, w layout.Widget, padL, padR unit.Dp) D {
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	d := layout.Inset{Left: padL, Right: padR, Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, w)
	call := macro.Stop()
	insetBorder(gtx, d.Size, d.Size.Y/2, th.Over(th.Border))
	call.Add(gtx.Ops)
	return d
}

// swatch is the mockup's 10px rounded square of a project's color.
func swatch(gtx C, c color.NRGBA) D {
	size := image.Pt(gtx.Dp(10), gtx.Dp(10))
	gtx.Constraints = layout.Exact(size)
	return ui.Fill(gtx, c, unit.Dp(3))
}

func settingsProjectsOf(tabs []projectTab) []settingsProject {
	projects := make([]settingsProject, len(tabs))
	for i, t := range tabs {
		projects[i] = settingsProject{id: t.ID, name: t.Name}
	}
	return projects
}

// loadSettings reads the Editor preferences the review panel starts from and
// the section Settings was left on.
func (u *window) loadSettings() error {
	prefs, err := loadEditorPrefs(u.ctx, u.client)
	if err != nil {
		return err
	}
	u.settings.editor.prefs = prefs
	u.review.hiding, u.review.split = prefs.hideWhitespace, prefs.split()
	if err := u.settings.loadHotkeys(); err != nil {
		return err
	}
	return u.settings.load()
}

// takeEditorPref is Settings › Editor's change reaching the review panel:
// whitespace at once, the layout as the panel's split, which its header
// toggle goes on flipping from there.
func (u *window) takeEditorPref(key string, prefs editorPrefs) {
	switch key {
	case hideWhitespaceKey:
		u.review.setHiding(prefs.hideWhitespace)
	case diffLayoutKey:
		u.review.split = prefs.split()
	}
}
