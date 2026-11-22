package chat

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// What a Claude Code chat can be set to, as measured on 2.1.327: initialize
// lists the models, each with the efforts it takes and whether it has an auto
// mode; get_settings' `applied` is the model and effort the process runs on;
// the permission mode is initialize's and every system/status' afterwards.
// set_model and set_permission_mode change them in flight, and so does
// apply_flag_settings for the effort, in this process only: the user's
// settings files are left as they were. A resume brings back the mode and the
// model of the conversation's last turn, not the effort, and after a turn in
// plan mode it came back on another model than the turn's.

// claudeModes are the permission modes `claude --help` lists, under the ids
// the protocol answers with: the flag's `manual` is the protocol's `default`.
// Their labels are the window's, by id.
var claudeModes = []string{"default", "acceptEdits", "plan", "auto", "dontAsk", claudeBypass}

// claudeAuto is the mode a model offers only when it says supportsAutoMode.
const claudeAuto = "auto"

// The ids of a Claude Code chat's options, which the window names by category.
const (
	claudeModelOption  = "model"
	claudeEffortOption = "effort"
	claudeModeOption   = "permission_mode"
)

// claudeBypassRefused is set_permission_mode's error code for a process
// launched without a flag that allows bypassPermissions.
const claudeBypassRefused = "bypass_not_launched"

type claudeModel struct {
	Value          string   `json:"value"`
	Resolved       string   `json:"resolvedModel"`
	Name           string   `json:"displayName"`
	Description    string   `json:"description"`
	SupportsEffort bool     `json:"supportsEffort"`
	Efforts        []string `json:"supportedEffortLevels"`
	Auto           bool     `json:"supportsAutoMode"`
}

// claudeRunning is what a Claude Code chat runs on now. model is the id the
// process answers with, chosen the alias lich last asked for.
type claudeRunning struct {
	models        []claudeModel
	model, chosen string
	effort, mode  string
}

// readInitialize keeps what initialize says the chat can be set to.
func (d *claudeDriver) readInitialize(body json.RawMessage, chosen string) {
	var init struct {
		Models   []claudeModel   `json:"models"`
		Mode     string          `json:"current_permission_mode"`
		Commands []claudeCommand `json:"commands"`
		Agents   []claudeAgent   `json:"agents"`
	}
	_ = json.Unmarshal(body, &init)
	d.mu.Lock()
	d.run.models, d.run.mode, d.run.chosen = init.Models, init.Mode, chosen
	d.agentList = init.Agents
	d.mu.Unlock()
	d.listCommands(init.Commands)
}

// readApplied asks the process which model and effort it runs on.
func (d *claudeDriver) readApplied(ctx context.Context) error {
	r, err := d.control(ctx, map[string]any{"subtype": "get_settings"})
	if err != nil {
		return err
	}
	var settings struct {
		Applied struct {
			Model  string `json:"model"`
			Effort string `json:"effort"`
		} `json:"applied"`
	}
	_ = json.Unmarshal(r.Body, &settings)
	d.mu.Lock()
	d.run.model, d.run.effort = settings.Applied.Model, settings.Applied.Effort
	d.mu.Unlock()
	return nil
}

// running updates what the chat runs on from a line Claude Code wrote, and
// tells the window when that changed it.
func (d *claudeDriver) running(model, mode string) {
	if model == "" && mode == "" {
		return
	}
	d.mu.Lock()
	if model != "" {
		d.run.model = model
	}
	if mode != "" {
		d.run.mode = mode
	}
	d.mu.Unlock()
	d.publish()
}

// publish tells the window, once initialize has said what the chat can be
// set to: a resumed process writes its mode before it answers initialize
// (measured on 2.1.327).
func (d *claudeDriver) publish() {
	d.mu.Lock()
	run := d.run
	d.mu.Unlock()
	if run.models != nil {
		d.c.setOptions(claudeOptions(run))
	}
}

func (d *claudeDriver) setOption(ctx context.Context, o api.ChatOption, value string) error {
	var err error
	switch o.Category {
	case api.OptionModel:
		err = d.setModel(ctx, value)
	case api.OptionEffort:
		if _, err = d.control(ctx, map[string]any{"subtype": "apply_flag_settings", "settings": map[string]string{"effortLevel": value}}); err == nil {
			d.mu.Lock()
			d.run.effort = value
			d.mu.Unlock()
		}
	case api.OptionMode:
		err = d.setMode(ctx, value)
	}
	if err != nil {
		return err
	}
	d.publish()
	return nil
}

