// Package codexhooks reads Codex's hooks/list, the app-server's account of
// each hook and whether the person trusted it (measured on 0.170.0), which a
// Codex chat and a Codex terminal session both read.
package codexhooks

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	ctx, cancel := context.WithTimeout(ctx, probeLimit)
	defer cancel()
	in, err := cmd.StdinPipe()
	if err != nil {
		return List{}, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return List{}, err
	}
	if err := cmd.Start(); err != nil {
		return List{}, err
	}
	defer func() { _ = in.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	go func() { <-ctx.Done(); _ = cmd.Process.Kill() }()
	for _, m := range []any{
		map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "lich", "version": "1"}}},
		map[string]any{"method": "initialized"},
		map[string]any{"id": 2, "method": "hooks/list", "params": map[string]any{"cwds": []string{cwd}}},
	} {
		if err := json.NewEncoder(in).Encode(m); err != nil {
			return List{}, err
		}
	}
	return readList(out)
}

// readList reads app-server lines until the answer to request 2.
func readList(out io.Reader) (List, error) {
	scanner := bufio.NewScanner(out)
	scanner.Buffer(make([]byte, 0, 1<<20), 8<<20)
	for scanner.Scan() {
		var msg struct {
			ID     *int            `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(scanner.Bytes(), &msg) != nil || msg.ID == nil || *msg.ID != 2 {
			continue
		}
		if msg.Error != nil {
			return List{}, fmt.Errorf("hooks/list: %s", msg.Error.Message)
		}
		var list List
		err := json.Unmarshal(msg.Result, &list)
		return list, err
	}
	if err := scanner.Err(); err != nil {
		return List{}, err
	}
	return List{}, errors.New("app-server ended before answering hooks/list")
}
