package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// memoryDriver is a driver whose provider reports the instruction files it
// loads and its subagents: Claude Code's and Codex's. The ACP providers
// report neither.
type memoryDriver interface {
	memory(ctx context.Context) (api.ChatMemoryList, error)
	// setMemory takes one of the settings memory reported.
	setMemory(ctx context.Context, setting string, enabled bool) error
	agents(ctx context.Context) (api.ChatAgentList, error)
	// agentDir is the folder a new subagent of scope is written to, one of
	// those agents offers.
	agentDir(scope string) (string, error)
}

// agentFileMode is a new subagent's or instruction file's, as an editor
// writes one.
const agentFileMode = 0o644

var (
	errNoMemory        = errors.New("this agent does not report its memory or subagents")
	errNotMissing      = errors.New("only an instruction file the agent lists as missing is created here")
	errAgentName       = errors.New("a subagent's name is letters, digits, - and _, and starts with a letter or digit")
	errAgentDesc       = errors.New("a subagent needs a one-line description: the agent reads it to choose one")
	errAgentScope      = errors.New("this agent does not take a new subagent there")
	errNoMemorySetting = errors.New("this agent has no memory setting to change")
)

// agentNameRe is the file name lich gives a new subagent.
var agentNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func (s *Service) memoryOf(ctx context.Context, id string) (memoryDriver, bool, error) {
	c, err := s.ensure(ctx, id)
	if err != nil {
		return nil, false, err
	}
	d, ok := c.drv.(memoryDriver)
	return d, ok, nil
}

// memoryCall is session id's driver and a context bounded for one call.
func (s *Service) memoryCall(ctx context.Context, id string) (memoryDriver, context.Context, context.CancelFunc, error) {
	d, ok, err := s.memoryOf(ctx, id)
	if err != nil {
		return nil, nil, nil, err
	}
	if !ok {
		return nil, nil, nil, errNoMemory
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	return d, callCtx, cancel, nil
}

// Memory answers api.ChatMemory.
func (s *Service) Memory(ctx context.Context, opts api.SessionIDOptions) (api.ChatMemoryList, error) {
	d, callCtx, cancel, err := s.memoryCall(ctx, opts.ID)
	if errors.Is(err, errNoMemory) {
		return api.ChatMemoryList{}, nil
	}
	if err != nil {
		return api.ChatMemoryList{}, err
	}
	defer cancel()
	return d.memory(callCtx)
}

// SetMemory answers api.ChatSetMemory.
func (s *Service) SetMemory(ctx context.Context, opts api.ChatSetMemoryOptions) (api.None, error) {
	if opts.Setting != api.MemoryAuto && opts.Setting != api.MemoryDream {
		return api.None{}, fmt.Errorf("no agent has a memory setting %q", opts.Setting)
	}
	d, callCtx, cancel, err := s.memoryCall(ctx, opts.ID)
	if err != nil {
		return api.None{}, err
	}
	defer cancel()
	return api.None{}, d.setMemory(callCtx, opts.Setting, opts.Enabled)
}

// CreateMemory answers api.ChatCreateMemory: only a file the agent itself
// lists as missing, so the window cannot have lich write anywhere else.
func (s *Service) CreateMemory(ctx context.Context, opts api.ChatPathOptions) (api.None, error) {
	d, callCtx, cancel, err := s.memoryCall(ctx, opts.ID)
	if err != nil {
		return api.None{}, err
	}
	defer cancel()
	list, err := d.memory(callCtx)
	if err != nil {
		return api.None{}, err
	}
	if !slices.ContainsFunc(list.Files, func(f api.ChatMemoryFile) bool { return f.Path == opts.Path && !f.Exists }) {
		return api.None{}, errNotMissing
	}
	return api.None{}, createNew(opts.Path, nil)
}

// Agents answers api.ChatAgents.
func (s *Service) Agents(ctx context.Context, opts api.SessionIDOptions) (api.ChatAgentList, error) {
	d, callCtx, cancel, err := s.memoryCall(ctx, opts.ID)
	if errors.Is(err, errNoMemory) {
		return api.ChatAgentList{}, nil
	}
	if err != nil {
		return api.ChatAgentList{}, err
	}
	defer cancel()
	return d.agents(callCtx)
}

// NewAgent answers api.ChatNewAgent: a file of its own name, never over one
// that is there.
func (s *Service) NewAgent(ctx context.Context, opts api.ChatNewAgentOptions) (api.ChatNewAgentResult, error) {
	desc := strings.TrimSpace(opts.Description)
	switch {
	case !agentNameRe.MatchString(opts.Name):
		return api.ChatNewAgentResult{}, errAgentName
	case desc == "" || strings.ContainsAny(desc, "\r\n"):
		return api.ChatNewAgentResult{}, errAgentDesc
	}
	d, callCtx, cancel, err := s.memoryCall(ctx, opts.ID)
	if err != nil {
		return api.ChatNewAgentResult{}, err
	}
	defer cancel()
	list, err := d.agents(callCtx)
	if err != nil {
		return api.ChatNewAgentResult{}, err
	}
	if !slices.Contains(list.Create, opts.Scope) {
		return api.ChatNewAgentResult{}, errAgentScope
	}
	dir, err := d.agentDir(opts.Scope)
	if err != nil {
		return api.ChatNewAgentResult{}, err
	}
	path := filepath.Join(dir, opts.Name+".md")
	if err := createNew(path, agentFile(opts.Name, desc)); err != nil {
		return api.ChatNewAgentResult{}, err
	}
	return api.ChatNewAgentResult{Path: path}, nil
}

// agentFile is a new subagent's file: the frontmatter Claude Code lists an
// agent by, name and a description it refuses to go without (2.1.330), the
// prompt left for the person to write. The description is a JSON string,
// which YAML reads as the same double-quoted text.
func agentFile(name, desc string) []byte {
	quoted, _ := json.Marshal(desc)
	return fmt.Appendf(nil, "---\nname: %s\ndescription: %s\n---\n\n", name, quoted)
}

// createNew writes content to a file that must not be there yet, and the
// folders over it.
func createNew(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, agentFileMode)
	if err != nil {
		return err
	}
	_, werr := f.Write(content)
	return errors.Join(werr, f.Close())
}
