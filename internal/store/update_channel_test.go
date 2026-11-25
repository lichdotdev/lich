package store

import (
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestBetaUpdates(t *testing.T) {
	svc := newTestStore(t)
	if svc.BetaUpdates() {
		t.Fatal("BetaUpdates() = true before any choice, want the stable channel")
	}
	for value, want := range map[string]bool{"beta": true, "stable": false, "Beta": false, "": false} {
		if _, err := svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "update.channel", Value: value}); err != nil {
			t.Fatal(err)
		}
		if got := svc.BetaUpdates(); got != want {
			t.Errorf("update.channel %q: BetaUpdates() = %v, want %v", value, got, want)
		}
	}
}
