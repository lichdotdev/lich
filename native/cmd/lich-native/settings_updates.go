package main

import (
	"log"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const (
	updatesAppKey    = "updates.app" // no backend key: the rows only act
	updatesNotesKey  = "updates.notes"
	updatesPluginKey = "updates.plugin"
	// updateChannelKey is the backend's (store.BetaUpdates): the release
	// channel the update check follows.
	updateChannelKey    = "update.channel"
	updateChannelStable = "stable"
	updateChannelBeta   = "beta"
	updatesReleaseTagAt = "/releases/tag/v"
)

// updatesSettings is Settings › Updates: what the backend answered about the
// release notes and the plugin, nil until it does, kept across visits and
// read again under the drawn answer when Settings opens again.
type updatesSettings struct {
	mu     sync.Mutex
	loaded bool

	notes         *backend.PatchNotes
	startupNotes  bool // startupPatchNotes found a release to show
	plugins       []backend.PluginStatus
	pluginsFailed bool
	busy          bool // a plugin action or check is in flight
	checked       bool // the user asked for a plugin check since Settings opened
	checkingApp   bool
	appResult     string
	channel       string // "" until read

	checkApp, viewNotes widget.Clickable
	channelPick         widget.Enum
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
	go u.readChannel(s)
}

func (u *updatesSettings) readChannel(s *settingsScreen) {
	raw, err := s.client.GetSetting(s.ctx, updateChannelKey, "")
	if err != nil {
		log.Printf("settings: update channel: %v", err)
		return
	}
	u.mu.Lock()
	u.channel = parseUpdateChannel(raw)
	u.mu.Unlock()
	s.invalidate()
}

// parseUpdateChannel is store.BetaUpdates: stable until beta is chosen.
func parseUpdateChannel(raw string) string {
	if raw == updateChannelBeta {
		return updateChannelBeta
	}
	return updateChannelStable
}

func (u *updatesSettings) readNotes(s *settingsScreen) {
	n, err := s.client.PatchNotes(s.ctx, string(i18n.Current()))
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

func (u *updatesSettings) patchNotes() *backend.PatchNotes {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.notes
}

// hasPatchNotes is `notes?.groups`: a section the changelog does not have is null,
// and an empty one still opens.
func hasPatchNotes(n *backend.PatchNotes) bool { return n != nil && n.Groups != nil }

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
	case updateChannelKey:
		u.ensure(s)
		r.description = i18n.T("settings.updatesSettings.channelDescription")
		return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D { return u.channelControl(gtx, th, s) }), true
	case updatesNotesKey:
		u.ensure(s)
		notes := u.patchNotes()
		if u.viewNotes.Clicked(gtx) && hasPatchNotes(notes) {
			u.patch.show()
			s.invalidate() // the dialog is laid out before the pane, a frame behind the click
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
	children := []layout.FlexChild{layout.Rigid(func(gtx C) D {
		if checking {
			return th.SpinnerBeside(gtx, b.Layout)
		}
		return b.Layout(gtx)
	})}
	if result != "" {
		children = append(children, ui.Gap(3), layout.Rigid(th.Text(ui.TextXS, result).Muted().Layout))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// channelControl stores a new channel and checks against it at once, so a
// switch to beta finds the pre-release without waiting for the hourly poll.
func (u *updatesSettings) channelControl(gtx C, th *ui.Theme, s *settingsScreen) D {
	if u.channelPick.Update(gtx) {
		channel := parseUpdateChannel(u.channelPick.Value)
		u.mu.Lock()
		u.channel = channel
		u.mu.Unlock()
		go func() {
			if err := s.client.SetSetting(s.ctx, updateChannelKey, "", channel); err != nil {
				log.Printf("settings: save %s: %v", updateChannelKey, err)
				return
			}
			u.runAppCheck(s)
		}()
	}
	u.mu.Lock()
	channel := u.channel
	u.mu.Unlock()
	if channel == "" {
		return D{}
	}
	u.channelPick.Value = channel
	return th.ToggleGroup(&u.channelPick,
		ui.ToggleItem{Value: updateChannelStable, Text: i18n.T("settings.updatesSettings.channelStable")},
		ui.ToggleItem{Value: updateChannelBeta, Text: i18n.T("settings.updatesSettings.channelBeta")},
	).Layout(gtx)
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
