package main

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

func chatEventOf[T any](t *testing.T, e api.Event[T], payload T) backend.Event {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return backend.Event{Name: e.Name, Data: data}
}

func chatIDs(items []backend.ChatItem) []string {
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID+":"+it.Text)
	}
	return ids
}

func TestChatItemEventReplacesTheItemOfItsIDOrAppends(t *testing.T) {
	var s chatStore
	s.apply(api.ChatItemUpdate{SessionID: "a", Item: backend.ChatItem{ID: "1", Text: "hel"}})
	s.apply(api.ChatItemUpdate{SessionID: "a", Item: backend.ChatItem{ID: "2", Text: "tool"}})
	s.apply(api.ChatItemUpdate{SessionID: "a", Item: backend.ChatItem{ID: "1", Text: "hello"}})
	s.apply(api.ChatItemUpdate{SessionID: "b", Item: backend.ChatItem{ID: "1", Text: "other"}})
	if got := chatIDs(s.of("a")); !reflect.DeepEqual(got, []string{"1:hello", "2:tool"}) {
		t.Errorf("a = %v, want the first item replaced in place", got)
	}
	if got := chatIDs(s.of("b")); !reflect.DeepEqual(got, []string{"1:other"}) {
		t.Errorf("b = %v, want its own list", got)
	}
}

// The history is read with the stream held behind it: what came before is
// older than it, what comes after lands on top.
func TestChatHistoryReplacesWhatCameBeforeAndLaterEventsLandOnTop(t *testing.T) {
	var s chatStore
	s.apply(api.ChatItemUpdate{SessionID: "a", Item: backend.ChatItem{ID: "2", Text: "stale"}})
	s.history("a", []backend.ChatItem{{ID: "1", Text: "ask"}, {ID: "2", Text: "answer"}})
	if got := chatIDs(s.of("a")); !reflect.DeepEqual(got, []string{"1:ask", "2:answer"}) {
		t.Errorf("after history = %v", got)
	}
	s.apply(api.ChatItemUpdate{SessionID: "a", Item: backend.ChatItem{ID: "2", Text: "answer, longer"}})
	s.apply(api.ChatItemUpdate{SessionID: "a", Item: backend.ChatItem{ID: "3", Text: "next"}})
	if got := chatIDs(s.of("a")); !reflect.DeepEqual(got, []string{"1:ask", "2:answer, longer", "3:next"}) {
		t.Errorf("after events = %v", got)
	}
}

func chatModel(views ...string) *model {
	m := newModel(nil, func() {})
	var sessions []backend.Session
	for i, v := range views {
		// Each has a conversation, as a session that has talked does: one that
		// never had any opens on the start screen before its chat is read.
		id := "s" + string(rune('1'+i))
		sessions = append(sessions, backend.Session{ID: id, Kind: "claude", View: v, ProviderSessionID: "conv-" + id})
	}
	m.projects = []backend.Project{{ID: "p", Sessions: sessions}}
	return m
}

func TestChatEventsFeedTheStoreAndTheSessionsView(t *testing.T) {
	m := chatModel(api.ViewTerminal)
	ctx := context.Background()
	m.chat.beginSwitch("s1", api.ViewChat)
	var reached []string
	m.toChat = func(id string) { reached = append(reached, id) }
	m.chatEvent(ctx, chatEventOf(t, api.ChatTurnEvent, api.ChatTurn{SessionID: "s1", Running: true}))
	if !m.chat.isRunning("s1") {
		t.Error("a turn starting did not run")
	}
	m.chatEvent(ctx, chatEventOf(t, api.SessionViewEvent, api.SessionView{ID: "s1", View: api.ViewChat}))
	sv, _ := m.sessionViewOf("s1")
	if sv.View != api.ViewChat {
		t.Errorf("view = %q, want chat", sv.View)
	}
	if m.chat.isSwitching("s1") || m.chat.isRunning("s1") {
		t.Error("the switch landing left it switching, or the old process's turn running")
	}
	if !reflect.DeepEqual(reached, []string{"s1"}) {
		t.Errorf("toChat heard %v", reached)
	}
	m.chatEvent(ctx, chatEventOf(t, api.ChatItemEvent, api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "1", Kind: api.ItemUser, Text: "hi"}}))
	if got := chatIDs(m.chat.of("s1")); !reflect.DeepEqual(got, []string{"1:hi"}) {
		t.Errorf("items = %v", got)
	}
}

// The terminal's resume is readied while the store still says chat, so no
// frame in between starts a terminal without it.
func TestLeavingTheChatReadiesTheTerminalBeforeTheViewFlips(t *testing.T) {
	m := chatModel(api.ViewChat)
	var seen string
	m.toTerminal = func(_ context.Context, id string) {
		sv, _ := m.sessionViewOf(id)
		seen = sv.View
	}
	m.chatEvent(context.Background(), chatEventOf(t, api.SessionViewEvent, api.SessionView{ID: "s1", View: api.ViewTerminal}))
	if seen != api.ViewChat {
		t.Errorf("toTerminal saw view %q, want chat still", seen)
	}
	if sv, _ := m.sessionViewOf("s1"); sv.View != api.ViewTerminal {
		t.Errorf("view = %q, want terminal", sv.View)
	}
}

