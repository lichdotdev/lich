package ompprofile

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/profilestore"
)

func TestUpdateProfileReportsTheProfileOnceWritten(t *testing.T) {
	machine(t)
	var got []profilestore.Changed
	s := New(fakeBins{}, func(name string, data any) {
		if c, ok := data.(profilestore.Changed); ok && name == api.ProfileChangedEvent.Name {
			got = append(got, c)
		}
	})
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	teal := "teal"
	if _, err := s.UpdateProfile(t.Context(), api.UpdateProfileOptions{Slug: p.Slug, Update: profilestore.Update{Color: &teal}}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Provider != "omp" || got[0].Profile.Slug != p.Slug || got[0].Profile.Color != "teal" {
		t.Errorf("events = %+v", got)
	}
	if _, err := s.UpdateProfile(t.Context(), api.UpdateProfileOptions{Slug: p.Slug, Update: profilestore.Update{Name: " "}}); err == nil || len(got) != 1 {
		t.Errorf("a refused update reported anyway: %v, %+v", err, got)
	}
}
