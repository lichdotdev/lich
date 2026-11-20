package main

import (
	"image"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

const startPlaceholder = "Ask claude about this project"

// startFrames is session s1 drawn as a chat of project lichtrack, its
// conversation read and empty.
func startFrames(t *testing.T, size image.Point) (*window, *frameRig) {
	t.Helper()
	u, r, _ := chatFramesIn(t, api.ViewChat, size, busySession())
	u.model.projects[0].Name = "lichtrack"
	u.model.chat.history("s1", nil)
	r.settle(t)
	return u, r
}

// An empty chat asks its question over the composer, the project named in
// it, the block a little over the middle of the pane rather than at its foot.
func TestAnEmptyChatCentresTheComposerUnderTheQuestion(t *testing.T) {
	_, r := startFrames(t, rigWindow)
	before, after := chatStartQuestion("s1")
	if !r.showsAll("lichtrack", startPlaceholder) || (before != "" && !r.shows(before)) || (after != "" && !r.shows(after)) {
		t.Fatalf("no question over the composer:\n%s", r.drawn())
	}
	if y := labelTop(t, r, startPlaceholder); y > r.size.Y*2/3 || y < r.size.Y/3 {
		t.Errorf("the composer's text at y=%d of %d: want it near the middle", y, r.size.Y)
	}
}

// The first item takes the start screen away: the question fades and the
// composer runs down to the foot, half way at half the run.
func TestTheFirstItemRunsTheComposerDownToTheFoot(t *testing.T) {
	u, r := startFrames(t, rigWindow)
	centred := labelTop(t, r, startPlaceholder)
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go"}})
	r.frame()
	r.play(7) // half of the 220ms run
	mid := labelTop(t, r, startPlaceholder)
	r.settle(t)
	foot := labelTop(t, r, startPlaceholder)
	if mid <= centred || mid >= foot {
		t.Errorf("the composer at y=%d, %d half way, %d after: want it running down", centred, mid, foot)
	}
	if before, _ := chatStartQuestion("s1"); r.shows(before) {
		t.Error("the question stays once the conversation has an item")
	}
}

// A pane too low for the block keeps the composer at its foot, with no logo
// and no question.
func TestALowPaneKeepsTheComposerAtItsFoot(t *testing.T) {
	_, r := startFrames(t, image.Pt(800, 260))
	if before, _ := chatStartQuestion("s1"); r.shows(before) {
		t.Errorf("a low pane draws the question:\n%s", r.drawn())
	}
	if !r.shows(startPlaceholder) {
		t.Fatalf("no composer:\n%s", r.drawn())
	}
}

// A session keeps its question; sessions spread over the list; every
// language has its own list, each line naming the project once.
func TestTheStartQuestionIsTheSessionsAndEveryLanguageHasAList(t *testing.T) {
	first, _ := chatStartQuestion("s1")
	for range 20 {
		if got, _ := chatStartQuestion("s1"); got != first {
			t.Fatalf("the question moved from %q to %q", first, got)
		}
	}
	seen := map[string]bool{}
	for i := range 40 {
		b, a := chatStartQuestion("s" + string(rune('a'+i)))
		seen[b+"|"+a] = true
	}
	if len(seen) < 4 {
		t.Errorf("40 sessions asked %d questions, want the list spread", len(seen))
	}
	for _, l := range i18n.Locales {
		lines := strings.Split(i18n.In(l, "chat.start.lines", i18n.Args{"project": "{project}"}), chatStartSep)
		if len(lines) < 2 {
			t.Errorf("%s has %d questions", l, len(lines))
		}
		for _, line := range lines {
			if strings.Count(line, "{project}") != 1 || line != strings.TrimSpace(line) {
				t.Errorf("%s has the question %q", l, line)
			}
		}
	}
}
