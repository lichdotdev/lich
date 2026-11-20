//go:build !windows

package chat

import (
	"os"
	"slices"
)

// fakeLegacyEnv has the fake ACP server answer with the older modes and models
// instead of configOptions, the way Kiro does.
const fakeLegacyEnv = "LICH_CHAT_FAKE_LEGACY"

// fakeClaude is what the fake Claude Code runs on, the shapes measured on
// 2.1.327.
type fakeClaude struct {
	model, effort, mode string
	bypass              bool
}

var fakeClaudeModels = []any{
	map[string]any{"value": "default", "resolvedModel": "claude-opus-5-5", "displayName": "Default (recommended)", "supportsEffort": true, "supportedEffortLevels": []any{"low", "high"}, "supportsAutoMode": true},
	map[string]any{"value": "opus", "resolvedModel": "claude-opus-5-5", "displayName": "Opus 5.5", "supportsEffort": true, "supportedEffortLevels": []any{"low", "high"}, "supportsAutoMode": true},
	map[string]any{"value": "haiku", "resolvedModel": "claude-haiku-5-5", "displayName": "Haiku 5.5", "supportsEffort": true, "supportedEffortLevels": []any{"low", "medium"}},
	map[string]any{"value": "old", "resolvedModel": "claude-old", "displayName": "Old"},
}

func newFakeClaude(args []string) *fakeClaude {
	run := &fakeClaude{model: "claude-opus-5-5", effort: "high", mode: "auto"}
	if i := slices.Index(args, "--model"); i >= 0 && i+1 < len(args) {
		run.model = fakeResolve(args[i+1])
	}
	run.bypass = slices.Contains(args, "--allow-dangerously-skip-permissions") || slices.Contains(args, "--dangerously-skip-permissions")
	if slices.Contains(args, "--dangerously-skip-permissions") {
		run.mode = "bypassPermissions"
	}
	return run
}

func fakeResolve(alias string) string {
	for _, m := range fakeClaudeModels {
		if mm := m.(map[string]any); mm["value"] == alias {
			return mm["resolvedModel"].(string)
		}
	}
	return alias
}

func (f *fake) claudeControl(run *fakeClaude, m map[string]any) {
	id := str(m, "request_id")
	ok := func(body any) {
		f.send(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": id, "response": body}})
	}
	switch str(m, "request", "subtype") {
	case "initialize":
		ok(map[string]any{"models": fakeClaudeModels, "current_permission_mode": run.mode, "commands": fakeClaudeCommands})
	case "get_settings":
		ok(map[string]any{"applied": map[string]any{"model": run.model, "effort": run.effort}})
	case "set_model":
		run.model = fakeResolve(str(m, "request", "model"))
		ok(map[string]any{"model": run.model})
	case "apply_flag_settings":
		run.effort = str(m, "request", "settings", "effortLevel")
		ok(nil)
	case "set_permission_mode":
		mode := str(m, "request", "mode")
		if mode == "bypassPermissions" && !run.bypass {
			f.send(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "error", "request_id": id, "error": "not launched with it", "error_code": "bypass_not_launched"}})
			return
		}
		// The real CLI takes dontAsk; the fake refuses it to have a refusal.
		if mode == "dontAsk" {
			f.send(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "error", "request_id": id, "error": "dontAsk is off here"}})
			return
		}
		run.mode = mode
		ok(map[string]any{"mode": mode})
		f.send(map[string]any{"type": "system", "subtype": "status", "permissionMode": mode})
	default:
		ok(map[string]any{})
	}
}

// fakeACP is what the fake ACP server offers, configOptions the way oh-my-pi
// and opencode do, or the older modes and models the way Kiro does.
type fakeACP struct {
	legacy              bool
	model, effort, mode string
}

func newFakeACP(args []string) *fakeACP {
	o := &fakeACP{legacy: os.Getenv(fakeLegacyEnv) != "", model: "m1", effort: "high", mode: "default"}
	if i := slices.Index(args, "--model"); i >= 0 && i+1 < len(args) {
		o.model = args[i+1]
	}
	return o
}

func (o *fakeACP) config() []any {
	opts := []any{
		map[string]any{"id": "mode", "name": "Mode", "category": "mode", "type": "select", "currentValue": o.mode, "options": []any{
			map[string]any{"value": "default", "name": "Default"}, map[string]any{"value": "plan", "name": "Plan"},
		}},
		map[string]any{"id": "model", "name": "Model", "category": "model", "type": "select", "currentValue": o.model, "options": []any{
			map[string]any{"value": "m1", "name": "M1"}, map[string]any{"value": "m2", "name": "M2"},
		}},
		map[string]any{"id": "sound", "name": "Sound", "category": "other", "type": "select", "currentValue": "on"},
	}
	// An effort that only one model has, the way opencode's appears.
	if o.model == "m1" {
		opts = append(opts, map[string]any{"id": "thinking", "name": "Thinking", "category": "thought_level", "type": "select", "currentValue": o.effort, "options": []any{
			map[string]any{"value": "low", "name": "low"}, map[string]any{"value": "high", "name": "high"},
		}})
	}
	return opts
}

// session fills a session/new or session/load answer.
func (o *fakeACP) session(answer map[string]any) map[string]any {
	if !o.legacy {
		answer["configOptions"] = o.config()
		return answer
	}
	answer["modes"] = map[string]any{"currentModeId": o.mode, "availableModes": []any{
		map[string]any{"id": "default", "name": "default"}, map[string]any{"id": "plan", "name": "planner", "description": "plans"},
	}}
	answer["models"] = map[string]any{"currentModelId": o.model, "availableModels": []any{
		map[string]any{"modelId": "m1", "name": "m1"}, map[string]any{"modelId": "m2", "name": "m2"},
	}}
	return answer
}

func (f *fake) acpSet(o *fakeACP, id any, method string, params map[string]any) {
	value := str(params, "value")
	switch method {
	case "session/set_mode":
		o.mode = str(params, "modeId")
		f.result(id, map[string]any{})
		return
	case "session/set_model":
		o.model = str(params, "modelId")
		f.result(id, map[string]any{})
		return
	}
	switch str(params, "configId") {
	case "model":
		// Answers with every option and says so in an update too, as the
		// spec has it.
		o.model = value
		f.update(map[string]any{"sessionUpdate": "config_option_update", "configOptions": o.config()})
		f.result(id, map[string]any{"configOptions": o.config()})
	case "thinking":
		o.effort = value
		f.result(id, map[string]any{})
	case "mode":
		o.mode = value
		f.result(id, nil)
	default:
		f.result(id, map[string]any{})
	}
}
