package main

import (
	"cmp"
	"slices"
	"strings"

	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// /permissions and /hooks open the provider's permission rules and hooks
// (api.ChatPermissions, api.ChatHooks) over the composer, the screens its
// TUI opens for them. Codex reports no rules: its /permissions is the mode,
// which the panel lists for it to be picked.

const (
	chatPermissionsCommand = "/permissions"
	chatHooksCommand       = "/hooks"
)

// The /permissions panel's tabs, in the order they are drawn: one per
// behavior, then the folders.
const (
	permTabAllow = iota
	permTabAsk
	permTabDeny
	permTabFolders
)

var permTabBehaviors = []string{api.PermissionAllow, api.PermissionAsk, api.PermissionDeny}

// permAddDetail is the detail that adds a rule to the tab's behavior.
const permAddDetail = "+add"

// chatSettings is the /permissions and /hooks panels' own state; chatTools
// holds what they share with the other panels.
type chatSettings struct {
	// perms and hooks are the lists read, note what the last action left to
	// say, running the action still going and saved a rule the box gives up;
	// all guarded by chatTools.mu.
	perms   api.ChatPermissionList
	hooks   api.ChatHookList
	note    string
	running bool
	saved   bool
	tabs    ui.TabsState
	rule    widget.Editor
}

func (s *chatSettings) reset() {
	s.tabs.Selected, s.note, s.saved = permTabAllow, "", false
	s.rule.SingleLine = true
	s.rule.SetText("")
}

// ruleKey names a rule in the detail: the same text can be a rule of two
// behaviors and in two files.
func ruleKey(r api.ChatPermissionRule) string {
	return r.Behavior + "|" + r.Source + "|" + r.Rule
}

// tabRules is the rules of behavior, by their text as the TUI sorts them.
func tabRules(l api.ChatPermissionList, behavior string) []api.ChatPermissionRule {
	out := slices.DeleteFunc(slices.Clone(l.Rules), func(r api.ChatPermissionRule) bool { return r.Behavior != behavior })
	slices.SortStableFunc(out, func(a, b api.ChatPermissionRule) int {
		return cmp.Compare(strings.ToLower(a.Rule), strings.ToLower(b.Rule))
	})
	return out
}

func noRowEnter(int) {}

// permissionActs runs the /permissions panel's rows, detail and add form,
// and gives toolsActs the rows shown and what Enter does on one. A provider
// that reports no rules lists its modes instead.
func (u *window) permissionActs(gtx C, p *chatPane, id string) (int, func(int)) {
	t := &p.tools
	s := &t.set
	t.mu.Lock()
	l, saved := s.perms, s.saved
	s.saved = false
	t.mu.Unlock()
	if saved {
		s.rule.SetText("")
		t.detail, t.focus = "", true
	}
	if !l.Reported {
		return u.modeActs(p, id)
	}
	tab := s.tabs.Selected
	switch {
	case t.detail == permAddDetail:
		return u.addRuleActs(gtx, p, id, l)
	case t.detail != "":
		u.ruleDetailActs(gtx, p, id, l)
		return 0, noRowEnter
	case tab == permTabFolders:
		return 0, noRowEnter
	}
	rules := tabRules(l, permTabBehaviors[tab])
	add := len(l.Destinations) > 0
	n := len(rules)
	if add {
		n++
	}
	return n, func(i int) {
		if add && i == 0 {
			t.detail, t.focus, t.cursor = permAddDetail, true, 0
			return
		}
		if add {
			i--
		}
		t.detail, t.focus = ruleKey(rules[i]), true
	}
}

// addRuleActs runs the add form: a row per file, which saves the rule typed
// there as a rule of the tab's behavior.
func (u *window) addRuleActs(gtx C, p *chatPane, id string, l api.ChatPermissionList) (int, func(int)) {
	t := &p.tools
	tab := t.set.tabs.Selected
	if t.back.Clicked(gtx) || tab == permTabFolders {
		t.detail, t.focus = "", true
		return 0, noRowEnter
	}
	dests := l.Destinations
	return len(dests), func(i int) {
		if rule := strings.TrimSpace(t.set.rule.Text()); rule != "" {
			u.permissionAct(p, id, api.ChatPermissionActOptions{Action: api.PermissionAdd, Behavior: permTabBehaviors[tab], Rule: rule, Source: dests[i]})
		}
	}
}

// ruleDetailActs runs a rule's Back and Remove; a rule gone takes the panel
// back to its tab.
func (u *window) ruleDetailActs(gtx C, p *chatPane, id string, l api.ChatPermissionList) {
	t := &p.tools
	i := slices.IndexFunc(l.Rules, func(r api.ChatPermissionRule) bool { return ruleKey(r) == t.detail })
	if t.back.Clicked(gtx) || i < 0 {
		t.detail, t.focus = "", true
		return
	}
	if r := l.Rules[i]; r.Removable && t.act(chatPluginAction{action: api.PermissionRemove}.key()).Clicked(gtx) {
		u.permissionAct(p, id, api.ChatPermissionActOptions{Action: api.PermissionRemove, Behavior: r.Behavior, Rule: r.Rule, Source: r.Source})
	}
}

// modeActs is Codex's /permissions: its modes, the one picked set from the
// next message on and the panel shut, as its TUI's picker does; nothing while
// a turn runs, as the mode chip.
func (u *window) modeActs(p *chatPane, id string) (int, func(int)) {
	opt, ok := u.model.chat.optionOf(id, api.OptionMode)
	if !ok || u.chatBusy(id) {
		return len(opt.Choices), noRowEnter
	}
	return len(opt.Choices), func(i int) {
		u.setChatOption(id, opt.ID, opt.Choices[i].ID)
		p.tools.shut()
	}
}

// chatBusy reports whether session id's agent is in a turn, when its options
// wait.
func (u *window) chatBusy(id string) bool {
	sv, ok := u.model.sessionViewOf(id)
	return ok && chatWorking(sv, u.model.chat.isRunning(id))
}

// permissionAct adds or removes a rule and reads the rules again, keeping the
// warnings the provider gave. One runs at a time: the rows and buttons wait
// on it.
func (u *window) permissionAct(p *chatPane, id string, opts api.ChatPermissionActOptions) {
	t := &p.tools
	t.mu.Lock()
	if t.set.running {
		t.mu.Unlock()
		return
	}
	t.set.running, t.set.note = true, ""
	t.mu.Unlock()
	opts.ID = id
	go func() {
		res, err := u.client.ChatPermissionAct(u.ctx, opts)
		t.mu.Lock()
		t.set.running, t.set.note = false, strings.Join(res.Warnings, " ")
		t.set.saved = err == nil && opts.Action == api.PermissionAdd
		t.mu.Unlock()
		u.chatFailed(err)
		u.fetchTools(p, id)
	}()
}

func permSourceLabel(source string) string {
	switch source {
	case api.SettingsUser:
		return i18n.T("chat.permissions.source.user")
	case api.SettingsProject:
		return i18n.T("chat.permissions.source.project")
	case api.SettingsLocal:
		return i18n.T("chat.permissions.source.local")
	case api.SettingsManaged:
		return i18n.T("chat.permissions.source.managed")
	case api.SettingsFlag:
		return i18n.T("chat.permissions.source.flag")
	case api.SettingsSession:
		return i18n.T("chat.permissions.source.session")
	case api.SettingsPlugin:
		return i18n.T("chat.permissions.source.plugin")
	}
	return i18n.T("chat.permissions.source.misc")
}

// permDestination is who a rule saved in a file reaches, and the file in
// the checkout; the person's own is in the profile lich runs, unnamed.
func permDestination(source string) (string, string) {
	switch source {
	case api.SettingsLocal:
		return i18n.T("chat.permissions.dest.local"), ".claude/settings.local.json"
	case api.SettingsProject:
		return i18n.T("chat.permissions.dest.project"), ".claude/settings.json"
	}
	return i18n.T("chat.permissions.dest.user"), ""
}
