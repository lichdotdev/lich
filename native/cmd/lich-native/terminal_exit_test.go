package main

import "testing"

func TestSessionExitWording(t *testing.T) {
	cases := []struct {
		payload, notice, marker string
	}{
		{`{"code":0}`, "Session ended", "\r\n[process exited: 0]\r\n"},
		{`{"code":3}`, "Session ended with code 3", "\r\n[process exited: 3]\r\n"},
		{`{}`, "Session ended", "\r\n[process exited]\r\n"},
		{`null`, "Session ended", "\r\n[process exited]\r\n"},
	}
	for _, c := range cases {
		e := readSessionExit([]byte(c.payload))
		if got := exitNotice(e); got != c.notice {
			t.Errorf("%s: notice %q, want %q", c.payload, got, c.notice)
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
