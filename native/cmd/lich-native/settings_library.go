package main

import (
	"log"
	"strings"
	"sync"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// Settings › Skills, MCP servers and Plugins: lich's library, one screen per
// kind (internal/library). A screen lists what the providers hold that lich
// does not yet, to move in, then the library's own items; an item opens to
// one switch per provider and, under a provider with profiles, one per
// profile. Plugins holds lich's own plugin alone this round.

const (
	sectionSkills  = "skills"
	sectionMCP     = "mcp-servers"
	sectionPlugins = "plugins"

	librarySkillsKey  = "library.skills"
	libraryMCPKey     = "library.mcp"
	libraryPluginsKey = "library.plugins"

	// libraryMarkBox is a provider's mark beside a row, at the sidebar's cell.
	libraryMarkBox = unit.Dp(15)
	// libraryProfileIndent puts a profile's row under its provider's name.
	libraryProfileIndent = libraryMarkBox + 10
	// libraryOffOpacity fades a provider that cannot take the item.
	libraryOffOpacity = 0.45
)

var (
	icLibraryBack = icons.Lucide("chevron-left")
	icLibraryLock = icons.Lucide("lock")
)

// librarySettings is the three screens' state: the store they read, the item
// open, and every control drawn.
type librarySettings struct {
	store *libraryStore
	// open is the item whose detail shows, per kind; "" shows the list.
	open map[string]string

	rows, moves map[string]*ui.Clickable
	switches    map[string]*ui.SwitchState
	back        ui.Clickable
	moveAll     ui.Clickable
	lockTip     ui.Tooltip

	mu     sync.Mutex
	seen   map[string]bool // Seen already asked this visit, per kind
	moving bool
}

// libraryKind is the item kind a section lists, "" for one that is not a
// library screen.
func libraryKind(section string) string {
	switch section {
	case sectionSkills:
		return api.LibrarySkill
	case sectionMCP:
		return api.LibraryMCP
	}
	return ""
}

// libraryBadge is the nav count of a library section: what appeared in the
// providers since its last visit.
func (s *settingsScreen) libraryBadge(section string) int {
	kind := libraryKind(section)
	if kind == "" || s.library.store == nil {
		return 0
	}
	return s.library.store.newCount(kind)
}

// libraryPaneRows is a library screen under its title, or an item's detail.
func (s *settingsScreen) libraryPaneRows(th *ui.Theme, sec settingsSection) []layout.Widget {
	l := &s.library
	if l.rows == nil {
		l.rows, l.moves, l.switches, l.open, l.seen = map[string]*ui.Clickable{}, map[string]*ui.Clickable{}, map[string]*ui.SwitchState{}, map[string]string{}, map[string]bool{}
	}
	if l.store == nil {
		return nil
	}
	lib, _ := l.store.snapshot()
	if sec.id == sectionPlugins {
		return []layout.Widget{s.pageTitle(th, sec), func(gtx C) D { return s.lichPluginRow(gtx, th, lib.Plugin) }}
	}
	kind := libraryKind(sec.id)
	s.markLibrarySeen(kind)
	items := lib.Skills
	if kind == api.LibraryMCP {
		items = lib.MCP
	}
	if name := l.open[kind]; name != "" {
		for _, it := range items {
			if it.Name == name {
				return s.libraryDetail(th, sec, it, lib.Providers)
			}
		}
		l.open[kind] = ""
	}
	rows := []layout.Widget{s.pageTitle(th, sec)}
	var found []backend.LibraryFound
	for _, f := range lib.Found {
		if f.Kind == kind {
			found = append(found, f)
		}
	}
	if len(found) > 0 {
		rows = append(rows, s.foundHeading(th, found))
		for _, f := range found {
			rows = append(rows, func(gtx C) D { return s.foundRow(gtx, th, f) })
		}
		rows = append(rows, func(gtx C) D { return layout.Spacer{Height: ui.Space(5)}.Layout(gtx) })
	}
	rows = append(rows, libraryHeading(th, i18n.T("settings.librarySettings.library"), nil))
	if len(items) == 0 {
		empty := "settings.librarySettings.noSkills"
		if kind == api.LibraryMCP {
			empty = "settings.librarySettings.noMCP"
		}
		return append(rows, th.Text(settingsDescSize, i18n.T(empty)).Muted().Layout)
	}
	for _, it := range items {
		rows = append(rows, func(gtx C) D { return s.libraryRow(gtx, th, it) })
	}
	return rows
}

// markLibrarySeen tells the backend once per visit that what is found now
// has been seen, which clears the nav count.
func (s *settingsScreen) markLibrarySeen(kind string) {
	l := &s.library
	l.mu.Lock()
	ask := !l.seen[kind] && l.store.newCount(kind) > 0
	if ask {
		l.seen[kind] = true
	}
	l.mu.Unlock()
	if ask {
		go func() {
			if err := s.client.LibrarySeen(s.ctx, kind); err != nil {
				log.Printf("library: seen: %v", err)
			}
		}()
	}
}

// forgetLibraryVisit lets the next visit to a library screen count as one.
func (s *settingsScreen) forgetLibraryVisit() {
	l := &s.library
	l.mu.Lock()
	clear(l.seen)
	l.mu.Unlock()
	clear(l.open)
}

// foundHeading is the found group's label with Move in all at its right.
func (s *settingsScreen) foundHeading(th *ui.Theme, found []backend.LibraryFound) layout.Widget {
	l := &s.library
	return libraryHeading(th, i18n.T("settings.librarySettings.found"), func(gtx C) D {
		if l.moveAll.Clicked(gtx) {
			go s.moveIn(found...)
		}
		all := th.Button(&l.moveAll, i18n.T("settings.librarySettings.moveInAll"))
		all.Size, all.Variant, all.Disabled = ui.SizeSM, ui.VariantOutline, l.isMoving()
		return all.Layout(gtx)
	})
}

// libraryHeading is a group's label, the nav's own kind, with action at its
// right when there is one.
func libraryHeading(th *ui.Theme, label string, action layout.Widget) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{layout.Flexed(1, th.Text(ui.Text2XS, strings.ToUpper(label)).Weight(font.SemiBold).Muted().Layout)}
		if action != nil {
			children = append(children, layout.Rigid(action))
		}
		return layout.Inset{Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	}
}

