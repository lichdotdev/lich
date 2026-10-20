package main

import (
	"log"
	"strings"
	"sync"

	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
)

// flowKind is what the profile dialog is doing.
type flowKind uint8

const (
	flowNew flowKind = iota
	flowRename
	flowDelete
	flowCopy
	flowLogin
)

// flowStep is how far along the dialog is: asking, waiting on the backend, or
// waiting on the user's browser, then the outcome.
type flowStep uint8

const (
	stepForm flowStep = iota
	stepWorking
	stepWaiting
	stepResult
)

// flowData is what the backend's goroutines write into the dialog and the
// frame reads; widgets stay with the frame.
type flowData struct {
	kind   flowKind
	open   bool
	step   flowStep
	target lichclient.Profile
	// fresh asks the frame to empty the inputs, which only it may touch;
	// refresh asks it to read Settings again, which a delete made stale.
	fresh   bool
	refresh bool
	working string
	// keys are the plugins to install once the login is through.
	keys []string
	url  string
	// authenticating is a login past the browser, or handed a code: the field
	// is hidden until the login ends or the hand-over fails.
	authenticating bool
	failure        string
	account        string
	results        []lichclient.ProfilePluginResult
	// pluginNote is why the lich plugin did not install, in the backend's
	// words.
	pluginNote string
	plugins    []lichclient.ProfilePlugin
	source     string
}

// profileFlow is the dialog every profile action runs in: New profile, Rename,
// Delete, Copy plugins, and the login that follows a creation.
type profileFlow struct {
	// ps is the provider's profiles the dialog belongs to.
	ps   *profileSettings
	mu   sync.Mutex
	data flowData

	dialog  ui.DialogState
	name    widget.Editor
	code    widget.Editor
	checks  map[string]*widget.Bool
	focus   bool
	source  ui.DropdownPanelState
	options map[string]*widget.Clickable
	list    widget.List

	cancel, confirm, done              widget.Clickable
	submitCode, openURL, copyURL, term widget.Clickable
}

func (f *profileFlow) view() flowData {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.data
}

func (f *profileFlow) set(change func(d *flowData)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change(&f.data)
}

// checkedKeys are the plugins the checklist has ticked, in the list's order.
// A plugin is ticked until the user unticks it.
func (f *profileFlow) checkedKeys(plugins []lichclient.ProfilePlugin) []string {
	var keys []string
	for _, p := range plugins {
		if c := f.checks[p.Key]; c == nil || c.Value {
			keys = append(keys, p.Key)
		}
	}
	return keys
}

// profileSettings is the profiles' own state of a provider's screen beyond the
// generic provider blocks: the profile list's rows, the picker and the dialog.
type profileSettings struct {
	provider profileProvider
	book     *profileBook
	// openTerminal runs a command in a new shell card and takes Settings out
	// of the way; the window wires it.
	openTerminal func(command string) bool
	// asked is whether the book was read again on this visit.
	asked    bool
	advanced bool
	fold     widget.Clickable
	binary   profileBinaryLinks
	picker   ui.DropdownPanelState
	options  map[string]*widget.Clickable
	rows     map[string]*profileRowState
	add      widget.Clickable
	flow     profileFlow
}

type profileRowState struct {
	tip   ui.Tooltip
	login widget.Clickable
	more  widget.Clickable
	menu  ui.MenuState
}

// bind connects the screen's profiles to the window's book of them.
func (c *profileSettings) bind(book *profileBook, openTerminal func(command string) bool) {
	c.provider, c.book, c.openTerminal = book.provider, book, openTerminal
	c.flow.ps = c
}

func (c *profileSettings) row(slug string) *profileRowState {
	if c.rows == nil {
		c.rows = map[string]*profileRowState{}
	}
	if c.rows[slug] == nil {
		c.rows[slug] = new(profileRowState)
	}
	return c.rows[slug]
}

