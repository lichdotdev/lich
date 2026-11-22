package project

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
)

// glabRunner runs one glab subcommand inside dir and returns its stdout. glab
// answers as its own active account: it has no per-project account the way gh
// does here. A field on Service so the GitLab flows run without a GitLab
// remote; production wiring is runGLab.
type glabRunner func(timeout time.Duration, dir string, args ...string) ([]byte, error)

// glabProbeTimeout bounds the local `glab config get` forgeOf asks.
const glabProbeTimeout = 3 * time.Second

// runGLab is runGH for glab: the call capped by timeout, a failure turned into
// the message the screen shows, glab's own words sent to the log.
func runGLab(timeout time.Duration, dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := commandContext(ctx, "glab", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, glabFailure(args, ghReport(stderr.String(), out), err, ctx.Err())
	}
	return out, nil
}

// glabFailure turns one failed glab call into the message the screen shows,
// errNoPullRequest for a branch with no open merge request. glab writes its
// error twice, on stderr and, under -F json, as {"error":...} on stdout, and
// `glab api` writes GitLab's body to stdout: report carries both.
func glabFailure(args []string, report string, runErr, ctxErr error) error {
	if strings.Contains(strings.ToLower(report), "no open merge request") {
		return errNoPullRequest
	}
	logToolFailure("glab", args, report, runErr)
	return errors.New(glabMessage(report, runErr, ctxErr))
}

const glabSignedOut = "glab is not signed in to GitLab. Run `glab auth login` in a terminal."

// glabFailures maps what glab reports onto what the person reading the screen
// can do about it. The merge refusals are glab's own checks (mrutils, glab
// 1.131), made before it calls GitLab.
var glabFailures = []toolFailure{
	{"401 unauthorized", glabSignedOut},
	{"glab auth login", glabSignedOut},
	{"404 project not found", "GitLab does not show this project to the account glab is signed in as."},
	{"404 not found", "GitLab has no merge request with that number, or does not show it to the account glab is signed in as."},
	{"403 forbidden", "The account glab is signed in as may not do that on this project."},
	{"not a git repository", "This folder is not a git repository."},
	{"still a draft", "This merge request is still a draft. Mark it ready on GitLab first."},
	{"requires a passing pipeline", "GitLab requires the pipeline to pass before this merge request can merge."},
	{"pipeline for this merge request has failed", "GitLab requires the pipeline to pass before this merge request can merge."},
	{"do not have permission to merge", "The account glab is signed in as may not merge this merge request."},
	{
		"merge conflicts exist",
		"This merge request conflicts with its target branch. Merge the target in and resolve them first.",
	},
	{"already been merged", "This merge request is already merged."},
	{"has been closed", "This merge request is closed."},
	{"405 method not allowed", "GitLab will not merge this merge request yet: it has conflicts or an unmet requirement."},
	{"406 not acceptable", "GitLab will not merge this merge request yet: it has conflicts or an unmet requirement."},
}

// glabMessage picks the message for one glab failure, falling back to a plain
// sentence that points at the log.
func glabMessage(report string, runErr, ctxErr error) string {
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		return "GitLab did not answer in time. Check the connection and try again."
	}
	if missingTool(report, runErr) {
		return "The GitLab CLI (glab) is not installed."
	}
	if message, ok := matchFailure(glabFailures, report); ok {
		return message
	}
	return "glab could not complete the request. lich's log has what it reported."
}

// errUnreadableGitLab is errUnreadableAnswer for a glab answer.
func errUnreadableGitLab(err error) error {
	slog.Warn("glab answer could not be decoded", "err", err)
	return errors.New("GitLab's answer could not be read.")
}
