package system

import "github.com/lichdotdev/lich/internal/shquote"

// urlOpenArgv is how this desktop opens a URL in the user's browser — the same
// resolver as openFolder, and the reason a link opens outside the app window.
func urlOpenArgv(rawURL string) []string {
	return []string{"xdg-open", rawURL}
}

// quoteForShell quotes a path for the POSIX shell a Linux session runs.
func (s *Service) quoteForShell(full string) string {
	return shquote.Quote(full)
}

// openFolder shows a directory in the desktop's file manager: xdg-open answers
// a directory with the file manager rather than an editor.
func (s *Service) openFolder(dir string) error {
	return s.run("xdg-open", dir)
}
