package browser

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

func viewService(t *testing.T) (*Service, *fakeLauncher, *emitted) {
	t.Helper()
	launch := &fakeLauncher{}
	s := testService(t, launch)
	out := &emitted{}
	s.emit = out.emit
	return s, launch, out
}

func TestAgentOpenTellsTheWindowItWasTheAgent(t *testing.T) {
	s, _, out := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	states := out.states()
	if len(states) != 1 || !states[0].Open || !states[0].ByAgent || states[0].Owner != "s" {
		t.Fatalf("states = %+v, want one open state by the agent", states)
	}
}

func TestAttachWatchesTheAgentsPageWithoutRelaunching(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Open("s", "https://example.com/app"); err != nil {
		t.Fatal(err)
	}
	v := Viewport{W: 640, H: 480, Scale: 1.5}
	h, err := s.Attach("s", v)
	if err != nil {
		t.Fatal(err)
	}
	if len(launch.pages) != 1 {
		t.Fatalf("attach launched %d browsers, want the agent's one", len(launch.pages))
	}
	if h.URL != "https://example.com/app" {
		t.Fatalf("attached handle = %+v", h)
	}
	page := launch.pages[0]
	if !page.casting || page.viewports[0] != v {
		t.Fatalf("screencast not started at %+v: casting=%v viewports=%v", v, page.casting, page.viewports)
	}
	page.onFrame("anBlZw==")
	frames := out.named(api.BrowserFrameEvents.Of("s").Name)
	if len(frames) != 1 || frames[0] != "anBlZw==" {
		t.Fatalf("frames = %v", frames)
	}
}

func TestAttachWithoutABrowserStartsOneTheWindowOwns(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	if len(launch.pages) != 1 || launch.headed[0] {
		t.Fatalf("attach launched headed=%v, want one headless", launch.headed)
	}
	if st := out.states(); len(st) != 1 || st[0].ByAgent {
		t.Fatalf("states = %+v, want one opened by the window", st)
	}
}

