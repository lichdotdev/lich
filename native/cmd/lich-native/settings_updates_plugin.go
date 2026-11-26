package main

import (
	"log"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// pluginVersionText is the release of the lich plugin sessions run, read
// only: lich installs and updates it itself (agentplugin.Ensure). The first
// provider that has it speaks for all, in display order, Claude Code first.
func pluginVersionText(statuses []backend.PluginStatus) string {
	for _, st := range statuses {
		if st.Installed {
			return "v" + st.InstalledVersion
		}
	}
	return i18n.T("settings.pluginSetting.notInstalled")
}

// readPlugins asks for the rows again and files the answer, nil when the read
// failed; the last good rows stay on screen beside the failure.
func (u *updatesSettings) readPlugins(s *settingsScreen) []backend.PluginStatus {
	statuses, err := s.client.PluginStatuses(s.ctx)
	u.mu.Lock()
	u.pluginsFailed = err != nil
	if err == nil {
		u.plugins = statuses
	}
	u.mu.Unlock()
	if err != nil {
		log.Printf("settings: plugin status: %v", err)
	}
	s.invalidate()
	return statuses
}

// pluginBlock is the lich plugin's row: its title and the release, nothing to
// press.
func (u *updatesSettings) pluginBlock(gtx C, th *ui.Theme, r settingRow, first bool) D {
	u.mu.Lock()
	statuses := u.plugins
	u.mu.Unlock()
	return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D {
		return th.Text(ui.TextSM, pluginVersionText(statuses)).Muted().Layout(gtx)
	})
}
