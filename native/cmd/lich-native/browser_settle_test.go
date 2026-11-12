package main

import (
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestAPaneGoneWithItsBrowserSendsNoDetach(t *testing.T) {
	u := &window{model: newModel(nil, func() {}), browserPanes: map[string]*browserPaneUI{}}
	pane := func(session string) *browserPaneUI {
		p := &browserPaneUI{queue: newBrowserQueue(), wasOnScreen: true, view: api.BrowserViewOptions{ID: session}}
		u.browserPanes[session] = p
		return p
	}
	closed, minimized := pane("closed"), pane("minimized")
	u.model.browsers.states["minimized"] = api.BrowserState{Owner: "minimized", Open: true}

	u.settleBrowserPanes()

	if _, ok := closed.queue.next(); ok {
		t.Fatal("asked the backend to detach a browser that is already closed")
	}
	if _, ok := minimized.queue.next(); !ok {
		t.Fatal("a page taken off screen with its browser running was not detached")
	}
	if closed.view.ID != "" || minimized.view.ID != "" {
		t.Fatal("a pane off screen kept the size it was attached at")
	}
}
