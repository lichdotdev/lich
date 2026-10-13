package main

import (
	"log"
	"strings"
	"sync"

	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n (settings.claudeProfile*,
// claudeProfileDialogs.*).
const (
	profileTitle  = "Profile"
	profileAlso   = "claude account login personal work"
	profileDesc   = "Which Claude login new sessions use. A session keeps the profile it was born in, resumes included."
	profilesTitle = "Profiles"
	profilesAlso  = "claude accounts logins personal work log in plugins"
	profilesDesc  = "Each profile is its own Claude login with its own plugins and history. Settings, CLAUDE.md, agents, commands, skills and hooks are the same in all of them."
	profilesCopy  = "Settings, CLAUDE.md, agents, commands, skills and hooks are copies in this profile: editing them in another profile no longer reaches it."

	claudeCreating      = "Creating the profile…"
	claudeStartingLogin = "Starting the login…"
	claudeRenaming      = "Renaming…"
	claudeDeleting      = "Deleting…"
	claudeInstalling    = "Installing plugins…"
	claudeLoginFailed   = "The login did not complete."
	claudeNoTerminal    = "Open a project to log in from a terminal."
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
	target lichclient.ClaudeProfile
	// fresh asks the frame to empty the inputs, which only it may touch;
	// refresh asks it to read Settings again, which a delete made stale.
	fresh   bool
	refresh bool
	working string
	// keys are the plugins to install once the login is through.
	keys    []string
	url     string
	failure string
	account string
	results []lichclient.ClaudeProfilePluginResult
	// pluginNote is why the lich plugin did not install, in the backend's
	// words.
	pluginNote string
	plugins    []lichclient.ClaudeProfilePlugin
	source     string
}

// profileFlow is the dialog every profile action runs in: New profile, Rename,
// Delete, Copy plugins, and the login that follows a creation.
type profileFlow struct {
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
func (f *profileFlow) checkedKeys(plugins []lichclient.ClaudeProfilePlugin) []string {
	var keys []string
	for _, p := range plugins {
		if c := f.checks[p.Key]; c == nil || c.Value {
			keys = append(keys, p.Key)
		}
	}
	return keys
}

// claudeSettings is the Claude Code screen's own state beyond the generic
// provider blocks: the profile list's rows, the picker and the dialog.
type claudeSettings struct {
	book *profileBook
	// openTerminal runs a command in a new shell card and takes Settings out
	// of the way; the window wires it.
	openTerminal func(command string) bool
	// asked is whether the book was read again on this visit.
	asked    bool
	advanced bool
	fold     widget.Clickable
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

func (c *claudeSettings) row(slug string) *profileRowState {
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
func (s *settingsScreen) ensureProfiles() {
	c := &s.providers.claude
	if c.asked || c.book == nil {
		return
	}
	c.asked = true
	go s.reloadProfiles()
}

func (s *settingsScreen) reloadProfiles() {
	if err := s.providers.claude.book.reload(s.ctx); err != nil {
		log.Printf("settings: claude profiles: %v", err)
	}
	s.invalidate()
}

// setClaudeProfile stores the profile the open scope's new sessions are born
// in.
func (s *settingsScreen) setClaudeProfile(slug string) {
	s.writeProviderKey(claudeProfileKey, s.scope, profileChoiceValue(slug, s.scope), nil)
}

// startNewProfile opens the New profile dialog on the plugins Personal has.
func (s *settingsScreen) startNewProfile() {
	f := &s.providers.claude.flow
	f.set(func(d *flowData) {
		*d = flowData{kind: flowNew, open: true, fresh: true, source: personalProfile}
	})
	f.name.SetText("")
	f.focus = true
	go s.loadPluginChoices(personalProfile)
	s.invalidate()
}

// startRename, startDelete and startCopy open the dialog of their name on p.
func (s *settingsScreen) startRename(p lichclient.ClaudeProfile) {
	f := &s.providers.claude.flow
	f.set(func(d *flowData) { *d = flowData{kind: flowRename, open: true, fresh: true, target: p} })
	f.name.SetText(p.Name)
	f.focus = true
	s.invalidate()
}

func (s *settingsScreen) startDelete(p lichclient.ClaudeProfile) {
	f := &s.providers.claude.flow
	f.set(func(d *flowData) { *d = flowData{kind: flowDelete, open: true, target: p} })
	s.invalidate()
}

func (s *settingsScreen) startCopy(p lichclient.ClaudeProfile) {
	f := &s.providers.claude.flow
	list, _ := s.providers.claude.book.snapshot()
	source := defaultCopySource(list, p.Slug)
	f.set(func(d *flowData) { *d = flowData{kind: flowCopy, open: true, fresh: true, target: p, source: source} })
	go s.loadPluginChoices(source)
	s.invalidate()
}

// defaultCopySource is where a copy reads from unless asked otherwise:
// Personal, or the first other profile when Personal is the target.
func defaultCopySource(list []lichclient.ClaudeProfile, target string) string {
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
func (s *settingsScreen) loadPluginChoices(slug string) {
	f := &s.providers.claude.flow
	plugins, err := s.client.ClaudeProfilePlugins(s.ctx, slug)
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
func (s *settingsScreen) pickCopySource(slug string) {
	f := &s.providers.claude.flow
	f.set(func(d *flowData) { d.source, d.plugins = slug, nil })
	go s.loadPluginChoices(slug)
}

// confirmFlow is the dialog's primary button.
func (s *settingsScreen) confirmFlow(d flowData) {
	f := &s.providers.claude.flow
	switch d.kind {
	case flowNew:
		s.createProfile(strings.TrimSpace(f.name.Text()), f.checkedKeys(d.plugins))
	case flowRename:
		s.renameProfile(d.target.Slug, strings.TrimSpace(f.name.Text()))
	case flowDelete:
		s.deleteProfile(d.target.Slug)
	case flowCopy:
		s.copyPlugins(d.target, f.checkedKeys(d.plugins))
	}
}

// failFlow puts the dialog back on its form with the backend's sentence.
func (s *settingsScreen) failFlow(err error) {
	text := lichclient.ErrorText(err)
	s.providers.claude.flow.set(func(d *flowData) { d.step, d.failure = stepForm, text })
	s.invalidate()
}

func (s *settingsScreen) createProfile(name string, keys []string) {
	f := &s.providers.claude.flow
	f.set(func(d *flowData) { d.step, d.working, d.failure = stepWorking, claudeCreating, "" })
	go func() {
		p, err := s.client.CreateClaudeProfile(s.ctx, name)
		if err != nil {
			s.failFlow(err)
			return
		}
		go s.reloadProfiles()
		if view := f.view(); !view.open || view.kind != flowNew {
			return
		}
		s.startLogin(lichclient.ClaudeProfile{Slug: p.Slug, Name: p.Name}, keys)
	}()
}

func (s *settingsScreen) renameProfile(slug, name string) {
	f := &s.providers.claude.flow
	f.set(func(d *flowData) { d.step, d.working, d.failure = stepWorking, claudeRenaming, "" })
	go func() {
		if err := s.client.RenameClaudeProfile(s.ctx, slug, name); err != nil {
			s.failFlow(err)
			return
		}
		f.set(func(d *flowData) { d.open = false })
		s.reloadProfiles()
	}()
}

func (s *settingsScreen) deleteProfile(slug string) {
	f := &s.providers.claude.flow
	f.set(func(d *flowData) { d.step, d.working, d.failure = stepWorking, claudeDeleting, "" })
	go func() {
		if err := s.client.DeleteClaudeProfile(s.ctx, slug); err != nil {
			s.failFlow(err)
			return
		}
		f.set(func(d *flowData) { d.open, d.refresh = false, true })
		s.reloadProfiles()
	}()
}

func (s *settingsScreen) copyPlugins(target lichclient.ClaudeProfile, keys []string) {
	f := &s.providers.claude.flow
	f.set(func(d *flowData) { d.step, d.working, d.failure = stepWorking, claudeInstalling, "" })
	go func() {
		results, err := s.client.InstallClaudeProfilePlugins(s.ctx, target.Slug, keys)
		if err != nil {
			s.failFlow(err)
			return
		}
		f.set(func(d *flowData) { d.step, d.results = stepResult, results })
		s.reloadProfiles()
	}()
}

// startLogin runs the login of p in the dialog and installs keys into it once
// it is through. It is called from any goroutine.
func (s *settingsScreen) startLogin(p lichclient.ClaudeProfile, keys []string) {
	f := &s.providers.claude.flow
	f.set(func(d *flowData) {
		*d = flowData{kind: flowLogin, open: true, fresh: true, step: stepWorking, working: claudeStartingLogin, target: p, keys: keys}
	})
	s.invalidate()
	go func() {
		err := s.client.LoginClaudeProfile(s.ctx, p.Slug)
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

// loginProgress takes one step of a login the backend reports: the sign-in
// address while the CLI waits, then how it ended.
func (s *settingsScreen) loginProgress(ev lichclient.ClaudeProfileLogin) {
	f := &s.providers.claude.flow
	d := f.view()
	mine := d.open && d.kind == flowLogin && d.target.Slug == ev.Profile
	switch {
	case !ev.Done:
		if mine && ev.URL != "" {
			f.set(func(d *flowData) { d.url = ev.URL })
		}
	case !mine:
	case ev.Cancelled:
		f.set(func(d *flowData) { d.open = false })
	case !ev.LoggedIn:
		failure := ev.Error
		if failure == "" {
			failure = claudeLoginFailed
		}
		f.set(func(d *flowData) { d.step, d.failure = stepResult, failure })
	default:
		go s.finishLogin(d, ev)
	}
	s.invalidate()
}

// finishLogin reads who is logged in now, then brings the plugins that were
// asked for: they need the login, so they come last.
func (s *settingsScreen) finishLogin(d flowData, ev lichclient.ClaudeProfileLogin) {
	c := &s.providers.claude
	c.book.forgetPlans()
	s.reloadProfiles()
	account := ""
	if p, ok := c.book.find(d.target.Slug); ok {
		account = loggedInAs(p)
	}
	var results []lichclient.ClaudeProfilePluginResult
	if len(d.keys) > 0 {
		c.flow.set(func(d *flowData) { d.step, d.working = stepWorking, claudeInstalling })
		s.invalidate()
		var err error
		if results, err = s.client.InstallClaudeProfilePlugins(s.ctx, d.target.Slug, d.keys); err != nil {
			ev.PluginError = lichclient.ErrorText(err)
		}
	}
	c.flow.set(func(d *flowData) {
		d.step, d.account, d.results, d.pluginNote = stepResult, account, results, ev.PluginError
	})
	s.invalidate()
}

// dismissFlow closes the dialog; a login still running is ended with it.
func (s *settingsScreen) dismissFlow(d flowData) {
	s.providers.claude.flow.set(func(data *flowData) { data.open = false })
	if d.kind == flowLogin && d.step != stepResult {
		go func() {
			if err := s.client.CancelClaudeLogin(s.ctx, d.target.Slug); err != nil {
				log.Printf("settings: cancel login %s: %v", d.target.Slug, err)
			}
		}()
	}
}

// loginInTerminal ends the login lich runs and hands the same command to a
// shell card, for a browser lich's own flow cannot work with.
func (s *settingsScreen) loginInTerminal(d flowData) {
	c := &s.providers.claude
	go func() {
		command, err := s.client.ClaudeLoginCommand(s.ctx, d.target.Slug)
		if err != nil {
			s.failFlow(err)
			return
		}
		if c.openTerminal == nil || !c.openTerminal(command) {
			c.flow.set(func(d *flowData) { d.failure = claudeNoTerminal })
			s.invalidate()
			return
		}
		s.dismissFlow(d)
	}()
}
