package browser

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

func TestClickReportsTheFileItDownloadedAndTellsTheWindowOnce(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.onClick = func() {
		p.onDownload("g1", api.BrowserDownload{Path: "/dl/report.csv", State: api.DownloadInProgress})
		p.onDownload("g1", api.BrowserDownload{Path: "/dl/report.csv", Bytes: 12, State: api.DownloadCompleted})
	}
	info, err := s.ClickAt(t.Context(), api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#dl"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []api.BrowserDownload{{Path: "/dl/report.csv", Bytes: 12, State: api.DownloadCompleted}}
	if !reflect.DeepEqual(info.Downloads, want) {
		t.Fatalf("click reported %+v, want %+v", info.Downloads, want)
	}
	done := out.named(api.BrowserDownloadEvent.Name)
	if len(done) != 1 || done[0].(api.BrowserDownload).Owner != "s" || done[0].(api.BrowserDownload).Path != "/dl/report.csv" {
		t.Fatalf("window heard %+v, want the one completed download, owned by s", done)
	}
	again, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Downloads) != 0 {
		t.Fatalf("a read after the click repeated %+v", again.Downloads)
	}
}

func TestClickWaitsForTheDownloadItStartedToFinish(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.onClick = func() {
		p.onDownload("g1", api.BrowserDownload{Path: "/dl/a.zip", State: api.DownloadInProgress})
		go func() {
			time.Sleep(100 * time.Millisecond)
			p.onDownload("g1", api.BrowserDownload{Path: "/dl/a.zip", Bytes: 5, State: api.DownloadCompleted})
		}()
	}
	info, err := s.ClickAt(t.Context(), api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#dl"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Downloads) != 1 || info.Downloads[0].State != api.DownloadCompleted {
		t.Fatalf("click reported %+v, want the download completed", info.Downloads)
	}
}

func TestADownloadStillRunningIsReportedAgainWhenItEnds(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.onDownload("g1", api.BrowserDownload{Path: "/dl/big.iso", Bytes: 3, State: api.DownloadInProgress})
	sess := s.current("s")
	if got := s.downloadsAfter(sess, 0); len(got) != 1 || got[0].State != api.DownloadInProgress {
		t.Fatalf("got %+v, want it in progress", got)
	}
	start := time.Now()
	if got := s.downloadsAfter(sess, time.Minute); len(got) != 0 || time.Since(start) > time.Second {
		t.Fatalf("got %+v after %v: a download already reported in progress must not hold the next action", got, time.Since(start))
	}
	p.onDownload("g1", api.BrowserDownload{Path: "/dl/big.iso", Bytes: 9, State: api.DownloadCompleted})
	info, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Downloads) != 1 || info.Downloads[0].State != api.DownloadCompleted || info.Downloads[0].Bytes != 9 {
		t.Fatalf("read reported %+v, want it completed", info.Downloads)
	}
	if len(sess.downloads) != 0 {
		t.Fatalf("kept %d downloads the agent heard end", len(sess.downloads))
	}
}

func TestAnActionAfterAnActionWaitsOutTheBrowsersStartup(t *testing.T) {
	s, launch, _ := viewService(t)
	s.startup = 300 * time.Millisecond
	launched := time.Now()
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	target := api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#dl"}}
	if _, err := s.ClickAt(t.Context(), target); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(launched); took >= s.startup {
		t.Fatalf("the browser's first action waited: %v", took)
	}
	if _, err := s.ClickAt(t.Context(), target); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(launched); took < s.startup {
		t.Fatalf("the second click landed %v after the launch, inside the startup", took)
	}
	if len(p.clicks) != 2 {
		t.Fatalf("clicks %d, want 2", len(p.clicks))
	}
}

func TestAnActionAfterAClickThatChangedThePageIsNotHeld(t *testing.T) {
	s, launch, _ := viewService(t)
	s.startup = time.Minute
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.onClick = func() {
		p.mu.Lock()
		p.title = "changed"
		p.mu.Unlock()
	}
	target := api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#dl"}}
	start := time.Now()
	for range 2 {
		if _, err := s.ClickAt(t.Context(), target); err != nil {
			t.Fatal(err)
		}
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("the clicks took %v: a click that changed the page held the next", took)
	}
}

func TestAReloadThatLandsReleasesTheHoldOnTheNextAction(t *testing.T) {
	s, launch, _ := viewService(t)
	s.startup, s.reloadWait = time.Minute, time.Minute
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	target := api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#dl"}}
	if _, err := s.ClickAt(t.Context(), target); err != nil {
		t.Fatal(err)
	}
	var landed atomic.Bool
	p.onClick = func() {
		if !landed.Load() {
			t.Error("the click landed before the reload did")
		}
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		p.onReload(true)
		time.Sleep(100 * time.Millisecond)
		landed.Store(true)
		p.onReload(false)
	}()
	if _, err := s.ClickAt(t.Context(), target); err != nil {
		t.Fatal(err)
	}
	if !landed.Load() {
		t.Fatal("the click went through without the reload")
	}
}

