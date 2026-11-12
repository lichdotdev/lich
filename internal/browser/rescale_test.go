package browser

import (
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/lichdotdev/lich/api"
)

func rescaleService(t *testing.T) (*Service, *fakeLauncher, *emitted) {
	t.Helper()
	s, launch, out := viewService(t)
	s.rescaleWait = 20 * time.Millisecond
	return s, launch, out
}

func (f *fakeLauncher) launched() ([]*fakePage, []float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*fakePage{}, f.pages...), append([]float64{}, f.scales...)
}

// rescaled waits out the debounce and the relaunch it starts.
func rescaled(s *Service) { time.Sleep(10 * s.rescaleWait) }

func waitLaunches(t *testing.T, f *fakeLauncher, n int) []*fakePage {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		pages, _ := f.launched()
		if len(pages) >= n {
			return pages
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d launches, want %d", len(pages), n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func watchedAt(t *testing.T, s *Service, scale float64) (Handle, Viewport) {
	t.Helper()
	s.SetDefaultView(Viewport{W: 1200, H: 800, Scale: scale}, scale)
	pane := Viewport{W: 640, H: 480, Scale: scale}
	h, err := s.Attach("s", pane)
	if err != nil {
		t.Fatal(err)
	}
	return h, pane
}

func TestScaleSteppingRelaunchesAWatchedBrowserOnceItSettles(t *testing.T) {
	s, launch, out := rescaleService(t)
	// Wide enough that a step a loaded machine runs late still lands inside it.
	s.rescaleWait = 100 * time.Millisecond
	h, pane := watchedAt(t, s, 1)
	for _, scale := range []float64{1.1, 1.2, 1.25, 1.3} {
		s.SetDefaultView(Viewport{W: 1200, H: 800, Scale: scale}, scale)
		time.Sleep(s.rescaleWait / 4)
	}
	rescaled(s)
	pages, scales := launch.launched()
	if len(pages) != 2 || scales[1] != 1.3 {
		t.Fatalf("launches at %v, want one more at 1.3 after the steps", scales)
	}
	if pages[0].Alive() {
		t.Fatal("the browser at the old scale still runs")
	}
	next := pages[1]
	next.mu.Lock()
	casting, viewports := next.casting, append([]Viewport{}, next.viewports...)
	next.mu.Unlock()
	if !casting || viewports[len(viewports)-1] != pane {
		t.Fatalf("new casting=%v at %v, want the pane's %v", casting, viewports, pane)
	}
	again, err := s.List("s")
	if err != nil || len(again) != 1 || again[0].ID != h.ID {
		t.Fatalf("after the relaunch the browser is %+v, want the same id %s", again, h.ID)
	}
	for _, st := range out.states() {
		if !st.Open {
			t.Fatalf("the window heard the browser close: %+v", st)
		}
	}
}

func TestNoRelaunchWhenTheClampedScaleIsTheSame(t *testing.T) {
	s, launch, _ := rescaleService(t)
	watchedAt(t, s, 1.5)
	s.SetDefaultView(Viewport{W: 1200, H: 800, Scale: 2}, 2)
	s.SetDefaultView(Viewport{W: 900, H: 700, Scale: 2}, 2)
	rescaled(s)
	if pages, scales := launch.launched(); len(pages) != 1 {
		t.Fatalf("launches at %v, want none past the first: 2 clamps to 1.5", scales)
	}
}

func TestNoRelaunchOfABrowserInItsOwnWindow(t *testing.T) {
	s, launch, _ := rescaleService(t)
	s.SetDefaultView(Viewport{W: 1200, H: 800, Scale: 1}, 1)
	if _, err := s.OpenVisible("s"); err != nil {
		t.Fatal(err)
	}
	s.SetDefaultView(Viewport{W: 1200, H: 800, Scale: 1.5}, 1.5)
	rescaled(s)
	if pages, _ := launch.launched(); len(pages) != 1 {
		t.Fatalf("%d launches, want the headed window left alone", len(pages))
	}
}

func TestUnwatchedBrowserRelaunchesOnlyWhenAPaneShowsIt(t *testing.T) {
	s, launch, _ := rescaleService(t)
	s.SetDefaultView(Viewport{W: 1200, H: 800, Scale: 1}, 1)
	if _, err := s.Open("s", "https://example.com/"); err != nil {
		t.Fatal(err)
	}
	s.SetDefaultView(Viewport{W: 1200, H: 800, Scale: 1.5}, 1.5)
	rescaled(s)
	if pages, _ := launch.launched(); len(pages) != 1 {
		t.Fatalf("%d launches, want the agent's page no pane shows left as it is", len(pages))
	}
	if _, err := s.Attach("s", Viewport{W: 640, H: 480, Scale: 1.5}); err != nil {
		t.Fatal(err)
	}
	pages, scales := launch.launched()
	if len(pages) != 2 || scales[1] != 1.5 || !pages[1].castingNow() {
		t.Fatalf("launches at %v, want the pane to show one drawn at 1.5", scales)
	}
}

func TestNoRelaunchWhileADialogIsOpen(t *testing.T) {
	s, launch, _ := rescaleService(t)
	watchedAt(t, s, 1)
	first, _ := launch.launched()
	first[0].openDialog(api.BrowserDialog{Type: "alert", Message: "hi"})
	s.SetDefaultView(Viewport{W: 1200, H: 800, Scale: 1.5}, 1.5)
	rescaled(s)
	if pages, _ := launch.launched(); len(pages) != 1 {
		t.Fatal("relaunched under an open dialog")
	}
	if _, err := s.Answer(t.Context(), api.BrowserDialogOptions{ID: "s", Accept: true}); err != nil {
		t.Fatal(err)
	}
	waitLaunches(t, launch, 2)
}

func TestRelaunchWaitsForTheAgentsActionToEnd(t *testing.T) {
	s, launch, _ := rescaleService(t)
	watchedAt(t, s, 1)
	first, _ := launch.launched()
	release := make(chan struct{})
	first[0].onClick = func() { <-release }
	clicked := make(chan error, 1)
	one := 1
	go func() {
		_, err := s.ClickAt(t.Context(), api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Index: &one}})
		clicked <- err
	}()
	time.Sleep(s.rescaleWait)
	s.SetDefaultView(Viewport{W: 1200, H: 800, Scale: 1.5}, 1.5)
	rescaled(s)
	if pages, _ := launch.launched(); len(pages) != 1 {
		t.Fatal("relaunched in the middle of the agent's click")
	}
	close(release)
	if err := <-clicked; err != nil {
		t.Fatalf("the agent's click failed: %v", err)
	}
	waitLaunches(t, launch, 2)
}

