package main

import (
	"context"
	"log"
	"slices"
	"time"

	"github.com/lichdotdev/lich/native/lichclient"
)

// baseStanding is the part of base-status.ts baseReadout a card draws: how
// many files would conflict with the base, else how many commits it is
// behind. The zero value draws nothing.
type baseStanding struct {
	conflict bool
	count    int
}

// sameBase is git-status-store.ts sameBase.
func sameBase(a, b *lichclient.BaseStatus) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Base == b.Base && a.Behind == b.Behind && slices.Equal(a.Conflicts, b.Conflicts)
}

func standingOf(b *lichclient.BaseStatus) baseStanding {
	if b == nil {
		return baseStanding{}
	}
	if n := len(b.Conflicts); n > 0 {
		return baseStanding{conflict: true, count: n}
	}
	return baseStanding{count: b.Behind}
}

// prRecheck is how long an open pull request is believed before gh is asked
// again: a merge or close on GitHub moves neither branch nor head, so nothing
// else would ever make the badge go.
const prRecheck = time.Minute

// prLookup is a path's pull request as asked for at a branch and head. The
// pr survives a new commit on the same branch, so the badge does not blink,
// and is dropped with a new branch (use-pull-request.ts resetOn).
type prLookup struct {
	branch, head string
	pr           *lichclient.PullRequest
	asked        time.Time
}

// refreshPath re-reads a shown path's changes and base standing, and asks gh
// for its pull request again when the branch or head moved: a merge that
// closed one shows without waiting. An open one is asked about again every
// prRecheck; one the session just opened, at the end of its turn
// (recheckMissingPR).
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
	now := time.Now()
	ask := !looked || prev.branch != d.Branch || prev.head != d.Head ||
		(prev.pr != nil && now.Sub(prev.asked) >= prRecheck)
	if ask {
		next := prLookup{branch: d.Branch, head: d.Head, asked: now}
		if prev.branch == d.Branch {
			next.pr = prev.pr
		}
		changed = changed || next.pr != prev.pr
		m.prs[path] = next
	}
	m.mu.Unlock()
	if ask {
		go m.lookupPR(ctx, path, d.Branch, d.Head)
	}
	if changed {
		m.invalidate()
	}
}

// lookupPR stores gh's answer unless the checkout moved on while it was
// asked. A failed lookup (no gh, not a GitHub repo) reads as no PR.
func (m *model) lookupPR(ctx context.Context, path, branch, head string) {
	pr, err := m.client.PullRequest(ctx, path)
	if err != nil {
		log.Printf("pull request of %s: %v", path, err)
		pr = nil
	}
	m.mu.Lock()
	cur := m.prs[path]
	current := cur.branch == branch && cur.head == head
	if current {
		cur.pr = pr
		m.prs[path] = cur
	}
	m.mu.Unlock()
	if current {
		m.invalidate()
	}
}

// stalePullRequests makes the next git tick ask gh again for every path,
// keeping the badges up meanwhile: a PR opened or merged in the browser
// shows when the user comes back. Callers hold m.mu.
func (m *model) stalePullRequests() {
	for path, l := range m.prs {
		l.head = ""
		m.prs[path] = l
	}
}

// recheckMissingPR forgets what gh answered for the card of a session whose
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
