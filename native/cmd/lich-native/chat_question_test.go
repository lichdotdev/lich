package main

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

var (
	askFruit = api.ChatAsk{Header: "Fruit", Text: "Which fruit?", Options: []api.ChatOffer{
		{Label: "Apple", Description: "Crisp and red"}, {Label: "Pear", Description: "Soft"},
	}}
	askColors = api.ChatAsk{Header: "Colors", Text: "Which colors?", Multi: true, Options: []api.ChatOffer{
		{Label: "Red"}, {Label: "Green"}, {Label: "Blue"},
	}}
	askName   = api.ChatAsk{Header: "Name", Text: "Which name?"}
	askLayout = api.ChatAsk{Header: "Layout", Text: "How do I lay it out?", Options: []api.ChatOffer{
		{Label: "By route", Preview: "routes/login.go"}, {Label: "By resource", Preview: "users/handler.go"},
	}}
)

// questionFrames is session s1 in a turn, its agent waiting on a question
// item holding asks, reply its answer (nil while it waits).
func questionFrames(t *testing.T, reply *api.ChatQReplied, asks ...api.ChatAsk) (*window, *frameRig, *chatBackend) {
	t.Helper()
	u, r, f := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "set it up"},
		{ID: "q1", Kind: api.ItemQuestion, Turn: 1, Question: &backend.ChatQuestion{Questions: asks, Reply: reply}},
	})
	if reply == nil {
		u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	}
	r.settle(t)
	return u, r, f
}

func (r *frameRig) pressKeys(names ...key.Name) {
	for _, n := range names {
		r.press(n)
	}
}

func wantReply(t *testing.T, f *chatBackend, parts ...string) {
	t.Helper()
	body := f.waitCall(t, api.ChatReply.Name)
	for _, p := range parts {
		if !strings.Contains(body, p) {
			t.Errorf("replied %s, want %s in it", body, p)
		}
	}
}

func TestClickingASingleChoiceAnswersAtOnce(t *testing.T) {
	_, r, f := questionFrames(t, nil, askFruit)
	for _, s := range []string{"FRUIT", "Which fruit?", "Crisp and red", i18n.T("chat.question.write")} {
		if !r.shows(s) {
			t.Errorf("the card lacks %q:\n%s", s, r.drawn())
		}
	}
	r.click(t, r.at(t, "Pear"))
	wantReply(t, f, `"itemId":"q1"`, `"answers":[{"picked":["Pear"]}]`)
}

func TestSingleChoiceKeys(t *testing.T) {
	for name, tc := range map[string]struct {
		keys []key.Name
		want string
	}{
		"a digit":         {[]key.Name{"2"}, `"picked":["Pear"]`},
		"Enter":           {[]key.Name{key.NameReturn}, `"picked":["Apple"]`},
		"arrows to Enter": {[]key.Name{key.NameDownArrow, key.NameReturn}, `"picked":["Pear"]`},
		"Esc":             {[]key.Name{key.NameEscape}, `"declined":true`},
	} {
		t.Run(name, func(t *testing.T) {
			_, r, f := questionFrames(t, nil, askFruit)
			r.pressKeys(tc.keys...)
			wantReply(t, f, tc.want)
		})
	}
}

func TestOtherTakesTheirOwnWords(t *testing.T) {
	u, r, f := questionFrames(t, nil, askFruit)
	r.press("3")
	q := u.chats.pane("s1").quizzes["q1"]
	if q.hot != 2 {
		t.Fatalf("the key 3 stands on option %d, want Other", q.hot)
	}
	r.router.Queue(key.EditEvent{Text: "a mango"})
	r.frame()
	r.press(key.NameReturn)
	wantReply(t, f, `"other":"a mango"`)
}

