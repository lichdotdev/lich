package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
)

// checkLogTimeout bounds one job's log download, larger than a read since a
// failed test suite's log runs to hundreds of kilobytes.
const checkLogTimeout = 30 * time.Second

// checkLogFields is the gh `pr view --json` selection the log read needs.
const checkLogFields = "number,headRefOid,statusCheckRollup"

// unknownStep is what `gh run view --log-failed` names every line's step with
// when GitHub's log archive does not split the job by step, which leaves the
// runner's own markers as the only step boundaries.
const unknownStep = "UNKNOWN STEP"

// The runner's markers: a step opens its log with "##[group]Run <command>",
// and a failed one ends it with "##[error]Process completed with exit code N".
const (
	stepMarker  = "##[group]Run "
	groupMarker = "##[group]"
	errorMarker = "##[error]"
)

var (
	actionsJobURL = regexp.MustCompile(`/actions/runs/\d+/job/(\d+)`)
	logTimestamp  = regexp.MustCompile(`^\d{4}-\d\d-\d\dT[\d:.]+Z ?`)
	// gh 2.102 prints a log's escapes as the caret text "^[" so the log
	// cannot drive a terminal; the ESC byte is matched too, for a gh that
	// passes it through.
	terminalCode = regexp.MustCompile(`(?:\x1b|\^\[)(?:\[[0-9;?]*[ -/]*[@-~]|[()][0-9A-Za-z])`)
)

// CheckLog reads the failed checks of the pull request of opts.ID's checkout,
// with the tail of each failing step of every failed Actions job.
func (s *Service) CheckLog(_ context.Context, opts api.CheckLogOptions) (api.CheckLog, error) {
	dir := ""
	if s.sessionCheckout != nil {
		dir = s.sessionCheckout(opts.ID)
	}
	if dir == "" {
		return api.CheckLog{}, fmt.Errorf("session %q is not running", opts.ID)
	}
	out, err := s.gh(prLookupTimeout, dir, "pr", "view", "--json", checkLogFields)
	if errors.Is(err, errNoPullRequest) {
		return api.CheckLog{}, nil
	}
	if err != nil {
		return api.CheckLog{}, err
	}
	log, err := parseCheckLog(out)
	if err != nil || log.Pending > 0 {
		return log, err
	}
	lines := opts.Lines
	if lines <= 0 {
		lines = api.CheckLogLines
	}
	for i, check := range log.Failed {
		job := actionsJobID(check.URL)
		if job == "" {
			continue
		}
		raw, err := s.gh(checkLogTimeout, dir, "run", "view", "--job", job, "--log-failed")
		if err != nil {
			return api.CheckLog{}, fmt.Errorf("read the log of %s: %w", check.Name, err)
		}
		log.Failed[i].Steps = trimJobLog(raw, lines)
	}
	return log, nil
}

func parseCheckLog(out []byte) (api.CheckLog, error) {
	var v struct {
		Number            int         `json:"number"`
		HeadRefOid        string      `json:"headRefOid"`
		StatusCheckRollup []checkItem `json:"statusCheckRollup"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return api.CheckLog{}, errUnreadableAnswer(err)
	}
	log := api.CheckLog{Number: v.Number, Head: v.HeadRefOid}
	for _, it := range v.StatusCheckRollup {
		switch checkState(it) {
		case checkPassed:
			log.Passed++
		case checkPending:
			log.Pending++
		default:
			log.Failed = append(log.Failed, api.FailedCheck{
				Name: firstNonEmpty(it.Name, it.Context),
				URL:  firstNonEmpty(it.DetailsURL, it.TargetURL),
			})
		}
	}
	return log, nil
}

// actionsJobID is the job a GitHub Actions check's URL names, "" for a check
// some other app or a commit status reported.
func actionsJobID(url string) string {
	if m := actionsJobURL.FindStringSubmatch(url); m != nil {
		return m[1]
	}
	return ""
}

// trimJobLog splits `gh run view --log-failed` output by step and keeps the
// tail of each. gh prefixes every line with "job<TAB>step<TAB>timestamp ".
func trimJobLog(raw []byte, lines int) []api.StepLog {
	text := strings.TrimRight(string(raw), "\n")
	if text == "" {
		return nil
	}
	var steps []api.StepLog
	var name string
	var body []string
	flush := func() {
		if body != nil {
			steps = append(steps, tailStep(name, body, lines))
		}
	}
	for _, line := range strings.Split(text, "\n") {
		step, text := splitLogLine(line)
		if step != name || body == nil {
			flush()
			name, body = step, []string{}
		}
		body = append(body, text)
	}
	flush()
	return steps
}

// splitLogLine reads one gh log line into its step and its text, without the
// timestamp, the byte order mark that opens the log, or terminal escapes.
func splitLogLine(line string) (step, text string) {
	fields := strings.SplitN(line, "\t", 3)
	text = fields[len(fields)-1]
	if len(fields) == 3 {
		step = fields[1]
	}
	text = strings.TrimPrefix(text, "\uFEFF")
	text = logTimestamp.ReplaceAllString(text, "")
	text = terminalCode.ReplaceAllString(text, "")
	return step, strings.TrimRight(text, "\r")
}

// tailStep keeps the last lines of the failing step within body, through its
// last error: what follows is the job's cleanup. With no step split from gh,
// body is the whole job, and the step is the last one the runner opened
// before that error.
func tailStep(name string, body []string, lines int) api.StepLog {
	start, end := failingStep(body)
	if name != unknownStep && name != "" {
		start = 0
	} else {
		name = ""
		if strings.HasPrefix(body[start], stepMarker) {
			name = strings.TrimPrefix(body[start], groupMarker)
		}
	}
	kept := body[start:end]
	cut := max(len(kept)-lines, 0)
	return api.StepLog{Step: name, Lines: kept[cut:], Cut: cut}
}

// failingStep is the span of body from the last step opened before the last
// error through that error; all of body when it holds no error.
func failingStep(body []string) (start, end int) {
	end = len(body)
	for i := len(body) - 1; i >= 0; i-- {
		if strings.HasPrefix(body[i], errorMarker) {
			end = i + 1
			break
		}
	}
	for i := end - 1; i >= 0; i-- {
		if strings.HasPrefix(body[i], stepMarker) {
			return i, end
		}
	}
	return 0, end
}
