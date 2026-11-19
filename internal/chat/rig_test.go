// The suite runs the fake provider through the test binary and isolates
// lich's store and every provider's home by $HOME and $XDG_CONFIG_HOME, which
// Windows does not read; the conversation and diff tests run everywhere.
//go:build !windows

package chat

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/store"
	"github.com/lichdotdev/lich/internal/terminal"
)

func TestMain(m *testing.M) {
	if os.Getenv(fakeEnv) != "" {
		runFake(os.Args[1:])
		os.Exit(0)
	}
	// A lich session exports the port its own lich listens on; the terminal
	// service under test binds a port of its own.
	_ = os.Unsetenv("LICH_LISTEN_PORT")
	os.Exit(m.Run())
}

// rig is a chat service over a real store and terminal service, whose
// provider is the fake (fake_test.go).
type rig struct {
	t    *testing.T
	svc  *Service
	term *terminal.Service
	db   *store.Service
	home string
	log  string

	mu     sync.Mutex
	events []event
}

type event struct {
	name string
	data any
}

func newRig(t *testing.T, kind string) *rig {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, "claude"))
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(home, "omp"))
	db, err := store.New()
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	key := "provider." + kind + ".bin"
	if kind == providers.Claude {
		key = "claude.bin"
	}
	ctx := t.Context()
	_, _ = db.SetSetting(ctx, api.SetSettingOptions{Key: key, Value: exe})
	project := filepath.Join(home, "proj")
	_ = os.MkdirAll(project, 0o755)
	_, _ = db.AddProject(ctx, api.AddProjectOptions{ID: "p1", Name: "proj", Path: project})
	_, _ = db.AddSession(ctx, api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "S", Kind: kind, NextSeq: 2})

	r := &rig{t: t, db: db, home: home, log: filepath.Join(home, "fake.log")}
	hub := events.New()
	hub.Observe(func(name string, data any) {
		r.mu.Lock()
		r.events = append(r.events, event{name, data})
		r.mu.Unlock()
	})
	env := []string{fakeEnv + "=1", fakeLogEnv + "=" + r.log, "PATH=" + os.Getenv("PATH"), "HOME=" + home}
	if v := os.Getenv("LICH_CHAT_FAKE_NOLOAD"); v != "" {
		env = append(env, "LICH_CHAT_FAKE_NOLOAD="+v)
	}
	r.term = terminal.New(db, env, hub)
	r.svc = New(r.term, db, hub, func() prompt.Lang { return prompt.English })
	t.Cleanup(func() {
		r.svc.Shutdown()
		_, _ = r.term.Close(t.Context(), api.SessionIDOptions{ID: "s1"})
		_ = r.term.CloseTransport()
	})
	return r
}

// eventually waits for cond, failing the test after a few seconds.
func (r *rig) eventually(what string, cond func() bool) {
	r.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			r.t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// turnEnds is the stop reason of the next turn to end after n turn ends.
func (r *rig) turnEnd(n int) api.ChatTurn {
	r.t.Helper()
	var got api.ChatTurn
	r.eventually("turn end", func() bool {
		seen := 0
		for _, e := range r.snapshot() {
			if turn, ok := e.data.(api.ChatTurn); ok && e.name == api.ChatTurnEvent.Name && !turn.Running {
				if seen == n {
					got = turn
					return true
				}
				seen++
			}
		}
		return false
	})
	return got
}

func (r *rig) snapshot() []event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]event(nil), r.events...)
}

// itemSends counts the ChatItemEvents of item id.
func (r *rig) itemSends(id string) int {
	n := 0
	for _, e := range r.snapshot() {
		if u, ok := e.data.(api.ChatItemUpdate); ok && u.Item.ID == id {
			n++
		}
	}
	return n
}

// sent reports whether the window was sent an item of kind holding text.
func (r *rig) sent(kind, text string) bool {
	for _, e := range r.snapshot() {
		if u, ok := e.data.(api.ChatItemUpdate); ok && u.Item.Kind == kind && strings.Contains(u.Item.Text, text) {
			return true
		}
	}
	return false
}

// history is the chat so far.
func (r *rig) history() []api.ChatItem {
	r.t.Helper()
	items, err := r.svc.History(r.t.Context(), api.SessionIDOptions{ID: "s1"})
	if err != nil {
		r.t.Fatalf("History: %v", err)
	}
	return items
}

// item is the first item of kind whose text contains text.
func (r *rig) item(kind, text string) (api.ChatItem, bool) {
	for _, it := range r.history() {
		if it.Kind == kind && strings.Contains(it.Text, text) {
			return it, true
		}
	}
	return api.ChatItem{}, false
}

func (r *rig) mustItem(kind, text string) api.ChatItem {
	r.t.Helper()
	var got api.ChatItem
	r.eventually(kind+" item "+text, func() bool {
		var ok bool
		got, ok = r.item(kind, text)
		return ok
	})
	return got
}

func (r *rig) prompt(text string) {
	r.t.Helper()
	if _, err := r.svc.Prompt(r.t.Context(), api.ChatPromptOptions{ID: "s1", Text: text}); err != nil {
		r.t.Fatalf("Prompt(%q): %v", text, err)
	}
}

func (r *rig) setView(view string) error {
	_, err := r.svc.SetView(r.t.Context(), api.ChatViewOptions{ID: "s1", View: view})
	return err
}

// fakeLog is every line the fake read, and the argv of each spawn.
func (r *rig) fakeLog() string {
	data, _ := os.ReadFile(r.log)
	return string(data)
}

// writeClaudeTranscript files conversation id as Claude Code would.
func (r *rig) writeClaudeTranscript(id string, lines ...string) {
	r.t.Helper()
	dir := filepath.Join(r.home, "claude", "projects", "-proj")
	_ = os.MkdirAll(dir, 0o755)
	if err := os.WriteFile(filepath.Join(dir, id+".jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		r.t.Fatal(err)
	}
}

// writeOMPSession files conversation id as oh-my-pi would.
func (r *rig) writeOMPSession(id string) {
	r.t.Helper()
	dir := filepath.Join(r.home, "omp", "sessions", "--proj--")
	_ = os.MkdirAll(dir, 0o755)
	if err := os.WriteFile(filepath.Join(dir, "1_"+id+".jsonl"), []byte("{}\n"), 0o600); err != nil {
		r.t.Fatal(err)
	}
}
