package main

import (
	"testing"
	"time"
)

var peekT0 = time.Unix(0, 0)

func peekAt(d time.Duration) time.Time { return peekT0.Add(d) }

func TestPeekComesOutOnceThePointerRests(t *testing.T) {
	var p panelPeek
	p.point(peekAt(0), true)
	if wake := p.update(peekAt(peekOpenDelay-time.Millisecond), false); p.shown || !wake.Equal(peekAt(peekOpenDelay)) {
		t.Fatalf("shown %v, wake %v before the delay; want hidden, waking at %v", p.shown, wake, peekAt(peekOpenDelay))
	}
	if p.update(peekAt(peekOpenDelay), false); !p.shown {
		t.Fatal("still in once the pointer rested for the delay")
	}
}

func TestPeekPassedOverBringsNothingOut(t *testing.T) {
	var p panelPeek
	p.point(peekAt(0), true)
	p.point(peekAt(peekOpenDelay/2), false)
	if p.update(peekAt(time.Second), false); p.shown {
		t.Fatal("came out for a pointer that only crossed the trigger")
	}
}

func TestPeekGoesBackOnceThePointerHasBeenAway(t *testing.T) {
	p := panelPeek{shown: true, over: true}
	p.point(peekAt(0), false)
	if p.update(peekAt(peekCloseDelay-time.Millisecond), false); !p.shown {
		t.Fatal("went back before the delay")
	}
	if p.update(peekAt(peekCloseDelay), false); p.shown {
		t.Fatal("still out once the pointer was away for the delay")
	}
}

func TestPeekStaysOutWhileInUseThenGivesTheFullDelay(t *testing.T) {
	p := panelPeek{shown: true, over: true}
	p.point(peekAt(0), false)
	p.update(peekAt(time.Second), true)
	if p.update(peekAt(time.Second+peekCloseDelay-time.Millisecond), false); !p.shown {
		t.Fatal("went back sooner than the delay after the menu closed")
	}
	if p.update(peekAt(time.Second+peekCloseDelay), false); p.shown {
		t.Fatal("still out the delay after the menu closed")
	}
}

func TestPeekJustCollapsedWaitsForThePointerToLeave(t *testing.T) {
	p := panelPeek{muted: true, since: peekAt(0)}
	p.point(peekAt(16*time.Millisecond), true)
	if p.update(peekAt(time.Second), false); p.shown {
		t.Fatal("came straight back out under the pointer that collapsed it")
	}
	p.point(peekAt(time.Second), false)
	p.point(peekAt(2*time.Second), true)
	if p.update(peekAt(2*time.Second+peekOpenDelay), false); !p.shown {
		t.Fatal("did not come out once the pointer left and came back")
	}
}

func TestPeekCollapsedWhileThePointerWasAwayComesOut(t *testing.T) {
	p := panelPeek{muted: true, since: peekAt(0)}
	p.point(peekAt(peekCloseDelay), true)
	if p.update(peekAt(peekCloseDelay+peekOpenDelay), false); !p.shown {
		t.Fatal("stayed in for a pointer that came to the rail after the collapse")
	}
}

func TestPeekBroughtOutStaysUntilThePointerHasBeenIn(t *testing.T) {
	var p panelPeek
	p.hold()
	if p.update(peekAt(time.Minute), false); !p.shown {
		t.Fatal("went back before the pointer ever came in")
	}
	p.point(peekAt(time.Minute), true)
	p.point(peekAt(time.Minute+time.Second), false)
	if p.update(peekAt(time.Minute+time.Second+peekCloseDelay), false); p.shown {
		t.Fatal("still out once the pointer came in and left")
	}
}

func TestPeekHeldUnderThePointerGoesBackWhenItLeaves(t *testing.T) {
	p := panelPeek{shown: true, over: true}
	p.hold()
	p.point(peekAt(0), false)
	if p.update(peekAt(peekCloseDelay), false); p.shown {
		t.Fatal("a tab switched from inside the panel kept it out after the pointer left")
	}
}

func TestPeekDismissedStaysInUntilThePointerLeaves(t *testing.T) {
	p := panelPeek{shown: true, over: true}
	p.dismiss()
	if p.update(peekAt(time.Second), false); p.shown {
		t.Fatal("came back out under the pointer after a click in it")
	}
	p.point(peekAt(time.Second), false)
	p.point(peekAt(2*time.Second), true)
	if p.update(peekAt(2*time.Second+peekOpenDelay), false); !p.shown {
		t.Fatal("did not come out again once the pointer left and came back")
	}
}