func TestSeveralChoicesMarkThenSend(t *testing.T) {
	_, r, f := questionFrames(t, nil, askColors)
	if !r.shows(i18n.T("chat.question.multi")) {
		t.Error("a multiple choice does not say so")
	}
	r.pressKeys(key.NameSpace, key.NameDownArrow, key.NameDownArrow, key.NameSpace)
	if !r.shows(i18n.T("chat.question.marked", i18n.Args{"count": 2})) {
		t.Errorf("the count of marks is not drawn:\n%s", r.drawn())
	}
	r.click(t, r.at(t, i18n.T("chat.composer.send")))
	wantReply(t, f, `"picked":["Red","Blue"]`)
}

func TestSeveralChoicesSendNothingUnmarked(t *testing.T) {
	_, r, f := questionFrames(t, nil, askColors)
	r.press(key.NameReturn)
	time.Sleep(50 * time.Millisecond)
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 0 {
		t.Errorf("Enter with nothing marked called %v", f.calls)
	}
}

func TestAnOpenQuestionTakesText(t *testing.T) {
	_, r, f := questionFrames(t, nil, askName)
	r.router.Queue(key.EditEvent{Text: "billing-worker"})
	r.frame()
	r.press(key.NameReturn)
	wantReply(t, f, `"other":"billing-worker"`)
}

func TestSeveralQuestionsStepThenSendAll(t *testing.T) {
	u, r, f := questionFrames(t, nil, askFruit, askColors)
	if !r.shows(i18n.T("chat.question.step", i18n.Args{"n": 1, "total": 2})) {
		t.Errorf("no step count:\n%s", r.drawn())
	}
	r.press("2")
	if !r.shows("Which colors?") || r.shows("Which fruit?") {
		t.Fatalf("answering did not move to the next question:\n%s", r.drawn())
	}
	r.pressKeys("1", key.NameReturn)
	if !r.shows(i18n.T("chat.question.sendAll")) {
		t.Fatalf("no summary after the last question:\n%s", r.drawn())
	}
	// A step already answered can be gone back to.
	r.click(t, r.at(t, "✓ FRUIT"))
	if q := u.chats.pane("s1").quizzes["q1"]; q.step != 0 {
		t.Errorf("the step click went to %d", q.step)
	}
	r.press("1")
	r.pressKeys(key.NameReturn)
	wantReply(t, f, `"answers":[{"picked":["Apple"]},{"picked":["Red"]}]`)
}

func TestAPreviewFollowsTheHighlightedOption(t *testing.T) {
	_, r, _ := questionFrames(t, nil, askLayout)
	if !r.shows("routes/login.go") {
		t.Errorf("the first option's preview is not drawn:\n%s", r.drawn())
	}
	r.press(key.NameDownArrow)
	if !r.shows("users/handler.go") || r.shows("routes/login.go") {
		t.Errorf("the preview did not follow the arrow:\n%s", r.drawn())
	}
}

func TestAnAnsweredQuestionIsALinePerAsk(t *testing.T) {
	_, r, _ := questionFrames(t, &api.ChatQReplied{Answers: []api.ChatAskAnswer{{Picked: []string{"Pear"}}, {Picked: []string{"Red"}, Other: "teal"}}}, askFruit, askColors)
	for _, s := range []string{"Fruit:", "Pear", "Colors:", "Red, " + i18n.T("chat.question.own", i18n.Args{"text": "teal"})} {
		if !r.shows(s) {
			t.Errorf("the answered card lacks %q:\n%s", s, r.drawn())
		}
	}
	if r.shows("Which fruit?") {
		t.Error("an answered question still asks")
	}
	_, r, _ = questionFrames(t, &api.ChatQReplied{Declined: true}, askFruit)
	if !r.shows(i18n.T("chat.question.declined")) {
		t.Errorf("a declined question does not say so:\n%s", r.drawn())
	}
}

// A message written in the composer while the agent waits goes to it: the
// backend declines the question first.
func TestTheComposerSendsPastAWaitingQuestion(t *testing.T) {
	u, r, f := questionFrames(t, nil, askFruit)
	r.click(t, r.at(t, "Ask claude about this project"))
	u.chats.pane("s1").editor.SetText("cheese instead")
	r.press(key.NameReturn)
	if body := f.waitCall(t, api.ChatPrompt.Name); !strings.Contains(body, "cheese instead") {
		t.Errorf("sent %s", body)
	}
}