// foundRow is one item a provider holds: its name, what it is, the marks of
// the providers that hold it, and Move in while the pointer is on it.
func (s *settingsScreen) foundRow(gtx C, th *ui.Theme, f backend.LibraryFound) D {
	l := &s.library
	key := f.Kind + "/" + f.Name
	row, move := clickableOf(l.rows, "found/"+key), clickableOf(l.moves, key)
	if move.Clicked(gtx) {
		go s.moveIn(f)
	}
	return libraryRowFrame(gtx, th, row, func(gtx C) D {
		children := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, f.Name).Weight(font.Medium).Layout)}
		if f.Transport != "" {
			children = append(children, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, f.Transport).Muted().Mono().Layout))
		}
		children = append(children, layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }),
			layout.Rigid(func(gtx C) D { return libraryMarks(gtx, th, f.Providers) }))
		if row.Hovered() || move.Hovered() {
			b := th.Button(move, i18n.T("settings.librarySettings.moveIn"))
			b.Size, b.Variant, b.Disabled = ui.SizeSM, ui.VariantOutline, l.isMoving()
			children = append(children, ui.Gap(2.5), layout.Rigid(b.Layout))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

// libraryRow is one library item: its name, what it is, and the marks of the
// providers it reaches; a click opens its detail.
func (s *settingsScreen) libraryRow(gtx C, th *ui.Theme, it backend.LibraryItem) D {
	l := &s.library
	row := clickableOf(l.rows, it.Kind+"/"+it.Name)
	if row.Clicked(gtx) {
		l.open[it.Kind] = it.Name
		s.invalidate()
	}
	var on []string
	for _, sw := range it.Switches {
		if sw.Profile == "" && sw.On {
			on = append(on, sw.Provider)
		}
	}
	return libraryRowFrame(gtx, th, row, func(gtx C) D {
		children := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, it.Name).Weight(font.Medium).Layout)}
		if it.Transport != "" {
			children = append(children, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, it.Transport).Muted().Mono().Layout))
		}
		children = append(children, layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }),
			layout.Rigid(func(gtx C) D { return libraryMarks(gtx, th, on) }))
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

// libraryRowFrame is a list row: no fill at rest, the hover fill on hover.
func libraryRowFrame(gtx C, th *ui.Theme, click *ui.Clickable, content layout.Widget) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx,
			func(gtx C) D {
				if !click.Hovered() {
					return D{Size: gtx.Constraints.Min}
				}
				return ui.Fill(gtx, th.Over(ui.Alpha(th.Accent, ghostHoverFill)), ui.RadiusMD)
			},
			func(gtx C) D {
				return layout.Inset{Left: ui.Space(1.5), Right: ui.Space(1.5), Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, content)
			},
		)
	})
}

