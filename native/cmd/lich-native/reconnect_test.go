package main

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// reconnectRig is a window attached to a backend, with a second backend that
// can stand in as its restarted successor.
type reconnectRig struct {
	u *window
	// closeWindow ends the window's context, as closing the window does.
	closeWindow context.CancelFunc
	old         *gateBackend
	next        *gateBackend
	runtime     string
}

// newReconnectRig wires what main wires for the drop: the model's hook and a
// followed /events, so ending that stream is what starts the reconnect.
func newReconnectRig(t *testing.T) *reconnectRig {
	t.Helper()
	u, old := newGateWindow(t)
	next := newGateBackend(t)
	next.state = []lichclient.Project{{ID: "p2", Name: "restarted", Path: "/restarted"}}
	u.gate = &spawnGate{}
	ctx, closeWindow := context.WithCancel(t.Context())
	t.Cleanup(closeWindow)
	u.ctx = ctx
	u.model.dropped = u.backendDropped

	path := filepath.Join(t.TempDir(), "runtime.json")
	prev := *flagRuntime
	*flagRuntime = path
	t.Cleanup(func() { *flagRuntime = prev })

	events, err := u.client.Events(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	go u.model.follow(t.Context(), events)
	waitFor(t, "the window attached to /events", func() bool { return old.eventsOpened.Load() == 1 })
	return &reconnectRig{u: u, closeWindow: closeWindow, old: old, next: next, runtime: path}
}

// name writes the runtime file as the lich that serves b would.
func (r *reconnectRig) name(t *testing.T, b *gateBackend) {
	t.Helper()
	base, err := url.Parse(b.srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(base.Port())
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(lichclient.Runtime{Port: port, Token: "tok"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.runtime, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (r *reconnectRig) lost() bool { return r.u.model.connectionLost() }

func (r *reconnectRig) projectNames() []string {
	r.u.model.mu.Lock()
	defer r.u.model.mu.Unlock()
	var names []string
	for _, p := range r.u.model.projects {
		names = append(names, p.Name)
	}
	return names
}

func TestDropWhileBackendStillAnswersMarksLostWithoutRetargeting(t *testing.T) {
	r := newReconnectRig(t)
	r.name(t, r.next)
	r.old.dropEventStream()
	waitFor(t, "the window lost", r.lost)

	if r.u.reconnecting.Load() {
		t.Error("a window another one replaced waited for a restart")
	}
	if err := r.u.model.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	r.old.await(t, "store.LoadState")
	if n := r.next.eventsOpened.Load() + r.next.terminalOpened.Load(); n != 0 || len(r.next.calls) != 0 {
		t.Errorf("the successor in the runtime file was reached: %d sockets, %d calls", n, len(r.next.calls))
	}
}

func TestDropWhileBackendRestartsMovesOntoTheSuccessor(t *testing.T) {
	r := newReconnectRig(t)
	r.u.toaster.Show(ui.Toast{ID: appUpdateToastID, Duration: ui.ToastForever})
	r.old.goAway()
	waitFor(t, "the window waiting for the restart", r.u.reconnecting.Load)
	r.name(t, r.next)
	waitFor(t, "the window done reconnecting", func() bool { return !r.u.reconnecting.Load() })

	if r.lost() {
		t.Error("the window gave up on a successor that answered")
	}
	if !r.u.client.Answers(t.Context()) {
		t.Error("the client still points at the backend that left")
	}
	if r.next.eventsOpened.Load() != 1 || r.next.terminalOpened.Load() != 1 {
		t.Errorf("successor sockets: events %d, terminal %d; want one each", r.next.eventsOpened.Load(), r.next.terminalOpened.Load())
	}
	r.u.terms.mu.Lock()
	typing := r.u.terms.stream != nil
	r.u.terms.mu.Unlock()
	if !typing {
		t.Error("the terminals still type into the stream of the backend that left")
	}
	if names := r.projectNames(); len(names) != 1 || names[0] != "restarted" {
		t.Errorf("projects = %v, want the successor's", names)
	}
	if r.u.toaster.Live(appUpdateToastID) {
		t.Error("the update toast of the release that left is still up")
	}
}

func TestDropWithNoSuccessorWithinTheWaitMarksLost(t *testing.T) {
	prev := reconnectWait
	reconnectWait = 3 * reconnectPoll
	t.Cleanup(func() { reconnectWait = prev })

	r := newReconnectRig(t)
	r.name(t, r.old) // a runtime file naming the backend that left answers nobody
	r.old.goAway()
	waitFor(t, "the window lost", r.lost)

	if r.u.reconnecting.Load() {
		t.Error("the window still reports reconnecting after giving up")
	}
	if r.u.client.Answers(t.Context()) {
		t.Error("the client moved onto a backend that does not answer")
	}
}

func TestDropWhenTheSuccessorRefusesASocketMarksLost(t *testing.T) {
	for name, refuse := range map[string]func(*gateBackend){
		"events":   func(b *gateBackend) { b.refuseEvents.Store(true) },
		"terminal": func(b *gateBackend) { b.refuseTerminal.Store(true) },
	} {
		t.Run(name, func(t *testing.T) {
			r := newReconnectRig(t)
			refuse(r.next)
			r.name(t, r.next)
			r.old.goAway()
			waitFor(t, "the window lost", r.lost)
		})
	}
}

func TestDropEndsTheWaitWhenTheWindowCloses(t *testing.T) {
	r := newReconnectRig(t)
	r.old.goAway()
	waitFor(t, "the window waiting for the restart", r.u.reconnecting.Load)

	// The wait is 15s; waitFor gives up well before that.
	r.closeWindow()
	waitFor(t, "the wait to end with the window", func() bool { return !r.u.reconnecting.Load() })
	if r.next.eventsOpened.Load() != 0 {
		t.Error("a window that closed attached to a backend")
	}
}
