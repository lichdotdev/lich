package themes

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// The settings the windows read the selection from, global ones. A theme
// key never written reads "", which is System.
const (
	ThemeSettingKey        = "appearance.theme"
	FollowSystemSettingKey = "appearance.followSystem"
	// SystemID selects the bundled theme of the scheme the OS asks for.
	SystemID = "system"
)

type Selection = api.ThemeSelection

// Settings is where the selection is stored: the store's global settings.
type Settings interface {
	GetSetting(key, projectID string) (string, error)
	SetSetting(key, projectID, value string) error
}

// Selection reads the stored selection.
func (s *Service) Selection() (Selection, error) {
	theme, err := s.settings.GetSetting(ThemeSettingKey, "")
	if err != nil {
		return Selection{}, err
	}
	if theme == "" {
		theme = SystemID
	}
	follow, err := s.settings.GetSetting(FollowSystemSettingKey, "")
	if err != nil {
		return Selection{}, err
	}
	return Selection{Theme: theme, FollowSystem: follow == "true"}, nil
}

type SelectOptions = api.SelectThemeOptions

// Select stores opts.ID, System or a theme List returns, as the selection.
func (s *Service) Select(opts SelectOptions) error {
	id := opts.ID
	if id != SystemID {
		themes, err := s.List()
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(themes, func(t Theme) bool { return t.ID == id }) {
			return fmt.Errorf("no theme %q; `lich theme list` shows the installed ones", id)
		}
	}
	return s.store(ThemeSettingKey, id)
}

// SetFollowSystem stores whether the window follows the OS to the selected
// theme's pair.
func (s *Service) SetFollowSystem(on bool) error {
	return s.store(FollowSystemSettingKey, strconv.FormatBool(on))
}

func (s *Service) store(key, value string) error {
	if err := s.settings.SetSetting(key, "", value); err != nil {
		return err
	}
	selection, err := s.Selection()
	if err != nil {
		return err
	}
	events.Send(s.emit, api.ThemeSelectionEvent, selection)
	return nil
}
