package main

import (
	"encoding/json"
	"image"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gioui.org/io/key"

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
// content as the island draws it, its footer reading busySession.
func chatFrames(t *testing.T, view string) (*window, *frameRig, *chatBackend) {
	return chatFramesIn(t, view, rigWindow, busySession())
}

// chatFramesIn is chatFrames on a stage of size, its footer reading in.
func chatFramesIn(t *testing.T, view string, size image.Point, in footerInput) (*window, *frameRig, *chatBackend) {
	t.Helper()
	r := newFrameRig(size, nil)
	f, client := newChatBackend(t)
	foot := newFooterRig(t, nil).f
	foot.dock, foot.git = new(dock), new(gitPanel)
	u := &window{ctx: t.Context(), client: client, model: chatModel(view), footer: foot}
	u.model.invalidate = r.invalidate
	u.model.chat.kinds = []string{"claude"}
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			u.footer.frame(gtx, in)
			u.footer.actGit(gtx)
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

func permissionFrames(t *testing.T) (*window, *frameRig, *chatBackend) {
	t.Helper()
	u, r, f := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "fix it"},
		{ID: "p1", Kind: api.ItemPermission, Turn: 1, Permission: &backend.ChatPermission{
			Tool: backend.ChatToolCall{Name: "Edit", Title: "src/total.ts", State: api.ToolPending},
			Options: []api.ChatPermitOption{
				{ID: "always", Label: "Always allow", Kind: api.PermitAllowAlways},
				{ID: "yes", Label: "Allow", Kind: api.PermitAllowOnce},
				{ID: "no", Label: "Deny", Kind: api.PermitRejectOnce},
			},
		}},
	})
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	r.frame()
	r.frame()
	return u, r, f
}

func (r *frameRig) press(name key.Name) {
	r.router.Queue(key.Event{Name: name, State: key.Press}, key.Event{Name: name, State: key.Release})
	r.frame()
}

func TestEnterAllowsOnceAndEscRejectsAWaitingPermission(t *testing.T) {
	for name, want := range map[key.Name]string{key.NameReturn: `"optionId":"yes"`, key.NameEscape: `"optionId":"no"`} {
		_, r, f := permissionFrames(t)
		r.press(name)
		if body := f.waitCall(t, api.ChatAnswer.Name); !strings.Contains(body, want) {
			t.Errorf("%s answered %s, want %s", name, body, want)
		}
	}
}

func TestEnterOverADraftAnswersNothing(t *testing.T) {
	u, r, f := permissionFrames(t)
	u.chats.pane("s1").editor.SetText("wait, use cents")
	r.press(key.NameReturn)
	time.Sleep(50 * time.Millisecond)
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 0 {
		t.Errorf("Enter over a draft called %v", f.calls)
	}
}

func userTurns(n int) []backend.ChatItem {
	var items []backend.ChatItem
	for i := range n {
		items = append(items, backend.ChatItem{ID: "u" + strconv.Itoa(i), Kind: api.ItemUser, Turn: i + 1, Text: "q" + strconv.Itoa(i)})
	}
	return items
}

func labelTop(t *testing.T, r *frameRig, label string) int {
	t.Helper()
	for _, n := range r.nodes() {
		if n.Desc.Label == label {
			return n.Desc.Bounds.Min.Y
		}
	}
	return -1
}

// A short conversation starts at the top; a long one shows its end, and keeps
// it as items arrive.
func TestTheChatStartsAtItsTopAndFollowsItsEnd(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", userTurns(2))
	r.settle(t)
	if y := labelTop(t, r, "q0"); y < 0 || y > rigWindow.Y/4 {
		t.Errorf("a short conversation's first message at y=%d, want near the top", y)
	}
	u.model.chat.history("s1", userTurns(60))
	r.settle(t)
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "late", Kind: api.ItemUser, Turn: 61, Text: "last one"}})
	r.settle(t)
	if labelTop(t, r, "last one") < 0 {
		t.Errorf("the newest message is off screen:\n%s", r.drawn())
	}
}
