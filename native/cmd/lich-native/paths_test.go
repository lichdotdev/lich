package main

import "testing"

func TestShortPathKeepsTheLastThreeParts(t *testing.T) {
	cases := []struct{ path, want string }{
		{"/home/u/try/lich-playground", "~/try/lich-playground"},
		{"/home/u", "~"},
		{"/home/u/.local/share/lich/worktrees/e48f04f46a4a/calm-raven", "…worktrees/e48f04f46a4a/calm-raven"},
		{"/srv/a/b/c", "…a/b/c"},
		{`C:\Users\u\.local\share\lich\worktrees\e48f\calm-raven`, `…worktrees\e48f\calm-raven`},
		{`C:\Users\u\code\lich`, `~\code\lich`},
	}
	for _, c := range cases {
		if got := shortPath(c.path); got != c.want {
			t.Errorf("shortPath(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
