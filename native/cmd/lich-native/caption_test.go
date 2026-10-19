package main

import (
	"slices"
	"testing"

	"gioui.org/app"
)

func TestParseButtonLayout(t *testing.T) {
	cases := []struct {
		name        string
		layout      string
		left, right []captionButton
	}{
		{"stock GNOME keeps only close", "appmenu:close", nil, []captionButton{captionClose}},
		{"Pop!_OS", "appmenu:minimize,maximize,close", nil, []captionButton{captionMinimize, captionMaximize, captionClose}},
		{"macOS-like left", "close,minimize,maximize:", []captionButton{captionClose, captionMinimize, captionMaximize}, nil},
		{"spacers and icons dropped", "icon, close :spacer,minimize", []captionButton{captionClose}, []captionButton{captionMinimize}},
		{"no colon is all left", "close", []captionButton{captionClose}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			left, right := parseButtonLayout(c.layout)
			if !slices.Equal(left, c.left) || !slices.Equal(right, c.right) {
				t.Errorf("parseButtonLayout(%q) = %v, %v; want %v, %v", c.layout, left, right, c.left, c.right)
			}
		})
	}
}

func TestWindowChromeDrawn(t *testing.T) {
	cases := []struct {
		name string
		cfg  app.Config
		want bool
	}{
		{"lich draws the caption", app.Config{Mode: app.Windowed}, true},
		{"maximized keeps it", app.Config{Mode: app.Maximized}, true},
		{"a tiling window manager decorates", app.Config{Mode: app.Windowed, Decorated: true}, false},
		{"fullscreen has none", app.Config{Mode: app.Fullscreen}, false},
	}
	for _, c := range cases {
		if got := chromeOf(c.cfg).drawn(); got != c.want {
			t.Errorf("%s: drawn() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCaptionIconRestoresWhileMaximized(t *testing.T) {
	maximized := windowChrome{maximized: true}
	if captionIcon(captionStyleWindows, captionMaximize, maximized) != icRestore {
		t.Error("Windows: a maximized window's Maximize is not Restore")
	}
	if captionIcon(captionStyleKDE, captionMaximize, maximized) != icKDERestore {
		t.Error("KDE: a maximized window's Maximize is not Restore")
	}
	if captionIcon(captionStyleGNOME, captionMaximize, windowChrome{}) != icMaximize {
		t.Error("GNOME: a windowed window's Maximize is not Maximize")
	}
}
