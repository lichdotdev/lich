package main

import (
	"path/filepath"
	"slices"
	"strings"

	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
)

// /memory and /agents open, where the provider reports them (api.ChatMemory,
// api.ChatAgents), the instruction files the chat loads with its memory
// settings, and its subagents, over the composer as /mcp does. A file opens
// in the checkout's stage editor, to edit in place.

const (
	chatMemoryCommand = "/memory"
	chatAgentsCommand = "/agents"
)

// chatMemoryKinds is the providers whose chat reports its memory and
// subagents.
var chatMemoryKinds = []string{claudeKind, codexKind}

// chatMemory is what the /memory and /agents panels hold beside chatTools:
// the lists, and the new subagent's fields.
type chatMemory struct {
	list   api.ChatMemoryList
	agents api.ChatAgentList
	name   widget.Editor
	desc   widget.Editor
	// onDesc is whether the description field holds the keys, the name's
	// otherwise.
	onDesc bool
}

func (m *chatMemory) reset() {
	m.name.SingleLine, m.desc.SingleLine, m.onDesc = true, true, false
	m.name.SetText("")
	m.desc.SetText("")
}

// memoryRow is one row of the /memory panel: an instruction file or a saved
// memory to open, or a setting to turn.
type memoryRow struct {
	file    *api.ChatMemoryFile
	note    *api.ChatMemoryNote
	setting string
	toggle  *api.ChatMemoryToggle
}

// memoryRows is the panel's rows in its order: the files as the provider
// lists them, its settings, then the memories it saved.
func memoryRows(l api.ChatMemoryList) []memoryRow {
	var rows []memoryRow
	for i := range l.Files {
		rows = append(rows, memoryRow{file: &l.Files[i]})
	}
	if l.AutoMemory != nil {
		rows = append(rows, memoryRow{setting: api.MemoryAuto, toggle: l.AutoMemory})
	}
	if l.AutoDream != nil {
		rows = append(rows, memoryRow{setting: api.MemoryDream, toggle: l.AutoDream})
	}
	for i := range l.Memories {
		rows = append(rows, memoryRow{note: &l.Memories[i]})
	}
	return rows
}

// agentGroupOrder is the order the /agents panel files subagents in.
var agentGroupOrder = []string{api.AgentProject, api.AgentUser, api.AgentPlugin, api.AgentManaged, api.AgentOther, api.AgentBuiltIn}

// agentsOrdered is the subagents by group, each in the provider's order.
func agentsOrdered(agents []api.ChatAgent) []api.ChatAgent {
	var out []api.ChatAgent
	for _, g := range agentGroupOrder {
		for _, a := range agents {
			if agentGroup(a.Source) == g {
				out = append(out, a)
			}
		}
	}
	return out
}

func agentGroup(source string) string {
	if slices.Contains(agentGroupOrder, source) {
		return source
	}
	return api.AgentOther
}

// memoryActs is the /memory panel's rows and what Enter does on one.
func (u *window) memoryActs(p *chatPane, sv sessionView, l api.ChatMemoryList) (int, func(int)) {
	rows := memoryRows(l)
	return len(rows), func(i int) {
		r := rows[i]
		switch {
		case r.toggle != nil:
			u.setMemory(p, sv.ID, r.setting, r.toggle)
		case r.file != nil:
			u.openMemoryFile(p, sv, *r.file)
		default:
			u.openChatFile(p, sv, r.note.Path, false)
		}
	}
}

// agentsActs is the /agents panel's rows, what Enter does on one, and its
// buttons that add a subagent.
func (u *window) agentsActs(gtx C, p *chatPane, sv sessionView, l api.ChatAgentList) (int, func(int)) {
	m := &p.tools.mem
	// a click moves the keys to the field clicked; a move lich asked for
	// (t.focus) lands on the next frame first
	if !p.tools.focus {
		switch {
		case gtx.Focused(&m.desc):
			m.onDesc = true
		case gtx.Focused(&m.name):
			m.onDesc = false
		}
	}
	for _, scope := range l.Create {
		if p.tools.act(agentAddKey(scope)).Clicked(gtx) {
			u.newAgent(p, sv.ID, scope)
		}
	}
	agents := agentsOrdered(l.Agents)
	return len(agents), func(i int) {
		if a := agents[i]; a.Path != "" {
			u.openChatFile(p, sv, a.Path, a.ReadOnly)
		}
	}
}

