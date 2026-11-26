package main

import (
	"context"
	"encoding/json"
	"image"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// pickBackend answers drop.Attach and artifact.Attach with picked, and terminal.Write as the
// backend does for a session with no PTY, recording what was written.
func pickBackend(t *testing.T, picked backend.Attachment, mu *sync.Mutex, writes *[]string) *backend.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var args []json.RawMessage
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &args)
		switch r.URL.Path {
		case "/rpc/drop.Attach", "/rpc/artifact.Attach":
			_ = json.NewEncoder(w).Encode(picked)
		case "/rpc/terminal.Write":
			var opts api.WriteOptions
			_ = json.Unmarshal(args[0], &opts)
			mu.Lock()
			*writes = append(*writes, opts.Data)
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(nil)
		default:
			_ = json.NewEncoder(w).Encode(nil)
		}
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return backend.New(backend.Runtime{Port: port, Token: "tok"})
}

// boxPlus is where the composer's attach "+" stands: the left end of the
// row under the text, inside the box.
var boxPlus = image.Rect(100, 720, 170, 770)

// holdPicks has the footer's "+" hand its pick to the composer only when the
// returned land says, as a person's pick lands long after the click: the
// fake backend answers at once, and a card landing between the two frames
// r.click compares reads as a click that drew the old state.
func holdPicks(t *testing.T, u *window) (land func()) {
	t.Helper()
	picked := make(chan struct{})
	u.footer.attachAt = func(ctx context.Context, id string, a backend.Attachment) error {
		<-picked
		return u.attachAtPrompt(ctx, id, a)
	}
	return func() {
		t.Helper()
		select {
		case picked <- struct{}{}:
		case <-time.After(5 * time.Second):
			t.Fatal("the click asked for no pick")
		}
	}
}

// waitCards runs frames until session s1's composer holds a card for each of
// paths, in order.
func waitCards(t *testing.T, r *frameRig, p *chatPane, paths ...string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !slices.Equal(filePaths(p.attached), paths); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the composer holds %q, want %q", filePaths(p.attached), paths)
		}
		r.now = r.now.Add(frameStep)
		r.frame()
	}
}

// The composer's "+" in a session drawn as a chat puts the picked file in a
// card over the text, which it leaves as it was, and writes nothing to a
// terminal the session has not got.
func TestAttachInAChatAddsACard(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	var mu sync.Mutex
	var writes []string
	c := pickBackend(t, backend.Attachment{Path: "/home/me/repo/notes.md"}, &mu, &writes)
	u.footer.client = c
	land := holdPicks(t, u)
	p := u.chats.pane("s1")
	p.editor.SetText("read ")
	r.settle(t)
	r.click(t, r.smallestClickIn(t, boxPlus))
	land()
	waitCards(t, r, p, "/home/me/repo/notes.md")
	if p.editor.Text() != "read " {
		t.Errorf("the composer reads %q, want the text it had", p.editor.Text())
	}
	mu.Lock()
	defer mu.Unlock()
	if len(writes) > 0 {
		t.Errorf("the attach wrote %q to a terminal", writes)
	}
}

// An artifact sent to a session drawn as a chat lands in its composer as a
// card, as the "+" does, and writes nothing to a terminal the session has not
// got.
func TestAnArtifactSentToAChatLandsInItsComposer(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	var mu sync.Mutex
	var writes []string
	u.client = pickBackend(t, backend.Attachment{Path: "/home/me/repo/shot.png"}, &mu, &writes)
	r.settle(t)
	p := u.chats.pane("s1")
	u.attachArtifact("s1", "s2", "shot.png")
	waitCards(t, r, p, "/home/me/repo/shot.png")
	mu.Lock()
	defer mu.Unlock()
	if len(writes) > 0 {
		t.Errorf("the artifact wrote %q to a terminal", writes)
	}
}

// The "+" or an artifact on a session drawn as its terminal pastes the
// file's path at its prompt, unsent, with the notice under it.
func TestAttachAtATerminalPastesThePath(t *testing.T) {
	u, _, _ := chatFrames(t, api.ViewTerminal)
	var mu sync.Mutex
	var writes []string
	u.terms = &terminals{client: pickBackend(t, backend.Attachment{}, &mu, &writes)}
	if err := u.attachAtPrompt(t.Context(), "s1", backend.Attachment{Path: "/cfg/dropped/b.png", Notice: "copy of b"}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if want := []string{"\x1b[200~/cfg/dropped/b.png\ncopy of b\n\x1b[201~"}; !slices.Equal(writes, want) {
		t.Errorf("wrote %q, want %q", writes, want)
	}
}

// A reference injected from the Code tab, the editor or a pull request's
// files into a session drawn as a chat lands in its composer, and writes
// nothing to a terminal the session has not got.
func TestAReferenceInjectedIntoAChatLandsInItsComposer(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	var mu sync.Mutex
	var writes []string
	u.terms = &terminals{client: pickBackend(t, backend.Attachment{}, &mu, &writes)}
	r.settle(t)
	p := u.chats.pane("s1")
	if !u.injector("s1")("a.go:3-4 ") {
		t.Fatal("the inject was refused")
	}
	want := "a.go:3-4 "
	for deadline := time.Now().Add(2 * time.Second); p.editor.Text() != want; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			mu.Lock()
			t.Fatalf("the composer reads %q, want %q; the terminal was written %q", p.editor.Text(), want, writes)
		}
		r.now = r.now.Add(frameStep)
		r.frame()
	}
	mu.Lock()
	defer mu.Unlock()
	if len(writes) > 0 {
		t.Errorf("the inject wrote %q to a terminal", writes)
	}
}
