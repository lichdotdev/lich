package project

import (
	"encoding/json"
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestURLHostReadsEveryRemoteShape(t *testing.T) {
	t.Parallel()
	for remote, want := range map[string]string{
		"git@gitlab.com:gitlab-org/cli.git":             "gitlab.com",
		"https://gitlab.com/gitlab-org/cli.git":         "gitlab.com",
		"ssh://git@GitLab.Example.org:2222/g/r.git":     "gitlab.example.org",
		"git+ssh://git@gitlab.example.org/g/r.git":      "gitlab.example.org",
		"https://gitlab.example.org:8443/g/r.git":       "gitlab.example.org:8443",
		"https://user:pw@github.com/o/r":                "github.com",
		"gitlab.example.org:g/r.git":                    "gitlab.example.org",
		"/home/me/origin":                               "",
		"./relative/path:with-colon":                    "",
		"file:///srv/git/r.git":                         "",
		`C:\repos\origin`:                               "",
		"https://%zz":                                   "",
		"ssh://git@gitlab.example.org/g/with:colon.git": "gitlab.example.org",
	} {
		if got := urlHost(remote); got != want {
			t.Errorf("urlHost(%q) = %q, want %q", remote, got, want)
		}
	}
}

// repoWithRemote is a repository on branch main whose remote names url.
func repoWithRemote(t *testing.T, name, url string) (string, func(args ...string) string) {
	t.Helper()
	repo, git := initRepo(t)
	git("remote", "add", name, url)
	return repo, git
}

func TestForgeOfReadsTheRemote(t *testing.T) {
	t.Parallel()
	probe := newFakeGLab().
		on("config get token --host gitlab.example.org", []byte("glpat-x\n")).
		on("config get token --host ghe.example.com", []byte("\n"))
	for _, tc := range []struct {
		name, remote, url, want string
	}{
		{"github.com", "origin", "git@github.com:o/r.git", api.ForgeGitHub},
		{"gitlab.com", "origin", "https://gitlab.com/gitlab-org/cli.git", api.ForgeGitLab},
		{"a host glab signed in to", "origin", "git@gitlab.example.org:g/r.git", api.ForgeGitLab},
		{"a host glab holds no token for", "origin", "https://ghe.example.com/o/r", api.ForgeGitHub},
		{"no origin: the first remote", "upstream", "https://gitlab.com/g/r", api.ForgeGitLab},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := repoWithRemote(t, tc.remote, tc.url)
			s := &Service{glab: probe.run}
			if got, _ := s.Forge(t.Context(), api.PathOptions{Path: repo}); got != tc.want {
				t.Errorf("forge = %q, want %q", got, tc.want)
			}
		})
	}
	t.Run("no remote", func(t *testing.T) {
		repo, _ := initRepo(t)
		if got := (&Service{glab: probe.run}).forgeOf(repo).kind(); got != api.ForgeGitHub {
			t.Errorf("forge = %q, want github", got)
		}
	})
	t.Run("origin wins over a remote listed before it", func(t *testing.T) {
		repo, git := repoWithRemote(t, "aaa", "https://gitlab.com/g/r")
		git("remote", "add", "origin", "git@github.com:o/r.git")
		if got := (&Service{glab: probe.run}).forgeOf(repo).kind(); got != api.ForgeGitHub {
			t.Errorf("forge = %q, want origin's github", got)
		}
	})
}

func TestGlabHostProbesOncePerHostAndWithoutGlab(t *testing.T) {
	t.Parallel()
	probe := newFakeGLab().fail("config get token --host h.example", errors.New("The GitLab CLI (glab) is not installed."))
	s := &Service{glab: probe.run}
	if s.glabHost("h.example") || s.glabHost("h.example") {
		t.Error("a host glab cannot answer for reads as GitLab")
	}
	if len(probe.calls) != 1 {
		t.Errorf("probes = %v, want one per host within the TTL", probe.calls)
	}
	if (&Service{}).glabHost("h.example") {
		t.Error("no glab wired reads as GitLab")
	}
}

// listRows is a glab list answer made of real rows of the fixture lists,
// picked by iid, with a row's fields overridden by its iid's entry.
func listRows(t *testing.T, iids []int, override map[int]map[string]any) []byte {
	t.Helper()
	var all []map[string]any
	for _, f := range []string{"list_open.json", "list_merged.json", "list_closed.json"} {
		var rows []map[string]any
		if err := json.Unmarshal(fixture(t, f), &rows); err != nil {
			t.Fatal(err)
		}
		all = append(all, rows...)
	}
	var picked []map[string]any
	for _, iid := range iids {
		for _, row := range all {
			if int(row["iid"].(float64)) == iid {
				maps.Copy(row, override[iid])
				picked = append(picked, row)
			}
		}
	}
	out, err := json.Marshal(picked)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

const branchList = "mr list --source-branch=main --per-page 20 -F json --all"

func TestBranchMergeRequestPicksTheCheckoutsOwn(t *testing.T) {
	t.Parallel()
	repo, git := repoWithRemote(t, "origin", "https://gitlab.com/gitlab-org/cli.git")
	head := strings.TrimSpace(git("rev-parse", "HEAD"))
	for _, tc := range []struct {
		name string
		list []byte
		want int
	}{
		{"a fork's open one this checkout does not hold is a stranger's", listRows(t, []int{4527, 4514}, nil), 4514},
		{"the open one before a newer closed one", listRows(t, []int{4495, 4512}, map[int]map[string]any{4512: {"state": "opened"}}), 4512},
		{"the newest when none is open", listRows(t, []int{4514, 4495}, nil), 4514},
		{"a fork's one whose head this checkout holds", listRows(t, []int{4527}, map[int]map[string]any{4527: {"sha": head}}), 4527},
		{"nothing of its own", listRows(t, []int{4527, 4526}, nil), 0},
		{"none at all", []byte("[]"), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Service{glab: newFakeGLab().on(branchList, tc.list).run}
			pr, err := s.BranchPullRequest(repo)
			if err != nil {
				t.Fatal(err)
			}
			got := 0
			if pr != nil {
				got = pr.Number
			}
			if got != tc.want {
				t.Errorf("branch merge request = !%d, want !%d (0: none)", got, tc.want)
			}
		})
	}
}

