package main

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

func TestSessionExitWording(t *testing.T) {
	cases := []struct {
		payload, title, marker string
	}{
		{`{"code":0}`, "Shell 78 ended", "\r\n[process exited: 0]\r\n"},
		{`{"code":3}`, "Shell 78 ended with code 3", "\r\n[process exited: 3]\r\n"},
		{`{}`, "Shell 78 ended", "\r\n[process exited]\r\n"},
		{`null`, "Shell 78 ended", "\r\n[process exited]\r\n"},
	}
	for _, c := range cases {
		e := readSessionExit(backend.Event{Name: api.TerminalExitEvents.Of("s78").Name, Data: []byte(c.payload)})
		if got := exitTitle("Shell 78", e); got != c.title {
			t.Errorf("%s: title %q, want %q", c.payload, got, c.title)
		}
		if got := exitMarker(e); got != c.marker {
			t.Errorf("%s: marker %q, want %q", c.payload, got, c.marker)
		}
	}
}

func TestExitedWithoutTerminalIsNoOp(t *testing.T) {
	ts := &terminals{byID: map[string]*termView{}, invalidate: func() { t.Fatal("invalidated for a session with no terminal") }}
	ts.exited("s1", sessionExit{})
	if _, ok := ts.exitOf("s1"); ok {
		t.Fatal("exitOf reports an exit for a session with no terminal")
	}
}
