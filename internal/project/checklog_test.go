package project

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

// unsplitJob is `gh run view --log-failed` for a job GitHub's archive did not
// split by step, the shape this repository's own failed runs come back in.
const unsplitJob = "test\tUNKNOWN STEP\t\uFEFF2026-10-08T17:12:43.0510572Z Current runner version: '2.337.0'\n" +
	"test\tUNKNOWN STEP\t2026-10-08T17:12:44.0000000Z ##[group]Run go vet ./...\n" +
	"test\tUNKNOWN STEP\t2026-10-08T17:12:45.0000000Z ok\n" +
	"test\tUNKNOWN STEP\t2026-10-08T17:12:46.0000000Z ##[group]Run bash .github/scripts/go-test-summary.sh\n" +
	"test\tUNKNOWN STEP\t2026-10-08T17:12:47.0000000Z \x1b[36;1mbash .github/scripts/go-test-summary.sh^[[0m\n" +
	"test\tUNKNOWN STEP\t2026-10-08T17:12:48.0000000Z FAIL\tgithub.com/lichdotdev/lich/internal/terminal [build failed]\r\n" +
	"test\tUNKNOWN STEP\t2026-10-08T17:12:49.0000000Z ##[error]Process completed with exit code 1.\n" +
	"test\tUNKNOWN STEP\t2026-10-08T17:12:50.0000000Z Post job cleanup.\n"

func TestTrimJobLogFindsTheFailingStepOfAnUnsplitJob(t *testing.T) {
	t.Parallel()
	steps := trimJobLog([]byte(unsplitJob), 80)
	if len(steps) != 1 {
		t.Fatalf("steps = %d, want 1: %+v", len(steps), steps)
	}
	want := api.StepLog{
		Step: "Run bash .github/scripts/go-test-summary.sh",
		Lines: []string{
			"##[group]Run bash .github/scripts/go-test-summary.sh",
			"bash .github/scripts/go-test-summary.sh",
			"FAIL\tgithub.com/lichdotdev/lich/internal/terminal [build failed]",
			"##[error]Process completed with exit code 1.",
		},
	}
	if steps[0].Step != want.Step || !slices.Equal(steps[0].Lines, want.Lines) || steps[0].Cut != 0 {
		t.Errorf("step = %+v, want %+v", steps[0], want)
	}
}

func TestTrimJobLogKeepsTheTailAndCountsTheCut(t *testing.T) {
	t.Parallel()
	steps := trimJobLog([]byte(unsplitJob), 2)
	if len(steps) != 1 || steps[0].Cut != 2 || len(steps[0].Lines) != 2 {
		t.Fatalf("steps = %+v, want the last 2 lines with 2 cut", steps)
	}
	if steps[0].Lines[1] != "##[error]Process completed with exit code 1." {
		t.Errorf("last line = %q, want the error", steps[0].Lines[1])
	}
}

func TestTrimJobLogSplitsNamedSteps(t *testing.T) {
	t.Parallel()
	raw := "build\tLint\t2026-10-08T17:12:43.0Z ##[group]Run lint\n" +
		"build\tLint\t2026-10-08T17:12:43.1Z ##[group]Run nested\n" +
		"build\tLint\t2026-10-08T17:12:43.2Z bad\n" +
		"build\tLint\t2026-10-08T17:12:43.3Z ##[error]Process completed with exit code 2.\n" +
		"build\tTest\t2026-10-08T17:12:44.0Z FAIL x\n"
	steps := trimJobLog([]byte(raw), 80)
	if len(steps) != 2 {
		t.Fatalf("steps = %+v, want 2", steps)
	}
	if steps[0].Step != "Lint" || len(steps[0].Lines) != 4 {
		t.Errorf("first step = %+v, want all 4 lines of Lint", steps[0])
	}
	if steps[1].Step != "Test" || !slices.Equal(steps[1].Lines, []string{"FAIL x"}) {
		t.Errorf("second step = %+v, want Test with its one line", steps[1])
	}
}

func TestTrimJobLogOfNothing(t *testing.T) {
	t.Parallel()
	if steps := trimJobLog([]byte("\n"), 80); steps != nil {
		t.Errorf("steps = %+v, want none", steps)
	}
}

func TestActionsJobID(t *testing.T) {
	t.Parallel()
	for url, want := range map[string]string{
		"https://github.com/lichdotdev/lich/actions/runs/37814777632/job/113440482759": "113440482759",
		"https://github.com/lichdotdev/lich/actions/runs/1/job/2?pr=3":                 "2",
		"https://ci.example.com/build/7":                                               "",
		"":                                                                             "",
	} {
		if got := actionsJobID(url); got != want {
			t.Errorf("actionsJobID(%q) = %q, want %q", url, got, want)
		}
	}
}

// fakeCheckLogGH answers the two gh calls the log read makes.
type fakeCheckLogGH struct {
	pr   string
	prE  error
	logs map[string]string
	jobs []string
}

