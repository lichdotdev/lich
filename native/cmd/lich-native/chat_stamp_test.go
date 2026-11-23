package main

import (
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

func novDay(day int, hour, min int) int64 {
	return time.Date(2026, 11, day, hour, min, 0, 0, time.UTC).UnixMilli()
}

// stampFrames is a chat in view at 18:00 on 2026-11-18, local time, which the
// footer's clock reads too: its minute tick must not be already behind the
// frame's.
func stampFrames(t testing.TB) (*window, *frameRig) {
	t.Helper()
	in := busySession()
	in.now = time.Date(2026, 11, 18, 18, 0, 0, 0, time.Local)
	u, r, _ := chatFramesIn(t, api.ViewChat, rigWindow, in)
	r.now = in.now
	return u, r
}

func TestADayRowSeparatesTurnsOfDifferentDays(t *testing.T) {
	items := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(17, 23, 50)},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(17, 23, 51)},
		{ID: "a2", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 0, 5)},
		{ID: "u2", Kind: api.ItemUser, Turn: 2, At: novDay(18, 0, 10)},
		{ID: "u3", Kind: api.ItemUser, Turn: 3, At: novDay(18, 9, 0)},
	}
	var days []int64
	for _, r := range chatRows(items, claudeKind, false, time.Time{}) {
		if r.kind == chatRowDay {
			days = append(days, r.day)
		}
	}
	if len(days) != 1 || days[0] != novDay(18, 0, 10) {
		t.Errorf("day rows %v, want the one before the turn that starts a new day: an answer crossing midnight and a first turn have none", days)
	}
}

func TestADayIsNamedAgainstNow(t *testing.T) {
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	for day, want := range map[time.Time]string{
		now.Add(-9 * time.Hour):               i18n.T("chat.day.today"),
		now.AddDate(0, 0, -1):                 i18n.T("chat.day.yesterday"),
		now.AddDate(0, 0, -3):                 "Nov 15",
		now.AddDate(-1, 0, 0):                 "Nov 18, 2025",
		now.AddDate(0, 0, -1).Add(-time.Hour): i18n.T("chat.day.yesterday"),
	} {
		if got := dayLabel(day, now); got != want {
			t.Errorf("%v is %q, want %q", day, got, want)
		}
	}
}

// clockFor is the short time the window shows for unix-ms at, in the rig's own
// zone.
func clockFor(r *frameRig, at int64) string {
	return time.UnixMilli(at).In(r.now.Location()).Format(chatClock)
}

func TestEachMessageShowsItsTimeAtTheEdgeItIsAlignedTo(t *testing.T) {
	u, r := stampFrames(t)
	userAt, agentAt := novDay(18, 14, 32), novDay(18, 14, 35)
	user, agent := clockFor(r, userAt), clockFor(r, agentAt)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: userAt, Text: "hi"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: agentAt, Text: "hello"},
	})
	r.settle(t)
	overAnswer(r, t, agent) // an answer's time shows under the pointer
	for _, label := range []string{user, agent} {
		if !r.shows(label) {
			t.Fatalf("no time %q:\n%s", label, r.drawn())
		}
	}
	if ay, uy := labelTop(t, r, agent), labelTop(t, r, user); ay <= uy {
		t.Errorf("the answer's time at y=%d, the message's at y=%d: want the answer below", ay, uy)
	}
	if ux, ax := labelLeft(t, r, user), labelLeft(t, r, agent); ux <= ax {
		t.Errorf("the message's time at x=%d, the answer's at x=%d: want the message's at the right edge, the answer's at the left", ux, ax)
	}
	// The tooltip is the footer clock's reading of the same moment.
	box := r.at(t, agent)
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(box.X, box.Y)})
	r.frame()
	r.now = r.now.Add(time.Second)
	r.frame()
	r.settle(t)
	if want := "claude · " + clockText(time.UnixMilli(agentAt).In(r.now.Location())); !r.shows(want) {
		t.Errorf("no tooltip %q over the time:\n%s", want, r.drawn())
	}
}

func TestAMessageWithNoTimeDrawsNone(t *testing.T) {
	u, r := stampFrames(t)
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "hi"}})
	r.settle(t)
	if r.shows(clockFor(r, 0)) {
		t.Errorf("a time was made up for a message that has none:\n%s", r.drawn())
	}
}

