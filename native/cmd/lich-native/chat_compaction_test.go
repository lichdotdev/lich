package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

func compactItem(id string, turn int, cc api.ChatCompaction) backend.ChatItem {
	return backend.ChatItem{ID: id, Kind: api.ItemCompaction, Turn: turn, Compaction: &cc}
}

// compactNow is the rigs' clock, half a WaveBar lap in so its wave shows; the
// person's /compact went 6 seconds before.
const compactNow = 1_795_000_000_500

// typedCompact is the person's /compact, at each point of its life.
func typedCompact(cc api.ChatCompaction) []backend.ChatItem {
	sent := int64(compactNow - 6000)
	compaction := compactItem("compaction-1", 2, cc)
	compaction.At = sent
	return []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "fix the parser", At: sent - 60000},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, Text: "Fixed: the parser reads a trailing comma now.", At: sent - 50000},
		{ID: "u2", Kind: api.ItemUser, Turn: 2, Text: "/compact", At: sent},
		compaction,
	}
}

func rowKinds(rows []chatRow) []string {
	var out []string
	for _, r := range rows {
		switch r.kind {
		case chatRowLive:
			out = append(out, "live"+map[bool]string{true: ":compacting"}[r.compacting])
		case chatRowDone:
			out = append(out, "done")
		case chatRowItem:
			out = append(out, r.item.Kind)
		}
	}
	return out
}

// While the turn compacts its compaction has no marker: the live line says it
// runs. Once over, the marker closes the person's /compact, with no turn end of
// its own; a cancelled one is drawn nowhere and the turn ends as any other.
func TestACompactionIsTheLiveLineThenTheMarkerThatClosesItsTurn(t *testing.T) {
	now := time.UnixMilli(9000)
	running := typedCompact(api.ChatCompaction{State: api.CompactionRunning})
	if got, want := rowKinds(chatRows(running, claudeKind, true, now)), []string{"user", "agent", "user", "live:compacting"}; !slices.Equal(got, want) {
		t.Errorf("a compaction running draws %q, want %q", got, want)
	}
	done := typedCompact(api.ChatCompaction{State: api.CompactionDone, Before: 182000, After: 9000})
	if got, want := rowKinds(chatRows(done, claudeKind, false, now)), []string{"user", "agent", "user", "compaction"}; !slices.Equal(got, want) {
		t.Errorf("a compaction done draws %q, want %q", got, want)
	}
	failed := typedCompact(api.ChatCompaction{State: api.CompactionFailed, Error: "Not enough messages to compact."})
	if got, want := rowKinds(chatRows(failed, claudeKind, false, now)), []string{"user", "agent", "user", "compaction"}; !slices.Equal(got, want) {
		t.Errorf("a compaction failed draws %q, want %q", got, want)
	}
	cancelled := typedCompact(api.ChatCompaction{State: api.CompactionCancelled})
	if got, want := rowKinds(chatRows(cancelled, claudeKind, false, now)), []string{"user", "agent", "user", "done"}; !slices.Equal(got, want) {
		t.Errorf("a compaction cancelled draws %q, want %q", got, want)
	}
}

// An automatic compaction stands where it happened in its turn, which carries
// on and ends as any other: the live line goes back to the turn's verb.
func TestAnAutomaticCompactionStandsWhereItHappened(t *testing.T) {
	items := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "refactor the store"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, Text: "Reading every caller first."},
		compactItem("compaction-1", 1, api.ChatCompaction{State: api.CompactionRunning, Auto: true}),
	}
	if got := rowKinds(chatRows(items, claudeKind, true, time.UnixMilli(9000))); !slices.Equal(got, []string{"user", "agent", "live:compacting"}) {
		t.Errorf("mid-compaction the turn draws %q", got)
	}
	items[2].Compaction = &api.ChatCompaction{State: api.CompactionDone, Auto: true, Before: 196000, After: 11000}
	items = append(items, backend.ChatItem{ID: "a2", Kind: api.ItemAgent, Turn: 1, Text: "Carrying on with the callers."})
	if got := rowKinds(chatRows(items, claudeKind, true, time.UnixMilli(9000))); !slices.Equal(got, []string{"user", "agent", "compaction", "agent", "live"}) {
		t.Errorf("after it the turn draws %q", got)
	}
	if got := rowKinds(chatRows(items, claudeKind, false, time.UnixMilli(9000))); !slices.Equal(got, []string{"user", "agent", "compaction", "agent", "done"}) {
		t.Errorf("once over the turn draws %q", got)
	}
}

func TestACompactionsMarkerSaysWhatItLeft(t *testing.T) {
	for _, tc := range []struct {
		cc            api.ChatCompaction
		title, detail string
	}{
		{api.ChatCompaction{State: api.CompactionDone, Before: 182000, After: 9000}, "Conversation compacted", "182K → 9K tokens"},
		{api.ChatCompaction{State: api.CompactionDone, Auto: true, Before: 196000, After: 11500}, "Conversation compacted automatically", "196K → 11.5K tokens"},
		{api.ChatCompaction{State: api.CompactionDone, Before: 35043}, "Conversation compacted", ""},
		{api.ChatCompaction{State: api.CompactionFailed, Error: "Not enough messages to compact."}, "Couldn't compact", "Not enough messages to compact."},
		{api.ChatCompaction{State: api.CompactionFailed}, "Couldn't compact", ""},
	} {
		title, detail := chatCompactTitle(&tc.cc)
		if title != tc.title || detail != tc.detail {
			t.Errorf("%+v reads %q · %q, want %q · %q", tc.cc, title, detail, tc.title, tc.detail)
		}
	}
	defer i18n.SetCurrent(i18n.EN)
	i18n.SetCurrent(i18n.PtBR)
	if _, detail := chatCompactTitle(&api.ChatCompaction{State: api.CompactionDone, Before: 196000, After: 11500}); detail != "196K → 11,5K tokens" {
		t.Errorf("in Portuguese the tokens read %q", detail)
	}
}

