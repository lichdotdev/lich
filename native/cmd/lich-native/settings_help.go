package main

import (
	"log"
	"net/url"
	"sync"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const (
	helpAboutUnknown  = "lich."
	helpBugKey        = "help.bug" // no backend key: the Help rows only act
	helpLogKey        = "help.log"
	helpAboutKey      = "help.about"
	helpRepoURL       = "https://github.com/lichdotdev/lich"
	helpIssueTemplate = "bug.yml"
)

// bugReportURL is the new-issue link prefilled from the diagnostics. The query
// keys are the ids in .github/ISSUE_TEMPLATE/bug.yml; GitHub ignores a key that
// matches no field.
func bugReportURL(d backend.Diagnostics) string {
	q := url.Values{"template": {helpIssueTemplate}, "version": {d.Version}, "platform": {d.Platform}}
	return helpRepoURL + "/issues/new?" + q.Encode()
}

// helpSettings is Settings › Help: the diagnostics, nil until the backend
// answers, read once per window, and its buttons.
type helpSettings struct {
	mu          sync.Mutex
	diagnostics *backend.Diagnostics
	asked       bool

	bug, reveal, repo ui.Clickable
}

func (h *helpSettings) read(s *settingsScreen) *backend.Diagnostics {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.asked {
		h.asked = true
		go func() {
			d, err := s.client.Diagnostics(s.ctx)
			if err != nil {
				log.Printf("settings: diagnostics: %v", err)
				return
			}
			h.mu.Lock()
			h.diagnostics = &d
			h.mu.Unlock()
			s.invalidate()
		}()
	}
	return h.diagnostics
}

// helpRow draws the Help rows, false for any other key.
func (s *settingsScreen) helpRow(gtx C, th *ui.Theme, e settingEntry, r settingRow, first bool) (D, bool) {
	h := &s.help
	d := h.read(s)
	switch e.key {
	case helpBugKey:
		if h.bug.Clicked(gtx) && d != nil {
			s.openExternal(bugReportURL(*d))
		}
		r.description = i18n.T("settings.settingsHelp.bugDescription")
		return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D {
			b := th.Button(&h.bug, i18n.T("settings.helpSettings.openBugReport"))
			b.Size, b.Disabled = ui.SizeSM, d == nil
			return b.Layout(gtx)
		}), true
	case helpLogKey:
		if h.reveal.Clicked(gtx) {
			s.revealLog()
		}
		r.description = i18n.T("settings.settingsHelp.logDescription")
		r.detail = logPath(d)
		return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D {
			b := th.Button(&h.reveal, i18n.T("settings.helpSettings.openLogFolder"))
			b.Size, b.Variant, b.Disabled = ui.SizeSM, ui.VariantOutline, d == nil || d.LogPath == ""
			return b.Layout(gtx)
		}), true
	case helpAboutKey:
		if h.repo.Clicked(gtx) {
			s.openExternal(helpRepoURL)
		}
		r.description = helpAboutUnknown
		if d != nil {
			r.description = i18n.T("settings.helpSettings.aboutVersion", i18n.Args{"version": d.Version, "platform": d.Platform})
		}
		return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D {
			return th.TrailingIconLink(gtx, &h.repo, i18n.T("settings.helpSettings.repository"), icProviderDocs)
		}), true
	}
	return D{}, false
}

// logPath is the log's path, or the line saying there is none.
func logPath(d *backend.Diagnostics) string {
	if d != nil && d.LogPath != "" {
		return d.LogPath
	}
	return i18n.T("settings.helpSettings.noLogFile")
}

// revealLog is runWithToast over System.RevealLog: a spinning toast that the
// outcome replaces in place.
func (s *settingsScreen) revealLog() {
	id := s.theme.toaster.Show(ui.Toast{Title: i18n.T("settings.helpSettings.openingLogFolder"), Spin: true, Duration: ui.ToastForever})
	go func() { s.theme.toaster.Show(revealLogOutcome(id, s.client.RevealLog(s.ctx))) }()
}

// revealLogOutcome is the toast that takes the loading one's place, id.
func revealLogOutcome(id string, err error) ui.Toast {
	t := ui.Toast{Variant: ui.ToastSuccess, Title: i18n.T("settings.helpSettings.logFolderOpened")}
	if err != nil {
		t = ui.Toast{Variant: ui.ToastError, Title: i18n.T("settings.settingsHelp.logFailed", i18n.Args{"error": backend.ErrorText(err)})}
	}
	t.ID = id
	return t
}
