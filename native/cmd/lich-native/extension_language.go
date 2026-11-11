package main

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// extensionLanguage tells running extensions the language the window shows.
// The backend starts one with the stored setting, which it cannot resolve to
// the system's language everywhere, so the window tells every running
// extension once it sees it, again on each change, and again when one
// starts: started comes with the status event, before any frame, so a
// restart between two frames is heard too.
type extensionLanguage struct {
	send func(string, extension.Message)
	// sent is the language each running extension was last told, on the
	// window's goroutine.
	sent map[string]i18n.Locale
}

func languageMessage(l i18n.Locale) extension.Message {
	return extension.Language.New(extension.LanguageData{Tag: string(l)})
}

// follow runs every frame, on the window's goroutine, the one that changes
// the language.
func (l *extensionLanguage) follow(list []backend.Extension) {
	current := i18n.Current()
	running := make(map[string]i18n.Locale, len(list))
	for _, e := range list {
		if e.Status != api.ExtensionRunning {
			continue
		}
		if l.sent[e.ID] != current {
			l.send(e.ID, languageMessage(current))
		}
		running[e.ID] = current
	}
	l.sent = running
}

// started tells extension id, which has just started, the language.
func (l *extensionLanguage) started(id string) { l.send(id, languageMessage(i18n.Current())) }