func TestAttachRefusesABrowserInItsOwnWindow(t *testing.T) {
	s, _, _ := viewService(t)
	if _, err := s.OpenVisible("s"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != errInWindow {
		t.Fatalf("attach on a headed browser = %v, want errInWindow", err)
	}
}

func TestReaperLeavesAWatchedBrowserAloneUntilDetach(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	later := time.Date(2026, 8, 16, 13, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return later }
	s.Reap()
	if launch.pages[0].closed {
		t.Fatal("reaper closed a browser a pane is showing")
	}
	if err := s.Detach("s"); err != nil {
		t.Fatal(err)
	}
	if launch.pages[0].casting {
		t.Fatal("detach left the screencast running")
	}
	s.now = func() time.Time { return later.Add(time.Hour) }
	s.Reap()
	if running := s.Running(); len(running) != 1 || !running[0].Idle {
		t.Fatalf("Running = %+v, want an unwatched idle browser asked about", running)
	}
}

func TestPageNavigatingOnItsOwnUpdatesTheWindow(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	launch.pages[0].onInfo("https://example.com/next", "Next")
	states := out.states()
	last := states[len(states)-1]
	if last.URL != "https://example.com/next" || last.Title != "Next" || !last.Open {
		t.Fatalf("last state = %+v", last)
	}
	if run := s.Running(); len(run) != 1 || run[0].Title != "Next" {
		t.Fatalf("running = %+v", run)
	}
}

func TestCloseTellsTheWindow(t *testing.T) {
	s, _, out := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseOwnedBy("s"); err != nil {
		t.Fatal(err)
	}
	states := out.states()
	if last := states[len(states)-1]; last.Open || last.Owner != "s" {
		t.Fatalf("last state = %+v, want closed", last)
	}
	if run := s.Running(); len(run) != 0 {
		t.Fatalf("running after close = %+v", run)
	}
}

func TestAgentClickReportsWhereAndWindowInputDoesNot(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	launch.pages[0].clickAt = [2]float64{120, 48}
	sel := "#go"
	if err := s.Click("s", Target{Selector: sel}); err != nil {
		t.Fatal(err)
	}
	acts := out.named(api.BrowserAgentEvent.Name)
	if len(acts) != 1 {
		t.Fatalf("agent acts = %v", acts)
	}
	act := acts[0].(api.BrowserAgentAct)
	if act.X == nil || *act.X != 120 || *act.Y != 48 {
		t.Fatalf("agent click at %+v, want 120,48", act)
	}

	if err := s.Nav("s", "reload", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MouseInput(t.Context(), api.BrowserMouseOptions{ID: "s", Type: "press", X: 1, Y: 2, Button: "left"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.KeyInput(t.Context(), api.BrowserKeyOptions{ID: "s", Type: "down", Key: "Enter"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TextInput(t.Context(), api.BrowserTextOptions{ID: "s", Text: "ção"}); err != nil {
		t.Fatal(err)
	}
	if n := len(out.named(api.BrowserAgentEvent.Name)); n != 1 {
		t.Fatalf("window input lit the agent chip: %d agent acts", n)
	}
	p := launch.pages[0]
	if p.reloads != 1 || len(p.mice) != 1 || len(p.keys) != 1 || len(p.texts) != 1 || p.texts[0] != "ção" {
		t.Fatalf("page got reloads=%d mice=%v keys=%v texts=%v", p.reloads, p.mice, p.keys, p.texts)
	}
}

func TestNavRejectsBlockedURLAndUnknownAction(t *testing.T) {
	s, _, _ := viewService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.Nav("s", "go", "file:///etc/passwd"); err == nil {
		t.Fatal("nav loaded a file: URL")
	}
	if err := s.Nav("s", "jump", ""); err == nil {
		t.Fatal("nav accepted an unknown action")
	}
}

func TestResizeRestartsTheFramesOfAWatchedPage(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	bigger := Viewport{W: 1600, H: 900, Scale: 1}
	if err := s.Resize("s", bigger); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	if !p.casting || p.viewports[len(p.viewports)-1] != bigger {
		t.Fatalf("after resize casting=%v viewports=%v, want frames at %v", p.casting, p.viewports, bigger)
	}
}

func TestWindowInputThatTimesOutKeepsTheBrowser(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	launch.pages[0].mouseErr = context.DeadlineExceeded
	if _, err := s.MouseInput(t.Context(), api.BrowserMouseOptions{ID: "s", Type: "release", Button: "left"}); err == nil {
		t.Fatal("a mouse event Chrome never answered reported success")
	}
	if launch.pages[0].closed {
		t.Fatal("a page that missed one input event was closed")
	}
	if st := out.states(); !st[len(st)-1].Open {
		t.Fatalf("last state %+v, want the browser still open", st[len(st)-1])
	}
}

func TestAgentScrollTurnsWhereItLastActedAndSaysWhere(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	if err := s.Scroll("s", 300); err != nil {
		t.Fatal(err)
	}
	p.clickAt = [2]float64{120, 48}
	if err := s.Click("s", Target{Selector: "#go"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Scroll("s", -200); err != nil {
		t.Fatal(err)
	}
	if p.scrolledAt[0] != nil {
		t.Fatalf("first scroll turned at %+v, want the middle (nil) before any click", p.scrolledAt[0])
	}
	if at := p.scrolledAt[1]; at == nil || at.X != 120 || at.Y != 48 {
		t.Fatalf("scroll after a click turned at %+v, want the click's point", at)
	}
	var acts []api.BrowserAgentAct
	for _, d := range out.named(api.BrowserAgentEvent.Name) {
		acts = append(acts, d.(api.BrowserAgentAct))
	}
	first, last := acts[0], acts[len(acts)-1]
	if first.Action != "scroll" || first.DY != 300 || first.X == nil || *first.X != 400 {
		t.Fatalf("first scroll reported %+v, want scroll 300 at the middle", first)
	}
	if last.Action != "scroll" || last.DY != -200 || *last.X != 120 || *last.Y != 48 {
		t.Fatalf("second scroll reported %+v", last)
	}
}

func TestAgentTypeAndPressReportTheElementTheyClicked(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	launch.pages[0].clickAt = [2]float64{30, 40}
	if err := s.Type("s", "hi", false, Target{Selector: "#q"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Press("s", "Enter", Target{}); err != nil {
		t.Fatal(err)
	}
	acts := out.named(api.BrowserAgentEvent.Name)
	typed, pressed := acts[0].(api.BrowserAgentAct), acts[1].(api.BrowserAgentAct)
	if typed.Action != "type" || typed.X == nil || *typed.X != 30 || *typed.Y != 40 {
		t.Fatalf("type reported %+v, want the field it clicked", typed)
	}
	if pressed.Action != "press" || pressed.X != nil {
		t.Fatalf("press without a target reported %+v, want no point", pressed)
	}
	p := launch.pages[0]
	if len(p.clicks) != 1 || len(p.typed) != 1 || !p.typed[0].t.empty() {
		t.Fatalf("clicks %v typed %+v: type should click its target once, then type into what has focus", p.clicks, p.typed)
	}
}

func TestAgentNavigationSaysWhichToolbarActionItWas(t *testing.T) {
	s, _, out := viewService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Navigate("s", "https://example.com/a"); err != nil {
		t.Fatal(err)
	}
	for _, step := range []func(string) error{s.Back, s.Forward, s.Reload} {
		if err := step("s"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Open("s", "https://example.com/b"); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range out.named(api.BrowserAgentEvent.Name) {
		a := d.(api.BrowserAgentAct)
		got = append(got, a.Action+" "+a.URL)
	}
	want := []string{"navigate https://example.com/a", "back ", "forward ", "reload ", "navigate https://example.com/b"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("agent actions %q, want %q", got, want)
	}
}

func TestAgentClickAnswersWithThePageItLeft(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.info = PageInfo{URL: "https://example.com/next", Title: "Next", Interactive: []Interactive{{Index: 0, Tag: "a", Text: "Home"}}}
	idx := 0
	info, err := s.ClickAt(t.Context(), api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Index: &idx}})
	if err != nil {
		t.Fatal(err)
	}
	if p.settles != 1 || info.URL != "https://example.com/next" || len(info.Interactive) != 1 {
		t.Fatalf("settles %d, info %+v: the click should settle and read the page", p.settles, info)
	}
	if run := s.Running(); run[0].Title != "Next" {
		t.Fatalf("running %+v: the page read after the click should update the window's title", run)
	}
	p.infoErr = errors.New("context destroyed by a navigation")
	if _, err := s.PressKey(t.Context(), api.BrowserPressOptions{ID: "s", Key: "Enter"}); err == nil {
		t.Fatal("a failed read after the press reported success")
	}
	if p.closed {
		t.Fatal("a failed read after a press that went through closed the browser")
	}
}

func TestAHeadlessBrowserStartsAtTheWindowsSize(t *testing.T) {
	s, launch, _ := viewService(t)
	window := Viewport{W: 1920, H: 1080, Scale: 1.25}
	s.SetDefaultView(window)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if p := launch.pages[0]; len(p.viewports) != 1 || p.viewports[0] != window {
		t.Fatalf("viewports = %v, want the page laid out at %v", p.viewports, window)
	}
	if _, err := s.OpenVisible("s"); err != nil {
		t.Fatal(err)
	}
	if p := launch.pages[len(launch.pages)-1]; len(p.viewports) != 0 {
		t.Fatalf("a browser in its own window was resized to %v", p.viewports)
	}
}

func TestDetachLaysThePageOutAtTheWindowsSizeAgain(t *testing.T) {
	s, launch, _ := viewService(t)
	window := Viewport{W: 1920, H: 1080, Scale: 1}
	s.SetDefaultView(window)
	if _, err := s.Attach("s", Viewport{W: 500, H: 400, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.Detach("s"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	if got := p.viewports[len(p.viewports)-1]; got != window {
		t.Fatalf("after detach the page is at %v, want %v", got, window)
	}
}

func TestDefaultViewResizesOnlyTheUnwatchedPages(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("agent", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	pane := Viewport{W: 500, H: 400, Scale: 1}
	if _, err := s.Attach("watched", pane); err != nil {
		t.Fatal(err)
	}
	window := Viewport{W: 1600, H: 900, Scale: 1}
	s.SetDefaultView(window)
	if p := launch.pages[0]; len(p.viewports) != 1 || p.viewports[0] != window {
		t.Fatalf("unwatched page viewports = %v, want %v", p.viewports, window)
	}
	if p := launch.pages[1]; p.viewports[len(p.viewports)-1] != pane {
		t.Fatalf("a watched page left its pane's size: %v", p.viewports)
	}
}
