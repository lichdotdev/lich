package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// A terminal attached to a running session takes nothing from the stream
// until its replay frame: output before it is already in the tail. The
// program's queries in the tail were answered when the program asked them,
// so reopening the window must not type the answers into the session again,
// while a query after the replay still gets its answer.
func TestAttachedTerminalStartsFromTheReplay(t *testing.T) {
	frames := make(chan []byte, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/rpc/") {
			_ = json.NewEncoder(w).Encode(true)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		for {
			_, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			frames <- data
		}
	}))
	t.Cleanup(srv.Close)
	client := clientOf(srv)
	stream, _, err := client.Terminal(t.Context(), func(string, []byte) {}, func(string, []byte) {})
	if err != nil {
		t.Fatal(err)
	}
	ts := &terminals{client: client, stream: stream, byID: map[string]*termView{}, invalidate: func() {}}
	tv := &termView{id: "s", r: fixedCells{}}
	if err := ts.attach(t.Context(), tv, 80, 24); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tv.vt.Close)

	ts.onOutput("s", []byte("already in the tail\r\n\r\n\r\n"))
	ts.onReplay("s", []byte("$ nvim\r\n\x1b[c\x1b[6n"))
	ts.onOutput("s", []byte("\x1b[6n"))

	select {
	case got := <-frames:
		if want := "\x01s\x1b[2;1R"; string(got) != want {
			t.Fatalf("session got %q, want only the live cursor report %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the query after the replay was never answered")
	}
}
