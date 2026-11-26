package main

import (
	"encoding/base64"
	"encoding/json"
	"image"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/browserwire"
	"github.com/lichdotdev/lich/native/internal/turbojpeg"
)

// stripesPage is red for its first 300 CSS pixels, then blue, then green, so
// a frame's top row says how far the page scrolled.
const stripesPage = "data:text/html,<body style='margin:0'>" +
	"<div style='height:300px;background:%23f00'></div>" +
	"<div style='height:300px;background:%2300f'></div>" +
	"<div style='height:5000px;background:%230f0'></div>"

func needsOffscreen(t *testing.T) {
	t.Helper()
	if !offscreen() {
		t.Skip("the window embeds these pages: no frames")
	}
}

// frame waits for a frame of page from index from on, and decodes it.
func (c *client) frame(from int, page string) (browserwire.FrameEvent, *image.RGBA, int) {
	c.t.Helper()
	var f browserwire.FrameEvent
	_, next := c.event(from, browserwire.EventFrame, func(raw json.RawMessage) bool {
		return json.Unmarshal(raw, &f) == nil && f.Page == page
	})
	data, err := base64.StdEncoding.DecodeString(f.Data)
	if err != nil {
		c.t.Fatalf("a frame's data is no base64: %v", err)
	}
	img, err := turbojpeg.Decode(data)
	if err != nil {
		c.t.Fatalf("a frame is no JPEG: %v", err)
	}
	return f, img, next
}

// frameWhere waits for a frame of page from index from on that ok accepts.
func (c *client) frameWhere(from int, page string, ok func(browserwire.FrameEvent, *image.RGBA) bool) (browserwire.FrameEvent, *image.RGBA) {
	c.t.Helper()
	for {
		f, img, next := c.frame(from, page)
		if ok(f, img) {
			return f, img
		}
		from = next
	}
}

// colour names the colour of p's pixel at x, y: red, green, blue or other.
func colour(img *image.RGBA, x, y int) string {
	c := img.RGBAAt(x, y)
	switch {
	case c.R > 200 && c.G < 60 && c.B < 60:
		return "red"
	case c.G > 200 && c.R < 60 && c.B < 60:
		return "green"
	case c.B > 200 && c.R < 60 && c.G < 60:
		return "blue"
	}
	return "other"
}

func TestFramesArriveAtTheScaleAndChangeOnScroll(t *testing.T) {
	needsOffscreen(t)
	for _, tc := range []struct {
		name  string
		scale float64
	}{{"at 1x", 1}, {"at 2x", 2}} {
		t.Run(tc.name, func(t *testing.T) {
			c := start(t)
			c.open("b1", stripesPage, 400, 200, 1)
			from := c.mark()
			c.must(browserwire.Screencast, browserwire.ScreencastParams{Page: "b1", W: 400, H: 200, Scale: tc.scale}, nil)
			want := image.Rect(0, 0, int(400*tc.scale), int(200*tc.scale))
			// A page just loaded may not have drawn yet, and the first frames
			// may be of the size before the screencast's.
			f, _ := c.frameWhere(from, "b1", func(_ browserwire.FrameEvent, img *image.RGBA) bool {
				return img.Bounds() == want && colour(img, 10, 10) == "red"
			})
			if f.Tab == "" {
				t.Fatal("a frame names no tab")
			}
			from = c.mark()
			c.must(browserwire.Mouse, browserwire.MouseParams{Page: "b1", Kind: browserwire.MouseWheel, X: 100, Y: 100, DY: 400}, nil)
			c.frameWhere(from, "b1", func(_ browserwire.FrameEvent, img *image.RGBA) bool { return colour(img, 10, 10) == "blue" })
		})
	}
}

func TestNoFramesOnceTheScreencastStops(t *testing.T) {
	needsOffscreen(t)
	c := start(t)
	c.open("b1", stripesPage, 400, 200, 1)
	c.must(browserwire.Screencast, browserwire.ScreencastParams{Page: "b1", W: 400, H: 200, Scale: 1}, nil)
	c.frame(0, "b1")
	c.must(browserwire.StopScreencast, browserwire.PageParams{Page: "b1"}, nil)
	from := c.mark()
	c.must(browserwire.Mouse, browserwire.MouseParams{Page: "b1", Kind: browserwire.MouseWheel, X: 100, Y: 100, DY: 400}, nil)
	if got := c.eval("b1", "new Promise(r => setTimeout(() => r(scrollY), 500))"); got != "400" {
		t.Fatalf("the page scrolled to %s, want 400", got)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.events[from:] {
		if e.Event == browserwire.EventFrame {
			t.Fatalf("a frame came after the screencast stopped: %.80s", e.Params)
		}
	}
}

func TestFramesFollowTheShownTab(t *testing.T) {
	needsOffscreen(t)
	c := start(t)
	c.open("b1", stripesPage, 400, 200, 1)
	c.must(browserwire.Screencast, browserwire.ScreencastParams{Page: "b1", W: 400, H: 200, Scale: 1}, nil)
	first, _, _ := c.frame(0, "b1")
	from := c.mark()
	c.must(browserwire.NewTab, browserwire.PageParams{Page: "b1"}, nil)
	tabs := c.tabsWhere(from, func(tabs []browserwire.Tab) bool { return len(tabs) == 2 })
	for _, tab := range tabs {
		if tab.XID != 0 {
			t.Fatalf("tab %+v has a surface off screen", tab)
		}
	}
	second := tabs[1].ID
	_, img := c.frameWhere(from, "b1", func(f browserwire.FrameEvent, _ *image.RGBA) bool { return f.Tab == second })
	if colour(img, 10, 10) == "red" {
		t.Fatal("the blank tab's frame shows the first tab's page")
	}
	from = c.mark()
	c.must(browserwire.SelectTab, browserwire.TabParams{Page: "b1", Tab: first.Tab}, nil)
	c.frameWhere(from, "b1", func(f browserwire.FrameEvent, img *image.RGBA) bool {
		return f.Tab == first.Tab && colour(img, 10, 10) == "red"
	})
}

func TestAScreencastNeedsAnOpenPage(t *testing.T) {
	needsOffscreen(t)
	c := start(t)
	err := c.call(browserwire.Screencast, browserwire.ScreencastParams{Page: "b9", W: 400, H: 200, Scale: 1}, nil)
	if err == nil || !strings.Contains(err.Error(), "b9") {
		t.Fatalf("a screencast of no page answered %v", err)
	}
}
