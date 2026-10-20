package system

import "github.com/lichdotdev/lich/internal/shquote"

// quoteForShell quotes a path for PowerShell, the shell a Windows session runs
// (internal/terminal, windowsShells). Every path can be expressed, so this never
// refuses — cmd.exe, which lich no longer opens a session in, could not express
// a path holding a double quote or a %VAR% and had to hand those to the default
// handler instead.
func (s *Service) quoteForShell(full string) string {
	return shquote.QuotePwsh(full)
}

// urlOpenArgv is how Windows opens a URL in the default browser. Deliberately
// not `cmd /c start`: cmd parses metacharacters out of its command
// line before start ever sees them, so a `&` in a query string would end the
// command and run what follows it. rundll32 is launched directly, with no shell
// in between.
func urlOpenArgv(rawURL string) []string {
	return []string{"rundll32", "url.dll,FileProtocolHandler", rawURL}
}

// openFolder shows a directory in File Explorer, or hands a file to its shell
// association. explorer takes the path as a plain argument, which is the point:
// a `cmd /c start` hands it to a shell that reads & | < > as command
// separators, and Go quotes an argument only when it holds a space, so a file
// named "a&calc.txt", which a checked-out branch is free to carry, would run
// calc. explorer exits 1 even when it worked; nothing waits on it.
func (s *Service) openFolder(dir string) error {
	return s.run("explorer", dir)
}
