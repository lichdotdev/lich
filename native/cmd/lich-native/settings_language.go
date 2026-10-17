package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/fonts"
	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// The two languages, never derived from each other. The interface's is the
// web's localStorage key, kept as a global setting here; the web window
// writes its own choice to the same setting, so a language picked there is
// the one this window opens in. The prompt language is the backend's own
// setting (internal/store promptLanguageKey).
const (
	uiLanguageKey     = "lich.appearance.language"
	promptLanguageKey = "prompt.language"
)

var flagHanFontOffer = flag.String("han-font-offer", "", "open the CJK-font offer at start as if the language were chosen in Settings: zh-CN, ja or ko, with \":download\" (ko:download) to also start the download, for -shot")

// languageMenus are the two pickers' menus, and the offer to download a
// CJK font when the interface switches to Chinese, Japanese or Korean
// without one.
type languageMenus struct {
	ui, prompt ui.MenuState
	// draws says whether a face, installed or downloaded, draws a language's
	// script; reloadFonts builds the shaper again, for a font just downloaded.
	draws       func(i18n.Locale) bool
	reloadFonts func() error
	offer       hanFontOffer
}

// needsHanFont says whether showing the interface in l waits for a CJK font:
// every other language draws with the faces the window has.
func needsHanFont(l i18n.Locale, draws func(i18n.Locale) bool) bool {
	switch l {
	case i18n.ZhCN, i18n.JA, i18n.KO:
		return !draws(l)
	}
	return false
}

// drawsLanguage says whether hanFaces has a face for l. An error reading
// them counts as none, which offers the download: it is logged, and the
// download reports its own failure.
func drawsLanguage(l i18n.Locale) bool {
	faces, err := hanFaces(l)
	if err != nil {
		log.Printf("fonts: CJK faces for %s: %v", l, err)
	}
	return len(faces) > 0
}

// launchLanguage is the language the window opens in: l, or English while l
// waits for a CJK font, which is then offered.
func launchLanguage(l i18n.Locale, draws func(i18n.Locale) bool) (shown i18n.Locale, offer bool) {
	if needsHanFont(l, draws) {
		return i18n.EN, true
	}
	return l, false
}

// loadLanguages reads both languages: the interface's is the stored choice,
// or system's before anyone made one; the prompt language is English when
// absent, as the backend reads it.
func loadLanguages(ctx context.Context, c *lichclient.Client, system func() i18n.Locale) (iface, prompt i18n.Locale, err error) {
	stored, err := c.GetSetting(ctx, uiLanguageKey, "")
	if err != nil {
		return "", "", err
	}
	iface = system()
	if stored != "" {
		iface = i18n.Match(stored)
	}
	storedPrompt, err := c.GetSetting(ctx, promptLanguageKey, "")
	if err != nil {
		return "", "", err
	}
	return iface, i18n.Match(storedPrompt), nil
}

// loadLanguages sets both languages before the first frame.
func (u *window) loadLanguages() error {
	iface, prompt, err := loadLanguages(u.ctx, u.client, i18n.System)
	if err != nil {
		return err
	}
	languages := &u.settings.languages
	languages.reloadFonts, languages.draws = u.reloadInterfaceFonts, drawsLanguage
	shown, offer := launchLanguage(iface, languages.draws)
	i18n.SetCurrent(shown)
	if offer {
		languages.offer.show(iface, i18n.SetCurrent)
	}
	if *flagHanFontOffer != "" {
		tag, mode, _ := strings.Cut(*flagHanFontOffer, ":")
		languages.offer.show(i18n.Match(tag), u.settings.keepLanguage)
		if mode == "download" {
			u.settings.startHanFontDownload()
		}
	}
	i18n.SetPromptLanguage(prompt)
	return nil
}

// languageRow is LanguageSetting and PromptLanguageSetting: a select of the
// four languages, each named in itself.
func (s *settingsScreen) languageRow(gtx C, th *ui.Theme, key string, r settingRow, first bool) D {
	menu, current, choose := &s.languages.ui, i18n.Current(), s.chooseLanguage
	r.description = i18n.T("settings.language.uiDescription")
	if key == promptLanguageKey {
		menu, current, choose = &s.languages.prompt, i18n.PromptLanguage(), s.choosePromptLanguage
		r.description = i18n.T("settings.language.promptDescription")
	}
	return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D {
		entries := make([]ui.MenuEntry, len(i18n.Locales))
		for i, l := range i18n.Locales {
			entries[i] = ui.MenuEntry{Kind: ui.MenuRadioItem, Label: l.Name(), Checked: l == current, OnClick: func() { choose(l) }}
		}
		return th.DropdownMenu(s.root, menu, entries).Layout(gtx, func(gtx C) D { return fontTrigger(gtx, th, current.Name()) })
	})
}

// chooseLanguage shows the interface in l from the next frame on and keeps
// the choice, once a CJK font is in place for a CJK language.
func (s *settingsScreen) chooseLanguage(l i18n.Locale) {
	if needsHanFont(l, s.languages.draws) {
		s.languages.offer.show(l, s.keepLanguage)
		return
	}
	s.keepLanguage(l)
}

func (s *settingsScreen) keepLanguage(l i18n.Locale) {
	i18n.SetCurrent(l)
	s.invalidate()
	go func() {
		if err := s.client.SetSetting(s.ctx, uiLanguageKey, "", string(l)); err != nil {
			log.Printf("settings: save %s: %v", uiLanguageKey, err)
		}
	}()
}

