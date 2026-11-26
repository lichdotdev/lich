package spawn

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/relay"
)

// fakeChat stands in for the chat service: drawn says which sessions the
// store drew as a chat, but those terminal names, and opened records each one
// started. calls is what control asked of it, answered with receipt, options
// and err; deadline is the bound a prompt's wait was given.
type fakeChat struct {
	drawn    bool
	terminal map[string]bool
	opened   []string

	calls    []string
	receipt  relay.PromptReceipt
	options  []api.ChatOption
	err      error
	deadline time.Time
	// compact is the compaction command the chat lists, "" for none.
	compact string
}

func (f *fakeChat) Drawn(id string) bool { return f.drawn && !f.terminal[id] }

func (f *fakeChat) Open(_ context.Context, id string) error {
	f.opened = append(f.opened, id)
	return nil
}

func (f *fakeChat) Take(id, text string) (relay.HandedPrompt, error) {
	f.calls = append(f.calls, "take "+id+" "+text)
	if f.err != nil {
		return relay.HandedPrompt{}, f.err
	}
	return relay.HandedPrompt{Await: func(ctx context.Context) relay.PromptReceipt {
		f.deadline, _ = ctx.Deadline()
		return f.receipt
	}}, nil
}

func (f *fakeChat) Cancel(_ context.Context, opts api.SessionIDOptions) (api.None, error) {
	f.calls = append(f.calls, "cancel "+opts.ID)
	return api.None{}, f.err
}

func (f *fakeChat) Options(context.Context, api.SessionIDOptions) ([]api.ChatOption, error) {
	return f.options, nil
}

func (f *fakeChat) SetOption(_ context.Context, opts api.ChatSetOptionOptions) (api.None, error) {
	f.calls = append(f.calls, "set "+opts.ID+" "+strings.Join([]string{opts.OptionID, opts.Value}, "="))
	return api.None{}, f.err
}

func (f *fakeChat) CompactCommand(context.Context, api.SessionIDOptions) (string, error) {
	if f.compact == "" {
		return "", errors.New("its agent lists no command that compacts its conversation")
	}
	return f.compact, nil
}

func (f *fakeChat) New(_ context.Context, opts api.SessionIDOptions) (api.None, error) {
	f.calls = append(f.calls, "new "+opts.ID)
	return api.None{}, f.err
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
	fakeChat
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