func agentAddKey(scope string) string { return "agent-add-" + scope }

// setMemory turns setting the other way and reads the memory again.
func (u *window) setMemory(p *chatPane, id, setting string, t *api.ChatMemoryToggle) {
	if t.Locked {
		return
	}
	on := !t.Enabled
	go func() {
		u.chatFailed(u.client.ChatSetMemory(u.ctx, id, setting, on))
		u.fetchTools(p, id)
	}()
}

// openMemoryFile opens f, created first when the provider lists it as
// missing. What lands off the frame opens on the next one (askCode).
func (u *window) openMemoryFile(p *chatPane, sv sessionView, f api.ChatMemoryFile) {
	if f.Exists {
		u.openChatFile(p, sv, f.Path, false)
		return
	}
	go func() {
		if err := u.client.ChatCreateMemory(u.ctx, sv.ID, f.Path); err != nil {
			u.chatFailed(err)
			return
		}
		u.askCode(func() { u.openChatFile(p, sv, f.Path, false) })
	}()
}

// newAgent writes the subagent the fields name in scope and opens its file
// to write its prompt; the fields stay filled when it is refused.
func (u *window) newAgent(p *chatPane, id, scope string) {
	m := &p.tools.mem
	name, desc := strings.TrimSpace(m.name.Text()), strings.TrimSpace(m.desc.Text())
	go func() {
		path, err := u.client.ChatNewAgent(u.ctx, id, scope, name, desc)
		if err != nil {
			u.chatFailed(err)
			return
		}
		u.askCode(func() {
			if sv, ok := u.model.sessionViewOf(id); ok {
				u.openChatFile(p, sv, path, false)
			}
		})
	}()
}

// submitAgent adds the subagent the fields name in the first place the
// provider takes one, when tag is the description field and both are
// filled, and reports whether it did. Enter in the name moves to the
// description.
func (u *window) submitAgent(p *chatPane, id string, tag *widget.Editor) bool {
	t := &p.tools
	m := &t.mem
	switch {
	case t.kind != chatAgentsCommand || strings.TrimSpace(m.name.Text()) == "":
		return false
	case tag == &m.name:
		m.onDesc, t.focus = true, true
		return true
	case tag != &m.desc || strings.TrimSpace(m.desc.Text()) == "":
		return false
	}
	t.mu.Lock()
	create := m.agents.Create
	t.mu.Unlock()
	if len(create) == 0 {
		return false
	}
	u.newAgent(p, id, create[0])
	return true
}

// openChatFile puts path on the stage in sv's checkout editor and shuts the
// panel: a checkout's file as any other, one outside it in place, or shown
// only where it is not the person's to edit.
func (u *window) openChatFile(p *chatPane, sv sessionView, path string, readOnly bool) {
	p.tools.shut()
	if rel, ok := codeRefRel(sv.path, path); ok && sv.path != "" {
		u.openInEditor(sv.path, rel, 0, 0, agentNote{})
		return
	}
	dir, name := filepath.Dir(path), filepath.Base(path)
	if readOnly {
		u.stageFile(sv.path, dir, name, func(e *fileEditor) { e.openArtifact(dir, name) })
		return
	}
	u.stageFile(sv.path, dir, name, func(e *fileEditor) { e.openOutside(dir, name) })
}

// memoryKeyTag is the field of the /agents panel that holds the keys.
func (t *chatTools) memoryKeyTag() *widget.Editor {
	if t.mem.onDesc {
		return &t.mem.desc
	}
	return &t.mem.name
}
