package main

import (
	"bytes"
	"image"
	"image/png"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func lookEntry(entries []ui.MenuEntry, label string) (ui.MenuEntry, bool) {
	i := slices.IndexFunc(entries, func(e ui.MenuEntry) bool { return e.Label == label })
	if i < 0 {
		return ui.MenuEntry{}, false
	}
	return entries[i], true
}

func TestPickingAProfileColorSendsIt(t *testing.T) {
	b := &profileBackend{}
	s := flowScreen(t, b)
	c := &s.providers.claude
	color, ok := lookEntry(s.profileLookEntries(c, backend.Profile{Slug: "work", Name: "Work"}), "Color")
	if !ok {
		t.Fatal("the menu has no Color")
	}
	teal := slices.IndexFunc(color.Sub, func(e ui.MenuEntry) bool { return e.Label == "Teal" })
	color.Sub[teal].OnClick()
	eventually(t, "the update", func() bool {
		return slices.ContainsFunc(b.log(), func(call string) bool {
			return strings.HasPrefix(call, "claudeprofile.UpdateProfile") && strings.Contains(call, `"color":"teal"`)
		})
	})
}

func TestAProfileChangedEventRedrawsTheProfileKeepingItsLogin(t *testing.T) {
	m := newModel(nil, func() {})
	m.profiles.list = []backend.Profile{{Slug: personalProfile, Name: "Personal"}, {Slug: "work", Name: "Work", LoggedIn: true, Email: "w@x.y"}}
	m.profiles.images = map[string]*avatarImage{}
	m.profileChanged(backend.ProfileChanged{Provider: claudeKind, Profile: backend.Profile{Slug: "work", Name: "Client", Color: "teal", Image: tinyPNG(t)}})
	look := m.profiles.look("work")
	if look.name != "Client" || look.color != "teal" || look.image == nil {
		t.Errorf("look = %+v", look)
	}
	if work, _ := m.profiles.find("work"); !work.LoggedIn || work.Email != "w@x.y" {
		t.Errorf("the event dropped the login state: %+v", work)
	}
	m.profileChanged(backend.ProfileChanged{Provider: "gemini", Profile: backend.Profile{Slug: "work"}})
}

func TestTheProfileMenuOffersRemoveImageOnlyWithAnImage(t *testing.T) {
	s := flowScreen(t, &profileBackend{})
	c := &s.providers.claude
	plain := backend.Profile{Slug: "work", Name: "Work"}
	if _, ok := lookEntry(s.profileLookEntries(c, plain), "Remove image"); ok {
		t.Error("Remove image offered on a profile without one")
	}
	withImage := plain
	withImage.Image = tinyPNG(t)
	if _, ok := lookEntry(s.profileLookEntries(c, withImage), "Remove image"); !ok {
		t.Error("no Remove image on a profile with one")
	}
	color, _ := lookEntry(s.profileLookEntries(c, plain), "Color")
	first := color.Sub[0]
	if first.Label != "Automatic" || first.Swatch != projectSwatch("work") || !first.Checked {
		t.Errorf("first swatch = %+v, want the profile's own color, checked", first)
	}
}

func TestAnImageTheWindowCannotDecodeShowsTheInitial(t *testing.T) {
	if decodeAvatar(backend.Profile{Slug: "work", Image: []byte("not a png")}) != nil {
		t.Error("garbage decoded as an image")
	}
	if profileColor("work", "chartreuse") != projectSwatch("work") {
		t.Error("an unknown color name was not drawn as the derived color")
	}
}
