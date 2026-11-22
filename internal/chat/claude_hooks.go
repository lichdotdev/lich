package chat

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/lichdotdev/lich/api"
)

// A Claude Code chat's hooks, as measured on 2.1.330: get_hooks_listing gives
// what its /hooks lists, by event and matcher, with why none run. That screen
// is read only ("To add or change a hook, edit settings.json or ask Claude"),
// so lich's is too.

var errClaudeHooksReadOnly = errors.New("Claude Code's hooks are changed in its settings files, not from its hooks screen")

type claudeHookListing struct {
	Events []struct {
		Name    string `json:"name"`
		Summary string `json:"summary"`
	} `json:"events"`
	Hooks []struct {
		Event     string `json:"event"`
		Matcher   string `json:"matcher"`
		Source    string `json:"source"`
		Plugin    string `json:"pluginName"`
		Type      string `json:"type"`
		Display   string `json:"displayText"`
		Command   string `json:"commandText"`
		Condition string `json:"condition"`
		Timeout   int    `json:"timeout"`
		Disabled  bool   `json:"disabled"`
	} `json:"hooks"`
	Policy struct {
		DisabledByPolicy bool `json:"disabledByPolicy"`
		ManagedOnly      bool `json:"managedOnly"`
		AllDisabled      bool `json:"allDisabled"`
	} `json:"policy"`
	SafeMode *struct{}             `json:"safeMode"`
	BareMode *struct{}             `json:"bareMode"`
	Errors   []claudeSettingsError `json:"errors"`
}

func (d *claudeDriver) hooks(ctx context.Context) (api.ChatHookList, error) {
	r, err := d.control(ctx, map[string]any{"subtype": "get_hooks_listing"})
	if err != nil {
		return api.ChatHookList{}, err
	}
	var l claudeHookListing
	if json.Unmarshal(r.Body, &l) != nil || l.Events == nil {
		return api.ChatHookList{}, nil
	}
	return claudeHookList(l), nil
}

func claudeHookList(l claudeHookListing) api.ChatHookList {
	summaries := map[string]string{}
	for _, e := range l.Events {
		summaries[e.Name] = e.Summary
	}
	list := api.ChatHookList{Reported: true, Blocked: claudeHooksBlocked(l), Errors: claudeErrors(l.Errors)}
	for _, h := range l.Hooks {
		list.Hooks = append(list.Hooks, api.ChatHook{
			Event: h.Event, Summary: summaries[h.Event], Matcher: h.Matcher, Source: claudeSource(h.Source), Plugin: h.Plugin,
			Type: h.Type, Command: h.Command, Label: h.Display, Condition: h.Condition, Timeout: h.Timeout, Off: h.Disabled,
		})
	}
	return list
}

func claudeHooksBlocked(l claudeHookListing) string {
	switch {
	case l.Policy.DisabledByPolicy:
		return api.HooksOffByPolicy
	case l.Policy.ManagedOnly:
		return api.HooksManagedOnly
	case l.Policy.AllDisabled:
		return api.HooksOff
	case l.SafeMode != nil:
		return api.HooksSafeMode
	case l.BareMode != nil:
		return api.HooksBareMode
	}
	return ""
}

func (d *claudeDriver) hookAct(context.Context, api.ChatHookActOptions) error {
	return errClaudeHooksReadOnly
}
