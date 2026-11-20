package main

import (
	"encoding/json"
	"image"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// The composer's "+" in a session drawn as a chat types the picked file's
// path at the composer's caret, as a drop does, and writes nothing to a
// terminal the session has not got.
func TestAttachInAChatTypesThePathInTheComposer(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	var mu sync.Mutex
	var writes []string
	c := pickBackend(t, backend.Attachment{Path: "/home/me/repo/notes.md"}, &mu, &writes)
	u.footer.client = c
	u.footer.writePaste = u.writeAtPrompt
	p := u.chats.pane("s1")
	p.editor.SetText("read ")
	p.editor.SetCaret(p.editor.Len(), p.editor.Len())
	r.settle(t)
	r.click(t, r.smallestClickIn(t, boxPlus))
	want := "read /home/me/repo/notes.md "
	for deadline := time.Now().Add(5 * time.Second); p.editor.Text() != want; time.Sleep(time.Millisecond) {
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
		t.Errorf("the attach wrote %q to a terminal", writes)
	}
}

// An artifact sent to a session drawn as a chat lands in its composer as the
// "+" does, and writes nothing to a terminal the session has not got.
func TestAnArtifactSentToAChatLandsInItsComposer(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	var mu sync.Mutex
	var writes []string
	u.client = pickBackend(t, backend.Attachment{Path: "/home/me/repo/shot.png"}, &mu, &writes)
	r.settle(t)
	p := u.chats.pane("s1")
	u.attachArtifact("s1", "s2", "shot.png")
	want := "/home/me/repo/shot.png "
	for deadline := time.Now().Add(5 * time.Second); p.editor.Text() != want; time.Sleep(time.Millisecond) {
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
		t.Errorf("the artifact wrote %q to a terminal", writes)
	}
}
