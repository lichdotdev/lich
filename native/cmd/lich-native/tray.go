package main

import (
	"log"
	"os"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/i18n"
)

// trayLabels are the tray menu's words in the interface language. The
// backend fills Running's {count} placeholder itself.
func trayLabels() backend.TrayLabels {
	return backend.TrayLabels{
		Show:    i18n.T("shell.tray.show"),
		Running: i18n.T("shell.tray.running", i18n.Args{"count": "{count}"}),
		Quit:    i18n.T("shell.tray.quit"),
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
// interface language (the hotkeys' labels, the CJK faces) or hands the
// backend (the tray's words) is renewed when Settings changed the language
// since the last frame, and the CJK faces join the interface's shaper on the
// frame after CJK text first came up.
// Settings only sets the language, so the window notices rather than being
// told. A send that fails is not retried until the language changes again.
func (u *window) followLanguage() {
	lang := i18n.Current()
	if lang != u.hotkeyLanguage {
		u.hotkeyLanguage = lang
		localizeHotkeys()
	}
	cjkCame := u.uiFonts != nil && u.cjkSeen.Load() && !u.uiFonts.han
	if u.uiFonts != nil && (lang != u.fontsLanguage || cjkCame) {
		u.fontsLanguage = lang
		if err := u.setInterfaceFonts(*u.uiFonts); err != nil {
			u.toaster.Show(fontFailure(i18n.T("settings.appearancePrefs.uiFontFailed", i18n.Args{"family": systemOr(u.uiFonts.sans, fonts.SystemSans), "error": backend.ErrorText(err)})))
		}
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
