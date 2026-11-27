package main

import (
	"context"
	"image"
	"image/color"
	"log"
	"maps"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/gioterm"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

const (
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
	// settingsCloseInset and settingsCloseSize place the × in the pane's
	// top-right corner.
	settingsCloseInset = ui.Space(3.5)
	settingsCloseSize  = ui.Space(7)
	settingsTitle      = unit.Sp(24)
	settingsDescSize   = unit.Sp(13)
	icSettingsClose    = icons.Lucide("x")
)

// settingsScreen is the Settings screen: the nav island and the pane island
// over the stage, the sessions sidebar left beside it. It owns its backend
// reads and writes, as the footer does.
type settingsScreen struct {
	ctx        context.Context
	client     *backend.Client
	root       *ui.Root
	invalidate func()
	// applyEditor hands the review panel the Editor preference at key that
	// just changed.
	applyEditor func(key string, prefs editorPrefs)
	// editVimConfig opens the person's Neovim config, at path, in the editor.
	editVimConfig func(path string)
	// promptUpdate is the update gate's prompt, for the manual check.
	promptUpdate func(backend.AppUpdateStatus)

	open     bool
	section  string
	page     string // the page of a split section, "" for its first
	scope    string // a project id, "" for Global
	projects []settingsProject

	mu     sync.Mutex
	values scopedValues

	nav    settingsNav
	pane   widget.List
	close  ui.Clickable
	resets map[string]*ui.Clickable
	// openGlobal is the pane foot's way to the rows a project cannot change.
	openGlobal ui.Clickable
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

	general   generalSwitches
	languages languageMenus
	notify    *notifier
	sandbox   sandboxSettings
	vcs       vcsSettings
	help      helpSettings
	hotkeys   hotkeyEditor
	updates   updatesSettings
	providers providerSettings
	// sessionView is the view new sessions open in, on the Providers page.
	sessionView sessionViewSetting
	services    servicesSettings
	library     librarySettings
	subagents   subagentSettings
	extensions  extensionsSettings
}

func newSettingsScreen(ctx context.Context, c *backend.Client, root *ui.Root, invalidate func()) *settingsScreen {
	s := &settingsScreen{ctx: ctx, client: c, root: root, invalidate: invalidate, section: defaultSection,
		resets: map[string]*ui.Clickable{}, scrollTo: -1}
	s.providers.book = newProviderBook(c, invalidate)
	s.look.font = fontPicker{find: fonts.MonoFamilies, faces: gioterm.Faces, system: fonts.SystemMono}
	s.look.editorFont = fontPicker{find: fonts.MonoFamilies, faces: gioterm.Faces, system: fonts.SystemMono}
	s.look.uiFont = fontPicker{find: uiFamilies, faces: func(family string) ([]font.FontFace, error) {
		faces, _, err := uiFaces(family)
		return faces, err
	}, system: fonts.SystemSans}
	s.pane.Axis = layout.Vertical
	s.nav.init()
	return s
}

// load reads the remembered section before the first open.
func (s *settingsScreen) load() error {
	raw, err := s.client.GetSetting(s.ctx, settingsSectionKey, "")
	s.section, s.page = parseSettingsSection(raw), settingsPageOf(raw)
	return err
}

// show opens the screen in scope ("" for Global; a project that is not open
// falls back to it) and rereads what the open projects set.
func (s *settingsScreen) show(scope string, projects []settingsProject) {
	s.open, s.projects = true, projects
	s.scope = resolveScope(scope, projects)
	s.vcs.revalidate()
	s.updates.revalidate()
	s.providers.revalidate()
	s.refresh()
}

func (s *settingsScreen) hide() {
	s.open = false
	s.watchPlans()
}

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

// snapshot is what the open projects set, the providers' own defaults and
// profiles taken from the book rather than read a second time.
func (s *settingsScreen) snapshot() scopedValues {
	s.mu.Lock()
	values := maps.Clone(s.values)
	s.mu.Unlock()
	if values == nil {
		values = scopedValues{}
	}
	book := s.providers.book.snapshot()
	values[providerDefaultKey] = book.projectDefaults
	maps.Copy(values, book.projectProfiles)
	return values
}

func (s *settingsScreen) openSection(id string) { s.openPage(id, "") }

// choose opens a search result's section on the page that holds its row, and
// lights the row.
func (s *settingsScreen) choose(gtx C, h settingHit) {
	s.openPage(h.section, sectionByID(h.section).pageOfRow(h.title))
	if h.section == sectionExtensions && h.title == i18n.T("settings.extensionsSettings.listRow") {
		s.extensions.view.Selected = extensionsViewInstalled
	}
	if s.theme != nil && h.title == i18n.T("settings.themesPage.installedRow") {
		s.theme.view.Selected = themesViewInstalled
	}
	s.providers.open = h.providerID
	s.lit, s.litUntil, s.scrollTo = h.title, gtx.Now.Add(settingsLitFor), -1
	for i, e := range s.shown().rows {
		if e.titleText() == h.title {
			s.scrollTo = i
		}
	}
}

// useGlobal deletes a project's value: the backend reads "" as unset.
func (s *settingsScreen) useGlobal(key, projectID string) {
	switch {
	case key == providerDefaultKey:
		s.providers.book.setProjectDefault(projectID, "")
	case isProfileKey(key):
		s.providers.book.setProjectProfile(projectID, key, "")
	default:
		s.mu.Lock()
		delete(s.values[key], projectID)
		s.mu.Unlock()
	}
	go func() {
		if err := s.client.SetSetting(s.ctx, key, projectID, ""); err != nil {
			log.Printf("settings: use global: %v", err)
			return
		}
		if key == providerDefaultKey {
			s.providersChanged()
		}
	}()
}

// layout draws the two islands across gtx; th is on the window's canvas.
func (s *settingsScreen) layout(gtx C, th *ui.Theme, projects []settingsProject) {
	s.projects = projects
	s.scope = resolveScope(s.scope, projects)
	if s.close.Clicked(gtx) {
		s.open = false
		s.invalidate() // the window read open before laying Settings out
	}
	s.navClicks(gtx)
	if s.revealing != nil {
		s.choose(gtx, *s.revealing)
		s.revealing = nil
	}
	values := s.snapshot()
	s.watchPlans(s.planKeys(values)...)
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
	on := th.On(bg)
	defer ui.InsetBorder(gtx, gtx.Constraints.Max, gtx.Dp(ui.RadiusLG), on.Over(on.Border))
	return w(gtx, on)
}

// paneLayout is the open section: the project pill in a project's scope, the
// title, the description, and the rows, the × in the top-right corner.
func (s *settingsScreen) paneLayout(gtx C, th *ui.Theme, values scopedValues) D {
	s.providerPaneClicks(gtx)
	rows := s.paneRows(s.shown(), th, values)
	d := layout.Inset{Left: settingsPanePadX, Top: settingsPanePadY, Bottom: settingsPanePadY}.Layout(gtx, func(gtx C) D {
		return paneScroll(th, &s.pane).Layout(gtx, len(rows), func(gtx C, i int) D {
			return layout.Inset{Right: settingsPanePadX}.Layout(gtx, func(gtx C) D { return s.readingColumn(gtx, rows[i]) })
		})
	})
	inset := gtx.Dp(settingsCloseInset)
	size := gtx.Dp(settingsCloseSize)
	defer op.Offset(image.Pt(gtx.Constraints.Max.X-inset-size, inset)).Push(gtx.Ops).Pop()
	tip := th.Tooltip(s.root, &s.nav.closeTip, i18n.T("sidebar.settingsCard.close"))
	tip.Layout(gtx, func(gtx C) D { return th.BarIconButton(&s.close, icSettingsClose, false).Layout(gtx) })
	return d
}

// paneScroll is the pane's list with its scrollbar on the island's right
// edge, over the padding, its track starting below the × so the two never
// sit side by side.
func paneScroll(th *ui.Theme, state *widget.List) material.ListStyle {
	list := th.ThinScrollArea(state)
	list.Track.MinorPadding = ui.Space(0.5)
	list.Track.MajorPadding = settingsCloseInset + settingsCloseSize + ui.Space(2) - settingsPanePadY
	return list
}

// paneRows are the pane's list children: the header lines, then each row.
func (s *settingsScreen) paneRows(sec settingsSection, th *ui.Theme, values scopedValues) []layout.Widget {
	var rows []layout.Widget
	if s.scope != "" {
		rows = append(rows, func(gtx C) D {
			return layout.Inset{Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D { return s.scopePill(gtx, th) })
		})
	}
	if sec.id == sectionSkills || sec.id == sectionMCP || sec.id == sectionPlugins {
		return append(rows, s.libraryPaneRows(th, sec)...)
	}
	if sec.id == sectionSubagents {
		return append(rows, s.subagentsPaneRows(th, sec)...)
	}
	if p, ok := s.providerScreen(); ok && sec.id == sectionProviders {
		rows = append(rows, s.providerHeader(th, p)...)
	} else if sec.id == sectionExtensions {
		rows = append(rows, s.extensionsTitle(th, sec.label()))
	} else if s.onThemesPage() {
		rows = append(rows, s.themesTitle(th, sec.label()))
	} else {
		rows = append(rows, s.pageTitle(th, sec))
	}
	if sec.id == sectionServices {
		return append(rows, s.servicesPaneRows(th)...)
	}
	if sec.id == sectionExtensions {
		return append(rows, s.extensionsPaneRows(th)...)
	}
	if s.onThemesPage() {
		return append(rows, s.themesPaneRows(th)...)
	}
	if sec.id == sectionHotkeys {
		return append(rows, s.hotkeyPaneRows(th)...)
	}
	if sec.id == sectionProviders {
		return append(rows, s.providerPaneRows(th, values, len(rows))...)
	}
	if len(sec.rows) == 0 {
		return append(rows, th.Text(settingsDescSize, i18n.T("settings.settings.notYet")).Muted().Layout)
	}
	headerRows := len(rows)
	shown, group, elsewhere := 0, "", 0
	for i, e := range sec.rows {
		r := s.rowFor(e, values)
		if r.kind == rowGlobalOnly && s.rowApplies(e) {
			elsewhere++
		}
		if r.kind == rowHidden || r.kind == rowGlobalOnly || !s.rowApplies(e) {
			continue
		}
		if g := e.groupText(); g != group {
			rows = append(rows, groupHeading(th, g, shown == 0))
			group, shown = g, 0
		}
		if i == s.scrollTo {
			s.pane.ScrollTo(len(rows))
		}
		first := shown == 0
		shown++
		rows = append(rows, func(gtx C) D { return s.drawRow(gtx, th, e, r, first) })
	}
	s.scrollTo = -1
	if elsewhere > 0 {
		rows = append(rows, s.globalElsewhere(th, len(rows) == headerRows))
	}
	return rows
}

// globalElsewhere is the foot of a project's page that leaves rows out: they
// are the same in every project, and its button opens Global to change them.
// all is a page that left out every row.
func (s *settingsScreen) globalElsewhere(th *ui.Theme, all bool) layout.Widget {
	note := i18n.T("settings.settingsRow.restGlobal")
	if all {
		note = i18n.T("settings.settingsRow.allGlobal")
	}
	return func(gtx C) D {
		if s.openGlobal.Clicked(gtx) {
			s.scope = ""
			s.invalidate() // the rows above were laid out in the project's scope
		}
		return layout.Inset{Top: ui.Space(6)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				ui.VGap(3.5),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Left: rowPadX, Right: rowPadX}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, th.Text(ui.TextXS, note).Muted().Wrapped().Layout),
							layout.Rigid(layout.Spacer{Width: rowGap}.Layout),
							layout.Rigid(func(gtx C) D {
								b := th.Button(&s.openGlobal, i18n.T("settings.settingsRow.openGlobal"))
								b.Variant, b.Size = ui.VariantOutline, ui.SizeSM
								return b.Layout(gtx)
							}),
						)
					})
				}),
			)
		})
	}
}

