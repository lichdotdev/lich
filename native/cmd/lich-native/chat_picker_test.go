package main

import (
	"fmt"
	"strings"
	"testing"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// manyModels is a provider with eight models, the second the one in force, and
// the three effort levels of chatOptions.
func manyModels() []backend.ChatOption {
	var choices []backend.ChatChoice
	for i := range 8 {
		choices = append(choices, backend.ChatChoice{ID: fmt.Sprintf("m%d", i), Label: fmt.Sprintf("Model %d", i+1)})
	}
	choices[0].ID, choices[0].Label = "default", "Smart pick"
	return []backend.ChatOption{
		{ID: "model", Category: api.OptionModel, Current: "m1", Choices: choices},
		{ID: "effort", Category: api.OptionEffort, Current: "medium", Choices: []backend.ChatChoice{{ID: "low", Label: "Low"}, {ID: "medium", Label: "Medium"}, {ID: "high", Label: "High"}}},
	}
}

func openPicker(t *testing.T, options []backend.ChatOption) (*window, *frameRig, *chatBackend) {
	t.Helper()
	u, r, f := chatFrames(t, api.ViewChat)
	u.model.chat.setOptions("s1", options)
	r.settle(t)
	label := "Model 2 · Medium"
	r.clickLabel(t, label)
	r.settle(t)
	return u, r, f
}

func TestPickerListsTheModelsThatHoldTheSearch(t *testing.T) {
	choices := []backend.ChatChoice{{ID: "a", Label: "Opus 5.5"}, {ID: "b", Label: "Sonnet 5.5"}, {ID: "c", Label: "Opus 5"}}
	if got, folded := pickerVisible(choices, "OPUS", false); len(got) != 2 || got[0] != 0 || got[1] != 2 || folded {
		t.Errorf("visible %v folded %v for a search of Opus, want the two Opus models", got, folded)
	}
	if got, _ := pickerVisible(choices, "  ", false); len(got) != 3 {
		t.Errorf("a blank search shows %v, want every model", got)
	}
	if got, _ := pickerVisible(choices, "zzz", false); len(got) != 0 {
		t.Errorf("a search nothing matches shows %v", got)
	}
}

func TestPickerFoldsPastSixModelsAndSearchingUnfoldsThem(t *testing.T) {
	var choices []backend.ChatChoice
	for i := range 8 {
		choices = append(choices, backend.ChatChoice{ID: fmt.Sprint(i), Label: fmt.Sprintf("Model %d", i)})
	}
	if got, folded := pickerVisible(choices, "", false); len(got) != 6 || !folded {
		t.Errorf("%d shown, folded %v, want six and a Show more", len(got), folded)
	}
	if got, folded := pickerVisible(choices, "", true); len(got) != 8 || folded {
		t.Errorf("%d shown, folded %v after Show more, want all eight", len(got), folded)
	}
	if got, folded := pickerVisible(choices, "model", false); len(got) != 8 || folded {
		t.Errorf("%d shown, folded %v while searching, want every match", len(got), folded)
	}
	if got, folded := pickerVisible(choices[:6], "", false); len(got) != 6 || folded {
		t.Errorf("a list of six folded: %d shown, folded %v", len(got), folded)
	}
}

func TestEffortBarsRiseWithTheLevel(t *testing.T) {
	for n, want := range map[int][]int{5: {1, 2, 3, 4, 5}, 3: {1, 3, 5}, 2: {1, 5}, 1: {5}} {
		for i, w := range want {
			if got := pickerLit(i, n); got != w {
				t.Errorf("level %d of %d lights %d bars, want %d", i, n, got, w)
			}
		}
	}
}

func TestOnlyTheProvidersDefaultByIdWithoutThatNameIsMarkedDefault(t *testing.T) {
	if !pickerDefault(backend.ChatChoice{ID: "default", Label: "Smart pick"}) {
		t.Error("the default model is not marked")
	}
	if pickerDefault(backend.ChatChoice{ID: "default", Label: "Default (recommended)"}) {
		t.Error("a name that says Default got a second mark")
	}
	if pickerDefault(backend.ChatChoice{ID: "opus", Label: "Opus"}) {
		t.Error("a model that is not the default is marked")
	}
}

func TestTheModelChipOpensOnePanelWithBothColumns(t *testing.T) {
	_, r, _ := openPicker(t, manyModels())
	for _, label := range []string{
		i18n.T("chat.model.search", i18n.Args{"provider": "claude"}), i18n.T("chat.option.effort"),
		"Model 1" + "", "Model 2", "Low", "High", "Ctrl+1", "Ctrl+4",
		i18n.T("chat.model.showMore"), i18n.T("chat.model.default"),
	} {
		if label == "Model 1" {
			label = "Smart pick"
		}
		if !r.shows(label) {
			t.Errorf("the panel lacks %q:\n%s", label, r.drawn())
		}
	}
	if r.shows("Ctrl+5") || r.shows("Model 7") {
		t.Error("the panel shows past four shortcuts or six models")
	}
}

func TestChoosingAModelKeepsThePanelOpenForTheEffort(t *testing.T) {
	_, r, f := openPicker(t, manyModels())
	r.clickLabel(t, "Model 3")
	body := f.waitCall(t, api.ChatSetOption.Name)
	if !strings.Contains(body, `"optionId":"model"`) || !strings.Contains(body, `"value":"m2"`) {
		t.Errorf("set %s", body)
	}
	r.settle(t)
	if !r.shows(i18n.T("chat.option.effort")) {
		t.Error("the panel closed on a model")
	}
}

func TestChoosingAnEffortSetsItAndClosesThePanel(t *testing.T) {
	_, r, f := openPicker(t, manyModels())
	r.clickLabel(t, "High")
	body := f.waitCall(t, api.ChatSetOption.Name)
	if !strings.Contains(body, `"optionId":"effort"`) || !strings.Contains(body, `"value":"high"`) {
		t.Errorf("set %s", body)
	}
	r.settle(t)
	if r.shows(i18n.T("chat.option.effort")) {
		t.Error("the panel stayed open after an effort")
	}
}

func TestTypingFiltersTheModelsAndHidesShowMore(t *testing.T) {
	u, r, _ := openPicker(t, manyModels())
	u.chats.pane("s1").picker.query.SetText("model 7")
	r.settle(t)
	if !r.shows("Model 7") || r.shows("Model 3") || r.shows(i18n.T("chat.model.showMore")) {
		t.Errorf("the search of Model 7 left:\n%s", r.drawn())
	}
}

func TestShowMoreOpensTheRestOfTheModels(t *testing.T) {
	_, r, _ := openPicker(t, manyModels())
	if r.shows("Model 8") {
		t.Fatal("the folded list shows its eighth model")
	}
	r.clickLabel(t, i18n.T("chat.model.showMore"))
	r.settle(t)
	if !r.shows("Model 8") || r.shows(i18n.T("chat.model.showMore")) {
		t.Errorf("Show more did not open the list:\n%s", r.drawn())
	}
}

func TestCtrlNumberTakesTheNthModelWithThePanelOpen(t *testing.T) {
	_, r, f := openPicker(t, manyModels())
	r.router.Queue(key.Event{Name: "3", Modifiers: key.ModShortcut, State: key.Press})
	r.frame()
	body := f.waitCall(t, api.ChatSetOption.Name)
	if !strings.Contains(body, `"optionId":"model"`) || !strings.Contains(body, `"value":"m2"`) {
		t.Errorf("Ctrl+3 set %s, want the third model", body)
	}
}

func TestArrowsAndEnterWalkAndTakeAModelAndEscapeCloses(t *testing.T) {
	_, r, f := openPicker(t, manyModels())
	// The cursor starts on the model in force, the second.
	r.press(key.NameDownArrow)
	r.press(key.NameReturn)
	body := f.waitCall(t, api.ChatSetOption.Name)
	if !strings.Contains(body, `"value":"m2"`) {
		t.Errorf("Down then Enter set %s, want the third model", body)
	}
	r.settle(t)
	r.press(key.NameEscape)
	r.settle(t)
	if r.shows(i18n.T("chat.option.effort")) {
		t.Error("Escape left the panel open")
	}
}

func TestAProviderWithNoEffortHasOneColumnAndNoEffortProviderNoSearch(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.chat.setOptions("s1", manyModels()[:1])
	r.settle(t)
	r.clickLabel(t, "Model 2")
	r.settle(t)
	if r.shows(i18n.T("chat.option.effort")) || !r.shows("Model 3") {
		t.Errorf("a model-only panel:\n%s", r.drawn())
	}
	u2, r2, _ := chatFrames(t, api.ViewChat)
	u2.model.chat.setOptions("s1", manyModels()[1:])
	r2.settle(t)
	r2.clickLabel(t, "Medium")
	r2.settle(t)
	if r2.shows(i18n.T("chat.model.search", i18n.Args{"provider": "claude"})) || !r2.shows("High") {
		t.Errorf("an effort-only panel:\n%s", r2.drawn())
	}
}
