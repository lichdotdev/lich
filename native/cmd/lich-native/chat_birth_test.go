package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// birthBackend is the backend a window adding a session talks to: its
// projects as they stand, and a chat that answers with no history and the
// options it opened on.
func birthBackend(t *testing.T, projects []backend.Project) *backend.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		var reply any
		switch strings.TrimPrefix(r.URL.Path, "/rpc/") {
		case api.StoreLoadState.Name:
			reply = projects
		case api.ChatOptions.Name:
			reply = chatOptions()
		}
		_ = json.NewEncoder(w).Encode(reply)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return backend.New(backend.Runtime{Port: port, Token: "tok"})
}

// A session the window adds, born as a chat, has its options before anyone
// writes to it: the composer draws the model, effort and mode chips with no
// message sent.
func TestASessionBornAsAChatShowsItsOptionsBeforeTheFirstMessage(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.launched = true
	c := birthBackend(t, u.model.projects)
	u.client, u.model.client = c, c
	if err := u.loadSessions("s1"); err != nil {
		t.Fatalf("loadSessions: %v", err)
	}
	r.settle(t)
	if _, ok := u.model.chat.optionOf("s1", api.OptionModel); !ok {
		t.Fatal("no options in the store for a session born as a chat")
	}
	if !r.shows("Haiku 5.5 · Medium") {
		t.Errorf("the composer draws no model chip before the first message:\n%s", r.drawn())
	}
}