func TestADaySeparatorIsCenteredBetweenTurns(t *testing.T) {
	u, r := stampFrames(t)
	r.settle(t)
	today := r.now
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: today.AddDate(0, 0, -1).UnixMilli(), Text: "first"},
		{ID: "u2", Kind: api.ItemUser, Turn: 2, At: today.UnixMilli(), Text: "second"},
	})
	r.settle(t)
	label := i18n.T("chat.day.today")
	if !r.shows(label) {
		t.Fatalf("no day separator %q:\n%s", label, r.drawn())
	}
	if y1, ys, y2 := labelTop(t, r, "first"), labelTop(t, r, label), labelTop(t, r, "second"); !(y1 < ys && ys < y2) {
		t.Errorf("first at y=%d, separator at y=%d, second at y=%d: want it between", y1, ys, y2)
	}
}

// A conversation read back from the provider can leave the person's message
// without a time; its answer tells the day.
func TestATurnWithNoMessageTimeTakesItsDayFromItsAnswer(t *testing.T) {
	items := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(17, 21, 52)},
		{ID: "u2", Kind: api.ItemUser, Turn: 2, At: novDay(18, 5, 3)},
	}
	var days int
	for _, r := range chatRows(items, claudeKind, false, time.Time{}) {
		if r.kind == chatRowDay {
			days++
		}
	}
	if days != 1 {
		t.Errorf("%d day rows, want 1 between the turns", days)
	}
}

func TestAPersonsMessageNamesItsDayUnlessItIsToday(t *testing.T) {
	now := time.Date(2026, 11, 18, 18, 0, 0, 0, time.UTC)
	for at, want := range map[time.Time]string{
		time.Date(2026, 11, 18, 14, 32, 0, 0, time.UTC): "14:32",
		time.Date(2026, 11, 17, 23, 5, 0, 0, time.UTC):  "Nov 17 · 23:05",
		time.Date(2025, 12, 31, 9, 0, 0, 0, time.UTC):   "Dec 31, 2025 · 09:00",
	} {
		if got := datedClock(at, now); got != want {
			t.Errorf("%v reads %q, want %q", at, got, want)
		}
	}
}

func TestAPersonsMessageFromAnotherDayShowsItsDate(t *testing.T) {
	u, r := stampFrames(t)
	at := novDay(17, 14, 32)
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, At: at, Text: "hi"}})
	r.settle(t)
	want := datedClock(time.UnixMilli(at).In(r.now.Location()), r.now)
	if !r.shows(want) || r.shows(clockFor(r, at)) {
		t.Errorf("want the message dated %q, and not its bare time:\n%s", want, r.drawn())
	}
}

func TestTurnsAreSetApartMoreThanTheRowsInsideOne(t *testing.T) {
	u, r := stampFrames(t)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 14, 0), Text: "first"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 14, 1), Text: "answer"},
		{ID: "u2", Kind: api.ItemUser, Turn: 2, At: novDay(18, 14, 5), Text: "second"},
		{ID: "u3", Kind: api.ItemUser, Turn: 3, At: novDay(19, 9, 0), Text: "third"},
	})
	r.settle(t)
	first, second, third := labelTop(t, r, "first"), labelTop(t, r, "second"), labelTop(t, r, "third")
	day := labelTop(t, r, dayLabel(time.UnixMilli(novDay(19, 9, 0)).In(r.now.Location()), r.now))
	if first < 0 || second < 0 || third < 0 {
		t.Fatalf("a message is off screen:\n%s", r.drawn())
	}
	// first and second are a turn apart, with an answer between: the room
	// before "second" is the turn gap, not the row gap.
	if second-first < 3*20+36 {
		t.Errorf("turns at y=%d and y=%d, want more room between them", first, second)
	}
	if day < 0 || day <= second || third <= day {
		t.Errorf("the day line at y=%d is not between the second (y=%d) and the third (y=%d) messages", day, second, third)
	}
}

func TestOnlyTheRowThatStartsATurnTakesTheTurnGap(t *testing.T) {
	user := chatRow{kind: chatRowItem, item: backend.ChatItem{Kind: api.ItemUser}}
	agent := chatRow{kind: chatRowItem, item: backend.ChatItem{Kind: api.ItemAgent}}
	day, work := chatRow{kind: chatRowDay}, chatRow{kind: chatRowWork}
	for name, c := range map[string]struct {
		r, prev chatRow
		want    bool
	}{
		"a message after an answer":    {user, agent, true},
		"a day line":                   {day, agent, true},
		"the message under a day line": {user, day, false},
		"an answer":                    {agent, user, false},
		"a group's line":               {work, user, false},
	} {
		if got := startsTurn(c.r, c.prev); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
}
