// These cover the read wired into the Service, which needs the suite's Store
// stub, and that stub lives in terminal_test.go, which is Unix-only for its
// real PTY spawns. Nothing here spawns anything; the tag is the stub's, not
// this file's. The read's own logic is in title_filed_test.go and runs on every
// OS.
//go:build !windows

package terminal

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/providers"
)

// titleStore records what reached the guarded write, which is the last thing
// lich does with a title before the card redraws.
type titleStore struct {
	stubBins
	titles chan string
}

func (s *titleStore) SetSessionTitle(_, title string) (bool, error) {
	s.titles <- title
	return true, nil
}

// plantKiroSession writes one conversation's metadata where kiroSessionPath
// looks for it, under a throwaway home.
func plantKiroSession(t *testing.T, providerSessionID, body string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(providers.KiroHomeVar, "")
	dir := filepath.Join(home, ".kiro", "sessions", "cli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, providerSessionID+".json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestKiroTitlePublishedOnStop is the whole feature: Kiro reports no title, so
// the turn's end is where lich reads the one Kiro filed itself and sends it
// down the same path a reported title takes.
func TestKiroTitlePublishedOnStop(t *testing.T) {
	const id = "kiro-uuid"
	plantKiroSession(t, id, `{"session_id":"`+id+`","title":"Port the PTY seam to Windows"}`)
	store := &titleStore{stubBins: stubBins{providerSession: id}, titles: make(chan string, 1)}
	svc := New(store, nil, events.New())
	svc.spawns.Store("s1", spawn{kind: providers.Kiro})

	svc.onHookState(hookRequest{SessionID: "s1", State: statusDone})

	select {
	case got := <-store.titles:
		if got != "Port the PTY seam to Windows" {
			t.Errorf("title = %q, want %q", got, "Port the PTY seam to Windows")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the turn ended and no title reached the store")
	}
}

// A tool call is not the turn's end. Kiro reports `busy` once per tool, and
// reading the metadata on each of them would buy a name lich already has at
// the cost of a file read per tool call.
func TestKiroTitleNotReadMidTurn(t *testing.T) {
	const id = "kiro-uuid"
	plantKiroSession(t, id, `{"session_id":"`+id+`","title":"Port the PTY seam to Windows"}`)
	store := &titleStore{stubBins: stubBins{providerSession: id}, titles: make(chan string, 1)}
	svc := New(store, nil, events.New())
	svc.spawns.Store("s1", spawn{kind: providers.Kiro})

	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy})

	select {
	case got := <-store.titles:
		t.Errorf("a mid-turn report titled the card %q", got)
	case <-time.After(200 * time.Millisecond):
	}
}

// The resolution every other session must miss on: the file is Kiro's, and a
// session with no conversation linked to it yet has no file to name.
func TestKiroTitleResolution(t *testing.T) {
	const id = "kiro-uuid"
	plantKiroSession(t, id, `{"session_id":"`+id+`","title":"Port the PTY seam to Windows"}`)
	tests := []struct {
		name            string
		kind            string
		providerSession string
		want            string
	}{
		{"a Kiro session naming its conversation", providers.Kiro, id, "Port the PTY seam to Windows"},
		{"another provider's session", providers.Claude, id, ""},
		{"a shell", KindShell, id, ""},
		{"before session-start linked a conversation", providers.Kiro, "", ""},
		{"a conversation whose file was pruned", providers.Kiro, "gone-uuid", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := New(stubBins{providerSession: tt.providerSession}, nil, events.New())
			svc.spawns.Store("s1", spawn{kind: tt.kind})
			got, ok := svc.filedTitle("s1")
			if ok != (tt.want != "") {
				t.Fatalf("filedTitle = %q, %v, want ok=%v", got, ok, tt.want != "")
			}
			if got != tt.want {
				t.Errorf("filedTitle = %q, want %q", got, tt.want)
			}
		})
	}
}

// plantCursorChat writes one chat's store.db, and its meta.json when meta is not
// empty, where cursorChatStore looks for them, and returns the checkout the
// chat ran in.
func plantCursorChat(t *testing.T, providerSessionID, meta string) string {
	t.Helper()
	base, cwd := t.TempDir(), t.TempDir()
	t.Setenv("CURSOR_CONFIG_DIR", base)
	chat := filepath.Join(base, "chats", cursorWorkspaceKey(cwd), providerSessionID)
	if err := os.MkdirAll(chat, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chat, "store.db"), []byte("SQLite"), 0o600); err != nil {
		t.Fatal(err)
	}
	if meta == "" {
		return cwd
	}
	if err := os.WriteFile(filepath.Join(chat, "meta.json"), []byte(meta), 0o600); err != nil {
		t.Fatal(err)
	}
	return cwd
}

// TestCursorTitlePublishedOnStop: Cursor CLI's hooks carry no title, so the
// turn's end is where lich reads the one the TUI filed beside the chat.
func TestCursorTitlePublishedOnStop(t *testing.T) {
	const id = "bec68d79-a208-4e40-8d2f-f8f8964da216"
	cwd := plantCursorChat(t, id, `{"hasConversation":true,"title":"Pong Game"}`)
	store := &titleStore{stubBins: stubBins{providerSession: id}, titles: make(chan string, 1)}
	svc := New(store, nil, events.New())
	svc.spawns.Store("s1", spawn{kind: providers.Cursor, cwd: cwd})

	// Every Cursor turn opens with a `busy`, and a `done` with none before it is
	// dropped as a repeated ending (repeatedEnding).
	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy})
	svc.onHookState(hookRequest{SessionID: "s1", State: statusDone})

	select {
	case got := <-store.titles:
		if got != "Pong Game" {
			t.Errorf("title = %q, want %q", got, "Pong Game")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the turn ended and no title reached the store")
	}
}

// A chat Cursor has not titled yet, and one in another checkout, are both
// silence: the card keeps the name lich gave it.
func TestCursorTitleResolution(t *testing.T) {
	const id = "bec68d79-a208-4e40-8d2f-f8f8964da216"
	titled := plantCursorChat(t, id, `{"title":"Pong Game"}`)
	tests := []struct {
		name string
		cwd  string
		want string
	}{
		{"the checkout the chat ran in", titled, "Pong Game"},
		{"another checkout", t.TempDir(), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := New(stubBins{providerSession: id}, nil, events.New())
			svc.spawns.Store("s1", spawn{kind: providers.Cursor, cwd: tt.cwd})
			got, ok := svc.filedTitle("s1")
			if ok != (tt.want != "") || got != tt.want {
				t.Errorf("filedTitle = %q, %v, want %q", got, ok, tt.want)
			}
		})
	}
	untitled := plantCursorChat(t, id, `{"hasConversation":true}`)
	svc := New(stubBins{providerSession: id}, nil, events.New())
	svc.spawns.Store("s1", spawn{kind: providers.Cursor, cwd: untitled})
	if got, ok := svc.filedTitle("s1"); ok {
		t.Errorf("an untitled chat titled the card %q", got)
	}
}
