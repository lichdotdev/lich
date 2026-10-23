package providers

import (
	"os"
	"path/filepath"
)

// KiroHomeVar is the variable Kiro CLI moves its root with.
const KiroHomeVar = "KIRO_HOME"

// KiroHome resolves Kiro CLI's root under home: $KIRO_HOME when it is set, else
// ~/.kiro. The root holds the agents lich installs into, the skills, the
// settings and the conversations (`sessions/cli/<id>.json`), and KIRO_HOME moves
// all of it (2.21.0, measured against a throwaway home; issue #766). The login
// is not here: it is in the SQLite store under the data home, which KIRO_HOME
// leaves where it was.
//
// It lives here because four packages need the same answer — the sandbox binds
// it, the plugin install writes the agent under it, and the terminal reads the
// conversations and stats the agent — and each had built ~/.kiro off the home
// alone, so a user with KIRO_HOME set got no transcript, no cost and no hooks.
//
// A relative variable is ignored rather than resolved, as CursorConfigDir does:
// lich's working directory is not the CLI's, so a relative path names a
// directory nobody involved would agree on.
func KiroHome(home string) string {
	if dir := os.Getenv(KiroHomeVar); filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(home, ".kiro")
}
