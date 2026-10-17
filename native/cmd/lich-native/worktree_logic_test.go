package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/lichclient"
)

func TestToBranchName(t *testing.T) {
	cases := map[string]string{
		"":                     "",
		"fix/footer":           "fix/footer",
		"Fix the footer quota": "fix-the-footer-quota",
		"a b c d e f g":        "a-b-c-d-e",
		"812 Footer quota shows the wrong account": "812-footer-quota-shows-the",
		"Corrigir ação do botão":                   "corrigir-ação-do-botão",
	}
	for in, want := range cases {
		if got := toBranchName(in); got != want {
			t.Errorf("toBranchName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseIssueRef(t *testing.T) {
	cases := map[string]int{
		"#812": 812, " #7 ": 7, "812": 0, "#0": 0, "fix #8": 0,
		"https://github.com/o/r/issues/42":     42,
		"https://github.com/o/r/issues/42#top": 42,
		"https://github.com/o/r/pull/42":       0,
	}
	for in, want := range cases {
		if got := parseIssueRef(in); got != want {
			t.Errorf("parseIssueRef(%q) = %d, want %d", in, got, want)
		}
	}
}

func doneCheck(f lichclient.BaseFreshness, err string) *baseCheck {
	c := &baseCheck{done: make(chan struct{}), freshness: f, err: err}
	close(c.done)
	return c
}

// TestStartPoint pins the rule the dialog promises: a local branch behind its
// upstream, with nothing of its own, starts from the upstream; every other
// case keeps the local branch.
func TestStartPoint(t *testing.T) {
	behind := lichclient.BaseFreshness{Upstream: "origin/main", Behind: 4}
	cases := []struct {
		name   string
		base   string
		check  *baseCheck
		carry  bool
		start  string
		remote bool
	}{
		{"behind", "local:main", doneCheck(behind, ""), false, "origin/main", true},
		{"behind but carrying", "local:main", doneCheck(behind, ""), true, "main", false},
		{"diverged", "local:main", doneCheck(lichclient.BaseFreshness{Upstream: "origin/main", Behind: 4, Ahead: 1}, ""), false, "main", false},
		{"up to date", "local:main", doneCheck(lichclient.BaseFreshness{Upstream: "origin/main"}, ""), false, "main", false},
		{"no upstream", "local:main", doneCheck(lichclient.BaseFreshness{}, ""), false, "main", false},
		{"fetch failed", "local:main", doneCheck(lichclient.BaseFreshness{Upstream: "origin/main", Behind: 4, FetchError: "offline"}, ""), false, "main", false},
		{"call failed", "local:main", doneCheck(lichclient.BaseFreshness{}, "boom"), false, "main", false},
		{"remote", "remote:origin/feat", nil, false, "origin/feat", true},
	}
	for _, c := range cases {
		start, remote := startPoint(c.base, c.check, c.carry)
		if start != c.start || remote != c.remote {
			t.Errorf("%s: startPoint = %q, %v; want %q, %v", c.name, start, remote, c.start, c.remote)
		}
	}
}

func TestFreshnessLine(t *testing.T) {
	pending := &baseCheck{done: make(chan struct{})}
	cases := []struct {
		name  string
		check *baseCheck
		carry bool
		text  string
		warn  bool
	}{
		{"pending", pending, false, "Checking the remote…", false},
		{"behind", doneCheck(lichclient.BaseFreshness{Upstream: "origin/main", Behind: 4}, ""), false, "4 behind origin/main · starts from origin/main", false},
		{"carry", doneCheck(lichclient.BaseFreshness{Upstream: "origin/main", Behind: 4}, ""), true, "Starts from local main to keep your changes", false},
		{"ahead", doneCheck(lichclient.BaseFreshness{Upstream: "origin/main", Ahead: 2}, ""), false, "2 commits not pushed · starts from local main", false},
		{"offline", doneCheck(lichclient.BaseFreshness{Upstream: "origin/main", FetchError: "x"}, ""), false, "Couldn't reach origin · starts from local main", true},
		{"up to date", doneCheck(lichclient.BaseFreshness{Upstream: "origin/main"}, ""), false, "", false},
	}
	for _, c := range cases {
		text, warn := freshnessLine("main", c.check, c.carry)
		if text != c.text || warn != c.warn {
			t.Errorf("%s: freshnessLine = %q, %v; want %q, %v", c.name, text, warn, c.text, c.warn)
		}
	}
}
