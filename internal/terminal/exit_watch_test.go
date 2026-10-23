// Drives a real PTY, so Unix-only like the rest of the service's spawn tests
// (terminal_test.go).
//go:build !windows

package terminal

import (
	"github.com/lichdotdev/lich/api"
	"slices"
	"sync"
	"testing"
)

// A process that ends on its own is reported to the exit watcher once it is
// reaped, which is how the relay learns to drop the state that process left
// (relay.SessionExited). A close is not: it reaches the relay through the
// close watcher instead.
func TestAReapedSessionIsReportedToTheExitWatcher(t *testing.T) {
	hub, _ := newProbeHub(t)
	svc := New(stubBins{bin: exitBin(t, 0)}, nil, hub)
	var mu sync.Mutex
	var exited []string
	svc.SetSessionExited(func(id string) {
		mu.Lock()
		defer mu.Unlock()
		exited = append(exited, id)
	})
	t.Cleanup(func() { _, _ = svc.Close(t.Context(), api.SessionIDOptions{ID: "s1"}) })

	if _, err := svc.Start(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: t.TempDir(), Kind: "claude", Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return slices.Contains(exited, "s1")
	}, "the exit watcher to hear s1")

	svc2 := New(stubBins{bin: echoBin(t)}, nil, hub)
	closed := false
	svc2.SetSessionExited(func(string) { closed = true })
	if _, err := svc2.Start(t.Context(), api.StartOptions{ID: "s2", ProjectID: "p1", Cwd: t.TempDir(), Kind: "claude", Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := svc2.Close(t.Context(), api.SessionIDOptions{ID: "s2"}); err != nil {
		t.Fatalf("close: %v", err)
	}
	if closed {
		t.Error("a closed session was reported to the exit watcher")
	}
}