// libraryMarks are the marks of providers, side by side.
func libraryMarks(gtx C, th *ui.Theme, providers []string) D {
	var children []layout.FlexChild
	for i, p := range providers {
		if i > 0 {
			children = append(children, ui.Gap(1))
		}
		children = append(children, layout.Rigid(func(gtx C) D { return providerGlyph(gtx, th, p, libraryMarkBox, markPixel) }))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// libraryDetail is an item's page: back to the list, its name and what it is,
// then a switch for each provider that takes it, with its profiles under it,
// and the ones that cannot last, saying why.
func (s *settingsScreen) libraryDetail(th *ui.Theme, sec settingsSection, it backend.LibraryItem, list []backend.LibraryProvider) []layout.Widget {
	l := &s.library
	rows := []layout.Widget{
		func(gtx C) D {
			if l.back.Clicked(gtx) {
				l.open[it.Kind] = ""
				s.invalidate()
			}
			return layout.Inset{Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				return l.back.Layout(gtx, func(gtx C) D {
					ink := th.MutedForeground
					if l.back.Hovered() {
						ink = th.Foreground
					}
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return icLibraryBack.Layout(gtx, ui.Space(3.5), ink) }),
						ui.Gap(1),
						layout.Rigid(th.Text(ui.TextXS, sec.label()).In(ink).Layout),
					)
				})
			})
		},
		func(gtx C) D {
			return layout.Inset{Bottom: ui.Space(1)}.Layout(gtx, th.Text(settingsTitle, it.Name).Weight(font.SemiBold).Leading(32).Layout)
		},
	}
	about := it.Description
	mono := false
	if it.Kind == api.LibraryMCP {
		about, mono = it.Command, true
	}
	if about != "" {
		rows = append(rows, func(gtx C) D {
			t := th.Text(settingsDescSize, about).Muted()
			if mono {
				t = t.Mono()
			}
			return layout.Inset{Bottom: ui.Space(4)}.Layout(gtx, t.Layout)
		})
	}
	var cannot []backend.LibraryProvider
	for _, p := range list {
		if reason := libraryReason(p, it.Kind); reason != "" {
			cannot = append(cannot, p)
			continue
		}
		shared := librarySharedReason(p, it.Kind)
		if len(p.Profiles) < 2 || shared != "" {
			rows = append(rows, func(gtx C) D { return s.librarySwitchRow(gtx, th, it, p.ID, p.Name, "", shared, 0) })
			continue
		}
		rows = append(rows, func(gtx C) D { return libraryProviderHeading(gtx, th, p) })
		for _, prof := range p.Profiles {
			rows = append(rows, func(gtx C) D {
				return s.librarySwitchRow(gtx, th, it, p.ID, prof.Name, prof.Slug, "", libraryProfileIndent)
			})
		}
	}
	if len(cannot) > 0 {
		rows = append(rows, func(gtx C) D { return layout.Spacer{Height: ui.Space(4)}.Layout(gtx) })
	}
	for _, p := range cannot {
		rows = append(rows, func(gtx C) D { return libraryCannotRow(gtx, th, p, libraryReason(p, it.Kind)) })
	}
	return rows
}

// libraryReason is why p cannot take an item of kind, "" when it can.
func libraryReason(p backend.LibraryProvider, kind string) string {
	code := p.NoSkills
	if kind == api.LibraryMCP {
		code = p.NoMCP
	}
	if code == "" {
		return ""
	}
	return i18n.T("settings.librarySettings.reason."+code, i18n.Args{"provider": p.Name})
}

// librarySharedReason is why p's profiles share one switch for an item of
// kind, "" when each has its own.
func librarySharedReason(p backend.LibraryProvider, kind string) string {
	code := p.OneSkills
	if kind == api.LibraryMCP {
		code = p.OneMCP
	}
	if code == "" {
		return ""
	}
	return i18n.T("settings.librarySettings.reason."+code, i18n.Args{"provider": p.Name})
}

// libraryProviderHeading is a provider whose profiles each have a switch: its
// mark and name, with nothing to press.
func libraryProviderHeading(gtx C, th *ui.Theme, p backend.LibraryProvider) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return providerGlyph(gtx, th, p.ID, libraryMarkBox, markPixel) }),
			ui.Gap(2.5),
			layout.Rigid(th.Text(ui.TextSM, p.Name).Layout),
		)
	})
}

