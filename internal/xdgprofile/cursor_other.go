//go:build !windows

package xdgprofile

import "github.com/omartelo/lich/internal/profilestore"

// Off Windows Cursor CLI's login and its config dir are one folder under
// XDG_CONFIG_HOME, so moving that variable moves both.
const (
	cursorHomeVar = configHomeVar
	cursorFolder  = "cursor"
)

var cursorPinned []profilestore.PinnedVar

func cursorBase() (string, error) { return configHome() }
