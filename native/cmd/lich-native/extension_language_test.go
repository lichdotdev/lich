package main

import (
	"slices"
	"sync"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// languageLog records, as "id tag", every language and surface.opened sent.
type languageLog struct {
	mu   sync.Mutex
	sent []string
}

func (l *languageLog) send(id string, msg extension.Message) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch d := msg.Data.(type) {
	case extension.LanguageData:
		l.sent = append(l.sent, id+" "+d.Tag)
	case extension.SurfaceData:
		l.sent = append(l.sent, id+" "+msg.Kind+" "+d.Language)
	}
}

func (l *languageLog) take() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := l.sent
	l.sent = nil
	return out
}

func TestSurfaceOpenedCarriesTheInterfaceLanguage(t *testing.T) {
	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	var log languageLog
	newExtensionScreen(&extensionStore{}, nil, log.send).show(extensionKey{"a", "main"})
	if got := log.take(); !slices.Equal(got, []string{"a surface.opened pt-BR"}) {
		t.Errorf("sent %v", got)
	}
}

func TestEveryRunningExtensionHearsTheLanguageOnceAndOnEachChange(t *testing.T) {
	defer i18n.SetCurrent(i18n.EN)
	var log languageLog
	l := extensionLanguage{send: log.send}
	running := []backend.Extension{{ID: "a", Status: api.ExtensionRunning}, {ID: "b", Status: api.ExtensionFailed}}
	l.follow(nil)
	l.follow(running)
	l.follow(running)
	if got := log.take(); !slices.Equal(got, []string{"a en"}) {
		t.Errorf("first frames sent %v, want en to a alone", got)
	}
	i18n.SetCurrent(i18n.JA)
	l.follow(running)
	if got := log.take(); !slices.Equal(got, []string{"a ja"}) {
		t.Errorf("after a change sent %v", got)
	}
	l.follow([]backend.Extension{{ID: "a", Status: api.ExtensionRestarting}})
	l.follow(running)
	if got := log.take(); !slices.Equal(got, []string{"a ja"}) {
		t.Errorf("after a restart sent %v", got)
	}
}

func TestAnExtensionThatStartsHearsTheLanguage(t *testing.T) {
	i18n.SetCurrent(i18n.ES)
	defer i18n.SetCurrent(i18n.EN)
	var log languageLog
	l := extensionLanguage{send: log.send}
	m := newTestModel(nil)
	m.extensionStarted = l.started
	for _, status := range []string{api.ExtensionStopped, api.ExtensionRunning, api.ExtensionRunning, api.ExtensionRestarting, api.ExtensionRunning} {
		m.extensionEvent(extensionEv(t, api.ExtensionStatusEvent.Name, backend.Extension{ID: "a", Status: status}))
	}
	if got := log.take(); !slices.Equal(got, []string{"a es", "a es"}) {
		t.Errorf("sent %v, want es on each start", got)
	}
}

func translatedExtension() backend.Extension {
	return backend.Extension{
		ID: "z", Name: "Zed", Status: api.ExtensionRunning,
		Surfaces: []backend.ExtensionSurface{{ID: "one", Title: "One"}, {ID: "two", Title: "Two"}},
		Text: map[string]backend.ExtensionText{
			"en":    {Name: "Zed (en)", Surfaces: map[string]string{"one": "One (en)"}},
			"pt-br": {Name: "Zé", Surfaces: map[string]string{"one": "Um"}},
			"ja":    {Surfaces: map[string]string{"two": "二"}},
		},
	}
}

func TestTheTopbarReadsTranslatedSurfaceTitles(t *testing.T) {
	defer i18n.SetCurrent(i18n.EN)
	cases := map[i18n.Locale][]string{
		i18n.PtBR: {"Um", "Two"},
		i18n.JA:   {"One (en)", "二"},
		i18n.KO:   {"One (en)", "Two"},
	}
	for l, want := range cases {
		i18n.SetCurrent(l)
		var got []string
		for _, b := range surfaceButtons([]backend.Extension{translatedExtension()}) {
			got = append(got, b.title)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: titles %v, want %v", l, got, want)
		}
	}
}

func TestSettingsReadsTheTranslatedName(t *testing.T) {
	defer i18n.SetCurrent(i18n.EN)
	e := translatedExtension()
	for l, want := range map[i18n.Locale]string{i18n.PtBR: "Zé", i18n.JA: "Zed (en)"} {
		i18n.SetCurrent(l)
		if got := extensionName(e); got != want {
			t.Errorf("%s: %q, want %q", l, got, want)
		}
	}
	e.Text = nil
	if got := extensionName(e); got != "Zed" {
		t.Errorf("no catalogs: %q, want the manifest's", got)
	}
}

// A process that starts under a surface already on screen never saw the
// surface.opened its predecessor got, so it would not know to render.
func TestAnExtensionThatStartsUnderItsOpenSurfaceHearsItOpened(t *testing.T) {
	var log languageLog
	s := newExtensionScreen(&extensionStore{}, nil, log.send)
	m := newTestModel(nil)
	m.extensionStarted = s.started
	restart := func(id string) []string {
		for _, status := range []string{api.ExtensionRestarting, api.ExtensionRunning} {
			m.extensionEvent(extensionEv(t, api.ExtensionStatusEvent.Name, backend.Extension{ID: id, Status: status}))
		}
		return log.take()
	}
	s.show(extensionKey{"a", "main"})
	log.take()
	if got := restart("a"); !slices.Equal(got, []string{"a surface.opened en"}) {
		t.Errorf("a restart under the open surface sent %v, want surface.opened", got)
	}
	if got := restart("b"); len(got) != 0 {
		t.Errorf("a restart of an extension off screen sent %v", got)
	}
	s.hide()
	log.take()
	if got := restart("a"); len(got) != 0 {
		t.Errorf("a restart after the surface closed sent %v", got)
	}
}
