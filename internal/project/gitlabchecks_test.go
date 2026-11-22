package project

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestJobVerdict(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		status string
		allow  bool
		want   string
	}{
		{"success", false, checkPassed},
		{"skipped", false, checkPassed},
		{"failed", false, checkFailed},
		{"failed", true, checkPassed},
		{"canceled", false, checkFailed},
		{"manual", true, checkPassed},
		{"manual", false, checkPending},
		{"running", false, checkPending},
		{"pending", false, checkPending},
		{"created", false, checkPending},
		{"something new", false, checkPending},
	} {
		if got := jobVerdict(glabJob{Status: tc.status, AllowFailure: tc.allow}); got != tc.want {
			t.Errorf("%s (allowed to fail %v) = %s, want %s", tc.status, tc.allow, got, tc.want)
		}
	}
}

func TestJobChecksOfAFailedPipelineRankTheFailureFirst(t *testing.T) {
	t.Parallel()
	var jobs []glabJob
	if err := json.Unmarshal(fixture(t, "jobs_failed.json"), &jobs); err != nil {
		t.Fatal(err)
	}
	rollup, items := jobChecks(jobs)
	// tests:integration failed; snapcraft_build_edge, build_test, tests:coverage
	// and tests:unit were canceled behind it; govulncheck was canceled too but
	// may fail.
	if rollup != (ChecksRollup{Passed: 14, Failed: 5, Total: 19}) {
		t.Errorf("rollup = %+v", rollup)
	}
	if items[0].State != checkFailed || items[len(items)-1].State != checkPassed {
		t.Errorf("order = %s first, %s last", items[0].State, items[len(items)-1].State)
	}
	if items[0].URL == "" || items[0].Description == "" {
		t.Errorf("first check = %+v, want its job page and stage", items[0])
	}
	if r, list := jobChecks(nil); r.Total != 0 || list != nil {
		t.Errorf("no jobs = %+v, %v", r, list)
	}
}

func TestTraceStepsKeepsTheScriptSection(t *testing.T) {
	t.Parallel()
	steps := traceSteps(fixture(t, "trace_failed.txt"), 12)
	if len(steps) != 1 || steps[0].Step != scriptSection {
		t.Fatalf("steps = %+v", steps)
	}
	s := steps[0]
	text := strings.Join(s.Lines, "\n")
	if !strings.Contains(text, "DONE 33 tests, 1 failure") || !strings.Contains(text, "Error 1") {
		t.Errorf("tail misses the failure:\n%s", text)
	}
	if strings.Contains(text, "Uploading artifacts") || strings.Contains(text, "section_") {
		t.Errorf("tail runs past the script or keeps markers:\n%s", text)
	}
	if strings.ContainsAny(text, "\x1b\r") || strings.Contains(text, "2026-11-20T") {
		t.Errorf("tail keeps escapes or timestamps:\n%q", text)
	}
	if len(s.Lines) != 12 || s.Cut == 0 {
		t.Errorf("kept %d lines, cut %d", len(s.Lines), s.Cut)
	}
}

func TestTraceStepsWithoutAScriptKeepsTheWholeTail(t *testing.T) {
	t.Parallel()
	raw := "section_start:1:prepare_executor\r\x1b[0K\x1b[0KPreparing\nERROR: Job failed: image pull failed\nsection_end:2:prepare_executor\r\x1b[0K\n"
	steps := traceSteps([]byte(raw), 80)
	if len(steps) != 1 || steps[0].Step != "" || strings.Join(steps[0].Lines, "|") != "Preparing|ERROR: Job failed: image pull failed" {
		t.Errorf("steps = %+v", steps)
	}
	if traceSteps(nil, 80) != nil {
		t.Error("an empty trace has steps")
	}
}

const (
	branchOpenList = "mr list --source-branch=main --per-page 20 -F json"
	failedPipeline = "api projects/34675721/pipelines/3093108776/jobs?per_page=100"
	failedTrace    = "api projects/34675721/jobs/18213579002/trace"
)

// failedMR is !4527 pointed at the failed pipeline the job fixtures come from.
func failedMR(t *testing.T) []byte {
	var mr map[string]any
	if err := json.Unmarshal(fixture(t, "mr4527.json"), &mr); err != nil {
		t.Fatal(err)
	}
	mr["head_pipeline"] = map[string]any{"id": 3093108776, "project_id": 34675721}
	out, _ := json.Marshal(mr)
	return out
}

func checkLogGLab(t *testing.T, repo string) (*Service, *fakeGLab) {
	g := newFakeGLab().
		on(branchOpenList, listRows(t, []int{4514}, map[int]map[string]any{4514: {"state": "opened", "iid": 4527}})).
		on(view4527, failedMR(t)).
		on(failedPipeline, fixture(t, "jobs_failed.json")).
		on(failedTrace, fixture(t, "trace_failed.txt"))
	s := &Service{glab: g.run}
	s.SetSessionCheckout(func(string) string { return repo })
	return s, g
}

