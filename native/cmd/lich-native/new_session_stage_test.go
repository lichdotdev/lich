package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/text"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// sessionsBackend is a store that keeps the sessions added to it, as the
// real one does, so a reload after AddSessionInFolder lists the new one.
type sessionsBackend struct {
	mu    sync.Mutex
	state []backend.Project
}

func (b *sessionsBackend) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	method := strings.TrimPrefix(r.URL.Path, "/rpc/")
	b.mu.Lock()
	defer b.mu.Unlock()
	switch method {
	case "store.AddSessionInFolder":
		var args []struct {
			SessionID string `json:"sessionId"`
			Label     string `json:"label"`
			Kind      string `json:"kind"`
			Folder    string `json:"folder"`
		}
		_ = json.Unmarshal(body, &args)
		if len(args) == 1 {
			p := &b.state[0]
			p.Sessions = append(p.Sessions, backend.Session{ID: args[0].SessionID, Label: args[0].Label, Kind: args[0].Kind, Folder: args[0].Folder})
			p.NextSeq++
		}
	case "store.SetActiveSession":
		var args []struct {
			SessionID string `json:"sessionId"`
		}
		_ = json.Unmarshal(body, &args)
		if len(args) == 1 {
			b.state[0].ActiveSessionID = args[0].SessionID
		}
	case "store.LoadState":
		_ = json.NewEncoder(w).Encode(b.state)
		return
	}
	_, _ = w.Write([]byte("null"))
}

// newSessionsWindow is a window on b with one project, p, whose session a1
// is on the stage.
func newSessionsWindow(t *testing.T) (*window, *sessionsBackend) {
	t.Helper()
	b := &sessionsBackend{state: []backend.Project{{ID: "p", Name: "repo", Path: "/repo", ActiveSessionID: "a1", NextSeq: 1,
		Sessions: []backend.Session{{ID: "a1", Label: "Claude 1", Kind: "claude"}}}}}
	srv := httptest.NewServer(http.HandlerFunc(b.serve))
	t.Cleanup(srv.Close)
	client := clientOf(srv)
	u := &window{ctx: t.Context(), client: client, win: new(app.Window),
		theme: &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Canvas}}
	u.model = newModel(client, func() {})
	u.model.launched, u.model.projects = true, append([]backend.Project(nil), b.state...)
	u.model.setActiveProject("p")
	u.model.setActive(u.ctx, "a1")
	u.terms = &terminals{client: client, byID: map[string]*termView{}}
	u.settings = newSettingsScreen(u.ctx, client, &u.root, func() {})
	u.pulls = new(pullsScreen)
	u.extension = new(extensionScreen)
	return u, b
}

// lastAdded is the id of the session b took last.
func (b *sessionsBackend) lastAdded() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.state[0].Sessions
	return s[len(s)-1].ID
}

// A session just opened takes the stage, and keeps it through the reload
// the backend's session-opened event brings.
func TestANewSessionTakesTheStage(t *testing.T) {
	u, b := newSessionsWindow(t)
	if !u.newSession(launchRequest{kind: "claude"}, "") {
		t.Fatal("the session was not added")
	}
	added := b.lastAdded()
	if sv, ok := u.model.activeSession(); !ok || sv.ID != added {
		t.Fatalf("the stage shows %q (%v), want the new session %q", sv.ID, ok, added)
	}
	if err := u.model.reload(u.ctx); err != nil {
		t.Fatal(err)
	}
	if sv, ok := u.model.activeSession(); !ok || sv.ID != added {
		t.Fatalf("after the session-opened reload the stage shows %q, want %q", sv.ID, added)
	}
}

// A session opened while Pulls or Settings covers the stage takes them away
// on the window's next frame, as a click on a session does.
func TestANewSessionUncoversTheStage(t *testing.T) {
	u, _ := newSessionsWindow(t)
	u.pulls.open, u.settings.open = true, true
	if !u.newSession(launchRequest{kind: "claude"}, "") {
		t.Fatal("the session was not added")
	}
	if !u.model.takeOpened() {
		t.Fatal("opening the session asked the window for nothing")
	}
	if u.model.takeOpened() {
		t.Fatal("the opening was reported twice")
	}
	u.uncoverStage()
	if u.pulls.open || u.settings.open {
		t.Fatalf("Pulls open %v, Settings open %v after the session opened, want both gone", u.pulls.open, u.settings.open)
	}
}
