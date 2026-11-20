//go:build !windows

package chat

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// A chat opened with no message sent already offers its model and mode: the
// window reads them as the chat opens, before anyone writes to it.
func TestAChatOffersItsOptionsBeforeTheFirstMessage(t *testing.T) {
	for _, kind := range []string{providers.Claude, providers.OpenCode, providers.Codex} {
		r := newChat(t, kind)
		for _, category := range []string{api.OptionModel, api.OptionMode} {
			if o, ok := r.option(category); !ok || len(o.Choices) == 0 {
				t.Errorf("%s: no %s offered before the first message: %+v", kind, category, r.options())
			}
		}
	}
}
