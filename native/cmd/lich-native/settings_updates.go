package main

import (
	"log"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

const (
	updatesAppKey       = "updates.app" // no backend key: the rows only act
	updatesNotesKey     = "updates.notes"
	updatesPluginKey    = "updates.plugin"
	updatesReleaseTagAt = "/releases/tag/v"
)

// updatesSettings is Settings › Updates: what the backend answered about the
// release notes and the plugin, nil until it does, kept across visits and
// read again under the drawn answer when Settings opens again.
type updatesSettings struct {
	mu     sync.Mutex
	loaded bool

	notes         *lichclient.PatchNotes
	startupNotes  bool // startupPatchNotes found a release to show
	plugins       []lichclient.PluginStatus
	pluginsFailed bool
	busy          bool // a plugin action or check is in flight
	checked       bool // the user asked for a plugin check since Settings opened
	checkingApp   bool
	appResult     string

	checkApp, viewNotes widget.Clickable
	plugin              pluginControls
	patch               patchNotesView
}

// revalidate has the next draw read everything again and drops what belonged
// to the last visit, as unmounting the pane does.
func (u *updatesSettings) revalidate() {
	u.mu.Lock()
	u.loaded, u.checked, u.appResult = false, false, ""
	u.mu.Unlock()
	u.patch.open = false
}

// ensure starts the section's reads the first time it is drawn after a
// revalidate.
func (u *updatesSettings) ensure(s *settingsScreen) {
	u.mu.Lock()
	start := !u.loaded
	u.loaded = true
	u.mu.Unlock()
	if !start {
		return
	}
	go u.readNotes(s)
	go u.readPlugins(s)
}

func (u *updatesSettings) readNotes(s *settingsScreen) {
	n, err := s.client.PatchNotes(s.ctx)
	if err != nil {
		log.Printf("settings: patch notes: %v", err)
		return
	}
	u.mu.Lock()
	u.notes = &n
	u.mu.Unlock()
	s.invalidate()
}

// version is the running build as the Application row names it, "" until the
// notes arrived.
func (u *updatesSettings) version() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.notes == nil {
		return ""
	}
	return "v" + u.notes.Version
}

func (u *updatesSettings) patchNotes() *lichclient.PatchNotes {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.notes
}

// hasPatchNotes is `notes?.groups`: a section the changelog does not have is null,
// and an empty one still opens.
func hasPatchNotes(n *lichclient.PatchNotes) bool { return n != nil && n.Groups != nil }

// updatesRow draws the Updates rows, false for any other key.
func (s *settingsScreen) updatesRow(gtx C, th *ui.Theme, e settingEntry, r settingRow, first bool) (D, bool) {
	u := &s.updates
	switch e.key {
	case updatesAppKey:
		u.ensure(s)
		if u.checkApp.Clicked(gtx) {
			go u.runAppCheck(s)
		}
		r.description = i18n.T("settings.updatesSettings.applicationDescription", i18n.Args{"version": u.version()})
		return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D { return u.appControl(gtx, th) }), true
	case updatesNotesKey:
		u.ensure(s)
		notes := u.patchNotes()
		if u.viewNotes.Clicked(gtx) && hasPatchNotes(notes) {
			u.patch.show()
		}
		r.description = i18n.T("settings.updatesSettings.noPatchNotes")
		if hasPatchNotes(notes) {
			r.description = i18n.T("settings.updatesSettings.patchNotesFor", i18n.Args{"version": notes.Version})
		}
		return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D {
			b := th.Button(&u.viewNotes, i18n.T("settings.updatesSettings.viewPatchNotes"))
			b.Size, b.Variant, b.Disabled = ui.SizeSM, ui.VariantOutline, !hasPatchNotes(notes)
			return b.Layout(gtx)
		}), true
	case updatesPluginKey:
		u.ensure(s)
		return u.pluginBlock(gtx, th, s, r, first), true
	}
	return D{}, false
}

func (u *updatesSettings) appControl(gtx C, th *ui.Theme) D {
	u.mu.Lock()
	checking, result := u.checkingApp, u.appResult
	u.mu.Unlock()
	b := th.Button(&u.checkApp, i18n.T("settings.updatesSettings.checkForUpdates"))
	b.Size, b.Disabled = ui.SizeSM, checking
	children := []layout.FlexChild{layout.Rigid(func(gtx C) D { return busyButton(gtx, th, b, checking) })}
	if result != "" {
		children = append(children, ui.Gap(3), layout.Rigid(th.Text(ui.TextXS, result).Muted().Layout))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// runAppCheck is the manual check of AppUpdateGate: the verdict beside the
// button and, for a newer release, the prompt, even for a version Later was
// clicked for.
func (u *updatesSettings) runAppCheck(s *settingsScreen) {
	u.mu.Lock()
	u.checkingApp, u.appResult = true, ""
	u.mu.Unlock()
	s.invalidate()
	st, err := s.client.AppUpdateStatus(s.ctx)
	result := i18n.T("settings.updatesSettings.upToDate")
	switch {
	case err != nil:
		log.Printf("settings: update check: %v", err)
		result = i18n.T("settings.updatesSettings.checkFailed")
	case st.UpdateAvailable:
		result = i18n.T("settings.updatesSettings.available", i18n.Args{"version": st.LatestVersion})
		if s.promptUpdate != nil {
			s.promptUpdate(st)
		}
	}
	u.mu.Lock()
	u.checkingApp, u.appResult = false, result
	u.mu.Unlock()
	s.invalidate()
}

// busyButton is a Button that draws LoaderCircle under animate-spin before
// its label while busy.
func busyButton(gtx C, th *ui.Theme, b ui.ButtonStyle, busy bool) D {
	if !busy {
		return b.Layout(gtx)
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return ui.Spinner(gtx, ui.Space(4), th.MutedForeground) }),
		ui.Gap(2),
		layout.Rigid(b.Layout),
	)
}
