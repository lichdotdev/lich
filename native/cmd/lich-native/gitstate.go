package main

import (
	"context"
	"log"
	"slices"

	"github.com/lichdotdev/lich/native/backend"
)

// baseStanding is what a card draws of a base status: how many files would
// conflict with the base, else how many commits it is behind. The zero value
// draws nothing.
type baseStanding struct {
	conflict bool
	count    int
}

func sameBase(a, b *backend.BaseStatus) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Base == b.Base && a.Behind == b.Behind && slices.Equal(a.Conflicts, b.Conflicts)
}

func standingOf(b *backend.BaseStatus) baseStanding {
	if b == nil {
		return baseStanding{}
	}
	if n := len(b.Conflicts); n > 0 {
		return baseStanding{conflict: true, count: n}
	}
	return baseStanding{count: b.Behind}
}

// prLookup is a path's pull request as last asked for at a branch and head.
// The pr survives a new commit on the same branch, so the badge does not blink,
// and is dropped with a new branch.
type prLookup struct {
	branch, head string
	pr           *backend.PullRequest
}

// refreshPath re-reads a shown path's changes and base standing, and asks the
// backend to read its pull request again when the branch or head moved: a
// merge that closed one shows without waiting. The backend reads an open one
// again on its own (internal/project branchpr.go); one the session just opened,
// the window asks for at the end of its turn (recheckMissingPR).
func (m *model) refreshPath(ctx context.Context, path string) {
	d, err := m.client.Diff(ctx, path)
	if err != nil {
		m.forgetIfGone(ctx, path)
		return
	}
	b, err := m.client.BaseStatus(ctx, path)
	if err != nil {
		return
	}
	m.mu.Lock()
	changed := m.git[path] != d || !sameBase(m.base[path], b)
	m.git[path], m.base[path] = d, b
	prev, looked := m.prs[path]
	ask := !looked || prev.branch != d.Branch || prev.head != d.Head
	if ask {
		next := prLookup{branch: d.Branch, head: d.Head}
		if prev.branch == d.Branch {
			next.pr = prev.pr
		}
		changed = changed || next.pr != prev.pr
		m.prs[path] = next
	}
	m.mu.Unlock()
	if ask {
		go m.recheckPR(ctx, path)
	}
	if changed {
		m.invalidate()
	}
}

func (m *model) recheckPR(ctx context.Context, path string) {
	if err := m.client.RecheckPullRequest(ctx, path); err != nil {
		log.Printf("recheck the pull request of %s: %v", path, err)
	}
}

// setBranchPR keeps a read of a shown path's pull request; the badge shows only
// an open one. A read of a path no card shows is dropped: the window asks for
// one when it first shows it.
func (m *model) setBranchPR(b backend.BranchPullRequest) {
	pr := b.PR
	if pr != nil && pr.State != "OPEN" {
		pr = nil
	}
	m.mu.Lock()
	l, shown := m.prs[b.Path]
	if shown {
		l.pr = pr
		m.prs[b.Path] = l
	}
	m.mu.Unlock()
	if shown {
		m.invalidate()
	}
}

// stalePullRequests makes the next git tick ask for a new read of every path,
// keeping the badges up meanwhile: a PR opened or merged in the browser
// shows when the user comes back. Callers hold m.mu.
func (m *model) stalePullRequests() {
	for path, l := range m.prs {
		l.head = ""
		m.prs[path] = l
	}
}

// recheckMissingPR forgets what was read for the card of a session whose
// turn just finished, when it shows no pull request: `gh pr create` moves
// neither branch nor head, so nothing else would bring up the PR the turn
// opened. It answers the path to refresh, "" when there is none to ask about
// again. Callers hold m.mu.
func (m *model) recheckMissingPR(id string) string {
	p, i := m.sessionAt(id)
	if p == nil {
		return ""
	}
	path := m.shownPath(*p, p.Sessions[i])
	l, looked := m.prs[path]
	if !looked || l.pr != nil {
		return ""
	}
	l.head = ""
	m.prs[path] = l
	return path
}

// forgetIfGone drops what is known of a path that no longer exists. Any other
// failure (git busy, a lock, the backend restarting) keeps the last reading,
// which the next tick replaces.
func (m *model) forgetIfGone(ctx context.Context, path string) {
	m.mu.Lock()
	_, known := m.git[path]
	m.mu.Unlock()
	if !known {
		return
	}
	there, err := m.client.PathExists(ctx, path)
	if err != nil || there {
		return
	}
	m.mu.Lock()
	m.dropPath(path)
	m.mu.Unlock()
	m.invalidate()
}

// forgetUntracked drops the readings of paths no open project can show any
// more: a session that closed, or a shell that left a directory for good. A
// path of another open project stays, so coming back to it draws the last
// reading while the next tick replaces it.
func (m *model) forgetUntracked() {
	m.mu.Lock()
	defer m.mu.Unlock()
	keep := map[string]bool{}
	for _, p := range m.projects {
		keep[p.Path] = true
		for _, s := range p.Sessions {
			keep[m.shownPath(p, s)] = true
		}
	}
	for path := range m.git {
		if !keep[path] {
			m.dropPath(path)
		}
	}
}

// dropPath forgets everything the store holds for path. Callers hold m.mu.
func (m *model) dropPath(path string) {
	delete(m.git, path)
	delete(m.base, path)
	delete(m.prs, path)
}
