package main

import (
	"slices"
	"strings"
	"sync"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/ui"
)

// /mcp, /skills and /plugin are lich's own commands where the provider
// reports its MCP servers, skills and plugins (api.ChatMCP, api.ChatSkills,
// api.ChatPlugins): instead of reaching the agent as text they open a panel
// over the composer, the screen the provider's TUI opens for them.

const (
	chatMCPCommand    = "/mcp"
	chatSkillsCommand = "/skills"
	chatPluginCommand = "/plugin"
)

// chatToolKinds is the providers whose chat reports its MCP servers, skills
// and plugins; elsewhere the commands go to the agent as they always did.
var chatToolKinds = []string{claudeKind, codexKind}

// chatToolsPoll is how often an open panel reads a server still connecting
// again: the provider answers a reconnect before its state changes.
const chatToolsPoll = 2 * time.Second

// chatTools is one chat's /mcp, /skills or /plugin panel, shut while kind is
// "".
type chatTools struct {
	kind string
	// mu guards what the reads and the plugin actions answer, which land off
	// the frame.
	mu       sync.Mutex
	mcp      api.ChatMCPList
	skills   api.ChatSkillList
	plugins  api.ChatPluginList
	loaded   bool
	fetching bool
	// gen counts the reads that landed, for what is kept from one.
	gen  int
	plug chatPlugins
	set  chatSettings
	mem  chatMemory
	// shell is the output of the shell the shells' panel shows.
	shell chatShellOut
	// base is the box's length when the panel opened: typing past it shuts
	// the panel.
	base int
	// next is when a server still connecting is read again.
	next time.Time
	// focus hands the keys to the panel on the frame after it opens.
	focus bool
	// detail is the server the panel shows, "" for the list; showTools opens
	// its tools.
	detail    string
	showTools bool
	query     widget.Editor
	byTokens  bool
	cursor    int
	moved     bool
	rows      []ui.Clickable
	back      ui.Clickable
	close     ui.Clickable
	toolsBtn  ui.Clickable
	sortBtn   ui.Clickable
	acts      map[string]*ui.Clickable
	list      widget.List
	// outside are the tags of the presses around the open panel, one per
	// band.
	outside [4]int
}

func (t *chatTools) open(kind string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.kind, t.loaded, t.detail, t.showTools, t.cursor, t.moved, t.next, t.focus = kind, false, "", false, 0, true, time.Time{}, true
	t.base, t.shell.id = 0, ""
	t.query.SetText("")
	t.plug.reset()
	t.set.reset()
	t.mem.reset()
}

func (t *chatTools) shut() { t.kind, t.detail = "", "" }

func (t *chatTools) opened() bool { return t.kind != "" }

// act is the clickable of action a in the detail view.
func (t *chatTools) act(a string) *ui.Clickable {
	if t.acts == nil {
		t.acts = map[string]*ui.Clickable{}
	}
	c, ok := t.acts[a]
	if !ok {
		c = new(ui.Clickable)
		t.acts[a] = c
	}
	return c
}

// openTools opens the panel text names, and reports whether it named one the
// session's provider has.
func (u *window) openTools(p *chatPane, id, text string) bool {
	kind, kinds := "", chatToolKinds
	switch text {
	case chatMCPCommand, chatSkillsCommand, chatPluginCommand, chatPermissionsCommand, chatHooksCommand:
		kind = text
	case chatShellsPanel, chatBashesCommand:
		kind, kinds = chatShellsPanel, chatShellKinds
	case chatMemoryCommand, chatAgentsCommand:
		kind, kinds = text, chatMemoryKinds
	}
	sv, ok := u.model.sessionViewOf(id)
	if kind == "" || !ok || !slices.Contains(kinds, sv.Kind) {
		return false
	}
	p.editor.SetText("")
	p.undo.Reset()
	if kind == chatShellsPanel {
		p.openPanel(chatShellsPanel)
		return true
	}
	p.tools.open(kind)
	u.fetchTools(p, id)
	return true
}

