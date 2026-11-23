package spawn

import (
	"context"
	"testing"
)

// fakeChat stands in for the chat service: drawn says which sessions the
// store drew as a chat, opened records each one started.
type fakeChat struct {
	drawn  bool
	opened []string
}

func (f *fakeChat) Drawn(string) bool { return f.drawn }

func (f *fakeChat) Open(_ context.Context, id string) error {
	f.opened = append(f.opened, id)
	return nil
}

// A session born drawn as a chat starts its chat and no terminal; one born on
// its terminal starts the terminal.
func TestOpenStartsTheViewTheSessionWasBornIn(t *testing.T) {
	for _, drawn := range []bool{true, false} {
		svc, _, _, term, _ := newService(t)
		chat := &fakeChat{drawn: drawn}
		svc.SetChat(chat)
		opened, err := svc.Open(t.Context(), OpenOptions{From: "s1"})
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if drawn && (len(chat.opened) != 1 || chat.opened[0] != opened.ID || len(term.spawns) != 0) {
			t.Errorf("chat: opened %v, terminals %v; want the chat alone", chat.opened, term.spawns)
		}
		if !drawn && (len(chat.opened) != 0 || len(term.spawns) != 1) {
			t.Errorf("terminal: opened %v, terminals %v; want the terminal alone", chat.opened, term.spawns)
		}
	}
}

// blockingChat is a chat whose handshake does not finish until released.
type blockingChat struct {
	release chan struct{}
	started chan string
}

func (b *blockingChat) Drawn(string) bool { return true }

func (b *blockingChat) Open(_ context.Context, id string) error {
	<-b.release
	b.started <- id
	return nil
}

// OpenSubagentNow answers once the session exists, with its chat still
// starting behind it, which then starts.
func TestOpenSubagentNowAnswersBeforeTheAgentIsUp(t *testing.T) {
	svc, _, _, _, _ := newService(t)
	chat := &blockingChat{release: make(chan struct{}), started: make(chan string, 1)}
	svc.SetChat(chat)
	opened, err := svc.OpenSubagentNow(t.Context(), OpenSubagentOptions{From: "s1"})
	if err != nil || opened.ID == "" {
		t.Fatalf("OpenSubagentNow = %+v, %v", opened, err)
	}
	close(chat.release)
	if got := <-chat.started; got != opened.ID {
		t.Errorf("started %q, want the session opened %q", got, opened.ID)
	}
}
