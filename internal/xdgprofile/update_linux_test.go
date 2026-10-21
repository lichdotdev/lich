package xdgprofile

import (
	"testing"

	"github.com/lichdotdev/lich/internal/profilestore"
	"github.com/lichdotdev/lich/internal/providers"
)

func TestUpdateProfileReportsTheProfileOnceWritten(t *testing.T) {
	machine(t)
	var got []profilestore.Changed
	s := New(Kiro, fakeBins{}, func(name string, data any) {
		if c, ok := data.(profilestore.Changed); ok && name == profilestore.ChangedEvent {
			got = append(got, c)
		}
	})
	p, err := s.CreateProfile("Work")
	if err != nil {
		t.Fatal(err)
	}
	teal := "teal"
	if _, err := s.UpdateProfile(p.Slug, profilestore.Update{Color: &teal}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Provider != providers.Kiro || got[0].Profile.Color != "teal" {
		t.Errorf("events = %+v", got)
	}
}