// fetchTools reads what the open panel shows; a failed read is a toast.
func (u *window) fetchTools(p *chatPane, id string) {
	t := &p.tools
	t.mu.Lock()
	kind := t.kind
	if t.fetching || kind == "" {
		t.mu.Unlock()
		return
	}
	t.fetching = true
	t.mu.Unlock()
	go func() {
		var (
			mcp     api.ChatMCPList
			skills  api.ChatSkillList
			plugins api.ChatPluginList
			perms   api.ChatPermissionList
			hooks   api.ChatHookList
			memory  api.ChatMemoryList
			agents  api.ChatAgentList
			err     error
		)
		switch kind {
		case chatMemoryCommand:
			memory, err = u.client.ChatMemory(u.ctx, id)
		case chatAgentsCommand:
			agents, err = u.client.ChatAgents(u.ctx, id)
		case chatMCPCommand:
			mcp, err = u.client.ChatMCP(u.ctx, id)
		case chatSkillsCommand:
			skills, err = u.client.ChatSkills(u.ctx, id)
		case chatPermissionsCommand:
			perms, err = u.client.ChatPermissions(u.ctx, id)
		case chatHooksCommand:
			hooks, err = u.client.ChatHooks(u.ctx, id)
		default:
			plugins, err = u.client.ChatPlugins(u.ctx, id)
		}
		t.mu.Lock()
		t.fetching = false
		if err == nil && t.kind == kind {
			t.mcp, t.skills, t.plugins, t.loaded = mcp, skills, plugins, true
			t.set.perms, t.set.hooks = perms, hooks
			t.mem.list, t.mem.agents = memory, agents
			t.gen++
		}
		t.mu.Unlock()
		u.chatFailed(err)
		u.model.invalidate()
	}()
}

// mcpAct runs action on server, opens the page a sign-in leaves, and reads
// the servers again.
func (u *window) mcpAct(p *chatPane, id, server, action string) {
	go func() {
		res, err := u.client.ChatMCPAct(u.ctx, id, server, action)
		if err != nil {
			u.chatFailed(err)
			return
		}
		if res.AuthURL != "" {
			openLink(u.client, func(t ui.Toast) { u.toaster.Show(t) }, res.AuthURL)
		}
		u.fetchTools(p, id)
	}()
}

// toggleSkill turns s on when it is off and off otherwise, and reads the
// skills again.
func (u *window) toggleSkill(p *chatPane, id string, s api.ChatSkill) {
	if s.Locked != "" {
		return
	}
	go func() {
		u.chatFailed(u.client.ChatSetSkill(u.ctx, id, s.Name, s.State == api.SkillOff))
		u.fetchTools(p, id)
	}()
}

// pollTools reads the servers again while one is still connecting.
func (u *window) pollTools(gtx C, p *chatPane, id string) {
	t := &p.tools
	t.mu.Lock()
	connecting := t.kind == chatMCPCommand && t.loaded && slices.ContainsFunc(t.mcp.Servers, func(s api.ChatMCPServer) bool { return s.Status == api.MCPPending })
	idle := !t.fetching
	t.mu.Unlock()
	switch {
	case !connecting:
		t.next = time.Time{}
	case t.next.IsZero():
		t.next = gtx.Now.Add(chatToolsPoll)
	case !gtx.Now.Before(t.next) && idle:
		t.next = time.Time{}
		u.fetchTools(p, id)
		return
	}
	if !t.next.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: t.next})
	}
}

// shownSkills is the skills the search holds, by name or by what they cost.
func (t *chatTools) shownSkills(all []api.ChatSkill) []api.ChatSkill {
	q := strings.ToLower(strings.TrimSpace(t.query.Text()))
	shown := slices.DeleteFunc(slices.Clone(all), func(s api.ChatSkill) bool {
		return q != "" && !strings.Contains(strings.ToLower(s.Name), q)
	})
	if t.byTokens {
		slices.SortStableFunc(shown, func(a, b api.ChatSkill) int { return b.Tokens - a.Tokens })
	}
	return shown
}

