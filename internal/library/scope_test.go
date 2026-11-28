package library

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

func putFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func foundNamed(list []api.LibraryFound, name string) (api.LibraryFound, bool) {
	for _, f := range list {
		if f.Name == name {
			return f, true
		}
	}
	return api.LibraryFound{}, false
}

// Every provider's user servers are found, Claude Code's ~/.claude.json,
// Codex's config.toml and opencode's opencode.jsonc among them.
func TestUserServersFoundInEveryProvider(t *testing.T) {
	for _, tc := range []struct {
		provider, path, body string
	}{
		{providers.Claude, ".claude.json", `{"mcpServers":{"memo":{"type":"http","url":"http://h/mcp"}},"projects":{}}`},
		{providers.Codex, ".codex/config.toml", "[mcp_servers.memo]\nurl = \"http://h/mcp\"\n"},
		{providers.Antigravity, ".gemini/config/mcp_config.json", `{"mcpServers":{"memo":{"serverUrl":"http://h/mcp"}}}`},
		{providers.OpenCode, ".config/opencode/opencode.jsonc", "{\n  // mine\n  \"mcp\": {\"memo\": {\"type\": \"remote\", \"url\": \"http://h/mcp\"}}\n}"},
		{providers.OMP, ".omp/agent/mcp.json", `{"mcpServers":{"memo":{"url":"http://h/mcp"}}}`},
		{providers.Crush, ".config/crush/crush.json", `{"mcp":{"memo":{"type":"http","url":"http://h/mcp"}}}`},
		{providers.Cursor, ".cursor/mcp.json", `{"mcpServers":{"memo":{"url":"http://h/mcp"}}}`},
		{providers.Kiro, ".kiro/settings/mcp.json", `{"mcpServers":{"memo":{"url":"http://h/mcp"}}}`},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			r := newRig(t)
			putFile(t, filepath.Join(r.home, filepath.FromSlash(tc.path)), tc.body)
			f, ok := foundNamed(r.found(), "memo")
			if !ok || !slices.Equal(f.Providers, []string{tc.provider}) || f.Description != "http://h/mcp" {
				t.Fatalf("found %+v, want memo from %s at http://h/mcp", f, tc.provider)
			}
		})
	}
}

// Claude Code's and Codex's user servers move in through their own CLI, which
// is what takes them out where they were; Codex's whole server comes from it.
func TestMoveInThroughTheProvidersCLI(t *testing.T) {
	for _, tc := range []struct {
		provider, path, body, get string
		want                      Server
		ran                       []string
	}{
		{
			provider: providers.Claude, path: ".claude.json",
			body: `{"mcpServers":{"memo":{"command":"memo","args":["mcp"]}}}`,
			want: Server{Command: "memo", Args: []string{"mcp"}},
			ran:  []string{"claude mcp remove -s user memo"},
		},
		{
			provider: providers.Codex, path: ".codex/config.toml",
			body: "[mcp_servers.memo]\ncommand = \"memo\"\nargs = [\"mcp\"]\n\n[mcp_servers.memo.env]\nA = \"b\"\n",
			get:  `{"name":"memo","transport":{"type":"stdio","command":"memo","args":["mcp"],"env":{"A":"b"}}}`,
			want: Server{Command: "memo", Args: []string{"mcp"}, Env: map[string]string{"A": "b"}},
			ran:  []string{"codex mcp get memo --json", "codex mcp remove memo"},
		},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			r := newRig(t)
			putFile(t, filepath.Join(r.home, filepath.FromSlash(tc.path)), tc.body)
			r.answers["codex mcp get memo --json"] = tc.get
			if _, err := r.MoveIn(t.Context(), api.LibraryItemOptions{Kind: api.LibraryMCP, Name: "memo"}); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(*r.ran, tc.ran) {
				t.Errorf("ran %q, want %q", *r.ran, tc.ran)
			}
			servers, _ := r.libraryServers()
			got := servers["memo"]
			if got.Command != tc.want.Command || !slices.Equal(got.Args, tc.want.Args) || len(got.Env) != len(tc.want.Env) {
				t.Errorf("library holds %+v, want %+v", got, tc.want)
			}
			if !r.On(api.LibraryMCP, "memo", tc.provider, "") {
				t.Error("the server is not on for the provider it came from")
			}
		})
	}
}

