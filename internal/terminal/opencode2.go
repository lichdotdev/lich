package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/winexec"
)

// opencode 2 is the same provider under the same binary name, config and
// database as 1.x (its upgrade replaces 1.x in place), but its terminal
// interface takes a different argv: measured on 2.2.5, the root command has no
// --model and no --fork (both live on `run` alone, and the root refuses either
// as an unrecognized flag), and by default it attaches to a background service
// shared by every opencode on the machine, which reads its config once, when it
// starts. A per-spawn OPENCODE_CONFIG_CONTENT, profile data home or lich MCP
// server reaches that service only if this spawn happened to start it; with
// --standalone every session runs a private server under its own environment.
const (
	opencode2StandaloneFlag = providers.OpenCode2StandaloneFlag

	// opencode2ModelKey is the config key that names the model, which is how
	// a 2.x session is born on one without a flag (measured on 2.2.5: a
	// standalone spawn ran the model OPENCODE_CONFIG_CONTENT named).
	opencode2ModelKey = "model"

	// opencode2CLIConfigContentVar is opencode 2's inline terminal config,
	// merged over cli.json, and opencode2PasteFull the `prompt.paste` value
	// that types a paste out rather than folding it ("compact").
	opencode2CLIConfigContentVar = "OPENCODE_CLI_CONFIG_CONTENT"
	opencode2PasteFull           = "full"

	// opencode2ForkTimeout bounds the fork call, which boots a private server
	// for one request (0.4s measured on 2.2.5).
	opencode2ForkTimeout = 30 * time.Second
)

// opencode2Spawn tells whether this spawn runs opencode 2, and for a fork
// makes the copy first and turns opts into a resume of it. Before the lock,
// since it may run the provider's CLI twice.
func (s *Service) opencode2Spawn(opts *api.StartOptions, profileDir string) (bool, error) {
	if opts.Kind != providers.OpenCode {
		return false, nil
	}
	bin := resolveBin(opts.Kind, s.store.ProviderBin(opts.Kind, opts.ProjectID))
	v2 := providers.IsOpencode2(bin)
	if !v2 || !opts.Fork {
		return v2, nil
	}
	env := append([]string{}, s.env...)
	if profileDir != "" {
		env = profileEnv(opts.Kind, env, profileDir)
	}
	id, err := opencode2Fork(bin, opts.Cwd, opts.Resume, env)
	if err != nil {
		return true, err
	}
	opts.Resume, opts.Fork = id, false
	return true, nil
}

// opencode2Args puts a 2.x session on a private server (opencode2StandaloneFlag).
func opencode2Args(oc2 bool, args []string) []string {
	if !oc2 {
		return args
	}
	return append([]string{opencode2StandaloneFlag}, args...)
}

// opencode2PasteUnfoldEnv is pasteUnfoldEnv for opencode 2, which drops the
// 1.x `experimental.disable_paste_summary` and reads the paste summary off its
// terminal config instead: `prompt.paste` "full" in cli.json, which
// OPENCODE_CLI_CONFIG_CONTENT merges over (measured on 2.2.5 with a 30-line
// bracketed paste: "full" typed every line, the 1.x key still folded them).
func opencode2PasteUnfoldEnv(env []string, kind string, on bool) ([]string, error) {
	if !on || kind != providers.OpenCode {
		return env, nil
	}
	env, err := extendInlineConfig(env, opencode2CLIConfigContentVar, func(config map[string]any) error {
		prompt, err := objectAt(config, "prompt")
		if err == nil {
			prompt["paste"] = opencode2PasteFull
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("turn off opencode's paste summary: %w", err)
	}
	return env, nil
}

// opencode2ModelEnv names a 2.x session's model in OPENCODE_CONFIG_CONTENT,
// extending one the user exports (extendInlineConfig). A birth value, as
// in modelArgs: a resume leaves the model to the conversation.
func opencode2ModelEnv(env []string, model, resume string) ([]string, error) {
	model, usable := flagValue(model)
	if !usable || resume != "" {
		return env, nil
	}
	env, err := extendInlineConfig(env, opencodeConfigContentVar, func(config map[string]any) error {
		config[opencode2ModelKey] = model
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("name opencode's model: %w", err)
	}
	return env, nil
}

// opencode2Fork copies conversation parent into a new one through opencode's
// own fork route and returns the copy's id, which the spawn then resumes. It is
// the verb 2.x moved off its terminal interface, not a copy lich writes: the
// provider files the session itself, under the checkout the call runs in
// (location.directory in its answer, measured on 2.2.5).
func opencode2Fork(bin, dir, parent string, env []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), opencode2ForkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "api", opencode2StandaloneFlag,
		"POST", "/api/session/"+parent+"/fork", "--data", "{}")
	cmd.Dir, cmd.Env = dir, env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = time.Second
	winexec.Hide(cmd)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("opencode could not fork %s: %w: %s", parent, err, strings.TrimSpace(stderr.String()))
	}
	return parseOpencode2Fork(stdout.Bytes())
}

func parseOpencode2Fork(out []byte) (string, error) {
	var answer struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &answer); err != nil {
		return "", fmt.Errorf("opencode answered a fork with something other than a session: %w", err)
	}
	if answer.Data.ID == "" {
		return "", fmt.Errorf("opencode answered a fork without the new session's id")
	}
	return answer.Data.ID, nil
}
