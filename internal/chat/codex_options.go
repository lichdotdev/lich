package chat

import (
	"context"
	"encoding/json"
	"maps"
	"slices"

	"github.com/lichdotdev/lich/api"
)

// What a Codex chat can be set to, as measured on 0.170.0: model/list names
// the models, each with the efforts it takes; thread/start and thread/resume
// answer with the model, effort and sandbox the thread runs on, and
// thread/settings/updated says when they change. turn/start takes a model, an
// effort, an approval policy and a sandbox policy for its turn and the ones
// after it, so every option is set in flight, from the next message on.

// The ids of a Codex chat's options, which the window names by category.
const (
	codexModelOption  = "model"
	codexEffortOption = "effort"
	codexModeOption   = "permissions"
)

// codexModes are Codex's built-in permission profiles, under the ids
// permissionProfile/list answers with; the window names them by id. Each is
// the TUI's preset of the same sandbox: asking before anything the sandbox
// does not allow, or, for full access, never asking.
const (
	codexReadOnly   = ":read-only"
	codexWorkspace  = ":workspace"
	codexFullAccess = ":danger-full-access"
)

var codexModes = []string{codexReadOnly, codexWorkspace, codexFullAccess}

// The approval policies a mode runs on.
const (
	codexApprovalAsk   = "on-request"
	codexApprovalNever = "never"
)

// codexSandboxP is a SandboxPolicy, by its type alone.
type codexSandboxP struct {
	Type string `json:"type"`
}

// codexModeOf is the mode a sandbox policy is, "" for one that is none of the
// built-in ones (an external sandbox).
func codexModeOf(p codexSandboxP) string {
	switch p.Type {
	case "readOnly", "read-only":
		return codexReadOnly
	case "workspaceWrite", "workspace-write":
		return codexWorkspace
	case "dangerFullAccess", "danger-full-access":
		return codexFullAccess
	}
	return ""
}

// codexModeParams is what turn/start takes for mode. A workspace's policy is
// the one thread/start answers with for workspace-write (measured).
func codexModeParams(mode string) map[string]any {
	switch mode {
	case codexReadOnly:
		return map[string]any{"approvalPolicy": codexApprovalAsk, "sandboxPolicy": map[string]any{"type": "readOnly", "networkAccess": false}}
	case codexWorkspace:
		return map[string]any{"approvalPolicy": codexApprovalAsk, "sandboxPolicy": map[string]any{
			"type": "workspaceWrite", "writableRoots": []string{}, "networkAccess": false, "excludeTmpdirEnvVar": false, "excludeSlashTmp": false,
		}}
	}
	return map[string]any{"approvalPolicy": codexApprovalNever, "sandboxPolicy": map[string]any{"type": "dangerFullAccess"}}
}

type codexModel struct {
	ID      string        `json:"id"`
	Name    string        `json:"displayName"`
	About   string        `json:"description"`
	Hidden  bool          `json:"hidden"`
	Default string        `json:"defaultReasoningEffort"`
	Efforts []codexEffort `json:"supportedReasoningEfforts"`
}

type codexEffort struct {
	Effort string `json:"reasoningEffort"`
	About  string `json:"description"`
}

// codexRunning is what a Codex chat is set to, which runs from the next turn
// on: staged holds the turn/start overrides the person set since the last
// one, and on the model and effort the thread runs on now.
type codexRunning struct {
	models              []codexModel
	model, effort, mode string
	staged              codexStaged
	on                  codexProducer
}

