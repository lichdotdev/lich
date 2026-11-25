package main

import (
	"context"
	"encoding/json"
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
)

func TestQuotedTextPutsEachQuoteAsBlockquoteLinesBeforeTheText(t *testing.T) {
	cases := map[string]struct {
		quotes []string
		typed  string
		want   string
	}{
		"no quote":        {nil, "plain", "plain"},
		"one quote":       {[]string{"attach the listener first"}, "Why?", "> attach the listener first\n\nWhy?"},
		"a quote alone":   {[]string{"attach the listener first"}, "", "> attach the listener first"},
		"lines of one":    {[]string{"first line\n\nthird line"}, "Why?", "> first line\n>\n> third line\n\nWhy?"},
		"in their order":  {[]string{"one", "two"}, "Both?", "> one\n\n> two\n\nBoth?"},
		"nothing at all":  {nil, "", ""},
		"trailing spaces": {[]string{"code  "}, "x", "> code\n\nx"},
	}
	for name, c := range cases {
		if got := quotedText(c.quotes, c.typed); got != c.want {
			t.Errorf("%s: quotedText(%q, %q) = %q, want %q", name, c.quotes, c.typed, got, c.want)
		}
	}
}

// A quote goes in under those already there, a blank one does not, and the
// message that went takes them all with it.
func TestQuotesStackInOrderAndGoWithTheMessage(t *testing.T) {
	p := newChatPane()
	p.addQuote("  first  ")
	p.addQuote(" \n ")
	p.addQuote("second")
	if !slices.Equal(p.quotes, []string{"first", "second"}) {
		t.Fatalf("quotes %q, want first then second", p.quotes)
	}
	p.editor.SetText("  and?  ")
	if m := p.composed(); m.text != "> first\n\n> second\n\nand?" {
		t.Errorf("composed %q", m.text)
	}
	p.clearComposer()
	if len(p.quotes) != 0 || !p.composed().empty() {
		t.Errorf("a cleared composer kept %q", p.quotes)
	}
}

// selectIn drags over the first line of the text drawn around from, which
// shows Quote over the selection once the press ends.
func selectIn(t *testing.T, r *frameRig, from image.Point, across float32) {
	t.Helper()
	start := textStart(t, r, from)
	r.dragAcross(start, start.Add(f32.Pt(across, 0)))
	r.frame()
}

// Quote over a selection, in an answer or in the person's message, puts what
// was selected over the composer's text and leaves the composer the keys.
func TestQuoteTakesTheSelectionIntoTheComposer(t *testing.T) {
	cases := map[string]struct {
		from func(*testing.T, *frameRig) image.Point
		read string
	}{
		"an answer": {func(t *testing.T, r *frameRig) image.Point {
			return image.Pt(composerBox(t, r).Min.X+48, composerBox(t, r).Min.Y-140)
		}, "Run the bold part and go test ./... before you push"},
		"the person's message": {func(t *testing.T, r *frameRig) image.Point {
			return labelBox(t, r, selectAsk).Min.Add(image.Pt(48, 0))
		}, "Can you check app.go before lunch?"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			u, r := selectFrames(t)
			p := u.chats.pane("s1")
			label := i18n.T("chat.quote.add")
			if r.shows(label) {
				t.Fatal("Quote is drawn with nothing selected")
			}
			selectIn(t, r, c.from(t, r), 90)
			shot := strings.ReplaceAll(strings.ReplaceAll(name, " ", "-"), "'", "")
			writeShot(t, r, "shot-chat-quote-selection-"+shot+".png")
			r.clickLabel(t, label)
			writeShot(t, r, "shot-chat-quote-composer-"+shot+".png")
			if len(p.quotes) != 1 || len(p.quotes[0]) < 5 || !strings.HasPrefix(c.read, p.quotes[0]) {
				t.Fatalf("quotes %q, want the opening of %q", p.quotes, c.read)
			}
			if r.shows(label) {
				t.Error("Quote stayed drawn after it quoted")
			}
			if got := r.copiedText(); got != "" {
				t.Errorf("the selection stayed after Quote: Ctrl+C copied %q", got)
			}
			if !r.router.Source().Focused(&p.editor) {
				t.Error("the composer does not have the keys after Quote")
			}
		})
	}
}

