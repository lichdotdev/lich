package main

import (
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

func cursorGtx(now time.Time, pxPerDp float32) layout.Context {
	return layout.Context{Ops: new(op.Ops), Now: now, Metric: unit.Metric{PxPerDp: pxPerDp, PxPerSp: pxPerDp}}
}

// The page draws the cursor where the pane does, moved to the page's corner
// and into its CSS pixels.
func TestPageCursorStandsWhereThePanesDoes(t *testing.T) {
	var c agentCursor
	t0 := time.Unix(100, 0)
	play(&c, t0, clickMark(t0, 150, 140))
	c.busy = true
	page := image.Rect(10, 40, 410, 340)
	pc := pageCursorOf(cursorGtx(t0, 2), &c, ui.Dark.Primary, ui.Dark.PrimaryForeground, providerIcon("claude"), page, 4)
	if pc == nil {
		t.Fatal("a cursor that is up gave the page none")
	}
	if pc.X != 35 || pc.Y != 25 || pc.Unit != 0.5 {
		t.Fatalf("cursor at %v,%v unit %v, want 35,25 unit 0.5", pc.X, pc.Y, pc.Unit)
	}
	if len(pc.Rings) != 0 || pc.Badge == nil || len(pc.Badge.Icon) == 0 {
		t.Fatalf("before landing: rings %v badge %+v, want no ring and the click's badge", pc.Rings, pc.Badge)
	}
	landed := pageCursorOf(cursorGtx(t0.Add(cursorGlide+cursorDip/2), 2), &c, ui.Dark.Primary, ui.Dark.PrimaryForeground, providerIcon("claude"), page, 4)
	if len(landed.Rings) != 1 || landed.Scale >= 1 {
		t.Fatalf("mid-dip: rings %v scale %v, want a ring and a dip", landed.Rings, landed.Scale)
	}
}

func TestPageCursorIsNoneWhileTheCursorIsDown(t *testing.T) {
	var c agentCursor
	if pageCursorOf(cursorGtx(time.Unix(100, 0), 1), &c, ui.Dark.Primary, ui.Dark.PrimaryForeground, providerIcon("claude"), image.Rect(0, 0, 100, 100), 1) != nil {
		t.Fatal("a cursor never shown gave the page one")
	}
}

// The script the page runs parses, with a cursor up.
func TestOverlayScriptWithCursorParses(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node to parse the script with")
	}
	var c agentCursor
	t0 := time.Unix(100, 0)
	play(&c, t0, scrollMark(t0, 50, 40, 3))
	c.busy = true
	o := pageOverlay{Cursor: pageCursorOf(cursorGtx(t0.Add(cursorGlide+cursorScroll/2), 1), &c, ui.Dark.Primary, ui.Dark.PrimaryForeground, providerIcon("claude"), image.Rect(0, 0, 400, 300), 1)}
	if o.Cursor == nil || o.Cursor.Chevrons == nil {
		t.Fatalf("mid-scroll cursor %+v, want its chevrons", o.Cursor)
	}
	file := filepath.Join(t.TempDir(), "overlay.js")
	if err := os.WriteFile(file, []byte(overlayScript(o)), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, "--check", file).CombinedOutput(); err != nil {
		t.Fatalf("the script does not parse: %v\n%s", err, strings.TrimSpace(string(out)))
	}
}
