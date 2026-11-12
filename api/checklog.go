package api

// ProjectCheckLog reads the failed checks of the pull request of session ID's
// own checkout, with the tail of each failed GitHub Actions job's log. A
// confined token may call it for its own session only (internal/scope).
var ProjectCheckLog = define[CheckLogOptions, CheckLog]("project.CheckLog")

// CheckLogLines is how much of a failing step reaches the agent unless it asks
// for more: on lich's own failed runs the error and the lines that explain it
// sit within the last few dozen.
const CheckLogLines = 80

// CheckLogOptions names the calling session. Lines is how many lines of each
// failing step to keep; 0 keeps CheckLogLines.
type CheckLogOptions struct {
	ID    string `json:"id"`
	Lines int    `json:"lines,omitempty"`
}

// CheckLog is the pull request's checks at its head commit. Number 0 is a
// branch with no pull request. Logs are read only once no check is pending:
// gh serves a job's log only when its whole run has finished.
type CheckLog struct {
	Number  int           `json:"number,omitempty"`
	Head    string        `json:"head,omitempty"`
	Passed  int           `json:"passed"`
	Pending int           `json:"pending"`
	Failed  []FailedCheck `json:"failed,omitempty"`
}

// FailedCheck is one failed check. Steps is empty for a check that is not a
// GitHub Actions job, whose log lives only at URL, and while checks are pending.
type FailedCheck struct {
	Name  string    `json:"name"`
	URL   string    `json:"url,omitempty"`
	Steps []StepLog `json:"steps,omitempty"`
}

// StepLog is the tail of one failing step's log, cleaned of gh's prefixes and
// terminal escapes. Cut is how many earlier lines of the step were left out.
type StepLog struct {
	Step  string   `json:"step,omitempty"`
	Lines []string `json:"lines"`
	Cut   int      `json:"cut,omitempty"`
}