// setModel switches the model and reads the effort back, which the new model
// may hold at another level.
func (d *claudeDriver) setModel(ctx context.Context, value string) error {
	r, err := d.control(ctx, map[string]any{"subtype": "set_model", "model": value})
	if err != nil {
		return err
	}
	var set struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(r.Body, &set)
	d.mu.Lock()
	d.run.model, d.run.chosen = set.Model, value
	d.mu.Unlock()
	return d.readApplied(ctx)
}

func (d *claudeDriver) setMode(ctx context.Context, value string) error {
	r, err := d.control(ctx, map[string]any{"subtype": "set_permission_mode", "mode": value})
	if r.Code == claudeBypassRefused {
		return errRelaunch
	}
	if err != nil {
		return err
	}
	var set struct {
		Mode string `json:"mode"`
	}
	_ = json.Unmarshal(r.Body, &set)
	d.mu.Lock()
	d.run.mode = set.Mode
	d.mu.Unlock()
	return nil
}

// claudeOptions is what a Claude Code chat running on run can be set to: its
// model, the efforts that model takes, if any, and the permission modes, auto
// only where the model has it.
func claudeOptions(run claudeRunning) []api.ChatOption {
	current, model := claudeCurrentModel(run)
	models := api.ChatOption{ID: claudeModelOption, Category: api.OptionModel, Current: current}
	for _, m := range run.models {
		models.Choices = append(models.Choices, api.ChatChoice{ID: m.Value, Label: m.Name, Description: m.Description})
	}
	opts := []api.ChatOption{models}
	if model.SupportsEffort && len(model.Efforts) > 0 {
		effort := api.ChatOption{ID: claudeEffortOption, Category: api.OptionEffort}
		if slices.Contains(model.Efforts, run.effort) {
			effort.Current = run.effort
		}
		for _, level := range model.Efforts {
			effort.Choices = append(effort.Choices, api.ChatChoice{ID: level})
		}
		opts = append(opts, effort)
	}
	mode := api.ChatOption{ID: claudeModeOption, Category: api.OptionMode, Current: run.mode}
	for _, id := range claudeModes {
		if id == claudeAuto && !model.Auto && run.mode != claudeAuto {
			continue
		}
		mode.Choices = append(mode.Choices, api.ChatChoice{ID: id, Danger: id == claudeBypass})
	}
	return append(opts, mode)
}

// claudeCurrentModel is the choice the process runs on: the alias lich asked
// for when it still names the running model, the first that does otherwise.
// Several aliases name one model (default and opus), so the id alone cannot
// tell them apart.
func claudeCurrentModel(run claudeRunning) (string, claudeModel) {
	names := func(m claudeModel) bool { return m.Value == run.model || m.Resolved == run.model }
	if i := slices.IndexFunc(run.models, func(m claudeModel) bool { return m.Value == run.chosen && names(m) }); i >= 0 {
		return run.models[i].Value, run.models[i]
	}
	if i := slices.IndexFunc(run.models, names); i >= 0 {
		return run.models[i].Value, run.models[i]
	}
	return "", claudeModel{}
}

// modelName is the name the window gives the model id: the display name of
// the choice that resolves to it, an alias other than "default" first, since
// that one only says which model the default is. An id the chat does not list
// stays as it is.
func (d *claudeDriver) modelName(id string) string {
	d.mu.Lock()
	models := d.run.models
	d.mu.Unlock()
	named := func(m claudeModel) bool { return m.Value == id || m.Resolved == id }
	if i := slices.IndexFunc(models, func(m claudeModel) bool { return named(m) && m.Value != claudeDefault }); i >= 0 {
		return models[i].Name
	}
	if i := slices.IndexFunc(models, named); i >= 0 {
		return models[i].Name
	}
	return id
}

// claudeDefault is the alias that stands for whichever model is the default.
const claudeDefault = "default"

// producer is the model and effort an item written now is attributed to: the
// model id Claude Code put on the message, or the one the process runs on
// when the line carries none, and the effort the process runs on. A
// conversation read from the transcript knows the model only, and a model
// that takes no effort has none.
func (d *claudeDriver) producer(id string, history bool) (model, effort string) {
	if d == nil {
		return "", ""
	}
	d.mu.Lock()
	run := d.run
	d.mu.Unlock()
	if id == "" && !history {
		id = run.model
	}
	// Claude Code names the messages it writes itself "<synthetic>".
	if id == "" || strings.HasPrefix(id, "<") {
		return "", ""
	}
	model = d.modelName(id)
	if history {
		return model, ""
	}
	if i := slices.IndexFunc(run.models, func(m claudeModel) bool { return m.Value == id || m.Resolved == id }); i >= 0 && !slices.Contains(run.models[i].Efforts, run.effort) {
		return model, ""
	}
	return model, run.effort
}
