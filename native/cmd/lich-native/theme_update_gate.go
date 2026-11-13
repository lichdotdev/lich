package main

import (
	"log"
	"slices"
	"strings"
	"sync"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const (
	// themeUpdatesDismissedKey holds the pack versions Later was clicked for,
	// one themePackUpdate.key a line.
	themeUpdatesDismissedKey = "lich.themeUpdatesDismissed"
	themeUpdateToastID       = "theme-update"
)

// themePackUpdate is a newer version of one installed theme pack; theme is
// one of the pack's installed themes, the id an update is asked through.
type themePackUpdate struct{ url, pack, version, theme string }

func (p themePackUpdate) key() string { return p.url + " " + p.version }

// themeUpdateGate remembers the pack versions prompted for this run.
type themeUpdateGate struct {
	mu       sync.Mutex
	prompted map[string]bool
}

// pendingThemeUpdates reads the newer pack versions off the themes, one per
// repository.
func pendingThemeUpdates(themes []themeEntry) []themePackUpdate {
	var out []themePackUpdate
	for _, e := range themes {
		d := e.def
		if d.Latest == nil || d.Source == nil || slices.ContainsFunc(out, func(p themePackUpdate) bool { return p.url == d.Source.URL }) {
			continue
		}
		out = append(out, themePackUpdate{url: d.Source.URL, pack: d.Latest.Pack, version: d.Latest.Version, theme: d.ID})
	}
	return out
}

// unpromptedThemeUpdates are the pending ones neither prompted for this run
// nor put off with Later.
func unpromptedThemeUpdates(pending []themePackUpdate, prompted map[string]bool, dismissed string) []themePackUpdate {
	later := strings.Split(dismissed, "\n")
	var out []themePackUpdate
	for _, p := range pending {
		if !prompted[p.key()] && !slices.Contains(later, p.key()) {
			out = append(out, p)
		}
	}
	return out
}

// themeUpdatesChanged is api.ThemeUpdatesEvent: the themes are read again,
// and a version not prompted for yet raises the toast.
func (u *window) themeUpdatesChanged() {
	go func() {
		if err := u.appearance.reload(); err != nil {
			log.Printf("theme updates: %v", err)
			return
		}
		u.promptThemeUpdates()
	}()
}

// promptThemeUpdates raises the toast for the pack versions it has not
// prompted for, once per version this run.
func (u *window) promptThemeUpdates() {
	pending := pendingThemeUpdates(u.appearance.view().themes)
	if len(pending) == 0 {
		return
	}
	dismissed, err := u.client.GetSetting(u.ctx, themeUpdatesDismissedKey, "")
	if err != nil {
		log.Printf("theme updates: %v", err)
		return
	}
	g := &u.themeUpdate
	g.mu.Lock()
	fresh := unpromptedThemeUpdates(pending, g.prompted, dismissed)
	if g.prompted == nil {
		g.prompted = map[string]bool{}
	}
	for _, p := range fresh {
		g.prompted[p.key()] = true
	}
	g.mu.Unlock()
	if len(fresh) > 0 {
		u.toaster.Show(themeUpdateToast(fresh, func() { u.dismissThemeUpdates(pending) }, u.settings.theme.updateTheme, u.showThemeUpdates))
	}
}

func (u *window) dismissThemeUpdates(pending []themePackUpdate) {
	keys := make([]string, len(pending))
	for i, p := range pending {
		keys[i] = p.key()
	}
	u.writePref(themeUpdatesDismissedKey, strings.Join(keys, "\n"))
}

// showThemeUpdates opens Settings › Appearance with the theme strip open,
// where each pack's update is.
func (u *window) showThemeUpdates() {
	u.settings.section = sectionAppearance
	u.settings.show("", nil)
	u.settings.theme.open = true
}

// themeUpdateToast is the app update's prompt for theme packs: one pack
// updates from the toast, several lead to Settings.
func themeUpdateToast(fresh []themePackUpdate, later func(), update func(theme string), settings func()) ui.Toast {
	t := ui.Toast{ID: themeUpdateToastID, Duration: ui.ToastForever,
		Cancel: &ui.ToastButton{Label: i18n.T("shell.appUpdateGate.later"), OnClick: later}}
	if len(fresh) == 1 {
		p := fresh[0]
		t.Title = i18n.T("shell.themeUpdateGate.available", i18n.Args{"pack": p.pack, "version": p.version})
		t.Action = &ui.ToastButton{Label: i18n.T("shell.themeUpdateGate.update"), OnClick: func() { update(p.theme) }}
		return t
	}
	t.Title = i18n.T("shell.themeUpdateGate.several", i18n.Args{"count": len(fresh)})
	t.Action = &ui.ToastButton{Label: i18n.T("shell.themeUpdateGate.openSettings"), OnClick: settings}
	return t
}
