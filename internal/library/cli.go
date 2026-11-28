package library

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/winexec"
)

// What lich asks a provider's own CLI to do: turn a plugin on or off, read or
// remove an MCP server it keeps in a file the provider rewrites while it runs.

const (
	// cliTimeout bounds one provider command, run on a click.
	cliTimeout = 20 * time.Second
	// cliWaitDelay bounds the wait for what the command left on its pipes.
	cliWaitDelay = 2 * time.Second
)

// gitTool runs git itself rather than a provider's CLI.
const gitTool = "git"

// command is one provider CLI call: its arguments, the folder it runs in
// ("" for lich's own) and the variables added to lich's environment.
type command struct {
	provider, dir string
	env, args     []string
}

// runCLI runs c with the provider's binary as the person set it, else its
// default name, and answers its output.
func (s *Service) runCLI(c command) ([]byte, error) {
	bin := gitTool
	if c.provider != gitTool {
		bin = s.settings.ProviderBin(c.provider, "")
	}
	if bin == "" {
		bin = providers.DefaultBinary(c.provider)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cliTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, c.args...)
	winexec.Hide(cmd)
	cmd.Dir, cmd.WaitDelay = c.dir, cliWaitDelay
	cmd.Env = append(os.Environ(), c.env...)
	out, err := cmd.Output()
	if err != nil {
		detail := ""
		if exit, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(exit.Stderr))
		}
		return nil, fmt.Errorf("%s %s: %w: %s", bin, strings.Join(c.args, " "), err, detail)
	}
	return out, nil
}

// wholeServer is f's server with everything the library keeps: Codex's TOML
// tables are read in full by its own CLI (`codex mcp get --json`, measured on
// 0.170.0).
func (s *Service) wholeServer(f foundServer) (Server, error) {
	if f.provider != providers.Codex {
		return f.server, nil
	}
	out, err := s.cli(command{provider: providers.Codex, args: []string{"mcp", "get", f.name, "--json"}})
	if err != nil {
		return Server{}, err
	}
	var got struct {
		Transport struct {
			Type, Command, URL string
			Args               []string
			Env                map[string]string
			Bearer             string            `json:"bearer_token_env_var"`
			Headers            map[string]string `json:"http_headers"`
			EnvHeaders         map[string]string `json:"env_http_headers"`
		}
	}
	if err := json.Unmarshal(out, &got); err != nil {
		return Server{}, fmt.Errorf("read codex mcp get %s: %w", f.name, err)
	}
	t := got.Transport
	if t.Bearer != "" || len(t.Headers) > 0 || len(t.EnvHeaders) > 0 {
		return Server{}, fmt.Errorf("%s sends headers of its own in %s, which lich's library cannot keep yet", f.name, f.doc.path)
	}
	if t.URL != "" {
		return Server{Type: "http", URL: t.URL}, nil
	}
	return Server{Command: t.Command, Args: t.Args, Env: t.Env}, nil
}

// removeServer takes name out of Personal's user servers through provider's
// own CLI: `claude mcp remove -s user`, whose change a running Claude Code
// keeps through its own rewrites of ~/.claude.json (measured on 2.1.332), and
// `codex mcp remove`.
func (s *Service) removeServer(provider, name string) error {
	args := []string{"mcp", "remove", name}
	if provider == providers.Claude {
		args = []string{"mcp", "remove", "-s", "user", name}
	}
	_, err := s.cli(command{provider: provider, args: args})
	return err
}
