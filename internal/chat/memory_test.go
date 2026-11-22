//go:build !windows

package chat

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

func (r *rig) memory() api.ChatMemoryList {
	r.t.Helper()
	list, err := r.svc.Memory(r.t.Context(), api.SessionIDOptions{ID: "s1"})
	if err != nil {
		r.t.Fatalf("Memory: %v", err)
	}
	return list
}

func (r *rig) agents() api.ChatAgentList {
	r.t.Helper()
	list, err := r.svc.Agents(r.t.Context(), api.SessionIDOptions{ID: "s1"})
	if err != nil {
		r.t.Fatalf("Agents: %v", err)
	}
	return list
}

func (r *rig) newAgent(scope, name, desc string) (string, error) {
	res, err := r.svc.NewAgent(r.t.Context(), api.ChatNewAgentOptions{ID: "s1", Scope: scope, Name: name, Description: desc})
	return res.Path, err
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Claude Code's get_memory_dialog is the list: the files in its order, the
// imported one marked, the missing ones said so, the saved memories with a
// float timestamp, and auto-dream left out where the dialog hides it.
func TestClaudeMemory(t *testing.T) {
	r := newChat(t, providers.Claude)
	project := filepath.Join(r.home, "proj")
	writeTestFile(t, filepath.Join(project, "CLAUDE.md"), "# proj\n")
	got := r.memory()
	want := []api.ChatMemoryFile{
		{Kind: api.MemoryUser, Path: filepath.Join(r.home, "claude", "CLAUDE.md")},
		{Kind: api.MemoryProject, Path: filepath.Join(project, "CLAUDE.md"), Exists: true},
		{Kind: api.MemoryOther, Path: filepath.Join(project, "imp.md"), Imported: true, Exists: true},
	}
	if !got.Reported || !slices.Equal(got.Files, want) {
		t.Errorf("files = %+v", got.Files)
	}
	if len(got.Memories) != 2 || got.Memories[1] != (api.ChatMemoryNote{Name: "a.md", Path: filepath.Join(r.home, "claude", "memory", "a.md"), Description: "A fact", Type: "user"}) || got.Memories[0].Type != "" {
		t.Errorf("memories = %+v", got.Memories)
	}
	if got.AutoMemory == nil || !got.AutoMemory.Enabled || got.AutoMemory.Locked || got.AutoDream != nil {
		t.Errorf("settings = %+v %+v", got.AutoMemory, got.AutoDream)
	}
	t.Setenv(fakeNoMemoryEnv, "1")
	r = newChat(t, providers.Claude)
	if got := r.memory(); got.Reported {
		t.Errorf("refused memory = %+v, want not reported", got)
	}
}

// A memory setting is written by the CLI, run without the variable that
// makes it refuse, and the next read says so.
func TestClaudeSetMemory(t *testing.T) {
	t.Setenv(claudeChildEnv, "1")
	r := newChat(t, providers.Claude)
	set := func(setting string, on bool) error {
		_, err := r.svc.SetMemory(r.t.Context(), api.ChatSetMemoryOptions{ID: "s1", Setting: setting, Enabled: on})
		return err
	}
	if err := set(api.MemoryAuto, false); err != nil {
		t.Fatalf("SetMemory: %v", err)
	}
	if got := r.memory(); got.AutoMemory == nil || got.AutoMemory.Enabled {
		t.Errorf("auto-memory after off = %+v", got.AutoMemory)
	}
	if err := set("telepathy", true); err == nil {
		t.Error("an unknown setting was taken")
	}
}

// Only a file the list names as missing is created, and never over one.
func TestClaudeCreateMemory(t *testing.T) {
	r := newChat(t, providers.Claude)
	user := filepath.Join(r.home, "claude", "CLAUDE.md")
	create := func(path string) error {
		_, err := r.svc.CreateMemory(r.t.Context(), api.ChatPathOptions{ID: "s1", Path: path})
		return err
	}
	if err := create(filepath.Join(r.home, "elsewhere.md")); !errors.Is(err, errNotMissing) {
		t.Errorf("unlisted file: %v", err)
	}
	if err := create(user); err != nil {
		t.Fatalf("CreateMemory: %v", err)
	}
	if _, err := os.Stat(user); err != nil {
		t.Errorf("not created: %v", err)
	}
	if err := create(user); !errors.Is(err, errNotMissing) {
		t.Errorf("second create: %v", err)
	}
}

// The subagents are initialize's built-ins and get_context_usage's files,
// each file read for its tools; one written through NewAgent is listed with
// the file's description.
func TestClaudeAgents(t *testing.T) {
	r := newChat(t, providers.Claude)
	reviewer := filepath.Join(r.home, "claude", "agents", "reviewer.md")
	writeTestFile(t, reviewer, "---\nname: reviewer\ndescription: From the file\ntools: Read, Grep\nmodel: haiku\n---\nReview.\n")
	got := r.agents()
	want := []api.ChatAgent{
		{Name: "reviewer", Description: "Reviews code", Model: "sonnet", Source: api.AgentUser, Path: reviewer, Tools: []string{"Read", "Grep"}},
		{Name: "Explore", Description: "Search the code", Model: "inherit", Source: api.AgentBuiltIn},
		{Name: "general-purpose", Description: "Anything else", Source: api.AgentBuiltIn},
	}
	if !got.Reported || !slices.EqualFunc(got.Agents, want, agentEqual) || !slices.Equal(got.Create, []string{api.AgentUser, api.AgentProject}) {
		t.Errorf("agents = %+v", got)
	}
	path, err := r.newAgent(api.AgentProject, "helper", `Helps with "quotes"`)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	if path != filepath.Join(r.home, "proj", ".claude", "agents", "helper.md") {
		t.Errorf("path = %s", path)
	}
	i := slices.IndexFunc(r.agents().Agents, func(a api.ChatAgent) bool { return a.Name == "helper" })
	if i < 0 || r.agents().Agents[i].Description != `Helps with "quotes"` || r.agents().Agents[i].Source != api.AgentProject {
		t.Errorf("new agent not listed: %+v", r.agents().Agents)
	}
	if _, err := r.newAgent(api.AgentProject, "helper", "again"); err == nil {
		t.Error("an agent was written over")
	}
}

func agentEqual(a, b api.ChatAgent) bool {
	return a.Name == b.Name && a.Description == b.Description && a.Model == b.Model && a.Source == b.Source &&
		a.Path == b.Path && slices.Equal(a.Tools, b.Tools) && a.ReadOnly == b.ReadOnly
}

// A new subagent's name is a file name and its description one line.
func TestNewAgentRefuses(t *testing.T) {
	r := newChat(t, providers.Claude)
	for _, c := range []struct{ scope, name, desc string }{
		{api.AgentUser, "../x", "d"},
		{api.AgentUser, "-flag", "d"},
		{api.AgentUser, "ok", ""},
		{api.AgentUser, "ok", "two\nlines"},
		{api.AgentBuiltIn, "ok", "d"},
	} {
		if _, err := r.newAgent(c.scope, c.name, c.desc); err == nil {
			t.Errorf("NewAgent(%q, %q, %q) was taken", c.scope, c.name, c.desc)
		}
	}
}

func TestFrontmatterTools(t *testing.T) {
	for text, want := range map[string][]string{
		"---\ntools: Read, Grep\n---\n":          {"Read", "Grep"},
		"---\ntools: [Read, \"Bash\"]\n---\n":    {"Read", "Bash"},
		"---\ntools:\n  - Read\n  - Bash\n---\n": {"Read", "Bash"},
		"---\ntools: \"Read, Grep\"\n---\n":      {"Read", "Grep"},
		"---\nname: x\n---\n":                    nil,
		"no frontmatter":                         nil,
		"---\ntools: Read\n":                     nil,
	} {
		if got := agentTools(parseFrontmatter(text)); !slices.Equal(got, want) {
			t.Errorf("%q: tools = %q, want %q", text, got, want)
		}
	}
	if got := parseFrontmatter("---\ndescription: |\n  long\n---\n").scalar("description"); got != "" {
		t.Errorf("block scalar = %q", got)
	}
}

// Codex's memory is thread/start's instructionSources, the one in its home
// marked the person's; its subagents the roles config/read lists.
func TestCodexMemoryAndAgents(t *testing.T) {
	t.Setenv("CODEX_HOME", "")
	r := newChat(t, providers.Codex)
	got := r.memory()
	want := []api.ChatMemoryFile{
		{Kind: api.MemoryUser, Path: filepath.Join(r.home, ".codex", "AGENTS.md"), Exists: true},
		{Kind: api.MemoryOther, Path: "/repo/AGENTS.md", Exists: true},
		{Kind: api.MemoryOther, Path: "/repo/sub/AGENTS.override.md", Exists: true},
	}
	if !got.Reported || !slices.Equal(got.Files, want) || got.AutoMemory != nil {
		t.Errorf("memory = %+v", got)
	}
	agents := r.agents()
	if !agents.Reported || agents.Create != nil || len(agents.Agents) != 1 ||
		!agentEqual(agents.Agents[0], api.ChatAgent{Name: "reviewer", Description: "Reviews code", Source: api.AgentOther, Path: "/x/reviewer.toml"}) {
		t.Errorf("agents = %+v", agents)
	}
	if _, err := r.newAgent(api.AgentUser, "x", "d"); !errors.Is(err, errAgentScope) {
		t.Errorf("NewAgent: %v", err)
	}
	if _, err := r.svc.SetMemory(r.t.Context(), api.ChatSetMemoryOptions{ID: "s1", Setting: api.MemoryAuto}); !errors.Is(err, errNoMemorySetting) {
		t.Errorf("SetMemory: %v", err)
	}
}

// The ACP providers and Kiro report neither.
func TestACPReportsNoMemory(t *testing.T) {
	for _, kind := range []string{providers.OpenCode, providers.Kiro} {
		t.Run(kind, func(t *testing.T) {
			r := newChat(t, kind)
			if got := r.memory(); got.Reported {
				t.Errorf("Memory = %+v", got)
			}
			if got := r.agents(); got.Reported {
				t.Errorf("Agents = %+v", got)
			}
			if _, err := r.newAgent(api.AgentUser, "x", "d"); !errors.Is(err, errNoMemory) || !strings.Contains(err.Error(), "memory") {
				t.Errorf("NewAgent: %v", err)
			}
		})
	}
}
