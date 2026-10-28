package browser

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

func waitService(t *testing.T) (*Service, *fakePage) {
	t.Helper()
	launch := &fakeLauncher{}
	s := testService(t, launch)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	return s, launch.pages[0]
}

func TestReadWaitsForTextToAppear(t *testing.T) {
	s, p := waitService(t)
	p.onTextRead = func(reads int) {
		if reads == 3 {
			p.mu.Lock()
			p.pageText = "Saved 3 rows"
			p.mu.Unlock()
		}
	}
	info, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s", WaitFor: "Saved"})
	if err != nil {
		t.Fatal(err)
	}
	if info.WaitTimedOut || p.textReads != 3 {
		t.Fatalf("timed out %v after %d reads, want the third read to find it", info.WaitTimedOut, p.textReads)
	}
	if info.URL != "https://example.com" {
		t.Fatalf("answered %q, want the page", info.URL)
	}
}

func TestReadWaitsForTextToGo(t *testing.T) {
	s, p := waitService(t)
	p.pageText = "Loading…"
	p.onTextRead = func(reads int) {
		if reads == 2 {
			p.mu.Lock()
			p.pageText = "Done"
			p.mu.Unlock()
		}
	}
	info, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s", WaitGone: "Loading"})
	if err != nil {
		t.Fatal(err)
	}
	if info.WaitTimedOut || p.textReads != 2 {
		t.Fatalf("timed out %v after %d reads, want the second read to miss it", info.WaitTimedOut, p.textReads)
	}
}

func TestReadThatGivesUpStillAnswersThePage(t *testing.T) {
	s, p := waitService(t)
	s.textWait = 3 * textPoll
	p.pageText = "saved"
	start := time.Now()
	info, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s", WaitFor: "Saved"})
	if err != nil {
		t.Fatal(err)
	}
	if !info.WaitTimedOut || info.URL != "https://example.com" {
		t.Fatalf("got %+v, want the page marked as timed out (the match is case-sensitive)", info)
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("took %v, want the wait bounded by textWait", took)
	}
}

func TestReadWithoutWaitReadsNoText(t *testing.T) {
	s, p := waitService(t)
	info, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if info.WaitTimedOut || p.textReads != 0 {
		t.Fatalf("timed out %v after %d text reads, want a plain read", info.WaitTimedOut, p.textReads)
	}
}

func TestReadStopsWaitingWhenTheBrowserDies(t *testing.T) {
	s, p := waitService(t)
	p.onTextRead = func(reads int) {
		if reads == 2 {
			p.mu.Lock()
			p.dead = true
			p.mu.Unlock()
		}
	}
	if _, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s", WaitFor: "never"}); err == nil {
		t.Fatal("a browser that died answered a page")
	}
	if p.textReads != 2 {
		t.Fatalf("read the text %d times, want the wait to stop once Chrome died", p.textReads)
	}
}