// rowFor is e's SettingRow without its control.
func (s *settingsScreen) rowFor(e settingEntry, values scopedValues) settingRow {
	r := settingRow{title: e.titleText(), kind: rowKindOf(e, s.scope, s.projects, values)}
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
	if s.lit == e.titleText() && gtx.Now.Before(s.litUntil) {
		r.lit = min(1, float32(s.litUntil.Sub(gtx.Now))/float32(settingsLitFadingFor))
		gtx.Execute(op.InvalidateCmd{})
	}
	switch e.key {
	case uiLanguageKey, promptLanguageKey:
		return s.languageRow(gtx, th, e.key, r, first)
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

// scopePill is the pane's pill naming the project whose scope is open.
func (s *settingsScreen) scopePill(gtx C, th *ui.Theme) D {
	name := ""
	for _, p := range s.projects {
		if p.id == s.scope {
			name = p.name
		}
	}
	return th.Pill(th.Text(ui.TextXS, name).Muted().Weight(font.Medium).Layout, ui.Space(2.25), ui.Space(2.25)).Layout(gtx)
}

func settingsProjectsOf(tabs []projectTab) []settingsProject {
	projects := make([]settingsProject, len(tabs))
	for i, t := range tabs {
		projects[i] = settingsProject{id: t.ID, name: t.Name, path: t.Path, agents: t.agents, working: t.working}
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
	for _, panel := range u.diffPanels() {
		panel.hiding, panel.split = prefs.hideWhitespace, prefs.split()
	}
	u.git.detail.split = prefs.split()
	if err := u.settings.loadHotkeys(); err != nil {
		return err
	}
	if err := u.settings.loadSessionView(); err != nil {
		return err
	}
	return u.settings.load()
}

// takeEditorPref is Settings › Editor's change reaching the diff panels:
// whitespace at once, the layout as each panel's split, which its header
// toggle goes on flipping from there. Vim turned off ends the editors'
// Neovim; the editors take their keys back on their next frame.
func (u *window) takeEditorPref(key string, prefs editorPrefs) {
	if key == diffLayoutKey {
		u.git.detail.split = prefs.split()
	}
	for _, panel := range u.diffPanels() {
		switch key {
		case hideWhitespaceKey:
			panel.setHiding(prefs.hideWhitespace)
		case diffLayoutKey:
			panel.split = prefs.split()
		}
	}
	if key == vimKey && !prefs.vim && u.vim != nil {
		u.vim.close()
	}
}

// diffPanels are the dock's Review tab and the Pulls screen's Files tab.
func (u *window) diffPanels() []*reviewPanel { return []*reviewPanel{u.review, u.pulls.view.files} }