// A waiting question stands in for the live row, and a window that opened
// mid-turn, which heard no turn start, draws no closing line under it.
func TestNoLiveOrDoneRowWhileAQuestionWaits(t *testing.T) {
	items := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1},
		{ID: "q1", Kind: api.ItemQuestion, Turn: 1, Question: &backend.ChatQuestion{Questions: []api.ChatAsk{askFruit}}},
	}
	for _, running := range []bool{true, false} {
		for _, r := range chatRows(items, running, time.Now()) {
			if r.kind == chatRowLive || r.kind == chatRowDone {
				t.Errorf("running=%v: a row of kind %d under a waiting question", running, r.kind)
			}
		}
	}
}

// An ask with no header, as Antigravity sends them, names its step by its
// number.
func TestAStepWithNoHeaderIsItsNumber(t *testing.T) {
	plain := func(a api.ChatAsk) api.ChatAsk { a.Header = ""; return a }
	_, r, _ := questionFrames(t, nil, plain(askFruit), plain(askColors))
	if !r.shows("1") || !r.shows("2") {
		t.Errorf("the steps are not numbered:\n%s", r.drawn())
	}
}

// An id that comes back for another question, as an ACP server numbering its
// requests again after a restart does, gets a card of its own.
func TestAnIDThatComesBackIsANewCard(t *testing.T) {
	u, r, f := questionFrames(t, nil, askFruit)
	r.click(t, r.at(t, "Pear"))
	f.waitCall(t, api.ChatReply.Name)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "set it up"},
		{ID: "q1", Kind: api.ItemQuestion, Turn: 1, Question: &backend.ChatQuestion{Questions: []api.ChatAsk{askFruit},
			Reply: &api.ChatQReplied{Answers: []api.ChatAskAnswer{{Picked: []string{"Pear"}}}}}},
	})
	r.frames(5) // the turn still runs: the flame never goes still
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u2", Kind: api.ItemUser, Turn: 1, Text: "again"},
		{ID: "q1", Kind: api.ItemQuestion, Turn: 1, Question: &backend.ChatQuestion{Questions: []api.ChatAsk{askFruit}}},
	})
	r.frames(5) // the turn still runs: the flame never goes still
	r.press("1")
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		f.mu.Lock()
		var replies []string
		for _, c := range f.calls {
			if strings.HasPrefix(c, api.ChatReply.Name+" ") {
				replies = append(replies, c)
			}
		}
		f.mu.Unlock()
		if len(replies) == 2 {
			if !strings.Contains(replies[1], `"picked":["Apple"]`) {
				t.Errorf("the new card replied %s", replies[1])
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the new card did not reply: %v", replies)
		}
	}
}

// The box of an option is the row's own target: a click on it marks the
// option once, as a click on its label does.
func TestClickingTheBoxMarksTheOptionOnce(t *testing.T) {
	u, r, _ := questionFrames(t, nil, askColors)
	p := u.chats.pane("s1")
	var box image.Point
	for _, n := range r.nodes() {
		if n.Desc.Label == "Red" {
			// The box stands left of the label: the row's 10dp gap and half
			// its 16dp size, at 1px per dp.
			box = image.Pt(n.Desc.Bounds.Min.X-18, n.Desc.Bounds.Min.Y+7)
		}
	}
	r.click(t, f32.Pt(float32(box.X), float32(box.Y)))
	if !p.quizzes["q1"].marks[0][0].Value {
		t.Fatalf("a click on the box at %v left Red unmarked:\n%s", box, r.drawn())
	}
	r.click(t, r.at(t, "Red"))
	if p.quizzes["q1"].marks[0][0].Value {
		t.Error("a click on the label after the box did not unmark Red")
	}
}
