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

func TestWindowButtonsShown(t *testing.T) {
	cases := []struct {
		name   string
		chrome windowChrome
		choice string
		want   bool
	}{
		{"system: lich draws the caption", windowChrome{}, windowButtonsSystem, true},
		{"system: the window manager decorates", windowChrome{decorated: true}, windowButtonsSystem, false},
		{"show: even when decorated", windowChrome{decorated: true}, windowButtonsShow, true},
		{"hide: even when lich draws the caption", windowChrome{}, windowButtonsHide, false},
		{"show: none in fullscreen", windowChrome{fullscreen: true}, windowButtonsShow, false},
	}
	for _, c := range cases {
		if got := c.chrome.buttonsShown(c.choice); got != c.want {
			t.Errorf("%s: buttonsShown = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCaptionUnderAlwaysShow(t *testing.T) {
	three := []captionButton{captionMinimize, captionMaximize, captionClose}
	none := captionLayout{style: captionStyleGNOME}
	if got := none.under(windowButtonsShow); !slices.Equal(got.right, three) || got.left != nil {
		t.Errorf("Always show over an empty layout = %v, %v; want the three on the right", got.left, got.right)
	}
	if got := none.under(windowButtonsSystem); got.right != nil {
		t.Errorf("Follow system over an empty layout drew %v", got.right)
	}
	left := captionLayout{left: []captionButton{captionClose}}
	if got := left.under(windowButtonsShow); got.right != nil {
		t.Errorf("Always show moved the desktop's own layout: right = %v", got.right)
	}
	mac := captionLayout{leftReserve: 1}
	if got := mac.under(windowButtonsShow); got.right != nil {
		t.Errorf("Always show drew lich's buttons beside macOS's: %v", got.right)
	}
}