// choosePromptLanguage is setPromptLanguage: the choice shows at once, and
// the one before it comes back, with a toast, when the backend refuses it.
func (s *settingsScreen) choosePromptLanguage(l i18n.Locale) {
	previous := i18n.PromptLanguage()
	i18n.SetPromptLanguage(l)
	go func() {
		err := s.client.SetSetting(s.ctx, promptLanguageKey, "", string(l))
		if err == nil {
			return
		}
		if i18n.PromptLanguage() == l {
			i18n.SetPromptLanguage(previous)
		}
		title := i18n.T("settings.language.promptSaveFailed", i18n.Args{"error": lichclient.ErrorText(err)})
		s.theme.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: title})
		s.invalidate()
	}()
}

// hanFontOffer is the dialog that offers to download target's Noto Sans
// (fonts.NotoSansFor) before the interface switches to it, and the download
// it starts. The download
// runs on its own goroutine and hands its outcome to the next frame, which
// loads the font and switches on the window's goroutine.
type hanFontOffer struct {
	dialog           ui.DialogState
	open             bool
	target           i18n.Locale
	apply            func(i18n.Locale)
	download, cancel widget.Clickable
	// run tells a download's outcome from one started before a cancel.
	run         int
	downloading bool
	done        atomic.Int64

	mu       sync.Mutex
	finished *hanFontOutcome
	stop     context.CancelFunc
}

type hanFontOutcome struct {
	run int
	err error
}

// show opens the dialog; apply switches to target once the font loads.
func (o *hanFontOffer) show(target i18n.Locale, apply func(i18n.Locale)) {
	o.open, o.target, o.apply = true, target, apply
}

// dismiss closes the dialog and stops a download under way: the language
// stays what it was.
func (o *hanFontOffer) dismiss() {
	o.mu.Lock()
	if o.stop != nil {
		o.stop()
		o.stop = nil
	}
	o.mu.Unlock()
	o.open, o.downloading = false, false
	o.run++
}

func (o *hanFontOffer) outcome() *hanFontOutcome {
	o.mu.Lock()
	defer o.mu.Unlock()
	r := o.finished
	o.finished = nil
	return r
}

func (s *settingsScreen) startHanFontDownload() {
	o := &s.languages.offer
	dir, err := fonts.DownloadDir()
	if err != nil {
		s.hanFontFailed(err)
		return
	}
	ctx, stop := context.WithCancel(s.ctx)
	o.mu.Lock()
	o.stop = stop
	o.mu.Unlock()
	o.downloading = true
	o.done.Store(0)
	run := o.run
	go func() {
		defer stop()
		_, files := fonts.NotoSansFor(string(o.target))
		err := fonts.Fetch(ctx, files, dir, func(done, _ int64) {
			o.done.Store(done)
			s.invalidate()
		})
		o.mu.Lock()
		o.finished = &hanFontOutcome{run: run, err: err}
		o.mu.Unlock()
		s.invalidate()
	}()
}

// finishHanFontDownload loads the downloaded font and only then switches.
func (s *settingsScreen) finishHanFontDownload(r *hanFontOutcome) {
	o := &s.languages.offer
	if r.run != o.run {
		return
	}
	target, apply := o.target, o.apply
	o.dismiss()
	err := r.err
	if err == nil {
		err = s.languages.reloadFonts()
	}
	if err == nil && !s.languages.draws(target) {
		family, _ := fonts.NotoSansFor(string(target))
		err = fmt.Errorf("the downloaded %s does not draw %s", family, target.Name())
	}
	if err != nil {
		s.hanFontFailed(err)
		return
	}
	apply(target)
	s.invalidate()
}

func (s *settingsScreen) hanFontFailed(err error) {
	family, _ := fonts.NotoSansFor(string(s.languages.offer.target))
	title := i18n.T("settings.language.hanFontFailed", i18n.Args{"font": family, "error": err.Error()})
	s.theme.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: title})
}

// wholeMegabytes rounds n for the dialog, which counts the download in MB.
func wholeMegabytes(n int64) int64 { return (n + bytesPerMB/2) / bytesPerMB }

// layoutHanFontDialog draws the offer while it is open, Settings shown or not:
// at launch it asks before Settings ever opens.
func (s *settingsScreen) layoutHanFontDialog(gtx C, th *ui.Theme) {
	o := &s.languages.offer
	if r := o.outcome(); r != nil {
		s.finishHanFontDownload(r)
	}
	if o.download.Clicked(gtx) && !o.downloading {
		s.startHanFontDownload()
	}
	if o.cancel.Clicked(gtx) {
		o.dismiss()
	}
	family, files := fonts.NotoSansFor(string(o.target))
	total := fonts.TotalSize(files)
	pop := th.On(th.Popover)
	d := th.Dialog(&o.dialog, o.open, i18n.T("settings.language.hanFontTitle", i18n.Args{"language": o.target.Name()}),
		i18n.T("settings.language.hanFontDescription", i18n.Args{"language": o.target.Name(), "font": family, "size": wholeMegabytes(total)}))
	if o.downloading {
		done := o.done.Load()
		d.Body = func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(pop.Progress(int(done*100/total)).Layout),
				ui.Gap(2),
				layout.Rigid(pop.Text(ui.TextSM, i18n.T("settings.language.hanFontProgress",
					i18n.Args{"done": wholeMegabytes(done), "total": wholeMegabytes(total)})).Muted().Layout),
			)
		}
	}
	cancel := pop.Button(&o.cancel, i18n.T("common.action.cancel"))
	cancel.Variant = ui.VariantGhost
	download := pop.Button(&o.download, i18n.T("settings.language.hanFontDownload"))
	download.Disabled = o.downloading
	d.Actions = []layout.Widget{cancel.Layout, download.Layout}
	if d.Layout(gtx) {
		o.dismiss()
	}
}