// ensureProfiles has the book read again once per visit: a login finished in a
// terminal, or a plugin installed outside lich, shows the next time Settings
// opens.
func (s *settingsScreen) ensureProfiles(c *profileSettings) {
	if c.asked || c.book == nil {
		return
	}
	c.asked = true
	go s.reloadProfiles(c)
}

func (s *settingsScreen) reloadProfiles(c *profileSettings) {
	if err := c.book.reload(s.ctx); err != nil {
		log.Printf("settings: %s profiles: %v", c.provider.kind, err)
	}
	s.invalidate()
}

// setProfile stores the profile the open scope's new sessions are born in.
func (s *settingsScreen) setProfile(c *profileSettings, slug string) {
	s.writeProviderKey(c.provider.key, s.scope, profileChoiceValue(slug, s.scope), nil)
}

// startNewProfile opens the New profile dialog on the plugins Personal has.
func (s *settingsScreen) startNewProfile(c *profileSettings) {
	f := &c.flow
	f.set(func(d *flowData) {
		*d = flowData{kind: flowNew, open: true, fresh: true, source: personalProfile}
	})
	f.name.SetText("")
	f.focus = true
	if c.provider.plugins {
		go s.loadPluginChoices(c, personalProfile)
	}
	s.invalidate()
}

// startRename, startDelete and startCopy open the dialog of their name on p.
func (s *settingsScreen) startRename(c *profileSettings, p lichclient.Profile) {
	f := &c.flow
	f.set(func(d *flowData) { *d = flowData{kind: flowRename, open: true, fresh: true, target: p} })
	f.name.SetText(p.Name)
	f.focus = true
	s.invalidate()
}

func (s *settingsScreen) startDelete(c *profileSettings, p lichclient.Profile) {
	f := &c.flow
	f.set(func(d *flowData) { *d = flowData{kind: flowDelete, open: true, target: p} })
	s.invalidate()
}

func (s *settingsScreen) startCopy(c *profileSettings, p lichclient.Profile) {
	f := &c.flow
	list, _ := c.book.snapshot()
	source := defaultCopySource(list, p.Slug)
	f.set(func(d *flowData) { *d = flowData{kind: flowCopy, open: true, fresh: true, target: p, source: source} })
	go s.loadPluginChoices(c, source)
	s.invalidate()
}

// defaultCopySource is where a copy reads from unless asked otherwise:
// Personal, or the first other profile when Personal is the target.
func defaultCopySource(list []lichclient.Profile, target string) string {
	if target != personalProfile {
		return personalProfile
	}
	for _, p := range list {
		if p.Slug != personalProfile {
			return p.Slug
		}
	}
	return personalProfile
}

// loadPluginChoices reads the plugins slug has, for the checklist.
func (s *settingsScreen) loadPluginChoices(c *profileSettings, slug string) {
	f := &c.flow
	plugins, err := c.provider.api(s.client).Plugins(s.ctx, slug)
	if err != nil {
		log.Printf("settings: plugins of %s: %v", slug, err)
	}
	f.set(func(d *flowData) {
		if d.source == slug {
			d.plugins = plugins
		}
	})
	s.invalidate()
}

// pickCopySource changes the profile a copy reads its checklist from.
func (s *settingsScreen) pickCopySource(c *profileSettings, slug string) {
	f := &c.flow
	f.set(func(d *flowData) { d.source, d.plugins = slug, nil })
	go s.loadPluginChoices(c, slug)
}

// confirmFlow is the dialog's primary button.
func (s *settingsScreen) confirmFlow(c *profileSettings, d flowData) {
	f := &c.flow
	switch d.kind {
	case flowNew:
		s.createProfile(c, strings.TrimSpace(f.name.Text()), f.checkedKeys(d.plugins))
	case flowRename:
		s.renameProfile(c, d.target.Slug, strings.TrimSpace(f.name.Text()))
	case flowDelete:
		s.deleteProfile(c, d.target.Slug)
	case flowCopy:
		s.copyPlugins(c, d.target, f.checkedKeys(d.plugins))
	}
}