func TestAReloadInFlightHoldsAnActionPastTheStartupUntilItsDocument(t *testing.T) {
	s, launch, _ := viewService(t)
	s.startup, s.reloadWait = 0, time.Minute
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	target := api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#dl"}}
	if _, err := s.ClickAt(t.Context(), target); err != nil {
		t.Fatal(err)
	}
	p.onReload(true)
	var ready atomic.Bool
	p.onClick = func() {
		if !ready.Load() {
			t.Error("the click landed on the page mid-reload")
		}
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		ready.Store(true)
		p.onReload(false)
	}()
	if _, err := s.ClickAt(t.Context(), target); err != nil {
		t.Fatal(err)
	}
}

func TestAReloadThatNeverLandsHoldsAnActionOnlyForReloadWait(t *testing.T) {
	s, launch, _ := viewService(t)
	s.startup, s.reloadWait = 0, 200*time.Millisecond
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	target := api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#dl"}}
	if _, err := s.ClickAt(t.Context(), target); err != nil {
		t.Fatal(err)
	}
	p.onReload(true)
	start := time.Now()
	if _, err := s.ClickAt(t.Context(), target); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < s.reloadWait || took > 5*time.Second {
		t.Fatalf("the click waited %v on a reload that never landed, want about %v", took, s.reloadWait)
	}
}

func TestOpenAURLThatDownloadsAnswersWithTheFileAndKeepsThePage(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com/app"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.navErr = errNavAborted
	p.onNavigate = func() {
		p.onDownload("g1", api.BrowserDownload{Path: "/dl/r.csv", Bytes: 3, State: api.DownloadCompleted})
	}
	for _, call := range []func(context.Context, api.BrowserOpenOptions) (api.BrowserHandle, error){s.OpenURL, s.Go} {
		h, err := call(t.Context(), api.BrowserOpenOptions{ID: "s", URL: "https://example.com/r.csv"})
		if err != nil {
			t.Fatal(err)
		}
		if len(h.Downloads) != 1 || h.Downloads[0].Path != "/dl/r.csv" || h.URL != "https://example.com/app" {
			t.Fatalf("handle %+v, want the download and the page it left alone", h)
		}
	}
}

func TestAnAbortedNavigationThatDownloadedNothingStaysAnError(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com/app"); err != nil {
		t.Fatal(err)
	}
	launch.pages[0].navErr = errNavAborted
	if _, err := s.OpenURL(t.Context(), api.BrowserOpenOptions{ID: "s", URL: "https://example.com/x"}); !errors.Is(err, errNavAborted) {
		t.Fatalf("err = %v, want the abort", err)
	}
}

// Chrome reloads the page seconds after a download begins, and the reload
// aborts the navigation the agent started meanwhile.
func TestNavigateRetriesWhatTheReloadAfterADownloadAborted(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.onDownload("g1", api.BrowserDownload{Path: "/dl/a.csv", Bytes: 1, State: api.DownloadCompleted})
	// The reload comes seconds after the download began; one at the
	// navigation's own instant, which Windows' clock gives both, is its
	// download.
	sess := s.current("s")
	s.mu.Lock()
	sess.downloadBegun = time.Now().Add(-4 * time.Second)
	s.mu.Unlock()
	p.navAborts = 1
	h, err := s.Navigate("s", "https://example.com/next")
	if err != nil {
		t.Fatalf("navigate after a download: %v", err)
	}
	if h.URL != "https://example.com/next" || p.url != "https://example.com/next" {
		t.Fatalf("handle %s, page %s, want the page navigated to", h.URL, p.url)
	}
}

func TestNavigateAbortedWithNoDownloadFails(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.navAborts = 1
	if _, err := s.Navigate("s", "https://example.com/next"); !errors.Is(err, errNavAborted) {
		t.Fatalf("navigate aborted with no download answered %v", err)
	}
	if len(p.navigations) != 1 {
		t.Fatalf("navigations %q, want only Open's: nothing to retry", p.navigations)
	}
}

// The retry after a reload can itself be a download: a URL that answers with
// a file aborts the navigation that fetched it.
func TestNavigateRetryThatDownloadsIsADownload(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.onDownload("g1", api.BrowserDownload{Path: "/dl/a.csv", Bytes: 1, State: api.DownloadCompleted})
	sess := s.current("s")
	s.mu.Lock()
	sess.downloadBegun = time.Now().Add(-4 * time.Second)
	s.mu.Unlock()
	calls := 0
	p.onNavigate = func() {
		if calls++; calls == 2 {
			p.onDownload("g2", api.BrowserDownload{Path: "/dl/b.csv", Bytes: 1, State: api.DownloadCompleted})
		}
	}
	p.navAborts = 2
	h, err := s.Navigate("s", "https://example.com/b.csv")
	if err != nil {
		t.Fatalf("a retry that downloaded the file answered %v", err)
	}
	if calls != 2 || h.URL != "https://example.com" {
		t.Fatalf("%d navigations, page at %s: want the retry, and the page where it was", calls, h.URL)
	}
}
