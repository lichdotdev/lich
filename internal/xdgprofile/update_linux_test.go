package xdgprofile

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/profilestore"
	"github.com/lichdotdev/lich/internal/providers"
)

func TestUpdateProfileReportsTheProfileOnceWritten(t *testing.T) {
	machine(t)
	var got []profilestore.Changed
	s := New(Kiro, fakeBins{}, func(name string, data any) {
		if c, ok := data.(profilestore.Changed); ok && name == api.ProfileChangedEvent.Name {
			got = append(got, c)
		}
	})
	p, err := s.CreateProfile(t.Context(), api.CreateProfileOptions{Name: "Work"})
	if err != nil {
		t.Fatal(err)
	}
	teal := "teal"
	if _, err := s.UpdateProfile(t.Context(), api.UpdateProfileOptions{Slug: p.Slug, Update: profilestore.Update{Color: &teal}}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Provider != providers.Kiro || got[0].Profile.Color != "teal" {
		t.Errorf("events = %+v", got)
	}
}