// failFlow puts the dialog back on its form with the backend's sentence.
func (s *settingsScreen) failFlow(c *profileSettings, err error) {
	text := lichclient.ErrorText(err)
	c.flow.set(func(d *flowData) { d.step, d.failure = stepForm, text })
	s.invalidate()
}

func (s *settingsScreen) createProfile(c *profileSettings, name string, keys []string) {
	f := &c.flow
	f.set(func(d *flowData) {
		d.step, d.working, d.failure = stepWorking, i18n.T("settings.settingsProfiles.creating"), ""
	})
	go func() {
		p, err := c.provider.api(s.client).Create(s.ctx, name)
		if err != nil {
			s.failFlow(c, err)
			return
		}
		go s.reloadProfiles(c)
		if view := f.view(); !view.open || view.kind != flowNew {
			return
		}
		s.startLogin(c, lichclient.Profile{Slug: p.Slug, Name: p.Name}, keys)
	}()
}

func (s *settingsScreen) renameProfile(c *profileSettings, slug, name string) {
	f := &c.flow
	f.set(func(d *flowData) {
		d.step, d.working, d.failure = stepWorking, i18n.T("settings.settingsProfiles.renaming"), ""
	})
	go func() {
		if err := c.provider.api(s.client).Rename(s.ctx, slug, name); err != nil {
			s.failFlow(c, err)
			return
		}
		f.set(func(d *flowData) { d.open = false })
		s.reloadProfiles(c)
	}()
}

func (s *settingsScreen) deleteProfile(c *profileSettings, slug string) {
	f := &c.flow
	f.set(func(d *flowData) {
		d.step, d.working, d.failure = stepWorking, i18n.T("settings.settingsProfiles.deleting"), ""
	})
	go func() {
		if err := c.provider.api(s.client).Delete(s.ctx, slug); err != nil {
			s.failFlow(c, err)
			return
		}
		f.set(func(d *flowData) { d.open, d.refresh = false, true })
		s.reloadProfiles(c)
	}()
}

func (s *settingsScreen) copyPlugins(c *profileSettings, target lichclient.Profile, keys []string) {
	f := &c.flow
	f.set(func(d *flowData) {
		d.step, d.working, d.failure = stepWorking, i18n.T("settings.settingsProfiles.installing"), ""
	})
	go func() {
		results, err := c.provider.api(s.client).InstallPlugins(s.ctx, target.Slug, keys)
		if err != nil {
			s.failFlow(c, err)
			return
		}
		f.set(func(d *flowData) { d.step, d.results = stepResult, results })
		s.reloadProfiles(c)
	}()
}

// startLogin runs the login of p in the dialog and installs keys into it once
// it is through. It is called from any goroutine.
func (s *settingsScreen) startLogin(c *profileSettings, p lichclient.Profile, keys []string) {
	f := &c.flow
	if c.provider.terminalLogin {
		s.startTerminalLogin(c, p)
		return
	}
	f.set(func(d *flowData) {
		*d = flowData{kind: flowLogin, open: true, fresh: true, step: stepWorking, working: i18n.T("settings.settingsProfiles.startingLogin"), target: p, keys: keys}
	})
	s.invalidate()
	go func() {
		err := c.provider.api(s.client).Login(s.ctx, p.Slug)
		f.set(func(d *flowData) {
			switch {
			case err != nil:
				d.step, d.failure = stepResult, lichclient.ErrorText(err)
			case d.step == stepWorking:
				d.step = stepWaiting
			}
		})
		s.invalidate()
	}()
}

