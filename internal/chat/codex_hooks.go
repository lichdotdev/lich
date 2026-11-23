package chat

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"unicode"
	"unicode/utf8"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/codexhooks"
)

// A Codex chat's hooks, as measured on 0.170.0: hooks/list gives each hook
// of the checkout with its key, its content's hash and whether the person
// trusted it, a hook running only once trusted. Its /hooks turns one on or
// off and trusts it; lich writes the same hooks.state entry of config.toml,
// keyed by the hook's key, through config/batchWrite: trusted_hash set to
// the hash makes hooks/list call it trusted, enabled false turns it off. An
// upsert of hooks.state keeps every other entry and field, lich-plugin's
// trust among them, and reloadUserConfig has the running thread take it.

var codexHookActions = []string{api.HookTrust, api.HookEnable, api.HookDisable}

var errHookHash = errors.New("a hook is trusted for the content the person saw, which the request does not name")

// codexHookSources is lich's name of each of Codex's hook sources; the rest
// are an organization's.
var codexHookSources = map[string]string{
	"user": api.SettingsUser, "project": api.SettingsProject, "plugin": api.SettingsPlugin,
	"sessionFlags": api.SettingsFlag, "unknown": api.SettingsOther,
}

func (d *codexDriver) hookList(ctx context.Context) (codexhooks.List, error) {
	raw, err := d.call(ctx, "hooks/list", map[string]any{"cwds": []string{d.c.cwd}})
	if err != nil {
		return codexhooks.List{}, err
	}
	var body codexhooks.List
	err = json.Unmarshal(raw, &body)
	return body, err
}

func (d *codexDriver) hooks(ctx context.Context) (api.ChatHookList, error) {
	body, err := d.hookList(ctx)
	if err != nil {
		return api.ChatHookList{}, err
	}
	list := api.ChatHookList{Reported: true, Actions: codexHookActions}
	for _, entry := range body.Data {
		for _, h := range entry.Hooks {
			list.Hooks = append(list.Hooks, codexChatHook(h))
		}
		for _, e := range entry.Errors {
			list.Errors = append(list.Errors, e.Path+": "+e.Message)
		}
	}
	return list, nil
}

func codexChatHook(h codexhooks.Hook) api.ChatHook {
	source, ok := codexHookSources[h.Source]
	if !ok {
		source = api.SettingsManaged
	}
	trust := h.Trust
	if h.IsManaged {
		trust = ""
	}
	return api.ChatHook{
		Event: upperFirst(h.Event), Matcher: h.Matcher, Source: source, Plugin: h.Plugin, Type: h.Type,
		Command: h.Command, Label: h.Status, Timeout: h.Timeout, Off: !h.Enabled || trust == api.HookUntrusted || trust == api.HookModified,
		Key: h.Key, Hash: h.Hash, Trust: trust, Managed: h.IsManaged,
	}
}

// upperFirst names an event as the other providers do: preToolUse is
// PreToolUse.
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

func (d *codexDriver) hookAct(ctx context.Context, opts api.ChatHookActOptions) error {
	state := map[string]any{"enabled": opts.Action == api.HookEnable}
	if opts.Action == api.HookTrust {
		if opts.Hash == "" {
			return errHookHash
		}
		state = map[string]any{"trusted_hash": opts.Hash}
	}
	_, err := d.call(ctx, "config/batchWrite", map[string]any{
		"edits":            []any{map[string]any{"keyPath": "hooks.state", "mergeStrategy": "upsert", "value": map[string]any{opts.Key: state}}},
		"reloadUserConfig": true,
	})
	return err
}

// checkLichHook tells the window once per run that Codex skips lich-plugin's
// PreToolUse hook (terminal.ReportCodexHook), read from a Codex chat's own
// hooks/list: one whose read failed leaves the check to the next session.
func (d *codexDriver) checkLichHook(ctx context.Context) {
	term := d.c.svc.term
	if term.CodexHookChecked() {
		return
	}
	list, err := d.hookList(ctx)
	if err != nil {
		slog.Debug("chat: read Codex hooks", "session", d.c.id, "err", err)
		return
	}
	term.ReportCodexHook(d.c.id, codexhooks.LichTrust(list))
}
