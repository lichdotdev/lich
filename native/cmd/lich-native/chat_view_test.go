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
	// answers is what a method answers, by its name; null for any other.
	answers map[string]any
}

func newChatBackend(t testing.TB) (*chatBackend, *backend.Client) {
	t.Helper()
	f := new(chatBackend)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		method := strings.TrimPrefix(r.URL.Path, "/rpc/")
		f.calls = append(f.calls, method+" "+string(body))
		answer := f.answers[method]
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(answer)
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
// bandedSession as the island draws it, under its strip, its footer reading
// busySession.
func chatFrames(t *testing.T, view string) (*window, *frameRig, *chatBackend) {
	return chatFramesIn(t, view, rigWindow, busySession())
}

// chatFramesIn is chatFrames on a stage of size, its footer reading in.
func chatFramesIn(t testing.TB, view string, size image.Point, in footerInput) (*window, *frameRig, *chatBackend) {
	t.Helper()
	r := newFrameRig(size, nil)
	f, client := newChatBackend(t)
	foot := newFooterRig(t, nil).f
	foot.dock, foot.git = new(dock), new(gitPanel)
	u := &window{ctx: t.Context(), client: client, model: chatModel(view), footer: foot, terms: &terminals{byID: map[string]*termView{}, invalidate: r.invalidate}}
	u.model.invalidate = r.invalidate
	u.model.chat.kinds = []string{"claude"}
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			u.footer.frame(gtx, in)
			u.footer.actGit(gtx)
			sv, _ := u.model.sessionViewOf("s1")
			band := u.sessionBand(gtx, stageView{active: sv, hasActive: true})
			if err := u.bandedSession(gtx, sv, band, th.Background, true); err != nil {
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
	r.settle(t) // the rows come to rest on the composer in a frame or two
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
	r.click(t, r.smallestClickIn(t, boxFoot))
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
	worked := i18n.T("chat.view.toolCalls", i18n.Args{"count": 1})
	r.clickLabel(t, worked)
	// The fold opens over ui.Collapse's 200ms, from the frame that took the
	// click: half way through, its call is already drawn.
	r.now = r.now.Add(100 * time.Millisecond)
	r.frame()
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

// A short conversation rests on the composer and new messages push the old
// ones up, as in a messaging app; a long one shows its end, and keeps it as
// items arrive. (Before, a short conversation started at the top.)
func TestTheChatRestsOnTheComposerAndFollowsItsEnd(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", userTurns(2))
	r.settle(t)
	composer := labelTop(t, r, "Ask claude about this project")
	last, first := labelTop(t, r, "q1"), labelTop(t, r, "q0")
	if last < 0 || composer-last > 150 || first >= last {
		t.Errorf("a short conversation's messages at y=%d and y=%d, the composer at y=%d: want them down on it", first, last, composer)
	}
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "q2", Kind: api.ItemUser, Turn: 3, Text: "q2"}})
	r.settle(t)
	if again, fresh := labelTop(t, r, "q1"), labelTop(t, r, "q2"); again >= last || fresh <= again || composer-fresh > 150 {
		t.Errorf("after a new message q1 at y=%d (was %d) and q2 at y=%d: want the new one at the bottom and the old pushed up", again, last, fresh)
	}
	u.model.chat.history("s1", userTurns(60))
	r.settle(t)
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "late", Kind: api.ItemUser, Turn: 61, Text: "last one"}})
	r.settle(t)
	if labelTop(t, r, "last one") < 0 {
		t.Errorf("the newest message is off screen:\n%s", r.drawn())
	}
}

// A row's top is the list's own position, walked over the heights its rows
// last measured, so a long answer's Markdown can window itself.
func TestRowTopFollowsTheListsPositionAndMeasuredHeights(t *testing.T) {
	v := &chatView{heights: map[string]int{"a": 10, "b": 20, "c": 30}}
	v.list.Position.First, v.list.Position.Offset = 1, 5
	shown := []string{"a", "b", "c"}
	if got := v.rowTop(shown, 1); got != -5 {
		t.Errorf("the first visible row's top = %d, want -5", got)
	}
	if got := v.rowTop(shown, 2); got != 15 {
		t.Errorf("the row under it = %d, want 15", got)
	}
	if got := v.rowTop(shown, 0); got != -15 {
		t.Errorf("the row above the viewport = %d, want -15", got)
	}
}