func TestOnlyOneSwitchAtATime(t *testing.T) {
	var s chatStore
	if !s.beginSwitch("a", api.ViewChat) || s.beginSwitch("a", api.ViewTerminal) {
		t.Fatal("a second switch began over the first")
	}
	if s.switchingTo("a") != api.ViewChat {
		t.Errorf("switching to %q", s.switchingTo("a"))
	}
	s.switched("a")
	if s.isSwitching("a") || !s.beginSwitch("a", api.ViewTerminal) {
		t.Error("the switch did not end")
	}
}

func TestOffersOnlyTheBackendsKindsAndNeverAShell(t *testing.T) {
	s := chatStore{kinds: []string{"claude", "shell"}}
	if !s.offers("claude") || s.offers("codex") || s.offers("shell") {
		t.Errorf("claude %v codex %v shell %v", s.offers("claude"), s.offers("codex"), s.offers("shell"))
	}
}

func TestChatRowsFoldEachTurnsToolsWhereItsFirstWas(t *testing.T) {
	tool := func(id string, turn int, at int64) backend.ChatItem {
		return backend.ChatItem{ID: id, Kind: api.ItemTool, Turn: turn, At: at, Tool: &backend.ChatToolCall{Name: "Read", State: api.ToolDone}}
	}
	items := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: 1000},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: 2000},
		tool("t1", 1, 3000),
		{ID: "a2", Kind: api.ItemAgent, Turn: 1, At: 4000},
		tool("t2", 1, 5000),
		{ID: "a3", Kind: api.ItemAgent, Turn: 1, At: 13000},
		{ID: "u2", Kind: api.ItemUser, Turn: 2, At: 20000},
		tool("t3", 2, 21000),
	}
	rows := chatRows(items, true, time.UnixMilli(50000))
	var got []string
	for _, r := range rows {
		switch r.kind {
		case chatRowItem:
			got = append(got, r.item.ID)
		case chatRowWork:
			got = append(got, "work")
		case chatRowLive:
			got = append(got, "live")
		}
	}
	if want := []string{"u1", "a1", "work", "a2", "a3", "u2", "work", "live"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if w := rows[2]; len(w.tools) != 2 || w.took != 12*time.Second {
		t.Errorf("turn 1: %d tools in %v, want 2 in 12s", len(w.tools), w.took)
	}
	if w := rows[6]; len(w.tools) != 1 || w.took != 30*time.Second {
		t.Errorf("turn 2 in flight: %d tools in %v, want 1 in 30s to now", len(w.tools), w.took)
	}
}

// A waiting permission's box says the turn waits on the person; a live row
// under it would say it again.
func TestChatRowsLiveRowGivesWayToAWaitingPermission(t *testing.T) {
	perm := func(chosen string) backend.ChatItem {
		return backend.ChatItem{ID: "p", Kind: api.ItemPermission, Permission: &backend.ChatPermission{Chosen: chosen}}
	}
	last := func(items []backend.ChatItem, running bool) chatRowKind {
		rows := chatRows(items, running, time.Time{})
		return rows[len(rows)-1].kind
	}
	if last([]backend.ChatItem{perm("")}, true) == chatRowLive {
		t.Error("a live row under a waiting permission")
	}
	if last([]backend.ChatItem{perm("allow")}, true) != chatRowLive {
		t.Error("no live row once the permission was answered")
	}
	if last([]backend.ChatItem{perm("allow")}, false) == chatRowLive {
		t.Error("a live row with no turn running")
	}
}

func TestPermitOptionTakesTheFirstOfTheKinds(t *testing.T) {
	p := &backend.ChatPermission{Options: []api.ChatPermitOption{
		{ID: "always", Kind: api.PermitAllowAlways}, {ID: "once", Kind: api.PermitAllowOnce}, {ID: "no", Kind: api.PermitRejectOnce},
	}}
	if got := permitOption(p, api.PermitAllowOnce, api.PermitAllowAlways); got != "once" {
		t.Errorf("allow = %q, want once before always", got)
	}
	if got := permitOption(&backend.ChatPermission{Options: p.Options[:1]}, api.PermitAllowOnce, api.PermitAllowAlways); got != "always" {
		t.Errorf("allow with no once = %q, want always", got)
	}
	if got := permitOption(p, api.PermitRejectOnce); got != "no" {
		t.Errorf("reject = %q", got)
	}
	if got := permitOption(&backend.ChatPermission{}, api.PermitRejectOnce); got != "" {
		t.Errorf("none = %q", got)
	}
}

