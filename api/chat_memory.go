package api

// A chat session's memory files and subagents, as its provider reports them,
// for the window's /memory and /agents panels.

var (
	// ChatMemory is the instruction files a chat session loads, its saved
	// memories and its memory settings. A provider that reports none leaves
	// Reported false.
	ChatMemory = define[SessionIDOptions, ChatMemoryList]("chat.Memory")
	// ChatSetMemory turns one of the memory settings on or off.
	ChatSetMemory = define[ChatSetMemoryOptions, None]("chat.SetMemory")
	// ChatCreateMemory creates, empty, an instruction file the list names
	// as missing.
	ChatCreateMemory = define[ChatPathOptions, None]("chat.CreateMemory")
	// ChatAgents is the subagents a chat session can hand work to. A
	// provider that reports none leaves Reported false.
	ChatAgents = define[SessionIDOptions, ChatAgentList]("chat.Agents")
	// ChatNewAgent writes a new subagent's file, which the person then
	// writes the prompt of.
	ChatNewAgent = define[ChatNewAgentOptions, ChatNewAgentResult]("chat.NewAgent")
)

// Instruction file kinds, as ChatMemoryFile.Kind names them: the person's
// own, the project's ./CLAUDE.md, its ./.claude/CLAUDE.md, the checkout's
// ./CLAUDE.local.md, and any other the provider loads.
const (
	MemoryUser       = "user"
	MemoryProject    = "project"
	MemoryProjectAlt = "project-alt"
	MemoryLocal      = "local"
	MemoryOther      = "other"
)

// ChatSetMemory settings.
const (
	MemoryAuto  = "auto-memory"
	MemoryDream = "auto-dream"
)

// ChatMemoryList is a chat session's memory: Files are the instruction files
// it loads in the provider's order, Memories what it saved itself, and
// AutoMemory and AutoDream its settings, nil where the provider has none or
// does not show one.
type ChatMemoryList struct {
	Reported   bool              `json:"reported"`
	Files      []ChatMemoryFile  `json:"files,omitempty"`
	Memories   []ChatMemoryNote  `json:"memories,omitempty"`
	AutoMemory *ChatMemoryToggle `json:"autoMemory,omitempty"`
	AutoDream  *ChatMemoryToggle `json:"autoDream,omitempty"`
}

// ChatMemoryFile is one instruction file. Imported is set on one another file
// pulls in; Exists is false only on a user or project file not written yet,
// which ChatCreateMemory creates.
type ChatMemoryFile struct {
	Kind     string `json:"kind"`
	Path     string `json:"path"`
	Imported bool   `json:"imported,omitempty"`
	Exists   bool   `json:"exists"`
}

// ChatMemoryNote is one memory the agent saved. Type is the provider's
// (Claude Code: user, feedback, project, reference), "" when it has none.
type ChatMemoryNote struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
}

// ChatMemoryToggle is one memory setting. Locked is set where it cannot be
// changed from here; Detail is the provider's own words on it, "" when it
// says nothing.
type ChatMemoryToggle struct {
	Enabled bool   `json:"enabled"`
	Locked  bool   `json:"locked,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// ChatSetMemoryOptions turns Setting of session ID on or off.
type ChatSetMemoryOptions struct {
	ID      string `json:"id"`
	Setting string `json:"setting"`
	Enabled bool   `json:"enabled"`
}

// ChatPathOptions names file Path of session ID.
type ChatPathOptions struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

// Where a subagent comes from, as ChatAgent.Source names it.
const (
	AgentBuiltIn = "built-in"
	AgentUser    = "user"
	AgentProject = "project"
	AgentPlugin  = "plugin"
	AgentManaged = "managed"
	AgentOther   = "other"
)

// ChatAgentList is a chat session's subagents. Create is where ChatNewAgent
// writes one, AgentProject or AgentUser, the default first, nil where the
// provider takes none.
type ChatAgentList struct {
	Reported bool        `json:"reported"`
	Agents   []ChatAgent `json:"agents,omitempty"`
	Create   []string    `json:"create,omitempty"`
}

// ChatAgent is one subagent. Path is the file that defines it, "" for one
// built in; Tools are those it is limited to, nil when it takes them all or
// its file lists them in a form lich does not read. ReadOnly is set on a file
// the person does not edit here: a plugin's or a managed one.
type ChatAgent struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Model       string   `json:"model,omitempty"`
	Source      string   `json:"source"`
	Path        string   `json:"path,omitempty"`
	Tools       []string `json:"tools,omitempty"`
	ReadOnly    bool     `json:"readOnly,omitempty"`
}

// ChatNewAgentOptions writes subagent Name of session ID where Scope says,
// AgentUser or AgentProject, described by Description.
type ChatNewAgentOptions struct {
	ID          string `json:"id"`
	Scope       string `json:"scope"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ChatNewAgentResult is the file ChatNewAgent wrote.
type ChatNewAgentResult struct {
	Path string `json:"path"`
}
