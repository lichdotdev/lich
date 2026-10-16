package main

import (
	"fmt"
	"log"
	"net/url"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n (settings.helpSettings).
const (
	helpBugTitle      = "Report a bug"
	helpBugAlso       = "issue github"
	helpBugDesc       = "Opens the bug form in your browser with the version and platform filled in. lich does not file the issue — you write it yourself. Run `lich rage` in a terminal to pack the logs, the versions and what lich found on this machine into one archive to attach."
	helpOpenBug       = "Open bug report"
	helpLogTitle      = "Log file"
	helpLogAlso       = "debug logs native window"
	helpLogDesc       = "The folder holds lich's log and, beside it, the native window's (lich-native.log). They hold absolute paths, project and branch names, and your gh login — never a session token. Read them before you attach them to anything."
	helpLogOpened     = "Log folder opened."
	helpLogFailed     = "Could not open the log folder: %v"
	helpOpenLog       = "Open log folder"
	helpNoLogFile     = "Logging to stderr only — there is no log file."
	helpAboutTitle    = "About"
	helpAboutAlso     = "version license"
	helpAboutVersion  = "lich %s — %s."
	helpAboutUnknown  = "lich."
	helpRepository    = "Repository"
	helpBugKey        = "help.bug" // no backend key: the Help rows only act
	helpLogKey        = "help.log"
	helpAboutKey      = "help.about"
	helpRepoURL       = "https://github.com/omartelo/lich" // support-url.ts REPO_URL
	helpIssueTemplate = "bug.yml"
)

// bugReportURL is support-url.ts bugReportUrl. The query keys are the ids in
// .github/ISSUE_TEMPLATE/bug.yml; GitHub ignores a key that matches no field.
func bugReportURL(d lichclient.Diagnostics) string {
	q := url.Values{"template": {helpIssueTemplate}, "version": {d.Version}, "platform": {d.Platform}}
	return helpRepoURL + "/issues/new?" + q.Encode()
}

// helpSettings is Settings › Help: the diagnostics, nil until the backend
// answers, read once per window, and its buttons.
type helpSettings struct {
	mu          sync.Mutex
	diagnostics *lichclient.Diagnostics
	asked       bool

	bug, reveal, repo widget.Clickable
}

func (h *helpSettings) read(s *settingsScreen) *lichclient.Diagnostics {
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
		r.description = helpBugDesc
		return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D {
			b := th.Button(&h.bug, helpOpenBug)
			b.Size, b.Disabled = ui.SizeSM, d == nil
			return b.Layout(gtx)
		}), true
	case helpLogKey:
		if h.reveal.Clicked(gtx) {
			s.revealLog()
		}
		r.description = helpLogDesc
		return r.layoutBlock(gtx, th, first, func(gtx C, th *ui.Theme) D { return h.logLine(gtx, th, d) }), true
	case helpAboutKey:
		if h.repo.Clicked(gtx) {
			s.openExternal(helpRepoURL)
		}
		r.description = helpAboutUnknown
		if d != nil {
			r.description = fmt.Sprintf(helpAboutVersion, d.Version, d.Platform)
		}
		return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D {
			b := th.Button(&h.repo, helpRepository)
			b.Size, b.Variant = ui.SizeSM, ui.VariantGhost
			return b.Layout(gtx)
		}), true
	}
	return D{}, false
}

// logLine is the Open log folder button and the log's path beside it.
func (h *helpSettings) logLine(gtx C, th *ui.Theme, d *lichclient.Diagnostics) D {
	path := helpNoLogFile
	if d != nil && d.LogPath != "" {
		path = d.LogPath
	}
	b := th.Button(&h.reveal, helpOpenLog)
	b.Size, b.Variant, b.Disabled = ui.SizeSM, ui.VariantOutline, d == nil || d.LogPath == ""
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(b.Layout),
		ui.Gap(3),
		layout.Flexed(1, th.Text(ui.TextXS, path).Muted().Mono().Layout),
	)
}

// revealLog is runWithToast over System.RevealLog without its loading
// spinner, which the toaster does not draw yet.
func (s *settingsScreen) revealLog() {
	go func() {
		if err := s.client.RevealLog(s.ctx); err != nil {
			s.theme.toaster.Show(failure(helpLogFailed, err))
			return
		}
		s.theme.toaster.Show(ui.Toast{Variant: ui.ToastSuccess, Title: helpLogOpened})
	}()
}
