package main

import (
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/grimoire"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The Welcome replaces the first-launch provider dialog and keeps its
// decision: the cases are that dialog's, under the screen's name.
func TestDecideWelcome(t *testing.T) {
	claude := providerInfo{launchProvider: launchProvider{id: "claude", enabled: true}}
	codex := providerInfo{launchProvider: launchProvider{id: "codex", installed: true}}
	cases := []struct {
		name      string
		m         providersMachine
		want      welcomeDecision
		preselect string
	}{
		{"detection not answered", providersMachine{}, welcomeWait, ""},
		{"default already chosen", providersMachine{loaded: true, list: []providerInfo{codex}, globals: map[string]string{providerDefaultKey: "codex"}}, welcomeSkip, ""},
		{"first installed is preselected", providersMachine{loaded: true, list: []providerInfo{claude, codex}, globals: map[string]string{}}, welcomeShow, "codex"},
		{"nothing installed", providersMachine{loaded: true, list: []providerInfo{claude}, globals: map[string]string{}}, welcomeShow, ""},
	}
	for _, c := range cases {
		got, preselect := decideWelcome(c.m)
		if got != c.want || preselect != c.preselect {
			t.Errorf("%s: got (%d, %q), want (%d, %q)", c.name, got, preselect, c.want, c.preselect)
		}
	}
}

func TestTomeStateReadsTheStores(t *testing.T) {
	cases := []struct {
		name  string
		facts tomeFacts
		want  tomeState
	}{
		{"absent", tomeFacts{}, tomeNotInstalled},
		{"installing", tomeFacts{install: installStep{running: true, progress: 0.4}}, tomeInstalling},
		{"install failed", tomeFacts{install: installStep{failure: "exit status 1"}}, tomeFailed},
		{"installed, needs no login", tomeFacts{installed: true}, tomeReady},
		{"installed, signed out", tomeFacts{installed: true, needsLogin: true, known: true}, tomeSignedOut},
		{"signing in", tomeFacts{installed: true, needsLogin: true, known: true, signingIn: true}, tomeSigningIn},
		{"signed in", tomeFacts{installed: true, needsLogin: true, known: true, loggedIn: true}, tomeReady},
		{"a failure under an installed provider is old", tomeFacts{installed: true, install: installStep{failure: "x"}}, tomeReady},
	}
	for _, c := range cases {
		if got := tomeStateOf(c.facts); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
}

func TestEachMovePlaysItsJourney(t *testing.T) {
	const id = "antigravity"
	cases := []struct {
		name     string
		from, to tomeState
		want     []grimoire.Grid
	}{
		{"installed, a login to go", tomeInstalling, tomeSignedOut, grimoire.Bind(id)},
		{"installed, no login", tomeInstalling, tomeReady, grimoire.BindAwake(id)},
		{"signed in", tomeSigningIn, tomeReady, grimoire.Unseal(id)},
		{"signed in outside", tomeSignedOut, tomeReady, grimoire.Unseal(id)},
		{"started installing", tomeNotInstalled, tomeInstalling, nil},
		{"failed", tomeInstalling, tomeFailed, nil},
		{"signing in", tomeSignedOut, tomeSigningIn, nil},
	}
	for _, c := range cases {
		got := journeyBetween(id, c.from, c.to)
		if len(got) != len(c.want) || (got != nil && !slices.EqualFunc(got[len(got)-1], c.want[len(c.want)-1], slices.Equal)) {
			t.Errorf("%s: %d frames, want %d", c.name, len(got), len(c.want))
		}
	}
}

func TestATomeOnlyAnimatesAMoveItSaw(t *testing.T) {
	const id = "claude"
	start := time.Unix(100, 0)
	var t1 tome
	t1.observe(id, tomeReady, true, start)
	if _, _, playing := t1.frame(start); playing {
		t.Fatal("the first look played a journey")
	}
	var t2 tome
	t2.observe(id, tomeSignedOut, false, start)
	t2.observe(id, tomeReady, true, start)
	if _, _, playing := t2.frame(start); playing {
		t.Fatal("a login read for the first time played the clasps opening")
	}
	var t3 tome
	t3.observe(id, tomeSignedOut, true, start)
	t3.observe(id, tomeReady, true, start)
	frames := grimoire.Unseal(id)
	g, next, playing := t3.frame(start.Add(welcomeJourneyStep))
	if !playing || !slices.EqualFunc(g, frames[1], slices.Equal) || next != start.Add(2*welcomeJourneyStep) {
		t.Fatalf("one step in: playing %v, next %v", playing, next)
	}
	if _, _, playing := t3.frame(start.Add(time.Duration(len(frames)) * welcomeJourneyStep)); playing {
		t.Fatal("the journey played past its last frame")
	}
}

func TestParseWelcomeShot(t *testing.T) {
	shots, err := parseWelcomeShot("open,kiro=installing:0.6,codex=unseal:4,cursor=failed")
	if err != nil {
		t.Fatal(err)
	}
	if s := shots["kiro"]; s.state != tomeInstalling || s.progress != 0.6 {
		t.Errorf("kiro = %+v", s)
	}
	state, _, g, pinned := shots["codex"].apply("codex")
	if state != tomeReady || !pinned || !slices.EqualFunc(g, grimoire.Unseal("codex")[4], slices.Equal) {
		t.Errorf("codex: state %d, pinned %v", state, pinned)
	}
	if shots["cursor"].failure == "" {
		t.Error("a failed shot has no line")
	}
	for _, bad := range []string{"kiro", "kiro=nope", "kiro=bind:x"} {
		if _, err := parseWelcomeShot(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

func (f *sidebarBackend) called(call string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Contains(f.calls, call)
}

// welcomeRig is a window on a fake backend with the topbar's menu and the
// Welcome, and Claude Code's profiles read.
func welcomeRig(t *testing.T, detected []backend.DetectedProvider) (*window, *frameRig, *sidebarBackend) {
	t.Helper()
	r := newFrameRig(rigWindow, nil)
	f, client := newSidebarBackend(t)
	setDetected(f, detected)
	u := &window{ctx: t.Context(), client: client}
	u.model = newModel(client, r.invalidate)
	u.settings = newSettingsScreen(t.Context(), client, &u.root, r.invalidate)
	u.settings.providers.book = u.model.providers
	u.settings.providers.claude.bind(u.model.profiles, func(string) bool { return false })
	u.model.profiles.list, u.model.profiles.loaded = []backend.Profile{{Slug: personalProfile, Name: "Personal"}}, true
	if err := u.model.providers.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	u.topbar = newTopbar(&u.root)
	u.bindWelcome()
	r.draw = func(gtx C, th *ui.Theme) {
		u.root.Layout(gtx, func(gtx C) D {
			u.layoutWelcome(gtx, th)
			var act topbarAction
			u.topbar.menuButton(gtx, th, false, nil, &act)
			u.applyTopbar(act)
			return D{Size: gtx.Constraints.Max}
		})
	}
	return u, r, f
}

var installedClaude = []backend.DetectedProvider{
	{ID: "claude", Name: "Claude Code", Installed: true, Installable: true},
	{ID: "codex", Name: "Codex", Installable: true},
	{ID: "kiro", Name: "Kiro CLI", Docs: "https://kiro.dev/docs/cli/installation/"},
}

func TestTheFirstLaunchOpensTheWelcome(t *testing.T) {
	u, r, f := welcomeRig(t, installedClaude)
	r.frames(3)
	if !r.shows(i18n.T("shell.welcome.tagline")) {
		t.Fatalf("the Welcome did not open:\n%s", r.drawn())
	}
	for _, label := range []string{"Claude Code", "Codex", "Kiro CLI", i18n.T("shell.welcome.signedOut"), i18n.T("shell.welcome.install"), i18n.T("shell.welcome.installPage")} {
		if !r.shows(label) {
			t.Errorf("no %q drawn", label)
		}
	}
	r.click(t, r.at(t, i18n.T("shell.welcome.continue")))
	r.frames(2)
	if u.welcome.open || r.shows(i18n.T("shell.welcome.tagline")) {
		t.Fatal("Continue left the Welcome open")
	}
	if got := f.get(providerDefaultKey + "@"); got != "claude" {
		t.Errorf("the default stored is %q, want claude", got)
	}
}

func TestALaterLaunchOpensNoWelcome(t *testing.T) {
	u, r, f := welcomeRig(t, installedClaude)
	f.mu.Lock()
	f.settings[providerDefaultKey+"@"] = "claude"
	f.mu.Unlock()
	if err := u.model.providers.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	r.frames(3)
	if r.shows(i18n.T("shell.welcome.tagline")) {
		t.Fatal("the Welcome opened with a default chosen")
	}
}

func TestTheMenuReopensTheWelcome(t *testing.T) {
	u, r, f := welcomeRig(t, installedClaude)
	f.mu.Lock()
	f.settings[providerDefaultKey+"@"] = "codex"
	f.mu.Unlock()
	if err := u.model.providers.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	r.frames(2)
	// The menu's button is the one clickable while the Welcome is shut.
	r.click(t, r.smallestClickIn(t, image.Rectangle{Max: r.size}))
	r.frames(1)
	r.click(t, r.at(t, i18n.T("tabs.mainMenu.welcome")))
	r.frames(3)
	if !r.shows(i18n.T("shell.welcome.tagline")) {
		t.Fatalf("the menu's Welcome did not open it:\n%s", r.drawn())
	}
	r.click(t, r.at(t, i18n.T("shell.welcome.continue")))
	r.frames(2)
	if got := f.get(providerDefaultKey + "@"); got != "codex" {
		t.Errorf("closing the menu's Welcome moved the default to %q", got)
	}
}

func TestInstallRunsAndThenBindsTheBook(t *testing.T) {
	u, r, f := welcomeRig(t, installedClaude)
	r.frames(3)
	r.click(t, r.at(t, i18n.T("shell.welcome.install")))
	for deadline := time.Now().Add(5 * time.Second); !f.called(`providers.Install({"id":"codex"})`); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("no install asked for")
		}
	}
	r.frames(1)
	if !r.shows(i18n.T("shell.welcome.installing")) {
		t.Fatalf("no Installing drawn:\n%s", r.drawn())
	}
	u.model.installEvent(t.Context(), api.ProviderInstall{ID: "codex", Progress: 0.5})
	r.frames(1)
	setDetected(f, []backend.DetectedProvider{installedClaude[0], {ID: "codex", Name: "Codex", Installed: true, Installable: true}, installedClaude[2]})
	u.model.installEvent(t.Context(), api.ProviderInstall{ID: "codex", Progress: 1, Done: true})
	for deadline := time.Now().Add(5 * time.Second); u.model.installs.of("codex").running; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the install never finished in the store")
		}
	}
	r.frame()
	if u.welcome.tome("codex").journey == nil {
		t.Fatal("the install's end played no binding")
	}
	r.frames(5)
	if !r.shows(i18n.T("shell.welcome.ready")) && !r.shows(i18n.T("shell.welcome.signedOut")) {
		t.Errorf("codex is neither signed out nor ready after its binding:\n%s", r.drawn())
	}
	if f.get("provider.codex.enabled@") != "1" {
		t.Error("the installed provider was not switched on")
	}
}

func TestAFailedInstallShowsItsLineAndRetry(t *testing.T) {
	u, r, _ := welcomeRig(t, installedClaude)
	u.model.installs.take(api.ProviderInstall{ID: "codex", Done: true, Error: "installing Codex needs npm, which is not on PATH"})
	r.frames(3)
	for _, label := range []string{"installing Codex needs npm, which is not on PATH", i18n.T("shell.welcome.retry")} {
		if !r.shows(label) {
			t.Errorf("no %q drawn:\n%s", label, r.drawn())
		}
	}
}

// Every state and every key frame of the mockup's strip draws, each its own
// picture.
func TestEveryStateAndKeyFrameDraws(t *testing.T) {
	specs := []string{
		"codex=notinstalled", "codex=installing:0.6", "codex=failed", "codex=signedout",
		"codex=signingin", "codex=ready", "codex=bind:6", "codex=bind:10", "codex=unseal:4",
	}
	paints := map[string]string{}
	for _, spec := range specs {
		u, r, _ := welcomeRig(t, installedClaude)
		if err := u.welcomeShot(spec); err != nil {
			t.Fatal(err)
		}
		r.frames(3)
		drawn, _ := r.frame()
		paint := strings.SplitN(drawn, "\n", 2)[0]
		for other, p := range paints {
			if p == paint {
				t.Errorf("%s draws the same as %s", spec, other)
			}
		}
		paints[spec] = paint
	}
}