func TestChatSwitchState(t *testing.T) {
	agent := func(status string) sessionView {
		return sessionView{Kind: "claude", status: status}
	}
	cases := []struct {
		name               string
		offers             bool
		sv                 sessionView
		running, switching bool
		shown              bool
		block              chatSwitchBlock
	}{
		{"kind not offered", false, agent(""), false, false, false, switchFree},
		{"idle agent", true, agent(statusDone), false, false, true, switchFree},
		{"busy agent", true, agent(statusBusy), false, false, true, switchWorking},
		{"compacting", true, agent(statusCompacting), false, false, true, switchWorking},
		{"waiting between turns", true, agent(statusWaiting), false, false, true, switchFree},
		{"chat turn running", true, agent(""), true, false, true, switchWorking},
		{"switch on its way", true, agent(statusBusy), false, true, true, switchPending},
	}
	for _, c := range cases {
		shown, block := chatSwitchState(c.offers, c.sv, c.running, c.switching)
		if shown != c.shown || block != c.block {
			t.Errorf("%s: shown %v block %v, want %v %v", c.name, shown, block, c.shown, c.block)
		}
	}
}

func TestLeavingAChatResumesItsConversationWithNoPrompt(t *testing.T) {
	cases := []struct {
		name    string
		probe   fakeProbe
		session backend.Session
		want    string
		toast   bool
	}{
		{"conversation still there", fakeProbe{available: true}, restored, "conv-1", false},
		{"resume check error keeps it", fakeProbe{availErr: errProbe}, restored, "conv-1", false},
		{"conversation gone starts fresh", fakeProbe{}, restored, "", true},
		{"no conversation", fakeProbe{available: true}, backend.Session{ID: "s1", Kind: "claude"}, "", false},
	}
	for _, c := range cases {
		var toasts int
		g := &spawnGate{probe: &c.probe, invalidate: func() {}, notify: func(ui.Toast) { toasts++ }}
		g.leaveChat(context.Background(), sessionView{Session: c.session})
		if got := released(t, g); got != c.want {
			t.Errorf("%s: resume = %q, want %q", c.name, got, c.want)
		}
		if (toasts > 0) != c.toast {
			t.Errorf("%s: %d toasts", c.name, toasts)
		}
		if g.prompt() != nil {
			t.Errorf("%s: asked", c.name)
		}
	}
}

func TestAChatEventThroughTheStreamReachesTheStore(t *testing.T) {
	m := chatModel(api.ViewChat)
	events := make(chan backend.Event, 2)
	events <- chatEventOf(t, api.ChatItemEvent, api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "1", Text: "hi"}})
	events <- chatEventOf(t, api.ChatTurnEvent, api.ChatTurn{SessionID: "s1", Running: true})
	close(events)
	m.follow(context.Background(), events)
	if got := chatIDs(m.chat.of("s1")); !reflect.DeepEqual(got, []string{"1:hi"}) || !m.chat.isRunning("s1") {
		t.Errorf("items %v running %v", got, m.chat.isRunning("s1"))
	}
}

// A reload brings back sessions in a chat (a reopen, a project opened): only
// those the store never read are read.
func TestLoadChatsReadsOnlyChatsTheStoreDoesNotHold(t *testing.T) {
	f, client := newChatBackend(t)
	m := chatModel(api.ViewChat, api.ViewTerminal, api.ViewChat)
	m.client = client
	m.chat.history("s1", nil)
	m.loadChats(context.Background())
	if body := f.waitCall(t, api.ChatHistory.Name); !strings.Contains(body, `"id":"s3"`) {
		t.Errorf("read %s, want s3", body)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	// A chat's options and commands are read with its history, since the
	// contract gave a chat both for the window to show.
	if len(f.calls) != 3 || !strings.HasPrefix(f.calls[1], "chat.Options ") || !strings.Contains(f.calls[1], `"id":"s3"`) ||
		!strings.HasPrefix(f.calls[2], "chat.Commands ") || !strings.Contains(f.calls[2], `"id":"s3"`) {
		t.Errorf("calls %v, want s3's history, options and commands alone", f.calls)
	}
}

func TestATerminalStoppedForTheChatRaisesNoExitToast(t *testing.T) {
	for _, chat := range []bool{false, true} {
		u := exitedWindow(t)
		if chat {
			u.model.projects[0].Sessions[0].View = api.ViewChat
		} else {
			u.model.chat.beginSwitch("s1", api.ViewChat)
		}
		u.sessionExited("s1", code(0))
		if u.toaster.Live("exit:s1") {
			t.Errorf("chat %v: an exit toast for a terminal the chat stopped", chat)
		}
	}
}

func TestAPermitSentByKindAloneIsNamedByLich(t *testing.T) {
	for kind, want := range map[string]string{
		api.PermitAllowOnce: i18n.T("chat.permit.allowOnce"), api.PermitAllowAlways: i18n.T("chat.permit.allowAlways"),
		api.PermitRejectOnce: i18n.T("chat.permit.rejectOnce"), "other": "x",
	} {
		if got := permitLabel(api.ChatPermitOption{ID: "x", Kind: kind}); got != want {
			t.Errorf("%s is %q, want %q", kind, got, want)
		}
	}
	if got := permitLabel(api.ChatPermitOption{ID: "x", Label: "Allow once", Kind: api.PermitAllowOnce}); got != "Allow once" {
		t.Errorf("the provider's label lost to ours: %q", got)
	}
}