// readModels lists every model Codex offers, page by page.
func (d *codexDriver) readModels(ctx context.Context) error {
	var models []codexModel
	cursor := ""
	for {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := d.call(ctx, "model/list", params)
		if err != nil {
			return err
		}
		var page struct {
			Data []codexModel `json:"data"`
			Next *string      `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return err
		}
		models = append(models, page.Data...)
		if page.Next == nil || *page.Next == "" {
			break
		}
		cursor = *page.Next
	}
	d.mu.Lock()
	d.run.models = models
	d.mu.Unlock()
	return nil
}

func (d *codexDriver) publish() {
	d.mu.Lock()
	run := d.run
	d.mu.Unlock()
	d.c.setOptions(codexOptions(run))
}

// codexOptions is what a Codex chat running on run can be set to: a model the
// account cannot use (a config.toml default model/list leaves out) stays the
// current one, with no efforts to offer.
func codexOptions(run codexRunning) []api.ChatOption {
	models := api.ChatOption{ID: codexModelOption, Category: api.OptionModel, Current: run.model}
	for _, m := range run.models {
		if !m.Hidden || m.ID == run.model {
			models.Choices = append(models.Choices, api.ChatChoice{ID: m.ID, Label: m.Name, Description: m.About})
		}
	}
	opts := []api.ChatOption{models}
	if m, ok := run.current(); ok && len(m.Efforts) > 0 {
		effort := api.ChatOption{ID: codexEffortOption, Category: api.OptionEffort, Current: run.effort}
		for _, e := range m.Efforts {
			effort.Choices = append(effort.Choices, api.ChatChoice{ID: e.Effort, Description: e.About})
		}
		opts = append(opts, effort)
	}
	mode := api.ChatOption{ID: codexModeOption, Category: api.OptionMode, Current: run.mode}
	for _, id := range codexModes {
		mode.Choices = append(mode.Choices, api.ChatChoice{ID: id, Danger: id == codexFullAccess})
	}
	return append(opts, mode)
}

func (run codexRunning) current() (codexModel, bool) {
	i := slices.IndexFunc(run.models, func(m codexModel) bool { return m.ID == run.model })
	if i < 0 {
		return codexModel{}, false
	}
	return run.models[i], true
}

// setOption takes the choice for the next turn: a model whose efforts leave
// out the one in force goes back to the model's default effort.
func (d *codexDriver) setOption(_ context.Context, o api.ChatOption, value string) error {
	d.stage(o.Category, value)
	d.publish()
	return nil
}

func (d *codexDriver) stage(category, value string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch category {
	case api.OptionModel:
		d.run.model, d.run.staged.Model = value, value
		if m, ok := d.run.current(); ok && !slices.ContainsFunc(m.Efforts, func(e codexEffort) bool { return e.Effort == d.run.effort }) {
			d.run.effort, d.run.staged.Effort = m.Default, m.Default
		}
	case api.OptionEffort:
		d.run.effort, d.run.staged.Effort = value, value
	case api.OptionMode:
		d.run.mode, d.run.staged.Mode = value, value
	}
}

// codexStaged is what the person set since the last turn/start.
type codexStaged struct {
	Model, Effort, Mode string
}

// params is what turn/start carries for it.
func (s codexStaged) params() map[string]any {
	params := map[string]any{}
	if s.Model != "" {
		params["model"] = s.Model
	}
	if s.Effort != "" {
		params["effort"] = s.Effort
	}
	if s.Mode != "" {
		maps.Copy(params, codexModeParams(s.Mode))
	}
	return params
}

// staged is the overrides the next turn/start carries.
func (d *codexDriver) staged() codexStaged {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.run.staged
}

// unstage drops the overrides a turn/start carried, keeping any set since,
// and takes the model and effort it carried as the ones the thread runs on.
func (d *codexDriver) unstage(sent codexStaged) {
	d.mu.Lock()
	defer d.mu.Unlock()
	drop := func(field *string, was string) {
		if *field == was {
			*field = ""
		}
	}
	drop(&d.run.staged.Model, sent.Model)
	drop(&d.run.staged.Effort, sent.Effort)
	drop(&d.run.staged.Mode, sent.Mode)
	if sent.Model != "" {
		d.run.on.Model = sent.Model
	}
	if sent.Effort != "" {
		d.run.on.Effort = sent.Effort
	}
}

// settingsUpdated reads thread/settings/updated, what the thread runs on now.
// A choice still staged is the person's for the next turn and stays.
func (d *codexDriver) settingsUpdated(params json.RawMessage) {
	var p struct {
		Settings struct {
			Model   string        `json:"model"`
			Effort  *string       `json:"effort"`
			Sandbox codexSandboxP `json:"sandboxPolicy"`
		} `json:"threadSettings"`
	}
	if json.Unmarshal(params, &p) != nil {
		return
	}
	s := p.Settings
	d.mu.Lock()
	if s.Model != "" {
		d.run.on.Model = s.Model
	}
	if s.Effort != nil {
		d.run.on.Effort = *s.Effort
	}
	if d.run.staged.Model == "" && s.Model != "" {
		d.run.model = s.Model
	}
	if d.run.staged.Effort == "" && s.Effort != nil {
		d.run.effort = *s.Effort
	}
	if d.run.staged.Mode == "" {
		d.run.mode = codexModeOf(s.Sandbox)
	}
	d.mu.Unlock()
	d.publish()
}

// modelName is the display name of model id, the id itself for one
// model/list leaves out.
func (d *codexDriver) modelName(id string) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if i := slices.IndexFunc(d.run.models, func(m codexModel) bool { return m.ID == id }); i >= 0 && d.run.models[i].Name != "" {
		return d.run.models[i].Name
	}
	return id
}

// producer is the model and effort an item that begins now is attributed to:
// the ones the thread runs on, not a choice staged for the next turn.
func (d *codexDriver) producer() (model, effort string) {
	d.mu.Lock()
	model, effort = d.run.on.Model, d.run.on.Effort
	d.mu.Unlock()
	if model == "" {
		return "", ""
	}
	return d.modelName(model), effort
}