// toolsActs runs the open panel's clicks and keys before the box reads its
// own, as the slash list's do: the rows, Back, the tools and the actions of a
// server, the sort and the skills.
func (u *window) toolsActs(gtx C, p *chatPane, id string) {
	t := &p.tools
	u.outsideActs(gtx, p)
	if !t.opened() {
		return
	}
	if t.close.Clicked(gtx) {
		t.shut()
		gtx.Execute(key.FocusCmd{Tag: &p.editor})
		return
	}
	t.mu.Lock()
	mcp, skills, plugins, loaded := t.mcp, t.skills, t.plugins, t.loaded
	memory, agents := t.mem.list, t.mem.agents
	t.mu.Unlock()
	sv, _ := u.model.sessionViewOf(id)
	rows, enter := 0, func(int) {}
	switch {
	case !loaded:
	case t.kind == chatMemoryCommand:
		rows, enter = u.memoryActs(p, sv, memory)
	case t.kind == chatAgentsCommand:
		rows, enter = u.agentsActs(gtx, p, sv, agents)
	case t.kind == chatShellsPanel:
		rows, enter = u.shellActs(gtx, p, id)
	case t.kind == chatChildrenPanel:
		rows, enter = u.childrenActs(gtx, p, id)
	case t.kind == chatWorkflowsPanel:
		rows, enter = u.workflowActs(gtx, p, id)
	case t.kind == chatPluginCommand:
		rows, enter = u.pluginActs(gtx, p, id, plugins)
	case t.kind == chatPermissionsCommand:
		rows, enter = u.permissionActs(gtx, p, id)
	case t.kind == chatHooksCommand:
		rows, enter = u.hookActs(gtx, p, id)
	case t.kind == chatSkillsCommand:
		if t.sortBtn.Clicked(gtx) {
			t.byTokens = !t.byTokens
		}
		shown := t.shownSkills(skills.Skills)
		rows, enter = len(shown), func(i int) { u.toggleSkill(p, id, shown[i]) }
	case t.detail != "":
		u.mcpDetailActs(gtx, p, id, mcp)
	default:
		servers := mcpOrdered(mcp.Servers)
		rows, enter = len(servers), func(i int) { t.detail, t.showTools = servers[i].Name, false }
	}
	t.ensureRows(rows)
	for i := range rows {
		if t.rows[i].Clicked(gtx) {
			t.cursor = i
			enter(i)
		}
	}
	u.toolsKeys(gtx, p, id, rows, enter)
	t.cursor = min(t.cursor, max(rows-1, 0))
}

// mcpDetailActs runs a server's Back, tools and actions.
func (u *window) mcpDetailActs(gtx C, p *chatPane, id string, l api.ChatMCPList) {
	t := &p.tools
	i := slices.IndexFunc(l.Servers, func(s api.ChatMCPServer) bool { return s.Name == t.detail })
	if t.back.Clicked(gtx) || i < 0 {
		t.detail, t.showTools = "", false
		return
	}
	if t.toolsBtn.Clicked(gtx) {
		t.showTools = !t.showTools
	}
	s := l.Servers[i]
	for _, a := range mcpActions(s, l.Actions) {
		if t.act(a).Clicked(gtx) {
			u.mcpAct(p, id, s.Name, a)
		}
	}
}

// toolsKeys runs the panel's keys: the arrows walk its rows, Enter opens the
// server or plugin or turns the skill under the cursor, Escape goes back from
// a detail and shuts the list. A field of the panel takes them where it has
// one; elsewhere the box keeps the keyboard, as it does under the slash list.
func (u *window) toolsKeys(gtx C, p *chatPane, id string, rows int, enter func(i int)) {
	t := &p.tools
	var tag event.Tag = t.keyTag(p)
	var filters []event.Filter
	for _, name := range []key.Name{key.NameUpArrow, key.NameDownArrow, key.NameReturn, key.NameEnter, key.NameEscape} {
		filters = append(filters, key.Filter{Focus: tag, Name: name})
	}
	if t.focus || !gtx.Focused(tag) {
		gtx.Execute(key.FocusCmd{Tag: tag})
		t.focus = false
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press {
			continue
		}
		switch e.Name {
		case key.NameDownArrow:
			t.cursor, t.moved = min(t.cursor+1, max(rows-1, 0)), true
		case key.NameUpArrow:
			t.cursor, t.moved = max(t.cursor-1, 0), true
		case key.NameEscape:
			if t.detail != "" {
				t.detail, t.showTools, t.focus = "", false, true
				continue
			}
			t.shut()
			gtx.Execute(key.FocusCmd{Tag: &p.editor})
			return
		default:
			if !u.submitSource(p, id, t.keyTag(p)) && !u.submitAgent(p, id, t.keyTag(p)) && t.cursor < rows {
				enter(t.cursor)
			}
		}
	}
}

// keyTag is the field that holds the panel's keys: the search of the skills
// and of the plugins on offer, the marketplace source on its tab, and the
// box everywhere else.
func (t *chatTools) keyTag(p *chatPane) *widget.Editor {
	switch {
	case t.kind == chatAgentsCommand:
		return t.memoryKeyTag(&p.editor)
	case t.kind == chatSkillsCommand:
		return &t.query
	case t.kind == chatPermissionsCommand && t.detail == permAddDetail:
		return &t.set.rule
	case t.kind != chatPluginCommand || t.detail != "":
		return &p.editor
	case t.plug.tabs.Selected == pluginTabDiscover:
		return &t.query
	case t.plug.tabs.Selected == pluginTabMarkets:
		return &t.plug.source
	}
	return &p.editor
}
