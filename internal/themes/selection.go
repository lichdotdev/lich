package themes

import (
	"context"
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
	GetSetting(_ context.Context, opts api.GetSettingOptions) (string, error)
	SetSetting(_ context.Context, opts api.SetSettingOptions) (api.None, error)
}

// Selection reads the stored selection.
func (s *Service) Selection(ctx context.Context, _ api.None) (Selection, error) {
	theme, err := s.settings.GetSetting(ctx, api.GetSettingOptions{Key: ThemeSettingKey})
	if err != nil {
		return Selection{}, err
	}
	if theme == "" {
		theme = SystemID
	}
	follow, err := s.settings.GetSetting(ctx, api.GetSettingOptions{Key: FollowSystemSettingKey})
	if err != nil {
		return Selection{}, err
	}
	return Selection{Theme: theme, FollowSystem: follow == "true"}, nil
}

type SelectOptions = api.SelectThemeOptions

// Select stores opts.ID, System or a theme List returns, as the selection.
func (s *Service) Select(ctx context.Context, opts SelectOptions) (api.None, error) {
	id := opts.ID
	if id != SystemID {
		themes, err := s.List(ctx, api.None{})
		if err != nil {
			return api.None{}, err
		}
		if !slices.ContainsFunc(themes, func(t Theme) bool { return t.ID == id }) {
			return api.None{}, fmt.Errorf("no theme %q; `lich theme list` shows the installed ones", id)
		}
	}
	return api.None{}, s.store(ThemeSettingKey, id)
}

// SetFollowSystem stores whether the window follows the OS to the selected
// theme's pair.
func (s *Service) SetFollowSystem(_ context.Context, opts api.FollowSystemOptions) (api.None, error) {
	return api.None{}, s.store(FollowSystemSettingKey, strconv.FormatBool(opts.On))
}

func (s *Service) store(key, value string) error {
	if _, err := s.settings.SetSetting(context.Background(), api.SetSettingOptions{Key: key, Value: value}); err != nil {
		return err
	}
	selection, err := s.Selection(context.Background(), api.None{})
	if err != nil {
		return err
	}
	events.Send(s.emit, api.ThemeSelectionEvent, selection)
	return nil
}