// startTerminalLogin hands the login of p to a shell card, the only place a
// provider with terminalLogin runs one, and closes the dialog. Where there is no
// card to open it in, the dialog ends on the failure.
func (s *settingsScreen) startTerminalLogin(c *profileSettings, p lichclient.Profile) {
	f := &c.flow
	f.set(func(d *flowData) {
		*d = flowData{kind: flowLogin, open: true, fresh: true, step: stepWorking, working: i18n.T("settings.settingsProfiles.startingLogin"), target: p}
	})
	s.invalidate()
	go func() {
		command, err := c.provider.api(s.client).LoginCommand(s.ctx, p.Slug)
		switch {
		case err != nil:
			f.set(func(d *flowData) { d.step, d.failure = stepResult, lichclient.ErrorText(err) })
		case c.openTerminal == nil || !c.openTerminal(command):
			f.set(func(d *flowData) { d.step, d.failure = stepResult, i18n.T("settings.settingsProfiles.noTerminal") })
		default:
			f.set(func(d *flowData) { d.open = false })
		}
		s.invalidate()
	}()
}

// loginProgress takes one step of a login of the provider kind names, as the
// backend reports it: the sign-in address while the CLI waits, then how it
// ended.
func (s *settingsScreen) loginProgress(kind string, ev lichclient.ProfileLogin) {
	c := s.providers.profiles(kind)
	if c == nil {
		return
	}
	f := &c.flow
	d := f.view()
	mine := d.open && d.kind == flowLogin && d.target.Slug == ev.Profile
	switch {
	case !ev.Done:
		if mine && ev.URL != "" {
			f.set(func(d *flowData) { d.url = ev.URL })
		}
		if mine && ev.Authenticating {
			f.set(func(d *flowData) { d.authenticating = true })
		}
	case !mine:
	case ev.Cancelled:
		f.set(func(d *flowData) { d.open = false })
	case !ev.LoggedIn:
		failure := ev.Error
		if failure == "" {
			failure = i18n.T("settings.settingsProfiles.loginFailed")
		}
		f.set(func(d *flowData) { d.step, d.failure = stepResult, failure })
	default:
		go s.finishLogin(c, d, ev)
	}
	s.invalidate()
}

// finishLogin reads who is logged in now, then brings the plugins that were
// asked for: they need the login, so they come last.
func (s *settingsScreen) finishLogin(c *profileSettings, d flowData, ev lichclient.ProfileLogin) {
	c.book.planChanged(d.target.Slug)
	s.reloadProfiles(c)
	account := ""
	if p, ok := c.book.find(d.target.Slug); ok {
		account = loggedInAs(p)
	}
	var results []lichclient.ProfilePluginResult
	if len(d.keys) > 0 {
		c.flow.set(func(d *flowData) { d.step, d.working = stepWorking, i18n.T("settings.settingsProfiles.installing") })
		s.invalidate()
		var err error
		if results, err = c.provider.api(s.client).InstallPlugins(s.ctx, d.target.Slug, d.keys); err != nil {
			ev.PluginError = lichclient.ErrorText(err)
		}
	}
	c.flow.set(func(d *flowData) {
		d.step, d.account, d.results, d.pluginNote = stepResult, account, results, ev.PluginError
	})
	s.invalidate()
}

// dismissFlow closes the dialog; a login still running is ended with it.
func (s *settingsScreen) dismissFlow(c *profileSettings, d flowData) {
	c.flow.set(func(data *flowData) { data.open = false })
	if d.kind == flowLogin && d.step != stepResult {
		go func() {
			if err := c.provider.api(s.client).CancelLogin(s.ctx, d.target.Slug); err != nil {
				log.Printf("settings: cancel login %s: %v", d.target.Slug, err)
			}
		}()
	}
}

// loginInTerminal ends the login lich runs and hands the same command to a
// shell card, for a browser lich's own flow cannot work with.
func (s *settingsScreen) loginInTerminal(c *profileSettings, d flowData) {
	go func() {
		command, err := c.provider.api(s.client).LoginCommand(s.ctx, d.target.Slug)
		if err != nil {
			s.failFlow(c, err)
			return
		}
		if c.openTerminal == nil || !c.openTerminal(command) {
			c.flow.set(func(d *flowData) { d.failure = i18n.T("settings.settingsProfiles.noTerminal") })
			s.invalidate()
			return
		}
		s.dismissFlow(c, d)
	}()
}
