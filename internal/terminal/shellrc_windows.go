//go:build windows

package terminal

// AddToShellRC has nothing to do on Windows: the installers lich runs there
// put their folder on the user's PATH themselves, and cmd.exe has no rc.
func AddToShellRC(_, _, _ string) (string, error) { return "", nil }
