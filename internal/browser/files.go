package browser

import (
	"github.com/chromedp/cdproto/page"
)

// chooseFileTitle and chooseFilesTitle are the file chooser's own titles,
// which are all the dialog says about why it opened, as drop.attachTitle.
const (
	chooseFileTitle  = "Choose File"
	chooseFilesTitle = "Choose Files"
)

// SetPicker wires the host's file picker (internal/project), which a page's
// file input opens. Startup wiring, called before anything serves.
func (s *Service) SetPicker(pickFile func(title string) (string, error), pickFiles func(title string) ([]string, error)) {
	s.pickFile, s.pickFiles = pickFile, pickFiles
}

// fileSink opens the picker for a file input of sess's page, in the input's
// mode. Only for a page someone sees: a chooser the agent opens on a page no
// pane shows would pop up a dialog nobody asked for, so the input stays empty.
func (s *Service) fileSink(sess *session) func(page.FileChooserOpenedMode) ([]string, error) {
	return func(mode page.FileChooserOpenedMode) ([]string, error) {
		s.mu.Lock()
		seen := sess.watching || sess.headed
		s.mu.Unlock()
		if !seen || s.pickFile == nil {
			return nil, nil
		}
		if mode == page.FileChooserOpenedModeSelectMultiple {
			return s.pickFiles(chooseFilesTitle)
		}
		path, err := s.pickFile(chooseFileTitle)
		if path == "" {
			return nil, err
		}
		return []string{path}, err
	}
}
