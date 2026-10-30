package browser

import (
	"errors"
	"slices"
	"testing"

	"github.com/chromedp/cdproto/page"
)

func TestFileSinkPicksInTheInputsMode(t *testing.T) {
	errPick := errors.New("no zenity")
	for _, c := range []struct {
		name      string
		watching  bool
		mode      page.FileChooserOpenedMode
		one       string
		many      []string
		err       error
		want      []string
		wantTitle string
	}{
		{"one file", true, page.FileChooserOpenedModeSelectSingle, "/a.png", nil, nil, []string{"/a.png"}, chooseFileTitle},
		{"many files", true, page.FileChooserOpenedModeSelectMultiple, "", []string{"/a.png", "/b.png"}, nil, []string{"/a.png", "/b.png"}, chooseFilesTitle},
		{"canceled", true, page.FileChooserOpenedModeSelectSingle, "", nil, nil, nil, chooseFileTitle},
		{"picker failed", true, page.FileChooserOpenedModeSelectSingle, "", nil, errPick, nil, chooseFileTitle},
		{"no pane shows the page", false, page.FileChooserOpenedModeSelectSingle, "/a.png", nil, nil, nil, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newService(nil, nil, nil, idleAfter)
			var title string
			s.SetPicker(func(tt string) (string, error) { title = tt; return c.one, c.err },
				func(tt string) ([]string, error) { title = tt; return c.many, c.err })
			got, err := s.fileSink(&session{watching: c.watching})(c.mode)
			if !errors.Is(err, c.err) || !slices.Equal(got, c.want) || title != c.wantTitle {
				t.Fatalf("got %v, %v under title %q; want %v, %v under %q", got, err, title, c.want, c.err, c.wantTitle)
			}
		})
	}
}

func TestFileSinkWithoutAPickerLeavesTheInput(t *testing.T) {
	s := newService(nil, nil, nil, idleAfter)
	if got, err := s.fileSink(&session{watching: true})(page.FileChooserOpenedModeSelectSingle); got != nil || err != nil {
		t.Fatalf("got %v, %v; want nothing", got, err)
	}
}
