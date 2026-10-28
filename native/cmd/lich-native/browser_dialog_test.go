package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
)

func dialogMark(at time.Time, action string) browserMark {
	markSeq++
	return browserMark{seq: markSeq, at: at, action: action}
}

func TestDialogCardShowsThePagesDialogAndSelectsAPromptsDefault(t *testing.T) {
	var d dialogCard
	d.sync(&api.BrowserDialog{Type: "prompt", Message: "Name?", Default: "Ada", URL: "http://127.0.0.1:5173/items"}, false)
	if d.shown == nil || d.shown.Message != "Name?" {
		t.Fatalf("shown %+v, want the page's prompt", d.shown)
	}
	if !d.focusField || d.field.Text() != "Ada" || d.field.SelectedText() != "Ada" {
		t.Fatalf("field %q selected %q focus %v, want the default selected and focused", d.field.Text(), d.field.SelectedText(), d.focusField)
	}
	d.focusField = false
	d.field.SetText("Grace")
	d.sync(&api.BrowserDialog{Type: "prompt", Message: "Name?", Default: "Ada", URL: "http://127.0.0.1:5173/items"}, false)
	if d.field.Text() != "Grace" || d.focusField {
		t.Fatalf("the same dialog again reset the field to %q", d.field.Text())
	}
	d.sync(&api.BrowserDialog{Type: "confirm", Message: "Delete this item?"}, false)
	if d.shown.Type != "confirm" || d.focusField {
		t.Fatalf("a confirm after the prompt: shown %+v, focus %v", d.shown, d.focusField)
	}
}

func TestDialogCardLeavesWhenThePageHasNoDialog(t *testing.T) {
	var d dialogCard
	d.sync(&api.BrowserDialog{Type: "alert", Message: "Saved."}, false)
	d.sync(nil, false)
	if d.shown != nil {
		t.Fatalf("shown %+v after the dialog closed", d.shown)
	}
}

func TestDialogCardStaysUntilTheAgentsAnswerHasPlayed(t *testing.T) {
	p := &browserPaneUI{}
	t0 := time.Unix(100, 0)
	p.dialog.sync(&api.BrowserDialog{Type: "confirm", Message: "Delete this item?"}, p.answering(t0))
	p.dialog.okAt, p.dialog.noAt = image.Rect(300, 100, 340, 132), image.Rect(220, 100, 290, 132)

	// The agent's act arrives, then the state without the dialog.
	p.cursor.take([]browserMark{dialogMark(t0, "accept")})
	p.dialog.sync(nil, p.answering(t0))
	if p.dialog.shown == nil {
		t.Fatal("the card left before the cursor pressed its button")
	}
	m, ok := p.cursor.advance(t0, func(m browserMark) f32.Point { return p.dialog.point(m.action, 800, 68) })
	if !ok || p.cursor.to != f32.Pt(320, 116) {
		t.Fatalf("cursor went to %v (started %v), want OK's middle", p.cursor.to, ok)
	}
	p.flash = agentPress{what: m.action, at: p.cursor.landing()}
	pressing := p.cursor.landing().Add(toolbarFlash / 2)
	p.dialog.sync(nil, p.answering(pressing))
	if p.dialog.shown == nil {
		t.Fatal("the card left while the cursor pressed OK")
	}
	after := p.cursor.landing().Add(toolbarFlash)
	p.dialog.sync(nil, p.answering(after))
	if p.dialog.shown != nil {
		t.Fatal("the card stayed after the cursor pressed OK")
	}
}

func TestDialogCardButtonTheAgentPresses(t *testing.T) {
	ok, no := image.Rect(300, 100, 340, 132), image.Rect(220, 100, 290, 132)
	for _, tc := range []struct {
		kind, action string
		want         image.Rectangle
	}{
		{"confirm", "accept", ok},
		{"confirm", "dismiss", no},
		{"prompt", "dismiss", no},
		{"beforeunload", "accept", ok},
		{"beforeunload", "dismiss", no},
		{"alert", "accept", ok},
		{"alert", "dismiss", ok},
	} {
		d := dialogCard{shown: &api.BrowserDialog{Type: tc.kind}, okAt: ok, noAt: no}
		if got := d.target(tc.action); got != tc.want {
			t.Errorf("%s %s pressed %v, want %v", tc.kind, tc.action, got, tc.want)
		}
	}
}

func TestDialogCardPointBeforeItWasDrawnIsItsTop(t *testing.T) {
	var d dialogCard
	if got := d.point("accept", 800, 68); got != f32.Pt(400, 68) {
		t.Fatalf("point %v, want the page's top middle", got)
	}
}

func TestAgentCursorQueuesTheDialogsAnswers(t *testing.T) {
	var c agentCursor
	t0 := time.Unix(100, 0)
	c.take([]browserMark{dialogMark(t0, "accept"), dialogMark(t0, "")})
	if len(c.pending) != 1 || c.pending[0].action != "accept" {
		t.Fatalf("pending %+v, want the accept alone", c.pending)
	}
}

func TestDialogKeys(t *testing.T) {
	for _, tc := range []struct {
		kind       string
		name       key.Name
		accept, ok bool
	}{
		{"confirm", key.NameReturn, true, true},
		{"confirm", key.NameEnter, true, true},
		{"confirm", key.NameEscape, false, true},
		{"prompt", key.NameEscape, false, true},
		{"beforeunload", key.NameReturn, true, true},
		{"beforeunload", key.NameEscape, false, true},
		{"alert", key.NameEscape, true, true},
		{"alert", key.NameReturn, true, true},
		{"confirm", "A", false, false},
		{"confirm", key.NameSpace, false, false},
	} {
		accept, ok := dialogKey(tc.kind, tc.name)
		if accept != tc.accept || ok != tc.ok {
			t.Errorf("%s %q: accept %v ok %v, want %v %v", tc.kind, tc.name, accept, ok, tc.accept, tc.ok)
		}
	}
}

func TestDialogFromNamesThePagesHost(t *testing.T) {
	if got := dialogFrom("http://127.0.0.1:5173/items"); got != "127.0.0.1:5173 says" {
		t.Errorf("got %q", got)
	}
	if got := dialogFrom("about:blank"); got != "This page says" {
		t.Errorf("about:blank: got %q", got)
	}
}
