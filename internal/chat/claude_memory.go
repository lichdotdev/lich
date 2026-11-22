package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/claudeprofile"
)

// A Claude Code chat's memory and subagents, as measured on 2.1.330.
// get_memory_dialog, marked @internal, answers the rows the TUI's /memory
// dialog draws: the instruction files that load, the auto-memory folder and
// what it holds, and the auto-memory and auto-dream settings, which
// `claude edit-memory-settings --json` writes to the profile's
// settings.json; the running chat reads the change at once. The /agents
// wizard is gone (it prints where the files go): initialize lists every
// subagent by name, description and model, and get_context_usage, which
// needs no turn, the source and file of each one not built in. Claude Code
// watches its agents folders, so a file written mid-session is listed.

// claudeMemoryCommand is the CLI that writes a memory setting.
var claudeMemoryCommand = []string{"edit-memory-settings", "--json"}

// claudeImported is the description get_memory_dialog gives a file another
// pulled in with an @ line.
const claudeImported = "@-imported"

// claudeMemorySettings is the settings.json key of each ChatSetMemory
// setting.
var claudeMemorySettings = map[string]string{api.MemoryAuto: "autoMemoryEnabled", api.MemoryDream: "autoDreamEnabled"}

type claudeMemoryDialog struct {
	Files *[]struct {
		Kind        string `json:"kind"`
		Path        string `json:"path"`
		Description string `json:"description"`
		Exists      bool   `json:"exists"`
	} `json:"files"`
	Memories []struct {
		Name        string  `json:"name"`
		Path        string  `json:"path"`
		Description *string `json:"description"`
		Type        *string `json:"type"`
	} `json:"memories"`
	AutoMemory claudeMemoryToggle `json:"auto_memory"`
	AutoDream  claudeMemoryToggle `json:"auto_dream"`
}

type claudeMemoryToggle struct {
	// Shown is auto-dream's alone: auto-memory's row is always drawn.
	Shown      *bool  `json:"shown"`
	Enabled    bool   `json:"enabled"`
	Toggleable bool   `json:"toggleable"`
	Detail     string `json:"detail"`
}

func (t claudeMemoryToggle) toggle() *api.ChatMemoryToggle {
	if t.Shown != nil && !*t.Shown {
		return nil
	}
	return &api.ChatMemoryToggle{Enabled: t.Enabled, Locked: !t.Toggleable, Detail: t.Detail}
}

// memory degrades to not reported on any refusal: the request is @internal.
func (d *claudeDriver) memory(ctx context.Context) (api.ChatMemoryList, error) {
	r, err := d.control(ctx, map[string]any{"subtype": "get_memory_dialog"})
	if r.Subtype == "error" {
		return api.ChatMemoryList{}, nil
	}
	if err != nil {
		return api.ChatMemoryList{}, err
	}
	var dialog claudeMemoryDialog
	if json.Unmarshal(r.Body, &dialog) != nil || dialog.Files == nil {
		return api.ChatMemoryList{}, nil
	}
	list := api.ChatMemoryList{Reported: true, AutoMemory: dialog.AutoMemory.toggle(), AutoDream: dialog.AutoDream.toggle()}
	for _, f := range *dialog.Files {
		list.Files = append(list.Files, api.ChatMemoryFile{Kind: f.Kind, Path: f.Path, Imported: f.Description == claudeImported, Exists: f.Exists})
	}
	for _, m := range dialog.Memories {
		list.Memories = append(list.Memories, api.ChatMemoryNote{Name: m.Name, Path: m.Path, Description: deref(m.Description), Type: deref(m.Type)})
	}
	return list, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (d *claudeDriver) setMemory(ctx context.Context, setting string, enabled bool) error {
	cmd, err := d.cli(ctx, claudeMemoryCommand...)
	if err != nil {
		return err
	}
	in, _ := json.Marshal(map[string]bool{claudeMemorySettings[setting]: enabled})
	cmd.Stdin = bytes.NewReader(in)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, runErr := cmd.Output()
	var reply struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if json.Unmarshal(out, &reply) != nil {
		said := strings.TrimSpace(stderr.String() + " " + string(out))
		return errors.Join(fmt.Errorf("claude edit-memory-settings: %s", said), runErr)
	}
	if !reply.OK {
		return fmt.Errorf("claude did not change the memory setting: %s", reply.Error)
	}
	return nil
}

// claudeAgent is a subagent as initialize lists it.
type claudeAgent struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Model       string `json:"model"`
}

// claudeAgentSources is the ChatAgent source of each settings source
// get_context_usage names.
var claudeAgentSources = map[string]string{
	"userSettings":    api.AgentUser,
	"projectSettings": api.AgentProject,
	"localSettings":   api.AgentProject,
	"plugin":          api.AgentPlugin,
	"policySettings":  api.AgentManaged,
}

func (d *claudeDriver) agents(ctx context.Context) (api.ChatAgentList, error) {
	r, err := d.control(ctx, map[string]any{"subtype": "get_context_usage"})
	if err != nil {
		return api.ChatAgentList{}, err
	}
	var usage struct {
		Agents []struct {
			Type   string `json:"agentType"`
			Source string `json:"source"`
			Path   string `json:"path"`
		} `json:"agents"`
	}
	_ = json.Unmarshal(r.Body, &usage)
	d.mu.Lock()
	listed := d.agentList
	d.mu.Unlock()
	known := map[string]claudeAgent{}
	for _, a := range listed {
		known[a.Name] = a
	}
	list := api.ChatAgentList{Reported: true, Create: []string{api.AgentUser, api.AgentProject}}
	filed := map[string]bool{}
	for _, u := range usage.Agents {
		filed[u.Type] = true
		list.Agents = append(list.Agents, claudeFiledAgent(u.Type, u.Source, u.Path, known[u.Type]))
	}
	// shortcut: an agent whose file went away mid-session still reads as
	// built in, initialize being the only list of those; a restart drops it.
	for _, a := range listed {
		if !filed[a.Name] {
			list.Agents = append(list.Agents, api.ChatAgent{Name: a.Name, Description: a.Description, Model: a.Model, Source: api.AgentBuiltIn})
		}
	}
	return list, nil
}

// claudeFiledAgent is an agent defined by the file at path. Its description
// and model are Claude Code's reading where initialize listed it, the file's
// for one added since; its tools are always the file's.
func claudeFiledAgent(name, source, path string, listed claudeAgent) api.ChatAgent {
	src, ok := claudeAgentSources[source]
	if !ok {
		src = api.AgentOther
	}
	a := api.ChatAgent{Name: name, Source: src, Path: path, ReadOnly: src == api.AgentPlugin || src == api.AgentManaged}
	front := readFrontmatter(path)
	a.Tools = agentTools(front)
	a.Description, a.Model = listed.Description, listed.Model
	if listed.Name == "" {
		a.Description, a.Model = front.scalar("description"), front.scalar("model")
	}
	return a
}

func (d *claudeDriver) agentDir(scope string) (string, error) {
	if scope == api.AgentProject {
		return filepath.Join(d.c.cwd, ".claude", "agents"), nil
	}
	dir := d.c.proc.ProfileDir
	if dir == "" {
		var err error
		if dir, err = claudeprofile.PersonalDir(); err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, "agents"), nil
}

// readFrontmatter is the frontmatter of the file at path, empty when it has
// none or cannot be read.
func readFrontmatter(path string) frontmatter {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseFrontmatter(string(data))
}
