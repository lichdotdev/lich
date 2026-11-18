package browser

import (
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
)

func TestFetchesHoldOnlyAnActionThatBeganBeforeThem(t *testing.T) {
	var f fetches
	before := time.Unix(100, 0)
	action := before.Add(time.Second)
	f.track(&network.EventRequestWillBeSent{RequestID: "poll", Type: network.ResourceTypeFetch}, before)
	if f.since(browserTime{wall: action}) {
		t.Fatal("a fetch from before the action holds it")
	}
	f.track(&network.EventRequestWillBeSent{RequestID: "api", Type: network.ResourceTypeXHR}, action)
	if !f.since(browserTime{wall: action}) {
		t.Fatal("the action's own XHR does not hold it")
	}
	f.track(&network.EventLoadingFinished{RequestID: "api"}, action)
	if f.since(browserTime{wall: action}) {
		t.Fatal("a finished XHR still holds the action")
	}
	f.track(&network.EventRequestWillBeSent{RequestID: "img", Type: network.ResourceTypeImage}, action)
	if f.since(browserTime{wall: action}) {
		t.Fatal("an image holds the action")
	}
}

// A fetch the page began before the action can reach lich after the action
// began, on a busy machine: when Chrome sent it, not when lich heard it,
// says which side of the action it is on.
func TestAFetchHeardAfterTheActionButSentBeforeItDoesNotHoldIt(t *testing.T) {
	var f fetches
	sent := time.Unix(100, 0)
	action := sent.Add(time.Second)
	wall := cdp.TimeSinceEpoch(sent)
	f.track(&network.EventRequestWillBeSent{RequestID: "hang", Type: network.ResourceTypeFetch, WallTime: &wall}, action.Add(time.Second))
	if f.since(browserTime{wall: action}) {
		t.Fatal("a fetch sent before the action holds it because lich heard it late")
	}
	own := cdp.TimeSinceEpoch(action.Add(time.Millisecond))
	f.track(&network.EventRequestWillBeSent{RequestID: "api", Type: network.ResourceTypeFetch, WallTime: &own}, action.Add(time.Second))
	if !f.since(browserTime{wall: action}) {
		t.Fatal("the action's own fetch does not hold it")
	}
}

func TestFetchesUseTheBrowserClockWhenWallTimeDiffers(t *testing.T) {
	var f fetches
	wall := time.Unix(100, 0)
	start := browserTime{wall: wall, ticks: time.Second}
	older := cdp.MonotonicTime(cdp.MonotonicTimeEpoch.Add(500 * time.Millisecond))
	newer := cdp.MonotonicTime(cdp.MonotonicTimeEpoch.Add(2 * time.Second))
	ahead := cdp.TimeSinceEpoch(wall.Add(time.Second))
	behind := cdp.TimeSinceEpoch(wall.Add(-time.Second))
	f.track(&network.EventRequestWillBeSent{RequestID: "poll", Type: network.ResourceTypeFetch, Timestamp: &older, WallTime: &ahead}, wall)
	if f.since(start) {
		t.Error("a fetch from before the action holds it")
	}
	f.end("poll")
	f.track(&network.EventRequestWillBeSent{RequestID: "api", Type: network.ResourceTypeFetch, Timestamp: &newer, WallTime: &behind}, wall)
	if !f.since(start) {
		t.Error("the action's own fetch does not hold it")
	}
}

func TestFetchesKeepARedirectAtItsFirstBeginAndEndAFailure(t *testing.T) {
	var f fetches
	first := time.Unix(100, 0)
	f.track(&network.EventRequestWillBeSent{RequestID: "r", Type: network.ResourceTypeFetch}, first)
	f.track(&network.EventRequestWillBeSent{RequestID: "r", Type: network.ResourceTypeFetch}, first.Add(time.Second))
	if f.since(browserTime{wall: first.Add(time.Millisecond)}) {
		t.Fatal("a redirect moved the fetch's begin past the action that started it")
	}
	f.track(&network.EventLoadingFailed{RequestID: "r"}, first)
	if f.since(browserTime{wall: first}) {
		t.Fatal("a failed fetch still holds the action")
	}
}

func TestFetchesLeaveOtherEventsToTheListener(t *testing.T) {
	var f fetches
	if f.track(&page.EventDomContentEventFired{}, time.Now()) {
		t.Fatal("a page event was taken as a request's")
	}
}
