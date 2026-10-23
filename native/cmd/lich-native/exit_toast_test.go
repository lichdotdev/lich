package main

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/gioterm"
	"github.com/lichdotdev/lich/native/ui"
)

const conversation = "ba9884b8-7877-4743-8ac3-b2fa0bac5b44"

func code(n int) sessionExit { return sessionExit{Code: &n} }

func toastButtons(t ui.Toast) (cancel, secondary, action string) {
	if t.Cancel != nil {
		cancel = t.Cancel.Label
	}
	if t.Secondary != nil {
		secondary = t.Secondary.Label
	}
	if t.Action != nil {
		action = t.Action.Label
	}
	return cancel, secondary, action
}

// exitToastOf builds sv's toast, recording what each button asks for.
func exitToastOf(sv sessionView, e sessionExit) (ui.Toast, *[]string) {
	var asked []string
	t := exitToast(sv, e, func(resume string) { asked = append(asked, "restart "+resume) }, func() { asked = append(asked, "close") })
	return t, &asked
}

func TestExitToastOffersTheCardsConversationBack(t *testing.T) {
	sv := sessionView{Session: backend.Session{ID: "s1", Label: "eager-pine", Kind: "claude", ProviderSessionID: conversation}, profileName: "Sankhya"}
	toast, asked := exitToastOf(sv, code(1))
	if toast.ID != "exit:s1" || toast.Variant != ui.ToastError || toast.Duration != ui.ToastForever {
		t.Fatalf("toast %q variant %v duration %v, want exit:s1, error, forever", toast.ID, toast.Variant, toast.Duration)
	}
	if toast.Title != "eager-pine ended with code 1" || toast.Description != "Resume opens ba9884b8… again in Sankhya." {
		t.Fatalf("title %q description %q", toast.Title, toast.Description)
	}
	if c, s, a := toastButtons(toast); c != "Close session" || s != "Start new" || a != "Resume" {
		t.Fatalf("buttons %q %q %q, want Close session, Start new, Resume", c, s, a)
	}
	toast.Action.OnClick()
	toast.Secondary.OnClick()
	toast.Cancel.OnClick()
	want := []string{"restart " + conversation, "restart ", "close"}
	if strings.Join(*asked, "|") != strings.Join(want, "|") {
		t.Fatalf("asked %q, want %q", *asked, want)
	}
}

func TestExitToastOnTheDefaultProfileNamesNoProfile(t *testing.T) {
	sv := sessionView{Session: backend.Session{ID: "s1", Label: "Erro Cassul", Kind: "codex", ProviderSessionID: conversation}}
	toast, _ := exitToastOf(sv, code(0))
	if toast.Variant != ui.ToastInfo || toast.Title != "Erro Cassul ended" || toast.Description != "Resume opens ba9884b8… again." {
		t.Fatalf("variant %v title %q description %q", toast.Variant, toast.Title, toast.Description)
	}
}

func TestExitToastWithoutAConversationOffersRestart(t *testing.T) {
	for _, s := range []backend.Session{
		{ID: "s1", Label: "Shell 78", Kind: "shell", ProviderSessionID: conversation},
		{ID: "s1", Label: "Shell 78", Kind: "claude"},
	} {
		toast, asked := exitToastOf(sessionView{Session: s}, sessionExit{})
		if toast.Variant != ui.ToastError || toast.Description != "" {
			t.Errorf("%s: variant %v description %q, want error and none", s.Kind, toast.Variant, toast.Description)
		}
		if c, sec, a := toastButtons(toast); c != "Close session" || sec != "" || a != "Restart" {
			t.Fatalf("%s: buttons %q %q %q, want Close session, Restart", s.Kind, c, sec, a)
		}
		toast.Action.OnClick()
		if len(*asked) != 1 || (*asked)[0] != "restart " {
			t.Errorf("%s: asked %q, want a restart into a new conversation", s.Kind, *asked)
		}
	}
}

