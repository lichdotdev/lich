package main

import (
	"strings"
	"testing"
	"time"
)

func TestWindowKindOfOpensTheWebWindowUnlessNativeWasChosen(t *testing.T) {
	for stored, want := range map[string]string{"": "web", "web": "web", "native": "native", "electron": "web"} {
		if got := windowKindOf(stored); got != want {
			t.Errorf("windowKindOf(%q) = %q, want %q", stored, got, want)
		}
	}
}

func switchDialog(u *window) *dialogDriver {
	u.settings.switcher.open = true
	return &dialogDriver{now: time.Unix(0, 0), draw: func(gtx C) { u.settings.layoutWindowSwitch(gtx, u.theme) }}
}

func TestSwitchNowOpensTheWebWindow(t *testing.T) {
	u, b := newGateWindow(t)
	d := switchDialog(u)
	d.open()
	d.press(t, &u.settings.switcher.now)
	if args := b.await(t, "system.OpenWindow"); args != `[{"kind":"web"}]` {
		t.Errorf("opened %s, want the web window", args)
	}
	if u.settings.switcher.open {
		t.Error("Switch now left the dialog up")
	}
}

func TestSwitchLaterOnlyClosesTheDialog(t *testing.T) {
	u, b := newGateWindow(t)
	d := switchDialog(u)
	d.open()
	d.press(t, &u.settings.switcher.later)
	if u.settings.switcher.open {
		t.Error("Later left the dialog up")
	}
	select {
	case call := <-b.calls:
		if strings.HasPrefix(call, "system.OpenWindow") {
			t.Errorf("Later opened a window: %s", call)
		}
	default:
	}
}
