package browser

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

var confirmDelete = api.BrowserDialog{Type: "confirm", Message: "Delete?", URL: "https://example.com/"}

func TestAClickThatOpensADialogAnswersWithTheDialogAndTellsTheWindow(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.onClick = func() { p.openDialog(confirmDelete) }
	info, err := s.ClickAt(t.Context(), api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#del"}})
	if err != nil {
		t.Fatal(err)
	}
	if info.Dialog == nil || *info.Dialog != confirmDelete || info.URL != "https://example.com" {
		t.Fatalf("click answered %+v, want the page with the dialog it opened", info)
	}
	states := out.states()
	if last := states[len(states)-1]; last.Dialog == nil || *last.Dialog != confirmDelete {
		t.Fatalf("the window last heard %+v, want the open dialog", last)
	}
}

func TestACallOnAPageHoldingADialogSaysToAnswerIt(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.openDialog(confirmDelete)
	p.shotErr = errDialogOpen
	_, err := s.Screenshot("s", "")
	if !errors.Is(err, errDialogOpen) || !strings.Contains(err.Error(), "browser_dialog") || !strings.Contains(err.Error(), `"Delete?"`) {
		t.Fatalf("err = %v, want the dialog named and browser_dialog", err)
	}
	if _, err := s.live("s"); err != nil {
		t.Fatalf("the browser was dropped: %v", err)
	}
}

func TestTheAgentAnswersAPromptWithItsDefaultText(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.openDialog(api.BrowserDialog{Type: "prompt", Message: "Name?", Default: "Ada"})
	info, err := s.AnswerDialog(t.Context(), api.BrowserDialogOptions{ID: "s", Accept: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := []answer{{accept: true, text: "Ada"}}; !reflect.DeepEqual(p.answers, want) {
		t.Fatalf("answers = %+v, want %+v", p.answers, want)
	}
	if info.Dialog != nil || len(info.Dialogs) != 0 {
		t.Fatalf("the page after the answer still names %+v %+v", info.Dialog, info.Dialogs)
	}
	acts := out.named(api.BrowserAgentEvent.Name)
	if len(acts) == 0 || acts[0].(api.BrowserAgentAct).Action != "accept" {
		t.Fatalf("the window heard %+v, want the agent's accept", acts)
	}
	if last := out.states()[len(out.states())-1]; last.Dialog != nil {
		t.Fatalf("the window last heard %+v, want the dialog gone", last)
	}
}

func TestTheAgentsTextAnswersAPrompt(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.openDialog(api.BrowserDialog{Type: "prompt", Message: "Name?", Default: "Ada"})
	empty := ""
	if _, err := s.AnswerDialog(t.Context(), api.BrowserDialogOptions{ID: "s", Accept: true, Text: &empty}); err != nil {
		t.Fatal(err)
	}
	if want := []answer{{accept: true, text: ""}}; !reflect.DeepEqual(p.answers, want) {
		t.Fatalf("answers = %+v, want %+v", p.answers, want)
	}
}

func TestTheAgentReadsOnceTheDialogThePersonAnswered(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.openDialog(confirmDelete)
	if _, err := s.Answer(t.Context(), api.BrowserDialogOptions{ID: "s"}); err != nil {
		t.Fatal(err)
	}
	if acts := out.named(api.BrowserAgentEvent.Name); len(acts) != 0 {
		t.Fatalf("the person's answer played as the agent's: %+v", acts)
	}
	read, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	want := confirmDelete
	want.Answer = api.DialogDismissed
	if !reflect.DeepEqual(read.Dialogs, []api.BrowserDialog{want}) {
		t.Fatalf("read reported %+v, want the person's Cancel", read.Dialogs)
	}
	again, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Dialogs) != 0 {
		t.Fatalf("a second read repeated %+v", again.Dialogs)
	}
}

func TestAnsweringWithNoDialogOpenFails(t *testing.T) {
	s, _, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AnswerDialog(t.Context(), api.BrowserDialogOptions{ID: "s", Accept: true}); !errors.Is(err, errNoDialog) {
		t.Fatalf("agent: err = %v, want errNoDialog", err)
	}
	if _, err := s.Answer(t.Context(), api.BrowserDialogOptions{ID: "s"}); !errors.Is(err, errNoDialog) {
		t.Fatalf("person: err = %v, want errNoDialog", err)
	}
}

func TestReadKeepsTheFirstAnswersOfAPageThatAlertsInALoop(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	for range answeredMax + 5 {
		p.openDialog(api.BrowserDialog{Type: "alert", Message: "again"})
		if _, err := s.Answer(t.Context(), api.BrowserDialogOptions{ID: "s", Accept: true}); err != nil {
			t.Fatal(err)
		}
	}
	read, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Dialogs) != answeredMax {
		t.Fatalf("read reported %d answers, want %d", len(read.Dialogs), answeredMax)
	}
}

func TestTheGateIsHeldFromOpenToClose(t *testing.T) {
	var g dialogGate
	held := func() bool {
		select {
		case <-g.held():
			return true
		default:
			return false
		}
	}
	waiting := g.held()
	if held() {
		t.Fatal("held before any dialog")
	}
	g.open()
	g.open()
	select {
	case <-waiting:
	default:
		t.Fatal("a call waiting from before the dialog was not released")
	}
	g.close()
	g.close()
	if held() {
		t.Fatal("still held after the dialog closed")
	}
}
