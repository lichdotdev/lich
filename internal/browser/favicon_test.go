package browser

import (
	"encoding/base64"
	"image/color"
	"testing"
	"time"
)

func TestIconPNGTakesOnlyASmallPNG(t *testing.T) {
	small := solidPNG(t, iconSide, color.Black)
	if got, err := iconPNG("data:image/png;base64," + base64.StdEncoding.EncodeToString(small)); err != nil || len(got) != len(small) {
		t.Fatalf("a %dpx PNG: %d bytes, %v", iconSide, len(got), err)
	}
	for name, url := range map[string]string{
		"too big":  "data:image/png;base64," + base64.StdEncoding.EncodeToString(solidPNG(t, iconSide+1, color.Black)),
		"not png":  "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("nope")),
		"not data": "https://example.com/icon.png",
	} {
		if _, err := iconPNG(url); err == nil {
			t.Errorf("%s: taken", name)
		}
	}
}

func TestIconCacheKeepsAnAnswerForAMinute(t *testing.T) {
	c := &iconCache{byURL: map[string]iconEntry{}, pngs: map[string][]byte{}}
	now := time.Unix(0, 0)
	key := c.store("https://a/icon.png", []byte("png"), now)
	if got, ok := c.lookup("https://a/icon.png", now.Add(iconFresh)); !ok || got != key || key == "" {
		t.Fatalf("lookup within a minute = %q, %v; stored %q", got, ok, key)
	}
	if data, ok := c.png(key); !ok || string(data) != "png" {
		t.Fatalf("png(%q) = %q, %v", key, data, ok)
	}
	if _, ok := c.lookup("https://a/icon.png", now.Add(iconFresh+time.Second)); ok {
		t.Fatal("an answer past a minute is reused")
	}
	if got := c.store("https://a/none.ico", nil, now); got != "" {
		t.Fatalf("an icon that did not load has key %q", got)
	}
	if got, ok := c.lookup("https://a/none.ico", now); !ok || got != "" {
		t.Fatalf("a failed icon is not remembered as none: %q, %v", got, ok)
	}
}

func TestTabStackDropsAnIconReadThatANavigationOvertook(t *testing.T) {
	s := stackOf(nil, nil)
	root := s.byID("root")
	seen := s.navigations(root)
	s.navigated(root)
	if s.setIcon(root, "old", seen) || s.list()[0].Icon != "" {
		t.Fatal("an icon read before the navigation was shown")
	}
	if !s.setIcon(root, "new", s.navigations(root)) || s.list()[0].Icon != "new" {
		t.Fatal("the current page's icon was not shown")
	}
	if agentTabs(s.list())[0].Icon != "" {
		t.Fatal("the agent's tab list carries the icon")
	}
}

func TestDarkColorReadsTheWindowsBackground(t *testing.T) {
	for hex, want := range map[string]bool{"#0a0a0a": true, "#18181b": true, "#ffffff": false, "#f4f4f5": false, "oops": false} {
		if got := darkColor(hex); got != want {
			t.Errorf("darkColor(%q) = %v, want %v", hex, got, want)
		}
	}
}
