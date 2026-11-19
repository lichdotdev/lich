package main

import (
	"image"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

func TestAChatCarriesNoBalloonsAndNoBand(t *testing.T) {
	u := &window{settings: new(settingsScreen)}
	chat := sessionView{ID: "a", Kind: "claude",
		View: api.ViewChat}
	terminal := sessionView{ID: "t", Kind: "claude"}
	cases := []struct {
		name string
		v    stageView
		want int
	}{
		{"a chat", stageView{active: chat, hasActive: true}, 0},
		{"a layout whose focus is a chat", stageView{active: chat, hasActive: true, hasLayout: true, panes: map[string]sessionView{"a": chat, "t": terminal}}, 0},
		{"a layout whose focus is a terminal", stageView{active: terminal, hasActive: true, hasLayout: true, panes: map[string]sessionView{"a": chat, "t": terminal}}, 52},
		{"the same agent as a terminal", stageView{active: terminal, hasActive: true}, 52},
	}
	for _, c := range cases {
		if got := u.ptyBand(pxGtx(), c.v); got != c.want {
			t.Errorf("%s: band %d, want %d", c.name, got, c.want)
		}
		if got := u.balloonsShown(c.v); got != (c.want > 0) {
			t.Errorf("%s: balloons shown %v with a band of %d", c.name, got, c.want)
		}
	}
}

// y of a control's centre, for checking which line it stands on.
func centreY(t *testing.T, r *frameRig, label string) float32 {
	t.Helper()
	return r.at(t, label).Y
}

func TestTheComposerCarriesTheFootersChipsAndSendOnOneLine(t *testing.T) {
	_, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	line := centreY(t, r, "PR #7")
	prev := r.at(t, "PR #7").X
	for _, label := range []string{"42%", "5h 28%", "10m", i18n.T("chat.composer.send")} {
		p := r.at(t, label)
		if p.Y < line-8 || p.Y > line+8 {
			t.Errorf("%q at y=%v, want the chips' line (y=%v)", label, p.Y, line)
		}
		if p.X <= prev {
			t.Errorf("%q at x=%v, want it after the item before it (x=%v): the left side, the right side, then Send", label, p.X, prev)
		}
		prev = p.X
	}
	if branch := centreY(t, r, "main"); branch <= line+8 {
		t.Errorf("the branch at y=%v, want it on its own line under the box (chips at y=%v)", branch, line)
	}
	if r.shows("Settings") {
		t.Error("the composer drew something past the footer's items")
	}
}

func labelLeft(t *testing.T, r *frameRig, label string) int {
	t.Helper()
	for _, n := range r.nodes() {
		if n.Desc.Label == label {
			return n.Desc.Bounds.Min.X
		}
	}
	return -1
}

func TestTheBranchUnderTheComposerOpensTheGitPanel(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	if u.footer.git.open {
		t.Fatal("the Git panel starts open")
	}
	r.click(t, r.at(t, "main"))
	if !u.footer.git.open {
		t.Error("the branch did not open the Git panel")
	}
}

func TestTheProjectLeadsTheBranchUnderTheComposer(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.projects[0].Name = "lich"
	r.settle(t)
	if !r.shows("lich") || !r.shows("/") {
		t.Fatalf("no project and slash before the branch:\n%s", r.drawn())
	}
	if project, branch := labelTop(t, r, "lich"), labelTop(t, r, "main"); project != branch {
		t.Errorf("project's top at y=%d, branch's at y=%d: want them on one line", project, branch)
	}
	if project, branch := labelLeft(t, r, "lich"), labelLeft(t, r, "main"); project >= branch {
		t.Errorf("project at x=%d, branch at x=%d: want the project first", project, branch)
	}
}

func TestAStopTakesSendsPlaceWhileTheAgentWorks(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	send := r.at(t, i18n.T("chat.composer.send"))
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	r.settle(t)
	stop := r.at(t, i18n.T("chat.composer.stop"))
	if r.shows(i18n.T("chat.composer.send")) {
		t.Error("Send stayed while the agent worked")
	}
	if d := stop.Y - send.Y; d < -8 || d > 8 || stop.X < send.X-40 {
		t.Errorf("Stop at %v, Send was at %v: want it in the same corner of the box", stop, send)
	}
}

func TestANarrowComposerLeavesOutWhatDoesNotFitRatherThanWrap(t *testing.T) {
	_, r, _ := chatFramesIn(t, api.ViewChat, image.Pt(420, 800), busySession())
	r.settle(t)
	if !r.shows(i18n.T("chat.composer.send")) || !r.shows("PR #7") {
		t.Fatalf("the narrow composer lost Send or the first chips:\n%s", r.drawn())
	}
	if r.shows("10m") {
		t.Errorf("the last chip fits where it should have been cut:\n%s", r.drawn())
	}
	line := centreY(t, r, "PR #7")
	for _, label := range []string{i18n.T("chat.composer.send"), "42%"} {
		if !r.shows(label) {
			continue
		}
		if y := centreY(t, r, label); y < line-8 || y > line+8 {
			t.Errorf("%q wrapped to y=%v, the chips' line is y=%v", label, y, line)
		}
	}
}

func TestAnotherPaneOfALayoutHasNoFooterItems(t *testing.T) {
	in := busySession()
	in.sessionID = "s2"
	_, r, _ := chatFramesIn(t, api.ViewChat, rigWindow, in)
	r.settle(t)
	if !r.shows(i18n.T("chat.composer.send")) {
		t.Fatal("the pane lost Send")
	}
	for _, label := range []string{"PR #7", "42%", "main"} {
		if r.shows(label) {
			t.Errorf("%q drawn in a pane the footer does not follow", label)
		}
	}
}

func chatOptions() []backend.ChatOption {
	return []backend.ChatOption{
		{ID: "model", Category: api.OptionModel, Current: "haiku", Choices: []backend.ChatChoice{{ID: "haiku", Label: "Haiku 5.5"}, {ID: "opus", Label: "Opus 5.5"}}},
		{ID: "effort", Category: api.OptionEffort, Current: "medium", Choices: []backend.ChatChoice{{ID: "low", Label: "Low"}, {ID: "medium", Label: "Medium"}}},
		{ID: "mode", Category: api.OptionMode, Current: "default", Choices: []backend.ChatChoice{
			{ID: "default"}, {ID: "plan", Label: "Plan mode"}, {ID: "bypassPermissions", Danger: true},
		}},
	}
}

func TestTheOptionsEventReplacesASessionsOptions(t *testing.T) {
	m := chatModel(api.ViewChat)
	m.chatEvent(t.Context(), chatEventOf(t, api.ChatOptionsEvent, api.ChatOptionsUpdate{SessionID: "s1", Options: chatOptions()}))
	if o, ok := m.chat.optionOf("s1", api.OptionEffort); !ok || o.Current != "medium" {
		t.Fatalf("effort = %+v, %v", o, ok)
	}
	m.chatEvent(t.Context(), chatEventOf(t, api.ChatOptionsEvent, api.ChatOptionsUpdate{SessionID: "s1", Options: chatOptions()[:1]}))
	if _, ok := m.chat.optionOf("s1", api.OptionEffort); ok {
		t.Error("an option the new list leaves out stayed")
	}
	if _, ok := m.chat.optionOf("s2", api.OptionModel); ok {
		t.Error("another session read the options")
	}
}

func TestAnOptionWithNoChoiceOffersNoChip(t *testing.T) {
	m := chatModel(api.ViewChat)
	m.chat.setOptions("s1", []backend.ChatOption{{ID: "model", Category: api.OptionModel, Current: "x"}})
	if got := m.chat.optionSlots("s1"); len(got) != 0 {
		t.Errorf("slots %v for an option with nothing to choose", got)
	}
}

func TestTheComposerShowsAChipForEachOptionTheProviderOffers(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.chat.setOptions("s1", chatOptions()[:2])
	r.settle(t)
	for _, label := range []string{"Haiku 5.5", "Medium"} {
		if !r.shows(label) {
			t.Errorf("no chip for %q:\n%s", label, r.drawn())
		}
	}
	if r.shows(i18n.T("chat.choice.manual")) {
		t.Error("a provider with no mode drew a mode chip")
	}
	if r.shows("opus 4.8") {
		t.Error("the model reading stayed beside the model chip")
	}
	u.model.chat.setOptions("s1", chatOptions()[1:])
	r.settle(t)
	if r.shows("Haiku 5.5") || !r.shows("opus 4.8") {
		t.Errorf("a provider with no model chip lost the model reading:\n%s", r.drawn())
	}
}

func TestAnOptionChipOpensItsMenuAndChoosingSetsIt(t *testing.T) {
	u, r, f := chatFrames(t, api.ViewChat)
	u.model.chat.setOptions("s1", chatOptions())
	r.settle(t)
	r.clickLabel(t, "Haiku 5.5")
	if !r.shows("Opus 5.5") {
		t.Fatalf("the model menu did not open:\n%s", r.drawn())
	}
	r.clickLabel(t, "Opus 5.5")
	body := f.waitCall(t, api.ChatSetOption.Name)
	if !strings.Contains(body, `"optionId":"model"`) || !strings.Contains(body, `"value":"opus"`) || !strings.Contains(body, `"id":"s1"`) {
		t.Errorf("set %s", body)
	}
}

func TestADangerousModeShowsAsYoloAndIsMarkedInTheMenu(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	opts := chatOptions()
	opts[2].Current = "bypassPermissions"
	u.model.chat.setOptions("s1", opts)
	r.settle(t)
	if !r.shows(i18n.T("chat.choice.yolo")) {
		t.Fatalf("the active dangerous mode is not named:\n%s", r.drawn())
	}
	r.clickLabel(t, i18n.T("chat.choice.yolo"))
	if !r.shows("Plan mode") || !r.shows(i18n.T("chat.choice.manual")) {
		t.Errorf("the mode menu lost a choice:\n%s", r.drawn())
	}
}

func TestTheOptionChipsTakeNoClickWhileTheAgentWorks(t *testing.T) {
	u, r, f := chatFrames(t, api.ViewChat)
	u.model.chat.setOptions("s1", chatOptions())
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	r.settle(t)
	r.clickLabel(t, "Haiku 5.5")
	if r.shows("Opus 5.5") {
		t.Error("the menu opened during a turn")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 0 {
		t.Errorf("called %v", f.calls)
	}
}

func TestAChoiceSentByIdAloneIsNamedForTheIdsLichKnows(t *testing.T) {
	for id, want := range map[string]string{
		"default": i18n.T("chat.choice.manual"), "manual": i18n.T("chat.choice.manual"),
		"plan": i18n.T("chat.choice.plan"), "auto": i18n.T("chat.choice.auto"),
		"bypassPermissions": i18n.T("chat.choice.yolo"), "yolo": i18n.T("chat.choice.yolo"),
		"somethingNew": "somethingNew",
	} {
		if got := choiceLabel(backend.ChatChoice{ID: id}); got != want {
			t.Errorf("%s is %q, want %q", id, got, want)
		}
	}
	if got := choiceLabel(backend.ChatChoice{ID: "default", Label: "Ask first"}); got != "Ask first" {
		t.Errorf("the provider's label lost to ours: %q", got)
	}
}
