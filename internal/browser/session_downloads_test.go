package browser

import (
	"context"
	"errors"
	"reflect"
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
	again, err := s.Read(t.Context(), api.SessionIDOptions{ID: "s"})
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
	info, err := s.Read(t.Context(), api.SessionIDOptions{ID: "s"})
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

func TestClickHoldsItsAnswerForTheQuietAfterADownload(t *testing.T) {
	s, launch, _ := viewService(t)
	s.quiet = 200 * time.Millisecond
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	target := api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#dl"}}
	start := time.Now()
	if _, err := s.ClickAt(t.Context(), target); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took >= s.quiet {
		t.Fatalf("a click that downloaded nothing took %v", took)
	}
	p.onClick = func() {
		p.onDownload("g1", api.BrowserDownload{Path: "/dl/a.csv", Bytes: 1, State: api.DownloadCompleted})
	}
	start = time.Now()
	if _, err := s.ClickAt(t.Context(), target); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < s.quiet {
		t.Fatalf("the click answered %v after its download began, before the quiet", took)
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
