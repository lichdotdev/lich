package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// cardFrames is session s1 drawn as a chat with nothing in it yet, and a
// file, notes.md, written for a card.
func cardFrames(t *testing.T) (*window, *frameRig, *chatBackend, *chatPane, string) {
	t.Helper()
	u, r, f := chatFrames(t, api.ViewChat)
	u.model.client = u.client
	u.model.chat.history("s1", nil)
	r.settle(t)
	notes := writeFile(t, filepath.Join(t.TempDir(), "notes.md"), "# notes")
	return u, r, f, u.chats.pane("s1"), notes
}

func promptAttachments(t *testing.T, body string) []string {
	t.Helper()
	var args []api.ChatPromptOptions
	if err := json.Unmarshal([]byte(body), &args); err != nil {
		t.Fatalf("decode chat prompt %q: %v", body, err)
	}
	if len(args) != 1 {
		t.Fatalf("chat prompt has %d argument objects, want 1", len(args))
	}
	paths := make([]string, len(args[0].Attachments))
	for i, a := range args[0].Attachments {
		paths[i] = a.Path
	}
	return paths
}

// A file's card shows its name and its type in capitals, said once.
func TestAFileCardShowsItsNameAndType(t *testing.T) {
	u, r, _, _, notes := cardFrames(t)
	u.chats.attachIn("s1", []backend.Attachment{{Path: notes}})
	r.frames(2)
	if !r.showsAll("notes", "MD") || r.shows("notes.md") {
		t.Errorf("the card does not show its name and type:\n%s", r.drawn())
	}
}

// A card's × takes it off the composer.
func TestACardsCrossTakesItOff(t *testing.T) {
	u, r, _, p, notes := cardFrames(t)
	other := writeFile(t, filepath.Join(t.TempDir(), "plan.txt"), "x")
	u.chats.attachIn("s1", []backend.Attachment{{Path: notes}, {Path: other}})
	r.frames(2)
	card, ok := r.smallestClickAt(r.glyph(t, "notes"))
	if !ok {
		t.Fatal("the card takes no click")
	}
	r.click(t, r.smallestClickIn(t, card))
	r.frame()
	if got := filePaths(p.attached); !slices.Equal(got, []string{other}) {
		t.Errorf("cards %q, want plan.txt alone", got)
	}
}

// A message of cards alone goes: drawn sent with its card at once, and handed
// to the agent with the file once the wait is up.
func TestAMessageOfCardsAloneGoesWithThem(t *testing.T) {
	u, r, f, p, notes := cardFrames(t)
	u.chats.attachIn("s1", []backend.Attachment{{Path: notes}})
	r.frame()
	r.press(key.NameReturn)
	if len(p.attached) != 0 {
		t.Errorf("the composer kept the cards: %q", filePaths(p.attached))
	}
	r.now = r.now.Add(time.Second)
	r.frames(3)
	if !r.shows("notes") {
		t.Errorf("the message sent does not show its card:\n%s", r.drawn())
	}
	body := f.waitCall(t, api.ChatPrompt.Name)
	if got, want := promptAttachments(t, body), []string{notes}; !slices.Equal(got, want) {
		t.Errorf("sent attachments %q, want %q", got, want)
	}
}

// Esc in the wait takes the message back with its cards, ahead of a card
// added since.
func TestEscTakesAWaitingMessageBackWithItsCards(t *testing.T) {
	u, r, _, p, notes := cardFrames(t)
	later := writeFile(t, filepath.Join(t.TempDir(), "later.md"), "x")
	u.chats.attachIn("s1", []backend.Attachment{{Path: notes}})
	typeIn(r, p, "read this")
	r.press(key.NameReturn)
	u.chats.attachIn("s1", []backend.Attachment{{Path: later}})
	r.frame()
	r.press(key.NameEscape)
	if p.editor.Text() != "read this" || !slices.Equal(filePaths(p.attached), []string{notes, later}) {
		t.Errorf("box %q with %q, want the message and its card back ahead of the later one", p.editor.Text(), filePaths(p.attached))
	}
}

// A message queued while the agent works keeps its cards, and the turn's end
// sends them.
func TestAQueuedMessageKeepsItsCards(t *testing.T) {
	u, r, f, p := queueFrames(t)
	notes := writeFile(t, filepath.Join(t.TempDir(), "notes.md"), "x")
	u.chats.attachIn("s1", []backend.Attachment{{Path: notes}})
	typeIn(r, p, "also this")
	r.press(key.NameReturn)
	q := u.model.chat.queued("s1")
	if len(q) != 1 || !slices.Equal(filePaths(q[0].files), []string{notes}) || len(p.attached) != 0 {
		t.Fatalf("queue %v, composer %q: want the message with its card queued", q, filePaths(p.attached))
	}
	u.model.flushQueue(context.Background(), api.ChatTurn{SessionID: "s1", StopReason: "end_turn"})
	body := f.waitCall(t, api.ChatPrompt.Name)
	if got, want := promptAttachments(t, body), []string{notes}; !slices.Equal(got, want) {
		t.Errorf("the turn's end sent attachments %q, want %q", got, want)
	}
}

// The queue taken back to edit brings its cards into the composer.
func TestEditingTheQueueBringsItsCards(t *testing.T) {
	u, r, _, p := queueFrames(t)
	u.model.chat.enqueue("s1", chatMessage{text: "first", files: []backend.Attachment{{Path: "/a.png"}}})
	u.model.chat.enqueue("s1", chatMessage{files: []backend.Attachment{{Path: "/b.md"}}})
	r.frame()
	r.press(key.NameUpArrow)
	if p.editor.Text() != "first" || !slices.Equal(filePaths(p.attached), []string{"/a.png", "/b.md"}) {
		t.Errorf("box %q with %q, want the queue's text and both cards", p.editor.Text(), filePaths(p.attached))
	}
}

// A click on a card opens its preview: the file's name over its path, and its
// size for a file that is no image.
func TestACardOpensItsPreview(t *testing.T) {
	u, r, _, _, notes := cardFrames(t)
	stage := r.draw
	r.draw = func(gtx C, th *ui.Theme) {
		u.layoutAttachPreview(gtx, th)
		stage(gtx, th)
	}
	u.chats.attachIn("s1", []backend.Attachment{{Path: notes}})
	r.frames(2)
	r.click(t, r.at(t, "notes"))
	r.frames(3)
	if !r.showsAll(notes, formatBlobSize(len("# notes"))) {
		t.Errorf("the preview does not show the file's path and size:\n%s", r.drawn())
	}
}