func TestRelaunchCarriesTabsCookiesAndTheIndexCounter(t *testing.T) {
	s, launch, _ := rescaleService(t)
	launch.listsTabs = true
	watchedAt(t, s, 1)
	first, _ := launch.launched()
	old := first[0]
	old.mu.Lock()
	old.idxNext = 42
	old.cookies = []*network.CookieParam{{Name: "sid", Value: "1"}}
	old.mu.Unlock()
	old.onTabs([]api.BrowserTab{
		{ID: "a", URL: "https://one.example/"},
		{ID: "b", URL: "https://two.example/", Front: true},
		{ID: "c", URL: "https://three.example/"},
	})
	s.SetDefaultView(Viewport{W: 1200, H: 800, Scale: 1.5}, 1.5)
	next := waitLaunches(t, launch, 2)[1]
	rescaled(s)
	next.mu.Lock()
	defer next.mu.Unlock()
	want := []string{"https://one.example/", "https://two.example/", "https://three.example/"}
	if len(next.navigations) != 3 || next.navigations[0] != want[0] || next.navigations[1] != want[1] || next.navigations[2] != want[2] {
		t.Fatalf("navigations = %v, want %v", next.navigations, want)
	}
	if last := next.tabActions[len(next.tabActions)-1]; last != "select t1" {
		t.Fatalf("tab actions = %v, want the second tab back in front", next.tabActions)
	}
	if next.idxNext != 42 || len(next.setCookies) != 1 || next.setCookies[0].Name != "sid" {
		t.Fatalf("index %d, cookies %v: want 42 and the old page's", next.idxNext, next.setCookies)
	}
}

func (p *fakePage) castingNow() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.casting
}

