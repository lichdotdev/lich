package agentplugin

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/lichdotdev/lich/internal/codexprofile"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/winexec"
)

// Codex and Antigravity take no plugin per session (measured on 0.170.0 and
// 1.5.15: Codex runs no hooks from a -c override, and Antigravity's --settings
// refuses every plugin key), so their sessions run the plugin lich installed in
// their config. A person, or another tool, can turn it off there; lich turns it
// back on before each spawn, since it is lich's own and a lich session without
// its hooks reports nothing. An open Codex session only picks it up in its next
// conversation (measured on 0.170.0).

// reenableTimeout bounds `agy plugin enable`, which runs on the spawn path.
const reenableTimeout = 10 * time.Second

// Reenable turns the lich plugin back on for kind's next session, when the
// person turned it off: in profileDir's Codex config ("" for Personal's), or
// through bin, Antigravity's CLI ("" for its default name). It reports what it
// changed, as an error, only when it could not.
func Reenable(kind, profileDir, bin string) (bool, error) {
	switch kind {
	case providers.Codex:
		dir := profileDir
		if dir == "" {
			personal, err := codexprofile.PersonalDir()
			if err != nil {
				return false, err
			}
			dir = personal
		}
		return codexprofile.EnablePlugin(dir, pluginKey)
	case providers.Antigravity:
		return antigravityReenable(bin)
	}
	return false, nil
}

func antigravityReenable(bin string) (bool, error) {
	dir, err := antigravityPluginDir()
	if err != nil || !antigravityDisabled(dir) {
		return false, err
	}
	if bin == "" {
		bin = providers.DefaultBinary(providers.Antigravity)
	}
	ctx, cancel := context.WithTimeout(context.Background(), reenableTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "plugin", "enable", pluginName)
	winexec.Hide(cmd)
	cmd.WaitDelay = waitDelay
	if out, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("%s plugin enable %s: %w: %s", bin, pluginName, err, strings.TrimSpace(string(out)))
	}
	return true, nil
}
