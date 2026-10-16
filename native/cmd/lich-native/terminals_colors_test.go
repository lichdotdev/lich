package main

import (
	"context"
	"image/color"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/omartelo/lich/native/gioterm"
)

// A theme that flips the scheme of a terminal whose program asked for mode
// 2031 types the report into that session, instead of hanging the window.
func TestSchemeFlippingThemeReportsToTheSession(t *testing.T) {
	frames := make(chan []byte, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		for {
			_, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			frames <- data
		}
	}))
	t.Cleanup(srv.Close)
	client := clientOf(srv)
	stream, _, err := client.Terminal(t.Context(), func(string, []byte) {})
	if err != nil {
		t.Fatal(err)
	}
	ts := &terminals{client: client, stream: stream, byID: map[string]*termView{}}
	vt, err := gioterm.New(80, 24, 10, 20, func(b []byte) { ts.send("s", b) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vt.Close)
	ts.byID["s"] = &termView{id: "s", vt: vt}
	dark := gioterm.Colors{Foreground: color.NRGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff}, Background: color.NRGBA{R: 0x10, G: 0x10, B: 0x14, A: 0xff}}
	light := gioterm.Colors{Foreground: color.NRGBA{R: 0x20, G: 0x20, B: 0x20, A: 0xff}, Background: color.NRGBA{R: 0xf4, G: 0xf4, B: 0xf6, A: 0xff}}
	ts.setColors(dark)
	vt.Write([]byte("\x1b[?2031h"))

	done := make(chan struct{})
	go func() {
		ts.setColors(light)
		close(done)
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("setColors never returned: the scheme report deadlocked on the terminals lock")
	}
	select {
	case got := <-frames:
		if want := "\x01s\x1b[?997;2n"; string(got) != want {
			t.Fatalf("session got %q, want %q", got, want)
		}
	case <-ctx.Done():
		t.Fatal("the session was never told the scheme changed")
	}
}
