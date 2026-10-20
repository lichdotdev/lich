package system

import "github.com/lichdotdev/lich/internal/shquote"

// urlOpenArgv is how macOS opens a URL in the default browser. No -t here: that
// flag forces the text editor, which is right for a source file and wrong for a
// link.
func urlOpenArgv(rawURL string) []string {
	return []string{"open", rawURL}
}

// quoteForShell quotes a path for the POSIX shell a macOS session runs.
func (s *Service) quoteForShell(full string) string {
	return shquote.Quote(full)
}

// openFolder shows a directory in Finder, or hands a file to the app its type
// opens with. Deliberately without `open -t`: that flag forces the default
// *text editor*, which a folder or an image has no use for.
func (s *Service) openFolder(dir string) error {
	return s.run("open", dir)
}