// librarySwitchRow is one provider, or one of its profiles, and its switch;
// note says why one switch serves all of a provider's profiles.
func (s *settingsScreen) librarySwitchRow(gtx C, th *ui.Theme, it backend.LibraryItem, provider, label, profile, note string, indent unit.Dp) D {
	l := &s.library
	key := it.Kind + "/" + it.Name + "/" + provider + "/" + profile
	st := l.switches[key]
	if st == nil {
		st = new(ui.SwitchState)
		l.switches[key] = st
	}
	if st.Update(gtx) {
		opts := api.LibrarySwitchOptions{Kind: it.Kind, Name: it.Name, Provider: provider, Profile: profile, On: st.Value}
		l.store.setSwitch(opts)
		go s.setLibrarySwitch(opts)
	}
	for _, sw := range it.Switches {
		if sw.Provider == provider && sw.Profile == profile {
			st.Value = sw.On
		}
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Inset{Left: indent, Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		children := []layout.FlexChild{}
		if profile == "" {
			children = append(children,
				layout.Rigid(func(gtx C) D { return providerGlyph(gtx, th, provider, libraryMarkBox, markPixel) }),
				ui.Gap(2.5))
		}
		children = append(children, layout.Rigid(th.Text(ui.TextSM, label).Layout))
		if note != "" {
			children = append(children, ui.Gap(2.5),
				layout.Flexed(1, func(gtx C) D { return layout.E.Layout(gtx, th.Text(ui.TextXS, note).Muted().Layout) }),
				ui.Gap(2.5))
		} else {
			children = append(children, layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }))
		}
		children = append(children, layout.Rigid(th.Switch(st).Layout))
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

// libraryCannotRow is a provider that cannot take the item, faded, with why.
func libraryCannotRow(gtx C, th *ui.Theme, p backend.LibraryProvider, reason string) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	defer paint.PushOpacity(gtx.Ops, libraryOffOpacity).Pop()
	return layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return providerGlyph(gtx, th, p.ID, libraryMarkBox, markPixel) }),
			ui.Gap(2.5),
			layout.Rigid(th.Text(ui.TextSM, p.Name).Layout),
			layout.Flexed(1, func(gtx C) D { return layout.E.Layout(gtx, th.Text(ui.TextXS, reason).Muted().Layout) }),
		)
	})
}

// lichPluginRow is lich's own plugin: its release and what it brings, and a
// lock, since every session lich starts runs it.
func (s *settingsScreen) lichPluginRow(gtx C, th *ui.Theme, p api.LibraryPlugin) D {
	l := &s.library
	version := i18n.T("settings.pluginSetting.notInstalled")
	if p.Version != "" {
		version = "v" + p.Version
	}
	brings := i18n.T("settings.librarySettings.pluginBrings", i18n.Args{"hooks": p.Hooks, "skills": p.Skills, "mcp": p.MCP})
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Inset{Left: ui.Space(1.5), Right: ui.Space(1.5), Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextSM, "lich").Weight(font.Medium).Layout),
			ui.Gap(2),
			layout.Rigid(th.Text(ui.TextXS, version).Muted().Mono().Layout),
			layout.Flexed(1, func(gtx C) D { return layout.E.Layout(gtx, th.Text(ui.TextXS, brings).Muted().Layout) }),
			ui.Gap(2.5),
			layout.Rigid(func(gtx C) D {
				return th.Tooltip(s.root, &l.lockTip, i18n.T("settings.librarySettings.pluginLocked")).Layout(gtx, func(gtx C) D {
					return icLibraryLock.Layout(gtx, ui.Space(3.5), th.MutedForeground)
				})
			}),
		)
	})
}

func (l *librarySettings) isMoving() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.moving
}

// moveIn moves each item into the library in turn, stopping at the first
// that fails so the toast names it.
func (s *settingsScreen) moveIn(found ...backend.LibraryFound) {
	l := &s.library
	l.mu.Lock()
	if l.moving {
		l.mu.Unlock()
		return
	}
	l.moving = true
	l.mu.Unlock()
	defer func() {
		l.mu.Lock()
		l.moving = false
		l.mu.Unlock()
		s.invalidate()
	}()
	for _, f := range found {
		if err := s.client.MoveIn(s.ctx, f.Kind, f.Name); err != nil {
			s.theme.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("settings.librarySettings.moveFailed", i18n.Args{"name": f.Name, "error": backend.ErrorText(err)})})
			return
		}
	}
}

func (s *settingsScreen) setLibrarySwitch(opts api.LibrarySwitchOptions) {
	if err := s.client.SetLibrarySwitch(s.ctx, opts); err != nil {
		s.theme.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("settings.librarySettings.setFailed", i18n.Args{"error": backend.ErrorText(err)})})
		if err := s.library.store.load(s.ctx, s.client); err != nil {
			log.Printf("library: reload: %v", err)
		}
	}
	s.invalidate()
}