func TestEveryResumableProviderOffersResume(t *testing.T) {
	for _, kind := range resumableKinds {
		sv := sessionView{Session: backend.Session{ID: "s1", Kind: kind, ProviderSessionID: conversation}}
		if _, _, a := toastButtons(exitToast(sv, code(0), func(string) {}, func() {})); a != "Resume" {
			t.Errorf("%s: action %q, want Resume", kind, a)
		}
	}
}

func exitedWindow(t *testing.T) *window {
	t.Helper()
	u := &window{model: newModel(nil, func() {}), terms: &terminals{byID: map[string]*termView{}, invalidate: func() {}}}
	u.model.projects = []backend.Project{{ID: "p", Sessions: []backend.Session{{ID: "s1", Label: "Shell 78", Kind: "shell"}}}}
	vt, err := gioterm.New(80, 24, 10, 20, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	u.terms.byID["s1"] = &termView{id: "s1", vt: vt}
	return u
}

func TestDismissedExitToastComesBackWhenTheCardIsFocused(t *testing.T) {
	u := exitedWindow(t)
	u.syncExitToasts("s1")
	u.sessionExited("s1", code(1))
	if !u.duplicates.Live("exit:s1") {
		t.Fatal("no toast once the process exited")
	}
	u.duplicates.Dismiss("exit:s1")
	u.syncExitToasts("s1")
	if u.duplicates.Live("exit:s1") {
		t.Fatal("the dismissed toast came back with no change of focus")
	}
	u.syncExitToasts("")
	u.syncExitToasts("s1")
	if !u.duplicates.Live("exit:s1") {
		t.Fatal("focusing the exited card again did not bring its toast back")
	}
}

func TestExitToastGoesOnceTheCardRunsAgain(t *testing.T) {
	u := exitedWindow(t)
	u.sessionExited("s1", code(1))
	u.terms.running("s1")
	u.syncExitToasts("s1")
	if u.duplicates.Live("exit:s1") {
		t.Fatal("the toast stayed over a card whose process runs again")
	}
	u.sessionExited("s1", code(1))
	u.terms.drop("s1")
	u.syncExitToasts("s1")
	if u.duplicates.Live("exit:s1") {
		t.Fatal("the toast stayed over a card whose terminal is gone")
	}
}

func TestRestartStartsTheConversationAsked(t *testing.T) {
	for _, resume := range []string{conversation, ""} {
		u, b := newGateWindow(t)
		vt, err := gioterm.New(80, 24, 10, 20, func([]byte) {})
		if err != nil {
			t.Fatal(err)
		}
		u.terms.invalidate = func() {}
		u.terms.byID["s1"] = &termView{id: "s1", vt: vt}
		u.terms.exited("s1", code(1))
		sv := sessionView{Session: backend.Session{ID: "s1", Kind: "claude", ProviderSessionID: conversation}, projectID: "p", path: "/repo"}
		if !u.terms.restart(u.ctx, sv, resume) {
			t.Fatal("restart reported a failure")
		}
		if args := b.await(t, "terminal.Start"); !strings.HasPrefix(args, `[{"id":"s1","projectId":"p","cwd":"/repo","kind":"claude","resume":"`+resume+`",`) {
			t.Errorf("Start %s, want resume %q", args, resume)
		}
		if _, exited := u.terms.exitOf("s1"); exited {
			t.Error("the terminal still shows the exit after a restart")
		}
	}
}

func TestAReportedConversationReachesTheCard(t *testing.T) {
	m := newModel(nil, func() {})
	m.projects = []backend.Project{{ID: "p", Sessions: []backend.Session{{ID: "s1", Kind: "claude"}}}}
	send(m, api.SessionConversationEvent.Name, backend.ConversationReported{ID: "s1", ProviderSessionID: conversation})
	if sv, ok := m.sessionViewOf("s1"); !ok || sv.ProviderSessionID != conversation {
		t.Fatalf("card names %q, want the reported conversation", sv.ProviderSessionID)
	}
}
