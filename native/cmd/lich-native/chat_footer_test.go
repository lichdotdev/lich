package main

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"

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

// boxFoot is where the composer's Send or Stop stands: the right end of the
// box's foot, in the chat rigs' window.
var boxFoot = image.Rect(1000, 720, 1100, 770)

func TestTheComposerHoldsOnlyTheProvidersControlsInItsBox(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.chat.setOptions("s1", chatOptions())
	r.settle(t)
	plus := r.smallestClickIn(t, image.Rect(120, 720, 170, 770))
	model, mode := r.at(t, "Haiku 5.5 · Medium"), r.at(t, i18n.T("chat.choice.manual"))
	send := r.smallestClickIn(t, boxFoot)
	if !(plus.X < model.X && model.X < mode.X && mode.X < send.X) {
		t.Errorf("x of + %v, model %v, mode %v, Send %v: want them in that order", plus.X, model.X, mode.X, send.X)
	}
	for _, y := range []float32{model.Y, mode.Y, send.Y} {
		if y < plus.Y-8 || y > plus.Y+8 {
			t.Errorf("a control at y=%v, off the box foot's line (y=%v)", y, plus.Y)
		}
	}
	if ring := r.at(t, "42%"); ring.X <= mode.X || ring.X >= send.X || ring.Y < plus.Y-8 || ring.Y > plus.Y+8 {
		t.Errorf("the context ring at %v: want it on the box foot's line after the mode (x=%v) and before Send (x=%v)", ring, mode.X, send.X)
	}
	for _, label := range []string{"PR #7", "5h 28%", "10m", "opus 4.8"} {
		if r.shows(label) {
			t.Errorf("%q drawn in a chat: the footer's other items stay out of it", label)
		}
	}
	if r.shows(i18n.T("chat.composer.send")) {
		t.Error("Send carries its word: it is an icon with a tooltip")
	}
}

func TestSendAndStopAreIconsThatNameThemselvesInATooltip(t *testing.T) {
	u, r, f := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go"}})
	r.settle(t)
	hover := func(label string) {
		r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: r.smallestClickIn(t, boxFoot)})
		r.frame()
		r.now = r.now.Add(time.Second)
		r.frame()
		r.frames(3)
		if !r.shows(label) {
			t.Errorf("no tooltip %q over the button:\n%s", label, r.drawn())
		}
	}
	hover(i18n.T("chat.composer.send"))
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	r.frames(3)
	hover(i18n.T("chat.composer.stop"))
	r.click(t, r.smallestClickIn(t, boxFoot))
	f.waitCall(t, api.ChatCancel.Name)
}

func TestTheComposerGrowsFromThreeLinesToACeiling(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	height := func() int {
		for _, n := range r.nodes() {
			if b := n.Desc.Bounds; n.Desc.Gestures != 0 && b.Dx() == 960 && b.Min.Y > 200 {
				return b.Dy()
			}
		}
		t.Fatalf("no composer:\n%s", r.drawn())
		return 0
	}
	empty := height()
	if empty < 3*22+2*14+32 {
		t.Errorf("empty box %dpx tall, want three lines of text, its padding and the foot", empty)
	}
	p := u.chats.pane("s1")
	p.editor.SetText(strings.Repeat("line\n", 3))
	r.settle(t)
	grown := height()
	p.editor.SetText(strings.Repeat("line\n", 80))
	r.settle(t)
	if capped := height(); !(empty < grown && grown < capped && capped <= 192) {
		t.Errorf("heights %d empty, %d at 4 lines, %d at 80: want growth to the 192 ceiling", empty, grown, capped)
	}
}