// A server that sends its own headers is refused before anything moves: the
// library would lose them and the server would stop answering.
func TestMoveInRefusesHeaders(t *testing.T) {
	for _, tc := range []struct{ provider, path, body, get string }{
		{providers.Claude, ".claude.json", `{"mcpServers":{"yt":{"type":"http","url":"https://yt/mcp","headers":{"Authorization":"x"}}}}`, ""},
		{providers.Codex, ".codex/config.toml", "[mcp_servers.yt]\nurl = \"https://yt/mcp\"\nbearer_token_env_var = \"T\"\n", `{"transport":{"type":"streamable_http","url":"https://yt/mcp","bearer_token_env_var":"T"}}`},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			r := newRig(t)
			putFile(t, filepath.Join(r.home, filepath.FromSlash(tc.path)), tc.body)
			r.answers["codex mcp get yt --json"] = tc.get
			if _, err := r.MoveIn(t.Context(), api.LibraryItemOptions{Kind: api.LibraryMCP, Name: "yt"}); err == nil || !strings.Contains(err.Error(), "headers") {
				t.Fatalf("MoveIn = %v, want a refusal naming the headers", err)
			}
			if servers, _ := r.libraryServers(); len(servers) != 0 {
				t.Errorf("library holds %v, want nothing", servers)
			}
			for _, line := range *r.ran {
				if strings.Contains(line, "remove") {
					t.Errorf("ran %q", line)
				}
			}
		})
	}
}

// An opencode.jsonc with comments is never rewritten, which would drop them.
func TestMoveInLeavesACommentedJSONC(t *testing.T) {
	r := newRig(t)
	path := filepath.Join(r.home, ".config", "opencode", "opencode.jsonc")
	body := "{\n  // mine\n  \"mcp\": {\"memo\": {\"type\": \"local\", \"command\": [\"memo\"]}}\n}"
	putFile(t, path, body)
	if _, err := r.MoveIn(t.Context(), api.LibraryItemOptions{Kind: api.LibraryMCP, Name: "memo"}); err == nil {
		t.Fatal("MoveIn = nil, want a refusal")
	}
	if got, _ := os.ReadFile(path); string(got) != body {
		t.Errorf("opencode.jsonc = %q, want it untouched", got)
	}
}

// A project's own servers show in its scope, one row per name with each file
// it is in, and never in the user's.
func TestProjectServers(t *testing.T) {
	for _, tc := range []struct {
		provider, path, body string
	}{
		{providers.Claude, ".mcp.json", `{"mcpServers":{"schema":{"command":"bash","args":["s.sh"]}}}`},
		{providers.Codex, ".codex/config.toml", "[mcp_servers.schema]\ncommand = \"bash\"\n"},
		{providers.OpenCode, "opencode.json", `{"mcp":{"schema":{"type":"local","command":["bash"]}}}`},
		{providers.Crush, ".crush.json", `{"mcp":{"schema":{"type":"stdio","command":"bash"}}}`},
		{providers.Cursor, ".cursor/mcp.json", `{"mcpServers":{"schema":{"command":"bash"}}}`},
		{providers.Kiro, ".kiro/settings/mcp.json", `{"mcpServers":{"schema":{"command":"bash"}}}`},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			r := newRig(t)
			project := t.TempDir()
			putFile(t, filepath.Join(project, filepath.FromSlash(tc.path)), tc.body)
			lib, err := r.List(t.Context(), api.LibraryListOptions{Project: project})
			if err != nil {
				t.Fatal(err)
			}
			f, ok := foundNamed(lib.Project, "schema")
			if !ok || !slices.Equal(f.Providers, []string{tc.provider}) || !slices.Equal(f.Paths, []string{tc.path}) || f.Transport != "stdio" {
				t.Fatalf("project %+v, want schema from %s in %s", lib.Project, tc.provider, tc.path)
			}
			if _, ok := foundNamed(lib.Found, "schema"); ok {
				t.Error("a project's server is offered to move in")
			}
			if user, _ := r.List(t.Context(), api.LibraryListOptions{}); len(user.Project) != 0 {
				t.Errorf("the user's scope shows %+v", user.Project)
			}
		})
	}
}

// Claude Code's local-scope servers for the project show in its scope too,
// from ~/.claude.json.
func TestClaudeLocalServersShowInTheProject(t *testing.T) {
	r := newRig(t)
	project := t.TempDir()
	writeJSON(t, filepath.Join(r.home, ".claude.json"), map[string]any{"projects": map[string]any{
		project: map[string]any{"mcpServers": map[string]any{"yt": map[string]any{"type": "http", "url": "https://yt/mcp"}}},
	}})
	lib, _ := r.List(t.Context(), api.LibraryListOptions{Project: project})
	f, ok := foundNamed(lib.Project, "yt")
	if !ok || !slices.Equal(f.Paths, []string{"~/.claude.json"}) || !slices.Equal(f.Providers, []string{providers.Claude}) {
		t.Fatalf("project %+v, want yt from ~/.claude.json", lib.Project)
	}
}

// The watcher reads again what a project the screens asked about declares.
func TestFingerprintFollowsAProjectAskedAbout(t *testing.T) {
	r := newRig(t)
	project := t.TempDir()
	if _, err := r.List(t.Context(), api.LibraryListOptions{Project: project}); err != nil {
		t.Fatal(err)
	}
	before := r.fingerprint()
	putFile(t, filepath.Join(project, ".mcp.json"), `{"mcpServers":{"schema":{"command":"bash"}}}`)
	if r.fingerprint() == before {
		t.Error("a server added to the project's .mcp.json did not change the fingerprint")
	}
}