func TestGitLabCheckLogReadsTheFailedJobsTrace(t *testing.T) {
	t.Parallel()
	repo, _ := repoWithRemote(t, "origin", "https://gitlab.com/gitlab-org/cli.git")
	s, g := checkLogGLab(t, repo)
	log, err := s.CheckLog(t.Context(), api.CheckLogOptions{ID: "s1", Lines: 5})
	if err != nil {
		t.Fatal(err)
	}
	if log.Number != 4527 || log.Head == "" || log.Passed != 14 || log.Pending != 0 || len(log.Failed) != 5 {
		t.Fatalf("log = %+v", log)
	}
	traced := 0
	for _, f := range log.Failed {
		if f.Steps != nil {
			traced++
			if f.Name != "tests:integration" || len(f.Steps[0].Lines) != 5 {
				t.Errorf("traced %+v", f)
			}
		}
	}
	if traced != 1 || !g.called(failedTrace) {
		t.Errorf("traced %d jobs, want the failed one and none of the canceled", traced)
	}
}

func TestGitLabCheckLogWaitsForPendingJobs(t *testing.T) {
	t.Parallel()
	repo, _ := repoWithRemote(t, "origin", "https://gitlab.com/gitlab-org/cli.git")
	s, g := checkLogGLab(t, repo)
	var jobs []map[string]any
	if err := json.Unmarshal(fixture(t, "jobs_failed.json"), &jobs); err != nil {
		t.Fatal(err)
	}
	jobs[1]["status"] = "running"
	out, _ := json.Marshal(jobs)
	g.on(failedPipeline, out)
	log, err := s.CheckLog(t.Context(), api.CheckLogOptions{ID: "s1"})
	if err != nil || log.Pending != 1 || g.called(failedTrace) {
		t.Errorf("log = %+v, %v; traced while a job runs: %v", log, err, g.called(failedTrace))
	}
}

func TestGitLabCheckLogNamesTheJobWhoseTraceFailed(t *testing.T) {
	t.Parallel()
	repo, _ := repoWithRemote(t, "origin", "https://gitlab.com/gitlab-org/cli.git")
	s, g := checkLogGLab(t, repo)
	g.fail(failedTrace, errors.New(glabSignedOut))
	if _, err := s.CheckLog(t.Context(), api.CheckLogOptions{ID: "s1"}); err == nil || !strings.Contains(err.Error(), "tests:integration") {
		t.Errorf("err = %v", err)
	}
}

func TestGitLabCheckLogOfABranchWithNoMergeRequest(t *testing.T) {
	t.Parallel()
	repo, _ := repoWithRemote(t, "origin", "https://gitlab.com/gitlab-org/cli.git")
	s, g := checkLogGLab(t, repo)
	g.on(branchOpenList, []byte("[]"))
	if log, err := s.CheckLog(t.Context(), api.CheckLogOptions{ID: "s1"}); err != nil || log.Number != 0 {
		t.Errorf("log = %+v, %v", log, err)
	}
}

func TestGLabMessage(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, report string
		runErr, ctx  error
		want         string
	}{
		{"timeout", "", errTest, context.DeadlineExceeded, "GitLab did not answer in time. Check the connection and try again."},
		{"missing", "", exec.ErrNotFound, nil, "The GitLab CLI (glab) is not installed."},
		// glab api on a token-only endpoint, measured: stderr, then GitLab's body.
		{"signed out", "glab: 401 Unauthorized (HTTP 401)\n{\"message\":\"401 Unauthorized\"}", errTest, nil, glabSignedOut},
		{"no such number", `{"error":{"message":"failed to get merge request 999999: 404 Not Found"}}`, errTest, nil,
			"GitLab has no merge request with that number, or does not show it to the account glab is signed in as."},
		{"not a repository", `not a git repository: fatal: not a git repository`, errTest, nil, "This folder is not a git repository."},
		{"pipeline", "this merge request requires a passing pipeline before merging", errTest, nil,
			"GitLab requires the pipeline to pass before this merge request can merge."},
		{"anything else", "a wording nobody has seen", errTest, nil, "glab could not complete the request. lich's log has what it reported."},
	} {
		if got := glabMessage(tc.report, tc.runErr, tc.ctx); got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestGLabFailureTellsNoMergeRequestApart(t *testing.T) {
	t.Parallel()
	// glab mr view on a branch with none, measured: stdout under -F json.
	report := "\n" + `{"error":{"message":"no open merge request available for \"nope\""}}`
	if err := glabFailure([]string{"mr", "view"}, report, errTest, nil); !errors.Is(err, errNoPullRequest) {
		t.Errorf("err = %v, want errNoPullRequest", err)
	}
	if err := glabFailure([]string{"api"}, "glab: 401 Unauthorized (HTTP 401)", errTest, nil); err.Error() != glabSignedOut {
		t.Errorf("err = %v", err)
	}
	if err := errUnreadableGitLab(errTest); !strings.Contains(err.Error(), "GitLab") {
		t.Errorf("err = %v", err)
	}
}

func TestRunGLabReportsAMissingGlab(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := runGLab(prReadTimeout, t.TempDir(), "version")
	if err == nil || err.Error() != "The GitLab CLI (glab) is not installed." {
		t.Errorf("err = %v", err)
	}
}
