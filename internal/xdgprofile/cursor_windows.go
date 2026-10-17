//go:build windows

package xdgprofile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/omartelo/lich/internal/profilestore"
	"github.com/omartelo/lich/internal/providers"
)

// On Windows Cursor CLI keeps its login in %APPDATA%\Cursor\auth.json and its
// config dir (cli-config.json, which names the account, and the chats) in
// ~/.cursor, which APPDATA does not move. A profile is a link home of %APPDATA%
// whose Cursor folder is its own, and CURSOR_CONFIG_DIR points the config dir
// at that same folder, as XDG_CONFIG_HOME does on Linux. The folder also holds
// the Cursor editor's own data, which a profile session therefore does not see.
const (
	cursorHomeVar = "APPDATA"
	cursorFolder  = "Cursor"
)

var cursorPinned = []profilestore.PinnedVar{{Name: cursorConfigDirVar, Rel: cursorFolder}}

// cursorBase is Personal's %APPDATA%, the folder holding the CLI's login folder.
func cursorBase() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Dir(providers.CursorAuthDir(home)), nil
}
