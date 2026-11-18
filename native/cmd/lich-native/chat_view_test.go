package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// chatBackend records the chat calls the window makes, and answers each with
// nothing.
type chatBackend struct {
	mu    sync.Mutex
	calls []string
}

func newChatBackend(t *testing.T) (*chatBackend, *backend.Client) {
	t.Helper()
	f := new(chatBackend)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.calls = append(f.calls, strings.TrimPrefix(r.URL.Path, "/rpc/")+" "+string(body))
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(nil)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return f, backend.New(backend.Runtime{Port: port, Token: "tok"})
}

// waitCall waits for a call to method and answers its body.
func (f *chatBackend) waitCall(t *testing.T, method string) string {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		f.mu.Lock()
		for _, c := range f.calls {
			if m, body, _ := strings.Cut(c, " "); m == method {
				f.mu.Unlock()
				return body
			}
		}
		f.mu.Unlock()
	}
	t.Fatalf("no %s call", method)
	return ""
}

// chatFrames is session s1, a Claude Code session in view, drawn through
// content as the island draws it.
func chatFrames(t *testing.T, view string) (*window, *frameRig, *chatBackend) {
	t.Helper()
	r := newFrameRig(rigWindow, nil)
	f, client := newChatBackend(t)
	u := &window{ctx: t.Context(), client: client, model: chatModel(view)}
	u.model.invalidate = r.invalidate
	u.model.chat.kinds = []string{"claude"}
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			sv, _ := u.model.sessionViewOf("s1")
			if err := u.content(gtx, sv, true); err != nil {
				t.Fatal(err)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	return u, r, f
}

func TestTheSwitchIsOffInTheFrameThatTookItsClick(t *testing.T) {
	u, r, f := chatFrames(t, api.ViewChat)
	r.clickLabel(t, i18n.T("chat.switch.tui"))
	if !u.model.chat.isSwitching("s1") {
		t.Fatal("the click missed TUI")
	}
	if body := f.waitCall(t, api.ChatSetView.Name); !strings.Contains(body, `"view":"terminal"`) {
		t.Errorf("asked %s", body)
	}
}

func TestTheSwitchTakesNoClickWhileTheAgentWorks(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewTerminal)
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	u.model.chat.kinds = []string{"claude"}
	// The gate holds the terminal's spawn: the pane draws nothing but the switch.
	u.terms = &terminals{byID: map[string]*termView{}, invalidate: r.invalidate}
	u.gate = &spawnGate{probe: &fakeProbe{live: true}, invalidate: r.invalidate, deciding: "another"}
	r.frame()
	r.click(t, r.at(t, i18n.T("chat.switch.gui")))
	if u.model.chat.isSwitching("s1") {
		t.Error("switched while the agent was in a turn")
	}
}

func TestAShellSessionHasNoSwitch(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewTerminal)
	u.model.projects[0].Sessions[0].Kind = "shell"
	u.model.chat.kinds = []string{"claude", "shell"}
	u.terms = &terminals{byID: map[string]*termView{}, invalidate: r.invalidate}
	u.gate = &spawnGate{probe: &fakeProbe{live: true}, invalidate: r.invalidate, deciding: "another"}
	r.frame()
	if r.shows(i18n.T("chat.switch.gui")) || r.shows(i18n.T("chat.switch.tui")) {
		t.Error("a shell session drew the switch")
	}
}

func TestAPermissionButtonAnswersWithItsOption(t *testing.T) {
	u, r, f := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "fix it"},
		{ID: "p1", Kind: api.ItemPermission, Turn: 1, Permission: &backend.ChatPermission{
			Tool:    backend.ChatToolCall{Name: "Edit", Title: "src/total.ts", State: api.ToolPending},
			Options: []api.ChatPermitOption{{ID: "yes", Label: "Allow", Kind: api.PermitAllowOnce}, {ID: "no", Label: "Deny", Kind: api.PermitRejectOnce}},
		}},
	})
	r.frame()
	r.click(t, r.at(t, "Deny"))
	if body := f.waitCall(t, api.ChatAnswer.Name); !strings.Contains(body, `"itemId":"p1"`) || !strings.Contains(body, `"optionId":"no"`) {
		t.Errorf("answered %s", body)
	}
}

func TestStopCancelsTheTurnInFlight(t *testing.T) {
	u, r, f := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go"}})
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	r.frame()
	r.click(t, r.at(t, i18n.T("chat.composer.stop")))
	if body := f.waitCall(t, api.ChatCancel.Name); !strings.Contains(body, `"id":"s1"`) {
		t.Errorf("cancelled %s", body)
	}
}

func TestAToolFoldOpensInTheFrameThatTookTheClick(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: 0, Text: "look"},
		{ID: "t1", Kind: api.ItemTool, Turn: 1, At: 3000, Tool: &backend.ChatToolCall{Name: "Grep", Title: "roundTax", State: api.ToolDone, Output: "4 results"}},
	})
	worked := i18n.T("chat.view.worked", i18n.Args{"duration": formatAge(3 * time.Second), "count": 1})
	r.clickLabel(t, worked)
	if !r.shows("4 results") {
		t.Errorf("the fold did not open:\n%s", r.drawn())
	}
}
