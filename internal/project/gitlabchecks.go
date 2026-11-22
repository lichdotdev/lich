package project

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// A GitLab merge request's checks are the jobs of its head pipeline. This file
// reads them into the verdicts prchecks.go defines, and a failed job's trace
// into the check log.

type glabJob struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Stage        string `json:"stage"`
	Status       string `json:"status"`
	AllowFailure bool   `json:"allow_failure"`
	WebURL       string `json:"web_url"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at"`
}

const glabJobFailed = "failed"

// jobVerdict reads a job's status as a check verdict. A job allowed to fail
// does not hold the merge, so it passes whatever became of it; so does a
// manual job allowed to fail, which may never run. A manual job that is not
// allowed to fail holds the pipeline until someone starts it: pending.
func jobVerdict(j glabJob) string {
	switch j.Status {
	case "success", "skipped":
		return checkPassed
	case glabJobFailed, "canceled":
		if j.AllowFailure {
			return checkPassed
		}
		return checkFailed
	case "manual":
		if j.AllowFailure {
			return checkPassed
		}
	}
	return checkPending
}

// jobs lists the pipeline's jobs, nil for a merge request with no pipeline.
// The first 100 only: one page is one call.
func (g gitlabForge) jobs(path string, p *glabPipeline) ([]glabJob, error) {
	if p == nil {
		return nil, nil
	}
	out, err := g.run(prReadTimeout, path, "api",
		fmt.Sprintf("projects/%d/pipelines/%d/jobs?per_page=100", p.ProjectID, p.ID))
	if err != nil {
		return nil, err
	}
	var jobs []glabJob
	if err := json.Unmarshal(out, &jobs); err != nil {
		return nil, errUnreadableGitLab(err)
	}
	return jobs, nil
}

func (g gitlabForge) checks(path string, p *glabPipeline) (ChecksRollup, []CheckItem, error) {
	jobs, err := g.jobs(path, p)
	if err != nil {
		return ChecksRollup{}, nil, err
	}
	rollup, items := jobChecks(jobs)
	return rollup, items, nil
}

// jobChecks is the rollup and the Checks tab's list of a pipeline's jobs, worst
// first. A job that may fail is the one GitLab does not require.
func jobChecks(jobs []glabJob) (ChecksRollup, []CheckItem) {
	r := ChecksRollup{Total: len(jobs)}
	if len(jobs) == 0 {
		return r, nil
	}
	items := make([]CheckItem, 0, len(jobs))
	for _, j := range jobs {
		state := jobVerdict(j)
		switch state {
		case checkPassed:
			r.Passed++
		case checkFailed:
			r.Failed++
		default:
			r.Pending++
		}
		items = append(items, CheckItem{Name: j.Name, State: state, Description: j.Stage, URL: j.WebURL,
			StartedAt: j.StartedAt, CompletedAt: j.FinishedAt, Required: !j.AllowFailure})
	}
	sort.SliceStable(items, func(i, k int) bool { return checkOrder[items[i].State] < checkOrder[items[k].State] })
	return r, items
}

// checkLog reads the branch's open merge request's failed jobs, with the tail
// of each failed job's script once nothing is pending.
func (g gitlabForge) checkLog(dir string, lines int) (api.CheckLog, error) {
	mr, err := g.view(dir, 0)
	if err != nil {
		return api.CheckLog{}, err
	}
	jobs, err := g.jobs(dir, mr.HeadPipeline)
	if err != nil {
		return api.CheckLog{}, err
	}
	log := api.CheckLog{Number: mr.IID, Head: mr.SHA}
	var traced []glabJob
	for _, j := range jobs {
		switch jobVerdict(j) {
		case checkPassed:
			log.Passed++
		case checkPending:
			log.Pending++
		default:
			log.Failed = append(log.Failed, api.FailedCheck{Name: j.Name, URL: j.WebURL})
			traced = append(traced, j)
		}
	}
	if log.Pending > 0 {
		return log, nil
	}
	for i, j := range traced {
		if j.Status != glabJobFailed {
			continue
		}
		raw, err := g.run(checkLogTimeout, dir, "api",
			fmt.Sprintf("projects/%d/jobs/%d/trace", mr.HeadPipeline.ProjectID, j.ID))
		if err != nil {
			return api.CheckLog{}, fmt.Errorf("read the log of %s: %w", j.Name, err)
		}
		log.Failed[i].Steps = traceSteps(raw, lines)
	}
	return log, nil
}

// scriptSection is the runner's section around the job's own script, where a
// failed job's reason almost always is: the sections before it set the job up,
// the ones after upload and clean.
const scriptSection = "step_script"

var (
	// A runner with timestamped logs opens each line with the time, the
	// stream and O or E, and "+" for a line that continues the one before
	// (measured, gitlab-runner 19.6).
	traceStamp = regexp.MustCompile(`^\d{4}-\d\d-\d\dT[\d:.]+Z [0-9a-f]{2}[OE]\+? ?`)
	// section_start:<unix time>:<name>[options]\r\e[0K, and section_end.
	traceSection = regexp.MustCompile(`section_(start|end):\d+:([^\[\r\s]+)(?:\[[^\]]*\])?\r?(?:\x1b\[0K)?`)
)

// traceSteps keeps the tail of the job's script section, or of the whole trace
// when it has none (a job that failed setting up).
func traceSteps(raw []byte, lines int) []api.StepLog {
	text := strings.TrimRight(string(raw), "\n")
	if text == "" {
		return nil
	}
	var all, script []string
	inScript, sawScript := false, false
	for line := range strings.SplitSeq(text, "\n") {
		line = traceStamp.ReplaceAllString(line, "")
		markers := traceSection.FindAllStringSubmatch(line, -1)
		for _, m := range markers {
			if m[2] == scriptSection {
				inScript = m[1] == "start"
				sawScript = sawScript || inScript
			}
		}
		line = traceSection.ReplaceAllString(line, "")
		line = strings.TrimRight(terminalCode.ReplaceAllString(line, ""), "\r")
		if line == "" && markers != nil {
			continue
		}
		all = append(all, line)
		if inScript {
			script = append(script, line)
		}
	}
	step, body := "", all
	if sawScript {
		step, body = scriptSection, script
	}
	cut := max(len(body)-lines, 0)
	return []api.StepLog{{Step: step, Lines: body[cut:], Cut: cut}}
}