func TestTheChangesItemStandsAtTheRightOfTheLineUnderTheBox(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	branch, changes := r.at(t, "main"), r.at(t, "2")
	if changes.X < float32(rigWindow.X/2) || branch.X > changes.X {
		t.Errorf("branch at x=%v, changes at x=%v: want the branch at the left, the changes at the right", branch.X, changes.X)
	}
	if d := changes.Y - branch.Y; d < -20 || d > 20 || changes.Y < float32(boxFoot.Max.Y) {
		t.Errorf("branch at y=%v, changes at y=%v: want them on the line under the box", branch.Y, changes.Y)
	}
	r.click(t, changes)
	if !u.footer.dock.showing(dockReview) {
		t.Error("the changes item did not open Review")
	}
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

func TestAChoiceSentByIdAloneIsNamedForTheIdsLichKnows(t *testing.T) {
	for id, want := range map[string]string{
		"default": i18n.T("chat.choice.manual"), "manual": i18n.T("chat.choice.manual"),
		"plan": i18n.T("chat.choice.plan"), "auto": i18n.T("chat.choice.auto"),
		"bypassPermissions": i18n.T("chat.choice.yolo"), "yolo": i18n.T("chat.choice.yolo"),
		":read-only": i18n.T("chat.choice.readOnly"), ":workspace": i18n.T("chat.choice.workspace"),
		":danger-full-access": i18n.T("chat.choice.yolo"), "somethingNew": "somethingNew",
	} {
		if got := choiceLabel(backend.ChatChoice{ID: id}); got != want {
			t.Errorf("%s is %q, want %q", id, got, want)
		}
	}
	if got := choiceLabel(backend.ChatChoice{ID: "default", Label: "Ask first"}); got != "Ask first" {
		t.Errorf("the provider's label lost to ours: %q", got)
	}
}

func TestAnOptionWithNoChoiceOffersNoChip(t *testing.T) {
	m := chatModel(api.ViewChat)
	m.chat.setOptions("s1", []backend.ChatOption{{ID: "model", Category: api.OptionModel, Current: "x"}})
	if got := m.chat.optionSlots("s1"); len(got) != 0 {
		t.Errorf("slots %v for an option with nothing to choose", got)
	}
}

func TestAProviderOffersAChipOnlyForTheCategoriesItHas(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	cases := []struct {
		name    string
		options []backend.ChatOption
		shown   []string
		absent  []string
	}{
		{"model and effort share a chip", chatOptions()[:2], []string{"Haiku 5.5 · Medium"}, []string{i18n.T("chat.choice.manual")}},
		{"a model alone", chatOptions()[:1], []string{"Haiku 5.5"}, []string{"Haiku 5.5 · Medium", "Medium"}},
		{"an effort alone", chatOptions()[1:2], []string{"Medium"}, []string{"Haiku 5.5"}},
		{"a mode alone", chatOptions()[2:], []string{i18n.T("chat.choice.manual")}, []string{"Haiku 5.5", "Medium"}},
	}
	for _, c := range cases {
		u.model.chat.setOptions("s1", c.options)
		r.settle(t)
		for _, label := range c.shown {
			if !r.shows(label) {
				t.Errorf("%s: no chip %q:\n%s", c.name, label, r.drawn())
			}
		}
		for _, label := range c.absent {
			if r.shows(label) {
				t.Errorf("%s: drew %q", c.name, label)
			}
		}
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
	r.frames(3)
	r.click(t, r.at(t, "Haiku 5.5 · Medium"))
	if r.shows("Opus 5.5") {
		t.Error("the menu opened during a turn")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 0 {
		t.Errorf("called %v", f.calls)
	}
}

func TestAnotherPaneOfALayoutHasTheProvidersChipsAndNotTheFootersAttachOrRing(t *testing.T) {
	in := busySession()
	in.sessionID = "s2"
	u, r, _ := chatFramesIn(t, api.ViewChat, rigWindow, in)
	u.model.chat.setOptions("s1", chatOptions())
	r.settle(t)
	if !r.shows("Haiku 5.5 · Medium") || !r.shows(i18n.T("chat.choice.manual")) {
		t.Fatalf("the pane lost the provider's chips:\n%s", r.drawn())
	}
	for _, label := range []string{"42%", "5h 28%", "10m"} {
		if r.shows(label) {
			t.Errorf("%q drawn in a pane the footer does not follow", label)
		}
	}
}

// Each pane of a layout has the line under its box for its own session,
// whether the footer follows it or not.
func TestEachPaneOfALayoutShowsItsOwnBranchAndChanges(t *testing.T) {
	in := busySession()
	in.sessionID = "s2" // the footer follows the other pane
	u, r, _ := chatFramesIn(t, api.ViewChat, rigWindow, in)
	u.model.projects[0].Path = "/home/me/other"
	u.model.mu.Lock()
	u.model.git["/home/me/other"] = backend.DiffStats{Branch: "feature-x", Files: 3}
	u.model.mu.Unlock()
	r.settle(t)
	for _, label := range []string{"feature-x", "3"} {
		if !r.shows(label) {
			t.Errorf("no %q under the pane's box, which is the session's own:\n%s", label, r.drawn())
		}
	}
	for _, label := range []string{"main", "2"} {
		if r.shows(label) {
			t.Errorf("%q is the followed session's, drawn in another pane", label)
		}
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

// pickEntry clicks the menu row whose text starts at label's glyph: a menu is
// one clickable, so the point at() finds for a row is the panel's centre.
func pickEntry(t *testing.T, r *frameRig, label string) {
	t.Helper()
	for _, n := range r.nodes() {
		if n.Desc.Label == label {
			min := n.Desc.Bounds.Min
			r.click(t, f32.Pt(float32(min.X+4), float32(min.Y+8)))
			return
		}
	}
	t.Fatalf("no menu row %q:\n%s", label, r.drawn())
}

func TestTheChangesItemIsThereOnlyWhenThereAreChanges(t *testing.T) {
	in := busySession()
	in.git.Files = 0
	_, r, _ := chatFramesIn(t, api.ViewChat, rigWindow, in)
	r.settle(t)
	if !r.shows("main") {
		t.Fatalf("the branch is gone with the changes:\n%s", r.drawn())
	}
	for _, label := range []string{"0", "+0", "-0", i18n.T("shell.footerBar.reviewChanges")} {
		if r.shows(label) {
			t.Errorf("%q drawn for a checkout with no changes", label)
		}
	}
	_, r, _ = chatFramesIn(t, api.ViewChat, rigWindow, busySession())
	r.settle(t)
	if !r.shows("2") {
		t.Errorf("no file count for a checkout with changes:\n%s", r.drawn())
	}
}

// The item is quiet: nothing under it at rest, the kit's hover fill under the
// pointer, and never the lit fill the footer's button wears while Review is up.
func TestTheChangesItemHasNoFillAtRestAndNeverTheLitOne(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	rest := r.drawn()
	u.footer.w("changes").pressed = true
	u.footer.dock.show(dockReview)
	r.settle(t)
	if r.drawn() != rest {
		t.Error("the item changed with Review open: it must not be drawn pressed")
	}
	r.hover(r.at(t, "2"))
	r.settle(t)
	if r.drawn() == rest {
		t.Error("the item has no hover fill")
	}
}

// The line under the box has chatCrumbsPad over it, between the box and the
// line, and under it, between the line and the bottom edge of the island, over
// the composer's own pad.
func TestTheLineUnderTheBoxHasSpace2OnBothSides(t *testing.T) {
	_, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	if chatCrumbsPad != 8 {
		t.Errorf("the room around the line is %v, want Space(2), 8dp", chatCrumbsPad)
	}
	top := labelTop(t, r, "main")
	boxBottom := -1
	for _, n := range r.nodes() {
		if b := n.Desc.Bounds; n.Desc.Gestures != 0 && b.Dx() == 960 && b.Min.Y > 200 {
			boxBottom = b.Max.Y
		}
	}
	if boxBottom < 0 {
		t.Fatalf("no composer box:\n%s", r.drawn())
	}
	const lineHeight = 16 // a TextXS line
	if got := top - boxBottom; got != int(chatCrumbsPad) {
		t.Errorf("the line starts %dpx under the box, want %v", got, chatCrumbsPad)
	}
	if got, want := rigWindow.Y-top, lineHeight+int(chatCrumbsPad)+int(chatComposerPad); got != want {
		t.Errorf("the line's top is %dpx from the bottom edge, want its %dpx height, %v under it and the composer's %v pad", got, lineHeight, chatCrumbsPad, chatComposerPad)
	}
}

// The hover fill of the changes item stands chatChangesPad round its text, and
// takes none of the line: the text keeps the place it has chatFootPad in from the
// box's edge, as the left side's does, and the fill reaches the edge.
func TestTheChangesItemsFillGrowsOutwardFromItsText(t *testing.T) {
	_, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	if chatChangesPadX != 6 || chatChangesPadY != 4 {
		t.Errorf("padding %v by %v, want Space(1.5) by Space(1)", chatChangesPadX, chatChangesPadY)
	}
	var box, item image.Rectangle
	label := image.Pt(labelLeft(t, r, "2"), labelTop(t, r, "2")+4)
	for _, n := range r.nodes() {
		b := n.Desc.Bounds
		switch {
		case n.Desc.Gestures != 0 && b.Dx() == 960 && b.Min.Y > 200:
			box = b
		case n.Desc.Gestures != 0 && label.In(b) && b.Dy() < 60 && (item.Empty() || b.Dx() > item.Dx()):
			item = b
		}
	}
	if box.Empty() || item.Empty() {
		t.Fatalf("no box or item (%v, %v):\n%s", box, item, r.drawn())
	}
	if item.Dy() != 16+2*int(chatChangesPadY) {
		t.Errorf("the item is %dpx tall, want the line's 16 and %vpx round it", item.Dy(), chatChangesPadY)
	}
	if item.Max.X != box.Max.X {
		t.Errorf("the item ends at x=%d, the box at x=%d: the fill should reach the edge, the text %v in", item.Max.X, box.Max.X, chatFootPad)
	}
	if text := labelTop(t, r, "2"); text-item.Min.Y != int(chatChangesPadY) {
		t.Errorf("the text is %dpx under the fill's top, want %v", text-item.Min.Y, chatChangesPadY)
	}
}
