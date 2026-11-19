//go:build !windows

package chat

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/terminal"
)

// TestOpenCodeQuestionThroughItsServer: an opencode chat runs its server on a
// port and password of lich's with the question tool on, and a question tool
// call becomes a question item answered through that server.
func TestOpenCodeQuestionThroughItsServer(t *testing.T) {
	r := newRig(t, providers.OpenCode)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	if !strings.Contains(r.fakeLog(), "argv acp --port ") {
		t.Errorf("fake log = %q, want acp on a port of lich's", r.fakeLog())
	}
	r.prompt("octool")
	q := r.waitingQuestion()
	asks := q.Question.Questions
	if q.ID != "opencode-que_1" || len(asks) != 2 || asks[0].Header != "Fruit" || asks[0].Options[1].Description != "soft" || !asks[1].Multi {
		t.Fatalf("question = %+v", q)
	}
	answers := []api.ChatAskAnswer{{Picked: []string{"Pear"}}, {Picked: []string{"Red"}, Other: "teal"}}
	if err := r.reply(q.ID, api.ChatQReplied{Answers: answers}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	r.turnEnd(0)
	r.mustItem(api.ItemAgent, `octool:reply{"answers":[["Pear"],["Red","teal"]]}`)

	r.prompt("octool")
	q = r.waitingQuestion()
	if err := r.reply(q.ID, api.ChatQReplied{Declined: true}); err != nil {
		t.Fatalf("Reply(declined): %v", err)
	}
	r.turnEnd(1)
	r.mustItem(api.ItemAgent, "octool:reject")
}

// TestOpenCodeQuestionNotOnTheServer: a question tool call with no request
// behind it, or a server that refuses lich, is said in a notice instead of
// waiting on nothing.
func TestOpenCodeQuestionNotOnTheServer(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
		want   string
	}{
		"no request": {http.StatusOK, "[]", "no pending question for tool call call_x"},
		"refused":    {http.StatusUnauthorized, "", "with 401"},
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			c := &chatSession{cwd: "/p", conv: newConversation(func(api.ChatItem) {})}
			d := &acpDriver{c: c, server: &terminal.OpenCodeServer{Port: srv.Listener.Addr().(*net.TCPAddr).Port}, asked: map[string]bool{}}
			d.serveOpenCodeQuestion("call_x")
			items := c.conv.snapshot()
			if len(items) != 1 || items[0].Kind != api.ItemNotice || !strings.Contains(items[0].Text, tc.want) {
				t.Errorf("items = %+v, want a notice with %q", items, tc.want)
			}
		})
	}
}
