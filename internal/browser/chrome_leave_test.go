package browser

import (
	"context"
	"testing"

	"github.com/chromedp/cdproto/page"
)

func landedPage(t *testing.T) (*chromePage, *tab) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	// A gone page: the re-navigation of an aborted leave fails quietly.
	cancel()
	return &chromePage{ctx: ctx}, &tab{ctx: ctx, id: "t"}
}

func isDone(l *leave) bool {
	select {
	case <-l.done:
		return true
	default:
		return false
	}
}

func TestALeaveEndsWhereTheNavigationLands(t *testing.T) {
	p, tb := landedPage(t)
	p.holdLeave(tb, "http://x/next")
	l := p.leaving(true)
	if l == nil {
		t.Fatal("an accepted beforeunload left for nothing")
	}
	p.leaveStarted(tb, page.FrameStartedNavigatingNavigationTypeDifferentDocument)
	p.leaveLanded(tb)
	if !isDone(l) || p.leave != nil {
		t.Fatal("the leave outlived the document it landed on")
	}
}

func TestAnAbortedLeaveIsTakenUpAgainUntilItsTriesRunOut(t *testing.T) {
	p, tb := landedPage(t)
	p.holdLeave(tb, "http://x/next")
	l := p.leaving(true)
	for try := 1; try <= leaveTries; try++ {
		p.leaveStarted(tb, page.FrameStartedNavigatingNavigationTypeReload)
		if !p.reloadAsks(tb) {
			t.Fatalf("try %d: the reload's beforeunload was left to show", try)
		}
		p.leaveLanded(tb)
		if isDone(l) || l.tries != try {
			t.Fatalf("try %d: done %v after %d tries, want it taken up again", try, isDone(l), l.tries)
		}
	}
	p.leaveStarted(tb, page.FrameStartedNavigatingNavigationTypeReload)
	p.leaveLanded(tb)
	if !isDone(l) {
		t.Fatal("the leave went on past its tries")
	}
}

func TestOnlyAnAcceptedBeforeunloadLeaves(t *testing.T) {
	p, tb := landedPage(t)
	p.holdLeave(tb, "http://x/next")
	if p.leaving(false) != nil {
		t.Fatal("a dismissed beforeunload left")
	}
	p.holdLeave(tb, "")
	if p.leaving(true) != nil {
		t.Fatal("an accepted alert left")
	}
	if p.reloadAsks(tb) {
		t.Fatal("a beforeunload with no leave was accepted unseen")
	}
}
