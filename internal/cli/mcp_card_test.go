package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
	"github.com/lichdotdev/lich/internal/routes"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/store"
)

// cardSessions holds, for every provider, a session nobody opened as a
// subagent ("top-<kind>") and a subagent card one level below it
// ("card-<kind>").
type cardSessions struct{ wiredSessions }

func (cardSessions) LoadState(_ context.Context, _ api.None) ([]store.Project, error) {
	var sessions []store.Session
	for _, p := range providers.Registry {
		sessions = append(sessions,
			store.Session{ID: "top-" + p.ID, Label: "top-" + p.ID, Kind: p.ID},
			store.Session{ID: "card-" + p.ID, Label: "card-" + p.ID, Kind: p.ID})
	}
	return []store.Project{{ID: "p1", Name: "lich", Sessions: sessions}}, nil
}

func (cardSessions) SessionSubagentDepth(id string) int {
	if strings.HasPrefix(id, "card-") {
		return 1
	}
	return 0
}

// listedTools runs tools/list in session id against the real relay.
func listedTools(t *testing.T, port, id string) map[string]bool {
	t.Helper()
	env := func(key string) string {
		if key == "LICH_SESSION_ID" {
			return id
		}
		return sessionEnv(port)(key)
	}
	var stdout, stderr bytes.Buffer
	c := &client{env: env, stdin: strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"),
		stdout: &stdout, stderr: &stderr, running: noInstance}
	if code := dispatch([]string{"mcp"}, c); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	var reply struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &reply); err != nil {
		t.Fatalf("decode %q: %v", stdout.String(), err)
	}
	names := map[string]bool{}
	for _, tool := range reply.Result.Tools {
		names[tool.Name] = true
	}
	return names
}

func TestMCPLeavesTheAdminToolsOutOfACardsList(t *testing.T) {
	t.Parallel()
	dispatcher := rpc.New()
	routes.Relay(dispatcher, relay.New(cardSessions{}, &wiredTerminal{}, nil))
	server := httptest.NewServer(dispatcher)
	t.Cleanup(server.Close)
	port := strconv.Itoa(server.Listener.Addr().(*net.TCPAddr).Port)

	admin := []string{
		"list_folders", "rename_folder", "color_folder", "file_session", "close_project",
		"list_services", "create_service", "edit_service", "delete_service", "start_service",
		"stop_service", "restart_service", "force_stop_service", "start_all_services",
		"stop_all_services", "read_service_output", "handoff_session",
	}
	kept := []string{
		"list_sessions", "open_session", "close_session", "send_to_session", "wait_for_answer",
		"reply_to_session", "list_worktrees", "control_session", "rename_session", "schedule_prompt",
		"show_code", "read_check_log", "mark_thread_fixed", "browser_open", "browser_screenshot",
	}
	for _, p := range providers.Registry {
		t.Run(p.ID, func(t *testing.T) {
			t.Parallel()
			top, card := listedTools(t, port, "top-"+p.ID), listedTools(t, port, "card-"+p.ID)
			if len(top) != len(mcpTools) {
				t.Errorf("a top-level session got %d tools, want all %d", len(top), len(mcpTools))
			}
			for _, name := range admin {
				if !top[name] {
					t.Errorf("a top-level session is missing %s", name)
				}
				if card[name] {
					t.Errorf("a card got %s", name)
				}
			}
			for _, name := range kept {
				if !card[name] {
					t.Errorf("a card is missing %s", name)
				}
			}
			if len(card) != len(mcpTools)-len(admin) {
				t.Errorf("a card got %d tools, want %d", len(card), len(mcpTools)-len(admin))
			}
		})
	}
}
