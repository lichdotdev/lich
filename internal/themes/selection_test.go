package themes

import (
	"context"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

type memorySettings map[string]string

func (m memorySettings) GetSetting(_ context.Context, opts api.GetSettingOptions) (string, error) {
	return m[opts.ProjectID+"/"+opts.Key], nil
}

func (m memorySettings) SetSetting(_ context.Context, opts api.SetSettingOptions) (api.None, error) {
	m[opts.ProjectID+"/"+opts.Key] = opts.Value
	return api.None{}, nil
}

type emitted struct {
	name string
	data any
}

func selectionService(t *testing.T) (*Service, memorySettings, *[]emitted) {
	t.Helper()
	settings := memorySettings{}
	var events []emitted
	s := NewInDir(t.TempDir())
	s.settings = settings
	s.emit = func(name string, data any) { events = append(events, emitted{name, data}) }
	return s, settings, &events
}

func TestSelectionDefaultsToSystem(t *testing.T) {
	s, _, _ := selectionService(t)
	got, err := s.Selection(t.Context(), api.None{})
	if err != nil || got != (Selection{Theme: SystemID}) {
		t.Fatalf("Selection = %+v, %v; want System without following", got, err)
	}
}

// The windows read the selection from the settings the web already uses,
// and hear the change on /events once it is stored.
func TestSelectStoresAndReportsTheSelection(t *testing.T) {
	s, settings, events := selectionService(t)
	if _, err := importTheme(t, s, customTheme("forest")); err != nil {
		t.Fatalf("Import: %v", err)
	}
	if _, err := s.Select(t.Context(), SelectOptions{ID: "forest"}); err != nil {
		t.Fatalf("Select: %v", err)
	}
	if got := settings["/appearance.theme"]; got != "forest" {
		t.Fatalf("appearance.theme = %q, want forest", got)
	}
	want := emitted{api.ThemeSelectionEvent.Name, Selection{Theme: "forest"}}
	if len(*events) != 1 || (*events)[0] != want {
		t.Fatalf("events = %+v, want [%+v]", *events, want)
	}
}

func TestSelectTakesSystemAndBundledThemes(t *testing.T) {
	s, _, _ := selectionService(t)
	for _, id := range []string{SystemID, "light", "dark"} {
		if _, err := s.Select(t.Context(), SelectOptions{ID: id}); err != nil {
			t.Errorf("Select(%q): %v", id, err)
		}
	}
}

func TestSelectRefusesAThemeNotInstalled(t *testing.T) {
	s, settings, events := selectionService(t)
	_, err := s.Select(t.Context(), SelectOptions{ID: "nord"})
	if err == nil || !strings.Contains(err.Error(), `no theme "nord"`) {
		t.Fatalf("Select(nord) = %v, want an error naming it", err)
	}
	if len(settings) != 0 || len(*events) != 0 {
		t.Fatalf("a refused Select stored %v and emitted %v", settings, *events)
	}
}

func TestSetFollowSystemStoresAndReportsIt(t *testing.T) {
	s, settings, events := selectionService(t)
	if _, err := s.Select(t.Context(), SelectOptions{ID: "dark"}); err != nil {
		t.Fatalf("Select: %v", err)
	}
	if _, err := s.SetFollowSystem(t.Context(), api.FollowSystemOptions{On: true}); err != nil {
		t.Fatalf("SetFollowSystem: %v", err)
	}
	if got := settings["/appearance.followSystem"]; got != "true" {
		t.Fatalf("appearance.followSystem = %q, want true", got)
	}
	want := emitted{api.ThemeSelectionEvent.Name, Selection{Theme: "dark", FollowSystem: true}}
	if last := (*events)[len(*events)-1]; last != want {
		t.Fatalf("last event = %+v, want %+v", last, want)
	}
}