// Escape lets go of the selection and Quote with it, and Ctrl+C still copies
// while Quote shows.
func TestEscapeDropsTheSelectionAndQuote(t *testing.T) {
	_, r := selectFrames(t)
	selectIn(t, r, labelBox(t, r, selectAsk).Min.Add(image.Pt(48, 0)), 90)
	label := i18n.T("chat.quote.add")
	if !r.shows(label) {
		t.Fatalf("no Quote over the selection:\n%s", r.drawn())
	}
	if got := r.copiedText(); len(got) < 5 {
		t.Errorf("Ctrl+C copied %q with Quote shown", got)
	}
	r.router.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	r.frame()
	if r.shows(label) {
		t.Error("Quote stayed after Escape")
	}
}

// A quote's × takes that quote off whole, leaving the others and the text.
func TestAQuotesXRemovesItWhole(t *testing.T) {
	u, r := selectFrames(t)
	p := u.chats.pane("s1")
	p.addQuote("first quote")
	p.addQuote("second quote")
	typeIn(r, p, "typed")
	r.settle(t)
	r.click(t, r.controlOf(t, "first quote"))
	if !slices.Equal(p.quotes, []string{"second quote"}) {
		t.Fatalf("quotes %q after the first one's ×, want the second alone", p.quotes)
	}
	if p.editor.Text() != "typed" {
		t.Errorf("the text became %q", p.editor.Text())
	}
}

// promptText is the text a ChatPrompt or ChatSteer call carried.
func promptText(t *testing.T, body string) string {
	t.Helper()
	var o []api.ChatPromptOptions
	if err := json.Unmarshal([]byte(body), &o); err != nil || len(o) != 1 {
		t.Fatalf("body %s: %v", body, err)
	}
	return o[0].Text
}

// A quoted message reaches the agent as the same plain text from every way
// a message goes, for each of the chat's providers.
func TestAQuotedMessageGoesAsTheSameTextEveryWay(t *testing.T) {
	const want = "> attach the listener first\n\nDoes that hold on Windows too?"
	type path struct {
		running bool
		method  string
		send    func(u *window, r *frameRig)
	}
	paths := map[string]path{
		"held, then sent": {false, api.ChatPrompt.Name, func(_ *window, r *frameRig) {
			r.press(key.NameReturn)
			r.now = r.now.Add(chatSendHold + time.Second)
			r.frame()
		}},
		"queued, then sent at the turn's end": {true, api.ChatPrompt.Name, func(u *window, r *frameRig) {
			r.press(key.NameReturn)
			u.model.flushQueue(context.Background(), api.ChatTurn{SessionID: "s1", StopReason: "end_turn"})
		}},
		"queued, then sent into the turn": {true, api.ChatSteer.Name, func(_ *window, r *frameRig) {
			r.press(key.NameReturn)
			r.pressWith(key.NameReturn, key.ModShortcut)
		}},
	}
	for _, kind := range []string{claudeKind, codexKind, opencodeKind} {
		for name, c := range paths {
			t.Run(kind+"/"+name, func(t *testing.T) {
				u, r, f := chatFrames(t, api.ViewChat)
				u.model.client = u.client
				u.model.projects[0].Sessions[0].Kind = kind
				u.model.chat.kinds = []string{kind}
				u.model.chat.history("s1", nil)
				u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: c.running})
				r.frame()
				p := u.chats.pane("s1")
				p.addQuote("attach the listener first")
				typeIn(r, p, "Does that hold on Windows too?")
				c.send(u, r)
				if got := promptText(t, f.waitCall(t, c.method)); got != want {
					t.Errorf("%s sent %q, want %q", c.method, got, want)
				}
				if len(p.quotes) != 0 {
					t.Errorf("the composer kept %q after the message went", p.quotes)
				}
			})
		}
	}
}
