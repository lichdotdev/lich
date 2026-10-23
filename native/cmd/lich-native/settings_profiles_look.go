package main

import (
	"log"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icProfileImage   = icons.Lucide("image")
	icProfileNoImage = icons.Lucide("image-off")
)

// profileLookEntries are a profile row's Color grid, Choose image… and, while
// it has one, Remove image. The grid is the card's, but its first swatch is the
// profile's own derived color rather than the theme.
func (s *settingsScreen) profileLookEntries(c *profileSettings, p backend.Profile) []ui.MenuEntry {
	swatches := colorSwatches(p.Color, func(name string) {
		go s.updateLook(c, p.Slug, backend.ProfileUpdate{Color: &name})
	})
	swatches[0].Swatch, swatches[0].Label = profileColor(p.Slug, ""), i18n.T("settings.settingsProfilesView.automaticColor")
	entries := []ui.MenuEntry{
		{Kind: ui.MenuSubTrigger, Label: i18n.T("settings.settingsProfilesView.color"), Icon: icMenuColor, Columns: 3, Sub: swatches},
		{Label: i18n.T("settings.settingsProfilesView.chooseImage"), Icon: icProfileImage, OnClick: func() { go s.chooseProfileImage(c, p.Slug) }},
	}
	if len(p.Image) > 0 {
		none := ""
		entries = append(entries, ui.MenuEntry{
			Label: i18n.T("settings.settingsProfilesView.removeImage"), Icon: icProfileNoImage,
			OnClick: func() { go s.updateLook(c, p.Slug, backend.ProfileUpdate{Image: &none}) },
		})
	}
	return entries
}

func (s *settingsScreen) chooseProfileImage(c *profileSettings, slug string) {
	path, err := s.client.PickFile(s.ctx, i18n.T("settings.settingsProfilesView.chooseImageTitle"))
	if err != nil {
		s.lookFailed(slug, err)
		return
	}
	if path != "" {
		s.updateLook(c, slug, backend.ProfileUpdate{Image: &path})
	}
}

func (s *settingsScreen) updateLook(c *profileSettings, slug string, u backend.ProfileUpdate) {
	if _, err := c.provider.api(s.client).Update(s.ctx, slug, u); err != nil {
		s.lookFailed(slug, err)
	}
}

func (s *settingsScreen) lookFailed(slug string, err error) {
	log.Printf("settings: profile %s look: %v", slug, err)
	title := i18n.T("settings.settingsProfilesView.lookFailed", failedWith(err))
	s.theme.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: title})
}
