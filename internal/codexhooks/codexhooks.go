// Package codexhooks reads Codex's hooks/list, the app-server's account of
// each hook and whether the person trusted it (measured on 0.170.0), which a
// Codex chat and a Codex terminal session both read.
package codexhooks

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/lichdotdev/lich/api"
)

// Hook is one hooks/list entry.
type Hook struct {
	Key       string `json:"key"`
	Event     string `json:"eventName"`
	Type      string `json:"handlerType"`
	Matcher   string `json:"matcher"`
	Command   string `json:"command"`
	Timeout   int    `json:"timeoutSec"`
	Status    string `json:"statusMessage"`
	Source    string `json:"source"`
	Plugin    string `json:"pluginId"`
	Enabled   bool   `json:"enabled"`
	IsManaged bool   `json:"isManaged"`
	Hash      string `json:"currentHash"`
	Trust     string `json:"trustStatus"`
}

// List is hooks/list's answer.
type List struct {
	Data []struct {
		Hooks  []Hook `json:"hooks"`
		Errors []struct {
			Message string `json:"message"`
			Path    string `json:"path"`
		} `json:"errors"`
	} `json:"data"`
}

// lichPluginID is how hooks/list names lich-plugin's hooks (measured on
// 0.170.0: "lich@lich-plugin", source "plugin").
const lichPluginID = "lich@lich-plugin"

// LichTrust is the trust of a lich-plugin PreToolUse hook Codex is not
// running because its content changed or was never trusted, "" when every one
// is trusted or none is installed. Codex runs such a hook not at all and says
// nothing (docs/hooks/subagent-cards.md, Known ceilings), so its subagents
// stay native.
func LichTrust(list List) string {
	for _, entry := range list.Data {
		for _, h := range entry.Hooks {
			if h.Plugin == lichPluginID && h.Event == "preToolUse" && (h.Trust == api.HookModified || h.Trust == api.HookUntrusted) {
				return h.Trust
			}
		}
	}
	return ""
}

// probeLimit bounds a short-lived app-server's whole read.
const probeLimit = 10 * time.Second

// Probe reads hooks/list for cwd through cmd, a `codex app-server` not yet
// started, and ends it: the app-server's stdio JSON-RPC, initialize then
// hooks/list, nothing written to Codex's config.
func Probe(ctx context.Context, cmd *exec.Cmd, cwd string) (List, error) {
	var list List
	err := session(ctx, cmd, cwd, func(_ *json.Encoder, _ *bufio.Scanner, l List) error {
		list = l
		return nil
	})
	return list, err
}

// TrustLich trusts every lich-plugin hook Codex holds as untrusted or modified
// (LichTrust's states), through cmd, a `codex app-server` not yet started: the
// same hooks.state write a Codex chat's /hooks makes, for lich's own hooks
// alone, so a session runs them with no /hooks step. It reports how many it
// trusted.
func TrustLich(ctx context.Context, cmd *exec.Cmd, cwd string) (int, error) {
	trusted := 0
	err := session(ctx, cmd, cwd, func(in *json.Encoder, out *bufio.Scanner, list List) error {
		state := map[string]any{}
		for _, entry := range list.Data {
			for _, h := range entry.Hooks {
				if h.Plugin == lichPluginID && h.Hash != "" && (h.Trust == api.HookModified || h.Trust == api.HookUntrusted) {
					state[h.Key] = map[string]any{"trusted_hash": h.Hash}
				}
			}
		}
		if len(state) == 0 {
			return nil
		}
		write := map[string]any{"id": 3, "method": "config/batchWrite", "params": map[string]any{
			"edits": []any{map[string]any{"keyPath": "hooks.state", "mergeStrategy": "upsert", "value": state}},
		}}
		if err := in.Encode(write); err != nil {
			return err
		}
		if _, err := answer(out, 3, "config/batchWrite"); err != nil {
			return err
		}
		trusted = len(state)
		return nil
	})
	return trusted, err
}

// session starts cmd, reads hooks/list for cwd and hands it to then with the
// app-server's stdin and stdout, ending the server once then returns.
func session(ctx context.Context, cmd *exec.Cmd, cwd string, then func(*json.Encoder, *bufio.Scanner, List) error) error {
	ctx, cancel := context.WithTimeout(ctx, probeLimit)
	defer cancel()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() { _ = stdin.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	go func() { <-ctx.Done(); _ = cmd.Process.Kill() }()
	in := json.NewEncoder(stdin)
	for _, m := range []any{
		map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "lich", "version": "1"}}},
		map[string]any{"method": "initialized"},
		map[string]any{"id": 2, "method": "hooks/list", "params": map[string]any{"cwds": []string{cwd}}},
	} {
		if err := in.Encode(m); err != nil {
			return err
		}
	}
	out := bufio.NewScanner(stdout)
	out.Buffer(make([]byte, 0, 1<<20), 8<<20)
	raw, err := answer(out, 2, "hooks/list")
	if err != nil {
		return err
	}
	var list List
	if err := json.Unmarshal(raw, &list); err != nil {
		return err
	}
	return then(in, out, list)
}

// answer reads app-server lines until the answer to request id.
func answer(out *bufio.Scanner, id int, method string) (json.RawMessage, error) {
	for out.Scan() {
		var msg struct {
			ID     *int            `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(out.Bytes(), &msg) != nil || msg.ID == nil || *msg.ID != id {
			continue
		}
		if msg.Error != nil {
			return nil, fmt.Errorf("%s: %s", method, msg.Error.Message)
		}
		return msg.Result, nil
	}
	if err := out.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("app-server ended before answering %s", method)
}
