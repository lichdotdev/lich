package browser

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestWithPageDeadlineKeepsChromedpTarget(t *testing.T) {
	pageCtx, cancel := chromedp.NewContext(context.Background())
	defer cancel()
	if chromedp.FromContext(pageCtx) == nil {
		t.Fatal("NewContext produced no chromedp target")
	}

	wait, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if chromedp.FromContext(wait) != nil {
		t.Fatal("a Background timeout unexpectedly carried a chromedp target")
	}

	got, done := withPageDeadline(pageCtx, wait)
	defer done()
	if chromedp.FromContext(got) == nil {
		t.Fatal("deadline wrapper dropped the chromedp target; CDP would return invalid context")
	}
	if _, ok := got.Deadline(); !ok {
		t.Fatal("deadline wrapper dropped the wait deadline")
	}
}

func TestAwaitActionKeepsAResultThatLandsJustAfterADialogOpens(t *testing.T) {
	done := make(chan error, 1)
	held := make(chan struct{})
	close(held)
	go func() {
		time.Sleep(20 * time.Millisecond)
		done <- nil
	}()
	if err := awaitAction(done, held); err != nil {
		t.Fatalf("awaitAction = %v, want the call's own result", err)
	}
}

func TestAwaitActionReportsADialogThatHoldsTheCall(t *testing.T) {
	held := make(chan struct{})
	close(held)
	if err := awaitAction(make(chan error), held); !errors.Is(err, errDialogOpen) {
		t.Fatalf("awaitAction = %v, want errDialogOpen", err)
	}
}

func TestAwaitActionReturnsTheCallsError(t *testing.T) {
	done := make(chan error, 1)
	want := errors.New("boom")
	done <- want
	if err := awaitAction(done, make(chan struct{})); !errors.Is(err, want) {
		t.Fatalf("awaitAction = %v, want %v", err, want)
	}
}