func TestBranchMergeRequestOfADetachedHeadIsNone(t *testing.T) {
	t.Parallel()
	repo, git := repoWithRemote(t, "origin", "https://gitlab.com/gitlab-org/cli.git")
	git("checkout", "--detach")
	g := newFakeGLab()
	if pr, err := (&Service{glab: g.run}).BranchPullRequest(repo); pr != nil || err != nil || len(g.calls) != 0 {
		t.Errorf("detached = %+v, %v after %v", pr, err, g.calls)
	}
}

func TestGitLabServiceFlows(t *testing.T) {
	t.Parallel()
	repo, _ := repoWithRemote(t, "origin", "https://gitlab.com/gitlab-org/cli.git")
	open := listRows(t, []int{4514}, map[int]map[string]any{4514: {"state": "opened"}})
	openList := "mr list --source-branch=main --per-page 20 -F json"
	g := detailGLab(t).on(openList, open).
		on("mr merge 4514 --yes --auto-merge=false --squash", nil).
		on("mr list --per-page 50 -F json --merged", fixture(t, "list_merged.json"))
	var changed []PRChanged
	s := &Service{glab: g.run, emit: func(_ string, data any) { changed = append(changed, data.(PRChanged)) }}

	if _, err := s.MergePullRequest(t.Context(), api.MergePullRequestOptions{Path: repo, Method: "squash"}); err != nil {
		t.Fatalf("merge of the branch's merge request: %v (%v)", err, g.calls)
	}
	if len(changed) != 1 || changed[0].Path != repo {
		t.Errorf("merge reported %+v", changed)
	}
	if d, err := s.PullRequestDetail(t.Context(), api.NumberOptions{Path: repo, Number: 4527}); err != nil || d == nil {
		t.Errorf("detail = %v, %v", d, err)
	}
	diff, err := s.PullRequestDiff(t.Context(), api.NumberOptions{Path: repo, Number: 4527})
	if err != nil || !strings.HasPrefix(diff, "diff --git") {
		t.Errorf("diff = %.40q, %v", diff, err)
	}
	list, err := s.ListPullRequests(t.Context(), api.ListPullRequestsOptions{Path: repo, State: "merged"})
	if err != nil || len(list) != 3 {
		t.Errorf("list = %v, %v", list, err)
	}
}

func TestGitLabServiceTurnsNoMergeRequestIntoNone(t *testing.T) {
	t.Parallel()
	repo, _ := repoWithRemote(t, "origin", "https://gitlab.com/gitlab-org/cli.git")
	g := newFakeGLab().on("mr list --source-branch=main --per-page 20 -F json", []byte("[]"))
	s := &Service{glab: g.run}
	if d, err := s.PullRequestDetail(t.Context(), api.NumberOptions{Path: repo}); d != nil || err != nil {
		t.Errorf("detail = %v, %v", d, err)
	}
	if diff, err := s.PullRequestDiff(t.Context(), api.NumberOptions{Path: repo}); diff != "" || err != nil {
		t.Errorf("diff = %q, %v", diff, err)
	}
}

func TestGitLabWatchReadFillsHeadAndChecks(t *testing.T) {
	t.Parallel()
	read := api.PRRead{Number: 4527, Open: true}
	if err := (gitlabForge{run: detailGLab(t).run}).watchRead("/w", &read); err != nil {
		t.Fatal(err)
	}
	if read.Head != "8de77cfeb0e714cb0a6fde52704fea0b6003639e" || read.Checks.Total != 13 || read.Review != "" {
		t.Errorf("read = %+v", read)
	}
}

func TestCreateWorktreeFromAForkMergeRequestThatTakesNoPush(t *testing.T) {
	t.Parallel()
	repo, _ := repoWithRemote(t, "origin", "https://gitlab.com/gitlab-org/cli.git")
	var mr map[string]any
	if err := json.Unmarshal(fixture(t, "mr4527.json"), &mr); err != nil {
		t.Fatal(err)
	}
	mr["allow_collaboration"] = false
	out, _ := json.Marshal(mr)
	g := newFakeGLab().on(view4527, out)
	_, err := (&Service{glab: g.run}).CreateWorktreeFromPR(t.Context(),
		api.CreateWorktreeFromPROptions{ProjectPath: repo, ProjectID: "p", Number: 4527})
	if err == nil || !strings.Contains(err.Error(), "Merge request !4527 comes from a fork") {
		t.Errorf("err = %v, want the GitLab refusal", err)
	}
}
