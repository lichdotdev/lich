package terminal

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
)

func TestTheBriefingIsWrittenInThePromptLanguage(t *testing.T) {
	t.Parallel()
	for _, lang := range prompt.Langs {
		args := providerArgs(providers.Claude, "", "", "", "", "", "", false, false, false, relay.RouteSessions, lang)
		want := relay.SpawnBriefing(lang, providers.AcceptsMCPServer(providers.Claude), relay.RouteSessions)
		if !slices.Contains(args, want) {
			t.Errorf("%s: the briefing is not in the spawn's argv: %v", lang, args)
		}
	}
}
