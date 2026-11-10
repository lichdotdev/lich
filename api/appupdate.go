package api

var (
	AppUpdateGetStatus = define[None, AppUpdateStatus]("appupdate.Status")
	AppUpdateApply     = define[None, None]("appupdate.Apply")
)

// AppUpdateStatus is lich's install/update state, reported to the window.
type AppUpdateStatus struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	// Required is true when a release newer than the running one, up to the
	// latest, marks itself required: no new agent session opens until lich is
	// updated.
	Required     bool   `json:"required"`
	CanSelfApply bool   `json:"canSelfApply"`
	ReleaseURL   string `json:"releaseUrl"`
	// InstallCommand is the shell command the UI pastes (never auto-runs) to
	// update a package-manager-owned install; empty on the self-apply platforms.
	InstallCommand string `json:"installCommand"`
}
