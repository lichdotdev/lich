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