func TestAStillCaptureInFlightNeitherBlocksARelaunchNorLandsAfterIt(t *testing.T) {
	s, launch, out := rescaleService(t)
	watchedAt(t, s, 1)
	first, _ := launch.launched()
	old := first[0]
	capturing, release := make(chan struct{}), make(chan struct{})
	old.mu.Lock()
	old.still, old.stillTab = stillFrame, stillTab
	old.onStill = func() {
		close(capturing)
		<-release
	}
	old.mu.Unlock()
	old.onFrame(stillTab, jpegFrame)
	<-capturing
	s.SetDefaultView(Viewport{W: 1200, H: 800, Scale: 1.5}, 1.5)
	waitLaunches(t, launch, 2)
	close(release)
	time.Sleep(100 * time.Millisecond)
	for _, f := range out.named(api.BrowserFrameEvents.Of("s").Name) {
		if f.(api.BrowserFrame).Data == stillFrame {
			t.Fatal("the closed browser's still capture reached the window after the relaunch")
		}
	}
}

// zoomed is the window at window's scale with its pages zoomed to zoom.
func zoomed(s *Service, window, zoom float64) {
	s.SetDefaultView(Viewport{W: 1200, H: 800, Scale: window * zoom}, window)
}

func TestAPageZoomAloneNeverRelaunches(t *testing.T) {
	s, launch, _ := rescaleService(t)
	watchedAt(t, s, 1)
	for _, zoom := range []float64{1.1, 1.25, 1.5, 0.9} {
		zoomed(s, 1, zoom)
		rescaled(s)
	}
	if pages, scales := launch.launched(); len(pages) != 1 {
		t.Fatalf("launches at %v, want none past the first: a page zoom loses the page's scroll and form", scales)
	}
}

func TestAMonitorMoveRelaunchesAtTheWindowScaleTimesTheZoom(t *testing.T) {
	s, launch, _ := rescaleService(t)
	watchedAt(t, s, 1)
	zoomed(s, 1, 1.25)
	rescaled(s)
	zoomed(s, 1.2, 1.25)
	waitLaunches(t, launch, 2)
	rescaled(s)
	if pages, scales := launch.launched(); len(pages) != 2 || scales[1] != 1.5 {
		t.Fatalf("launches at %v, want one more at 1.2 x 1.25 = 1.5", scales)
	}
}

func TestAZoomAfterAMoveTheClampHidNeverRelaunches(t *testing.T) {
	s, launch, _ := rescaleService(t)
	watchedAt(t, s, 1.5)
	zoomed(s, 2, 1)
	rescaled(s)
	zoomed(s, 2, 0.5)
	rescaled(s)
	if pages, scales := launch.launched(); len(pages) != 1 {
		t.Fatalf("launches at %v: the move from 1.5 to 2 drew the same, and the zoom after it relaunched", scales)
	}
}

func TestRelaunchOpensDevToolsAgainInFront(t *testing.T) {
	s, launch, _ := rescaleService(t)
	launch.listsTabs = true
	watchedAt(t, s, 1)
	first, _ := launch.launched()
	first[0].onTabs([]api.BrowserTab{
		{ID: "a", URL: "https://one.example/"},
		{ID: "a-dt", DevTools: true, Front: true},
		{ID: "b", URL: "https://two.example/"},
	})
	zoomed(s, 1.5, 1)
	next := waitLaunches(t, launch, 2)[1]
	rescaled(s)
	next.mu.Lock()
	defer next.mu.Unlock()
	want := []string{"new ", "select t0", "devtools ", "select t0", "devtools "}
	if len(next.tabActions) != len(want) {
		t.Fatalf("tab actions = %q, want %q", next.tabActions, want)
	}
	for i := range want {
		if next.tabActions[i] != want[i] {
			t.Fatalf("tab actions = %q, want %q", next.tabActions, want)
		}
	}
}

func TestNoRelaunchWhileThePageIsPausedInTheDebugger(t *testing.T) {
	s, launch, _ := rescaleService(t)
	watchedAt(t, s, 1)
	first, _ := launch.launched()
	first[0].onTabs([]api.BrowserTab{{ID: "a", Front: true}, {ID: "a-dt", DevTools: true, Paused: true}})
	zoomed(s, 1.5, 1)
	rescaled(s)
	if pages, _ := launch.launched(); len(pages) != 1 {
		t.Fatal("relaunched a page paused in the debugger")
	}
	first[0].onTabs([]api.BrowserTab{{ID: "a", Front: true}, {ID: "a-dt", DevTools: true}})
	waitLaunches(t, launch, 2)
}
