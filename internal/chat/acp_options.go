package chat

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// What an ACP chat can be set to is what session/new and session/load answer
// with, and what config_option_update and current_mode_update say later:
// configOptions where the server has them (opencode, oh-my-pi, Antigravity),
// and the older modes and models where it has none of a category (Kiro, which
// has only those). Measured on opencode 1.19.4, oh-my-pi 18.8.9, Antigravity
// 1.5.3 and Kiro 2.21.0.

// The ids lich gives the older modes and models, which carry none.
const (
	acpModesOption  = "mode"
	acpModelsOption = "model"
)

// acpCategories are the config option categories lich offers, the contract's.
var acpCategories = []string{api.OptionModel, api.OptionEffort, api.OptionMode}

// acpSession is the part of session/new's and session/load's answer that says
// what the conversation can be set to.
type acpSession struct {
	Config []acpConfigOption `json:"configOptions"`
	Modes  *struct {
		Current   string     `json:"currentModeId"`
		Available []acpNamed `json:"availableModes"`
	} `json:"modes"`
	Models *struct {
		Current   string     `json:"currentModelId"`
		Available []acpNamed `json:"availableModels"`
	} `json:"models"`
}

type acpNamed struct {
	ID          string `json:"id"`
	ModelID     string `json:"modelId"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type acpConfigOption struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Type     string `json:"type"`
	Current  string `json:"currentValue"`
	Options  []struct {
		Value       string `json:"value"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"options"`
}

// acpSettings is what an ACP chat can be set to: the config options, and the
// older modes and models for a category they lack.
type acpSettings struct {
	config        []api.ChatOption
	modes, models *api.ChatOption
}

// readSession keeps what a session/new or session/load answer says.
func (d *acpDriver) readSession(raw json.RawMessage) {
	var s acpSession
	_ = json.Unmarshal(raw, &s)
	var modes, models *api.ChatOption
	if s.Modes != nil {
		modes = &api.ChatOption{ID: acpModesOption, Category: api.OptionMode, Current: s.Modes.Current}
		for _, m := range s.Modes.Available {
			modes.Choices = append(modes.Choices, api.ChatChoice{ID: m.ID, Label: m.Name, Description: m.Description})
		}
	}
	if s.Models != nil {
		models = &api.ChatOption{ID: acpModelsOption, Category: api.OptionModel, Current: s.Models.Current}
		for _, m := range s.Models.Available {
			models.Choices = append(models.Choices, api.ChatChoice{ID: m.ModelID, Label: m.Name, Description: m.Description})
		}
	}
	d.mu.Lock()
	d.settings = acpSettings{config: acpConfig(s.Config), modes: modes, models: models}
	d.mu.Unlock()
	d.publish()
}

// acpConfig is the select options of the contract's categories.
func acpConfig(raw []acpConfigOption) []api.ChatOption {
	var opts []api.ChatOption
	for _, o := range raw {
		if !slices.Contains(acpCategories, o.Category) || (o.Type != "" && o.Type != "select") {
			continue
		}
		opt := api.ChatOption{ID: o.ID, Category: o.Category, Label: o.Name, Current: o.Current}
		for _, ch := range o.Options {
			opt.Choices = append(opt.Choices, api.ChatChoice{ID: ch.Value, Label: ch.Name, Description: ch.Description})
		}
		opts = append(opts, opt)
	}
	return opts
}

// options is the config options, then the older modes and models for a
// category they lack.
func (s acpSettings) options() []api.ChatOption {
	opts := slices.Clone(s.config)
	for _, legacy := range []*api.ChatOption{s.models, s.modes} {
		has := func(o api.ChatOption) bool { return legacy != nil && o.Category == legacy.Category }
		if legacy != nil && !slices.ContainsFunc(s.config, has) {
			opts = append(opts, *legacy)
		}
	}
	return opts
}

func (d *acpDriver) publish() {
	d.mu.Lock()
	opts := d.settings.options()
	d.mu.Unlock()
	d.c.setOptions(opts)
}

// configChanged reads a config_option_update, which carries every option:
// opencode's effort appears only once a model that has one is set.
func (d *acpDriver) configChanged(raw json.RawMessage) {
	var u struct {
		Config []acpConfigOption `json:"configOptions"`
	}
	if json.Unmarshal(raw, &u) != nil || u.Config == nil {
		return
	}
	d.mu.Lock()
	d.settings.config = acpConfig(u.Config)
	d.mu.Unlock()
	d.publish()
}

// modeChanged reads a current_mode_update, for the older modes or, when
// there are none, the config option of the mode category.
func (d *acpDriver) modeChanged(mode string) {
	if mode == "" {
		return
	}
	d.mu.Lock()
	d.settings.setMode(mode)
	d.mu.Unlock()
	d.publish()
}

func (s *acpSettings) setMode(mode string) {
	if s.modes != nil {
		s.modes.Current = mode
		return
	}
	for i := range s.config {
		if s.config[i].Category == api.OptionMode {
			s.config[i].Current = mode
		}
	}
}

func (d *acpDriver) setOption(ctx context.Context, o api.ChatOption, value string) error {
	session := d.sessionID()
	d.mu.Lock()
	config := slices.ContainsFunc(d.settings.config, func(c api.ChatOption) bool { return c.ID == o.ID })
	d.mu.Unlock()
	switch {
	case config:
		raw, err := d.call(ctx, "session/set_config_option", map[string]any{"sessionId": session, "configId": o.ID, "value": value})
		if err != nil {
			return err
		}
		d.configSet(o.ID, value, raw)
	case o.ID == acpModesOption:
		if _, err := d.call(ctx, "session/set_mode", map[string]any{"sessionId": session, "modeId": value}); err != nil {
			return err
		}
		d.mu.Lock()
		d.settings.setMode(value)
		d.mu.Unlock()
	case o.ID == acpModelsOption && d.c.kind == providers.Kiro:
		// Kiro's session/set_model never answers and leaves the model as it
		// was; `acp --model` holds on a session/load (measured on 2.21.0).
		return errRelaunch
	default:
		if _, err := d.call(ctx, "session/set_model", map[string]any{"sessionId": session, "modelId": value}); err != nil {
			return err
		}
		d.mu.Lock()
		if d.settings.models != nil {
			d.settings.models.Current = value
		}
		d.mu.Unlock()
	}
	d.publish()
	return nil
}

// configSet records a config option set: the answer carries every option
// where the server sends them, and only the one set changed where it does not.
func (d *acpDriver) configSet(id, value string, raw json.RawMessage) {
	var set struct {
		Config []acpConfigOption `json:"configOptions"`
	}
	_ = json.Unmarshal(raw, &set)
	d.mu.Lock()
	defer d.mu.Unlock()
	if set.Config != nil {
		d.settings.config = acpConfig(set.Config)
		return
	}
	for i := range d.settings.config {
		if d.settings.config[i].ID == id {
			d.settings.config[i].Current = value
		}
	}
}

// producer is the model and effort an item that begins now is attributed to:
// the current choice of each, by name where the provider gives one. What a
// session/load replays is not attributed: the provider does not say.
func (d *acpDriver) producer() (model, effort string) {
	if d.c.conv.replaying() {
		return "", ""
	}
	d.mu.Lock()
	opts := d.settings.options()
	d.mu.Unlock()
	named := func(o api.ChatOption) string {
		if i := slices.IndexFunc(o.Choices, func(c api.ChatChoice) bool { return c.ID == o.Current }); i >= 0 && o.Choices[i].Label != "" {
			return o.Choices[i].Label
		}
		return o.Current
	}
	for _, o := range opts {
		switch o.Category {
		case api.OptionModel:
			model = named(o)
		case api.OptionEffort:
			effort = named(o)
		}
	}
	return model, effort
}