// compactRig is the chat of s1 holding items, running or not.
func compactRig(t *testing.T, items []backend.ChatItem, running bool, light bool) *frameRig {
	t.Helper()
	u, r := envelopeRig(t, items)
	if light {
		r.theme.Palette, r.theme.Surface = ui.Light, ui.Light.Background
	}
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: running})
	r.frames(3)
	return r
}

// compactShot is compactRig on the clock its items were stamped by, its live
// line counting from the person's /compact. A screen on a real clock never
// settles: its times move.
func compactShot(t *testing.T, items []backend.ChatItem, running bool, light bool) *frameRig {
	t.Helper()
	r := compactRig(t, items, running, light)
	r.now = time.UnixMilli(compactNow)
	r.frames(3)
	return r
}

const compactSummary = "This session is being continued from a previous conversation. Summary: the parser now reads a trailing comma; its tests pass."

// The marker opens the summary on a click, and the turn ends in it: no Done
// line, none of the command's output.
func TestTheMarkerOpensTheSummary(t *testing.T) {
	r := compactRig(t, typedCompact(api.ChatCompaction{State: api.CompactionDone, Before: 182000, After: 9000,
		Summary: compactSummary, Hooks: "PreCompact [hook-a] completed successfully"}), false, false)
	if !r.showsAll("Conversation compacted", "· 182K → 9K tokens") {
		t.Fatalf("the marker is not drawn:\n%s", r.drawn())
	}
	if r.shows(compactSummary) || r.shows("Done") || r.shows("PreCompact [hook-a] completed successfully") {
		t.Fatalf("the summary, a Done line or the hooks are drawn at rest:\n%s", r.drawn())
	}
	r.clickLabel(t, "Conversation compacted")
	r.frames(maxSettleFrames)
	if !r.shows(compactSummary) {
		t.Errorf("a click on the marker did not open the summary:\n%s", r.drawn())
	}
}

// While it compacts, the live line says so in place of the turn's verb.
func TestTheLiveLineSaysItCompacts(t *testing.T) {
	r := compactRig(t, typedCompact(api.ChatCompaction{State: api.CompactionRunning}), true, false)
	if !r.shows("Compacting the conversation") {
		t.Fatalf("the live line does not say it compacts:\n%s", r.drawn())
	}
	verb := chatVerb("s1", 2)
	if strings.Contains(r.drawn(), verb) {
		t.Errorf("the live line still names the turn %q", verb)
	}
}

// TestCompactionShots writes the states the mockup drew, dark and light, where
// LICH_SHOT_DIR names a folder.
func TestCompactionShots(t *testing.T) {
	failed := typedCompact(api.ChatCompaction{State: api.CompactionFailed, Error: "Not enough messages to compact."})
	auto := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "refactor the store"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, Text: "Reading every caller first."},
		compactItem("compaction-1", 1, api.ChatCompaction{State: api.CompactionDone, Auto: true, Before: 196000, After: 11000}),
		{ID: "a2", Kind: api.ItemAgent, Turn: 1, Text: "Carrying on with the callers."},
	}
	// A conversation read back from a Claude Code transcript that compacted
	// twice, as the backend reads it: the person's /compact and its marker,
	// then an automatic one mid-turn.
	resumed := append(typedCompact(api.ChatCompaction{State: api.CompactionDone, Before: 35043, After: 13792, Summary: compactSummary}),
		backend.ChatItem{ID: "u3", Kind: api.ItemUser, Turn: 3, Text: "and now the writer"},
		backend.ChatItem{ID: "a3", Kind: api.ItemAgent, Turn: 3, Text: "Reading the writer's callers."},
		compactItem("compaction-2", 3, api.ChatCompaction{State: api.CompactionDone, Auto: true, Before: 196000, After: 11000}),
		backend.ChatItem{ID: "a4", Kind: api.ItemAgent, Turn: 3, Text: "The writer escapes commas too now."},
	)
	for _, light := range []bool{false, true} {
		scheme := map[bool]string{false: "dark", true: "light"}[light]
		writeShot(t, compactShot(t, typedCompact(api.ChatCompaction{State: api.CompactionRunning}), true, light), "shot-live-"+scheme+".png")
		writeShot(t, compactShot(t, typedCompact(api.ChatCompaction{State: api.CompactionDone, Before: 182000, After: 9000, Summary: compactSummary}), false, light), "shot-done-"+scheme+".png")
		writeShot(t, compactShot(t, failed, false, light), "shot-failed-"+scheme+".png")
		writeShot(t, compactShot(t, auto, true, light), "shot-auto-"+scheme+".png")
		writeShot(t, compactShot(t, resumed, false, light), "shot-resumed-"+scheme+".png")
	}
}
