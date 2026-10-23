package main

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// closeWindow returns once the window that read window-close is gone, not as
// soon as the event is sent: a Windows update replaces lich-native.exe right
// after lich exits.
func TestCloseWindowWaitsForTheWindowToLeave(t *testing.T) {
	hub := events.New()
	server := httptest.NewServer(hub)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), windowLeaveWait)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	for !hub.Attached() {
		if ctx.Err() != nil {
			t.Fatal("window never attached")
		}
		time.Sleep(5 * time.Millisecond)
	}
	const leaveDelay = 100 * time.Millisecond
	read := make(chan string, 1)
	go func() {
		_, payload, err := conn.Read(ctx)
		if err != nil {
			read <- err.Error()
			return
		}
		read <- string(payload)
		time.Sleep(leaveDelay)
		_ = conn.CloseNow()
	}()

	start := time.Now()
	closeWindow(hub)()
	waited := time.Since(start)

	if got := <-read; !strings.Contains(got, api.WindowCloseEvent.Name) {
		t.Fatalf("window read %q, want the %s event", got, api.WindowCloseEvent.Name)
	}
	if waited < leaveDelay || waited >= windowLeaveWait {
		t.Fatalf("closeWindow returned after %v, want once the window left (%v)", waited, leaveDelay)
	}
}

func TestCloseWindowWithoutAWindowReturnsAtOnce(t *testing.T) {
	start := time.Now()
	closeWindow(events.New())()
	if waited := time.Since(start); waited >= windowLeaveWait {
		t.Fatalf("closeWindow with no window waited %v", waited)
	}
}
