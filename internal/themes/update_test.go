package themes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// updateFixture is a service with one two-theme pack installed at 1.0.0, and
// a count of the theme-updates events it emitted.
func updateFixture(t *testing.T) (s *Service, repo string, emitted *int) {
	t.Helper()
	s = NewInDir(t.TempDir())
	emitted = new(int)
	s.emit = func(name string, _ any) {
		if name == api.ThemeUpdatesEvent.Name {
			*emitted++
		}
	}
	repo = themeRepo(t, map[string]string{
		manifestName:       manifestJSON(t, "Sample pack", "1.0.0"),
		"tokyo-night.json": rawTheme(t, customTheme("tokyo-night")),
		"tokyo-day.json":   rawTheme(t, customTheme("tokyo-day")),
	})
	if _, err := s.InstallFromGit(t.Context(), api.InstallThemesOptions{URL: repo}); err != nil {
		t.Fatalf("InstallFromGit: %v", err)
	}
	return s, repo, emitted
}

func bump(t *testing.T, repo, version string) {
	t.Helper()
	writeFile(t, repo, manifestName, manifestJSON(t, "Sample pack", version))
	commitAll(t, repo, "bump to "+version)
}

func latestOf(t *testing.T, s *Service) map[string]*api.ThemeLatest {
	t.Helper()
	themes, err := s.List(t.Context(), api.None{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	out := map[string]*api.ThemeLatest{}
	for _, theme := range themes {
		if theme.Origin == OriginCustom {
			out[theme.ID] = theme.Latest
		}
	}
	return out
}

func TestCheckUpdatesMarksEveryThemeOfANewerPack(t *testing.T) {
	s, repo, emitted := updateFixture(t)
	if _, err := s.CheckUpdates(t.Context(), api.None{}); err != nil {
		t.Fatalf("CheckUpdates: %v", err)
	}
	for id, latest := range latestOf(t, s) {
		if latest != nil {
			t.Fatalf("%s is up to date, yet Latest = %#v", id, latest)
		}
	}

	bump(t, repo, "1.1.0")
	before := *emitted
	if _, err := s.CheckUpdates(t.Context(), api.None{}); err != nil {
		t.Fatalf("CheckUpdates: %v", err)
	}
	if *emitted != before+1 {
		t.Fatalf("a newer version emitted %d events, want 1", *emitted-before)
	}
	got := latestOf(t, s)
	want := api.ThemeLatest{Pack: "Sample pack", Version: "1.1.0"}
	for _, id := range []string{"tokyo-night", "tokyo-day"} {
		if got[id] == nil || *got[id] != want {
			t.Fatalf("%s Latest = %#v, want %#v", id, got[id], want)
		}
	}
	stored, err := s.read("tokyo-night")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if stored.Source.Version != "1.0.0" {
		t.Fatalf("a check installed the pack: stored version %s", stored.Source.Version)
	}

	if _, err := s.CheckUpdates(t.Context(), api.None{}); err != nil {
		t.Fatalf("CheckUpdates: %v", err)
	}
	if *emitted != before+1 {
		t.Fatalf("a check that found nothing new emitted an event")
	}
}

func TestUpdateFromGitClearsLatest(t *testing.T) {
	s, repo, emitted := updateFixture(t)
	bump(t, repo, "1.1.0")
	if _, err := s.CheckUpdates(t.Context(), api.None{}); err != nil {
		t.Fatalf("CheckUpdates: %v", err)
	}
	before := *emitted
	if _, err := s.UpdateFromGit(t.Context(), api.UpdateThemeOptions{ID: "tokyo-night"}); err != nil {
		t.Fatalf("UpdateFromGit: %v", err)
	}
	if *emitted != before+1 {
		t.Fatalf("an installed update emitted %d events, want 1", *emitted-before)
	}
	for id, latest := range latestOf(t, s) {
		if latest != nil {
			t.Fatalf("%s was updated, yet Latest = %#v", id, latest)
		}
	}
}

func TestCheckUpdatesKeepsWhatItKnewOfAnUnreachablePack(t *testing.T) {
	s, repo, _ := updateFixture(t)
	bump(t, repo, "1.1.0")
	if _, err := s.CheckUpdates(t.Context(), api.None{}); err != nil {
		t.Fatalf("CheckUpdates: %v", err)
	}
	if err := os.Rename(repo, filepath.Join(t.TempDir(), "moved")); err != nil {
		t.Fatalf("move the repository away: %v", err)
	}
	if _, err := s.CheckUpdates(t.Context(), api.None{}); err == nil {
		t.Fatal("CheckUpdates of an unreachable repository reported no error")
	}
	if latest := latestOf(t, s)["tokyo-night"]; latest == nil || latest.Version != "1.1.0" {
		t.Fatalf("Latest after a failed check = %#v, want 1.1.0", latest)
	}
}

func TestStoredThemeCannotClaimALatest(t *testing.T) {
	s := NewInDir(t.TempDir())
	theme := customTheme("tokyo-night")
	theme.Latest = &api.ThemeLatest{Pack: "Forged", Version: "9.9.9"}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, s.dir, "tokyo-night.json", rawTheme(t, theme))
	if latest := latestOf(t, s)["tokyo-night"]; latest != nil {
		t.Fatalf("Latest read from a file = %#v", latest)
	}
}