func (f *fakeCheckLogGH) run(_ time.Duration, _, _ string, args ...string) ([]byte, error) {
	switch {
	case args[0] == "pr":
		return []byte(f.pr), f.prE
	case args[0] == "run":
		f.jobs = append(f.jobs, args[3])
		if log, ok := f.logs[args[3]]; ok {
			return []byte(log), nil
		}
		return nil, errors.New("no such job")
	}
	return nil, fmt.Errorf("unexpected gh %v", args)
}

func checkLogService(gh *fakeCheckLogGH) *Service {
	s := &Service{runner: gh.run}
	s.SetSessionCheckout(func(id string) string {
		if id == "s1" {
			return "/w"
		}
		return ""
	})
	return s
}

const failedRollup = `{"number":7,"headRefOid":"abc123","statusCheckRollup":[
  {"name":"test","status":"COMPLETED","conclusion":"FAILURE","detailsUrl":"https://github.com/o/r/actions/runs/1/job/11"},
  {"name":"lint","status":"COMPLETED","conclusion":"SUCCESS","detailsUrl":"https://github.com/o/r/actions/runs/1/job/12"},
  {"context":"ci/legacy","state":"ERROR","targetUrl":"https://ci.example.com/7"}]}`

func TestCheckLogReadsFailedJobsOnly(t *testing.T) {
	t.Parallel()
	gh := &fakeCheckLogGH{pr: failedRollup, logs: map[string]string{"11": unsplitJob}}
	log, err := checkLogService(gh).CheckLog(t.Context(), api.CheckLogOptions{ID: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if log.Number != 7 || log.Head != "abc123" || log.Passed != 1 || len(log.Failed) != 2 {
		t.Fatalf("log = %+v", log)
	}
	if !slices.Equal(gh.jobs, []string{"11"}) {
		t.Errorf("logs read = %v, want only job 11", gh.jobs)
	}
	if len(log.Failed[0].Steps) != 1 || log.Failed[1].Steps != nil {
		t.Errorf("failed = %+v, want a log for the Actions job and none for the commit status", log.Failed)
	}
	if log.Failed[1].Name != "ci/legacy" || log.Failed[1].URL != "https://ci.example.com/7" {
		t.Errorf("commit status = %+v", log.Failed[1])
	}
}

func TestCheckLogHonoursLines(t *testing.T) {
	t.Parallel()
	gh := &fakeCheckLogGH{pr: failedRollup, logs: map[string]string{"11": unsplitJob}}
	log, err := checkLogService(gh).CheckLog(t.Context(), api.CheckLogOptions{ID: "s1", Lines: 1})
	if err != nil {
		t.Fatal(err)
	}
	if step := log.Failed[0].Steps[0]; len(step.Lines) != 1 || step.Cut != 3 {
		t.Errorf("step = %+v, want 1 line with 3 cut", step)
	}
}

func TestCheckLogWaitsForPendingChecks(t *testing.T) {
	t.Parallel()
	gh := &fakeCheckLogGH{pr: strings.Replace(failedRollup, `"status":"COMPLETED","conclusion":"SUCCESS"`, `"status":"IN_PROGRESS"`, 1)}
	log, err := checkLogService(gh).CheckLog(t.Context(), api.CheckLogOptions{ID: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if log.Pending != 1 || len(log.Failed) != 2 || gh.jobs != nil {
		t.Errorf("log = %+v, jobs read %v: want the pending count and no log read", log, gh.jobs)
	}
}

func TestCheckLogWithNoPullRequest(t *testing.T) {
	t.Parallel()
	log, err := checkLogService(&fakeCheckLogGH{prE: errNoPullRequest}).CheckLog(t.Context(), api.CheckLogOptions{ID: "s1"})
	if err != nil || log.Number != 0 {
		t.Errorf("log = %+v, %v; want none", log, err)
	}
}

func TestCheckLogFailures(t *testing.T) {
	t.Parallel()
	if _, err := checkLogService(&fakeCheckLogGH{}).CheckLog(t.Context(), api.CheckLogOptions{ID: "gone"}); err == nil {
		t.Error("a session that is not running read a log")
	}
	if _, err := checkLogService(&fakeCheckLogGH{prE: errors.New("offline")}).CheckLog(t.Context(), api.CheckLogOptions{ID: "s1"}); err == nil {
		t.Error("a failed pull request read is not an error")
	}
	if _, err := checkLogService(&fakeCheckLogGH{pr: "{"}).CheckLog(t.Context(), api.CheckLogOptions{ID: "s1"}); err == nil {
		t.Error("an unreadable answer is not an error")
	}
	if _, err := checkLogService(&fakeCheckLogGH{pr: failedRollup}).CheckLog(t.Context(), api.CheckLogOptions{ID: "s1"}); err == nil ||
		!strings.Contains(err.Error(), "test") {
		t.Errorf("a failed log read = %v, want an error naming the check", err)
	}
}
