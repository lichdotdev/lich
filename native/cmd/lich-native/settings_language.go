package main

import (
	"context"
	"errors"
	"flag"
	"log"
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

var flagHanFontOffer = flag.String("han-font-offer", "", "open the Chinese-font offer at start as if 简体中文 were chosen in Settings, \"ask\" or \"download\" to also start the download, for -shot")

// languageMenus are the two pickers' menus, and the offer to download a
// Chinese font when the interface switches to Chinese without one.
type languageMenus struct {
	ui, prompt ui.MenuState
	// han says whether the interface's shaper has a face that draws Chinese;
	// reloadFonts builds the shaper again, for a font just downloaded.
	han         bool
	reloadFonts func() error
	offer       hanFontOffer
}

// needsHanFont says whether showing the interface in l waits for a Chinese
// font: every other language draws with the faces the window has.
func needsHanFont(l i18n.Locale, han bool) bool { return l == i18n.ZhCN && !han }

// launchLanguage is the language the window opens in: l, or English while l
// waits for a Chinese font, which is then offered.
func launchLanguage(l i18n.Locale, han bool) (shown i18n.Locale, offer bool) {
	if needsHanFont(l, han) {
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
	languages.reloadFonts = u.reloadInterfaceFonts
	shown, offer := launchLanguage(iface, languages.han)
	i18n.SetCurrent(shown)
	if offer {
		languages.offer.show(iface, i18n.SetCurrent)
	}
	if *flagHanFontOffer != "" {
		languages.offer.show(i18n.ZhCN, u.settings.keepLanguage)
		if *flagHanFontOffer == "download" {
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
// the choice, once a Chinese font is in place for Chinese.
func (s *settingsScreen) chooseLanguage(l i18n.Locale) {
	if needsHanFont(l, s.languages.han) {
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

// hanFontOffer is the dialog that offers to download Noto Sans SC before the
// interface switches to target, and the download it starts. The download
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
		err := fonts.Fetch(ctx, fonts.NotoSansSC, dir, func(done, _ int64) {
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
	if err == nil && !s.languages.han {
		err = errors.New("the downloaded font does not draw Chinese")
	}
	if err != nil {
		s.hanFontFailed(err)
		return
	}
	apply(target)
	s.invalidate()
}

func (s *settingsScreen) hanFontFailed(err error) {
	title := i18n.T("settings.language.hanFontFailed", i18n.Args{"error": err.Error()})
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
	total := fonts.TotalSize(fonts.NotoSansSC)
	pop := th.On(th.Popover)
	d := th.Dialog(&o.dialog, o.open, i18n.T("settings.language.hanFontTitle"),
		i18n.T("settings.language.hanFontDescription", i18n.Args{"size": wholeMegabytes(total)}))
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
