package browser

import (
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
)

func TestFetchesHoldOnlyAnActionThatBeganBeforeThem(t *testing.T) {
	var f fetches
	before := time.Unix(100, 0)
	action := before.Add(time.Second)
	f.track(&network.EventRequestWillBeSent{RequestID: "poll", Type: network.ResourceTypeFetch}, before)
	if f.since(action) {
		t.Fatal("a fetch from before the action holds it")
	}
	f.track(&network.EventRequestWillBeSent{RequestID: "api", Type: network.ResourceTypeXHR}, action)
	if !f.since(action) {
		t.Fatal("the action's own XHR does not hold it")
	}
	f.track(&network.EventLoadingFinished{RequestID: "api"}, action)
	if f.since(action) {
		t.Fatal("a finished XHR still holds the action")
	}
	f.track(&network.EventRequestWillBeSent{RequestID: "img", Type: network.ResourceTypeImage}, action)
	if f.since(action) {
		t.Fatal("an image holds the action")
	}
}

func TestFetchesKeepARedirectAtItsFirstBeginAndEndAFailure(t *testing.T) {
	var f fetches
	first := time.Unix(100, 0)
	f.track(&network.EventRequestWillBeSent{RequestID: "r", Type: network.ResourceTypeFetch}, first)
	f.track(&network.EventRequestWillBeSent{RequestID: "r", Type: network.ResourceTypeFetch}, first.Add(time.Second))
	if f.since(first.Add(time.Millisecond)) {
		t.Fatal("a redirect moved the fetch's begin past the action that started it")
	}
	f.track(&network.EventLoadingFailed{RequestID: "r"}, first)
	if f.since(first) {
		t.Fatal("a failed fetch still holds the action")
	}
}

func TestFetchesLeaveOtherEventsToTheListener(t *testing.T) {
	var f fetches
	if f.track(&page.EventDomContentEventFired{}, time.Now()) {
		t.Fatal("a page event was taken as a request's")
	}
}
