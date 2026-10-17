package main

import (
	"log"
	"os"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
)

// trayLabels are the tray menu's words in the interface language. The
// backend fills Running's {count} placeholder itself.
func trayLabels() lichclient.TrayLabels {
	return lichclient.TrayLabels{
		Show:       i18n.T("shell.tray.show"),
		Running:    i18n.T("shell.tray.running", i18n.Args{"count": "{count}"}),
		Quit:       i18n.T("shell.tray.quit"),
		OpenNative: i18n.T("shell.tray.openNative"),
		OpenWeb:    i18n.T("shell.tray.openWeb"),
	}
}

// attachTray makes this window the one the tray and a second launch bring
// back, then puts the tray up: the backend leaves it down until a window
// hands it its words.
func (u *window) attachTray() {
	executable, err := os.Executable()
	if err != nil {
		log.Printf("tray: find this window's binary: %v", err)
		return
	}
	if err := u.client.AttachNative(u.ctx, executable); err != nil {
		log.Printf("system.AttachNative: %v", err)
		return
	}
	lang := i18n.Current()
	if err := u.client.SetTrayLabels(u.ctx, trayLabels()); err != nil {
		log.Printf("tray.SetLabels: %v", err)
		return
	}
	u.trayLanguage.Store(lang)
}

// followLanguage runs each frame: what a table built once holds in the
// interface language (the hotkeys' labels) or hands the backend (the tray's
// words) is renewed when Settings changed the language since the last frame.
// Settings only sets the language, so the window notices rather than being
// told. A send that fails is not retried until the language changes again.
func (u *window) followLanguage() {
	lang := i18n.Current()
	if lang != u.hotkeyLanguage {
		u.hotkeyLanguage = lang
		localizeHotkeys()
	}
	sent, attached := u.trayLanguage.Load().(i18n.Locale)
	if !attached || sent == lang {
		return
	}
	u.trayLanguage.Store(lang)
	go func() {
		if err := u.client.SetTrayLabels(u.ctx, trayLabels()); err != nil {
			log.Printf("tray.SetLabels: %v", err)
		}
	}()
}
