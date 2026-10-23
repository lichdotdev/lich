package events

import (
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestSendNamesTheEventAndCarriesItsPayload(t *testing.T) {
	var name string
	var data any
	emit := func(n string, d any) { name, data = n, d }

	Send(emit, api.SessionTouchedEvent, api.SessionTouched{ID: "s1"})
	if name != "session-touched" || data != (api.SessionTouched{ID: "s1"}) {
		t.Errorf("sent %s %v, want session-touched with the session", name, data)
	}

	Send(emit, api.WindowRaiseEvent, api.None{})
	if name != "window-raise" || data != nil {
		t.Errorf("sent %s %v, want window-raise with no data", name, data)
	}
}
