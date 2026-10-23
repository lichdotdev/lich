package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/lichdotdev/lich/api"
)

type ClosedSession = api.ParkedSession

type ClosedHistory = api.ClosedHistory

// closedSessionLimit caps the history handed to the window — how many rows one
// call answers with, not how far back it looks: the search runs in the query,
// so a match older than the newest hundred is still found. The alternative is
// handing over every session ever closed to draw a list nobody scrolls to the
// end of.
const closedSessionLimit = 100

// likeEscape is the ESCAPE character the searches declare, so a name containing
// % or _ searches for those characters instead of matching anything.
const likeEscape = `\`

// escapeLike neutralises the LIKE wildcards in a user's search term. The escape
// character goes first, or escaping the wildcards would re-escape it.
func escapeLike(term string) string {
	term = strings.ReplaceAll(term, likeEscape, likeEscape+likeEscape)
	term = strings.ReplaceAll(term, "%", likeEscape+"%")
	return strings.ReplaceAll(term, "_", likeEscape+"_")
}

// ClosedSessions returns the parked sessions (is_open = 0) matching opts.Term, the
// last one closed first, up to closedSessionLimit of them. An empty opts.Term is the
// plain history: the most recently closed, whatever they are named. Sessions of
// closed projects answer too: a project hidden from the tab strip still owns the
// work done in it, and resuming one of its sessions is what reopens it.
//
// A row answers on either of two matches, and the OR between them is the whole
// point: every whitespace-separated word appearing in the name, the project's
// name, the path or the branch it was parked on (the same reading the palette's
// own filter gives a query), or the conversation itself mentioning them, out of
// the FTS index the close wrote (transcripts.go). A row matched on its
// conversation carries the Snippet that made it one, and one matched by name
// carries none.
//
// Both are matched here rather than in the window because the window only ever
// sees one page of rows, and a session parked further back than that page would
// be unfindable. LIKE is case-insensitive for ASCII in SQLite, which is what
// makes the name half a search and not a prefix test.
//
// Total counts the whole match, not the page: without it an opts.Term matching more
// parked sessions than fit would answer with closedSessionLimit rows and no way
// for the list to say so. Indexing counts the parked sessions the backfill has
// not reached yet, which is the other way this answer can be short; asking for
// one with an opts.Term is what starts that backfill.
//
// A conversation is listed once, by its newest park, and not at all while a
// card has it open (newestOfItsConversation): its copies, left by a `/resume`
// typed into another card, all reopen the same transcript.
//
// rowid is the tiebreak, not the order, for RecentProjects' reason twice over:
// it dates the insert, and a resumed session is reinserted — so rows parked
// before closed_at existed all carry 0 and fall back to it together.
func (s *Service) ClosedSessions(_ context.Context, opts api.SearchTermOptions) (ClosedHistory, error) {
	if strings.TrimSpace(opts.Term) != "" {
		// Started on any term, not only one the index can be asked about: the
		// count of what is still unindexed goes back with every answer, and a
		// term made of punctuation alone would otherwise leave the list
		// reporting a backfill nothing had started.
		s.backfill()
	}
	match := closedSessionsMatch(opts.Term)

	// Counted before the rows are asked for, never while they are being walked:
	// the store holds a single connection, and a second query opened inside the
	// first one's iteration waits for a connection the first will not give back.
	indexing := s.unindexed()

	// COUNT(*) OVER () rides along on every row rather than costing a second
	// query: a window function is computed before LIMIT, so it counts the match
	// and not the page.
	rows, err := s.db.Query(
		match.with+`
		 SELECT s.id, s.project_id, p.name, p.path, s.label, s.kind, s.path,
		        s.parked_branch, s.closed_at, `+match.said+`,
		        COALESCE(x.truncated, 0), COUNT(*) OVER ()
		   FROM sessions s JOIN projects p ON p.id = s.project_id
		   LEFT JOIN session_texts x ON x.session_id = s.id
		   `+match.join+`
		   `+match.where+`
		  ORDER BY s.closed_at DESC, s.rowid DESC
		  LIMIT ?`,
		append(match.args, closedSessionLimit)...,
	)
	if err != nil {
		return ClosedHistory{}, fmt.Errorf("query closed sessions: %w", err)
	}
	defer rows.Close()

	history := ClosedHistory{Sessions: []ClosedSession{}, Indexing: indexing}
	var said []string
	for rows.Next() {
		var c ClosedSession
		if err := rows.Scan(
			&c.ID, &c.ProjectID, &c.ProjectName, &c.ProjectPath,
			&c.Label, &c.Kind, &c.Path, &c.ParkedBranch, &c.ClosedAt,
			&c.MatchedConversation, &c.Truncated, &history.Total,
		); err != nil {
			return ClosedHistory{}, fmt.Errorf("scan closed session: %w", err)
		}
		if c.MatchedConversation {
			said = append(said, c.ID)
		}
		history.Sessions = append(history.Sessions, c)
	}
	if err := rows.Err(); err != nil {
		return ClosedHistory{}, fmt.Errorf("iterate closed sessions: %w", err)
	}
	// After the rows are drained, for the single connection's reason above, and
	// only for the page: cutting a snippet costs the conversation it is cut out
	// of, and the match can be wider than the list.
	s.attachSnippets(history.Sessions, said, opts.Term)
	return history, nil
}

// sessionMatch is the SQL a term turns into, in the pieces ClosedSessions
// splices around its SELECT. args are bound in the order the pieces name them:
// the CTE's MATCH first, then one LIKE per word.
type sessionMatch struct {
	with, join, where string
	// said is the column reporting whether a row was matched on its
	// conversation. A term nothing could be searched for has no m to read, so
	// it reads 0 and no conversation is opened for a snippet.
	said string
	args []any
}

// newestOfItsConversation keeps a parked row s that runs no conversation, or
// whose conversation no other row holds open or parked after it, in the same
// order ClosedSessions sorts by.
// provider_session_id has no index, so this scans the table once per parked
// row: 18 ms over a 900-row workspace (measured 2026-10-17).
const newestOfItsConversation = `(s.provider_session_id = '' OR NOT EXISTS (
	SELECT 1 FROM sessions o
	 WHERE o.provider_session_id = s.provider_session_id AND o.id <> s.id
	   AND (o.is_open = 1 OR o.closed_at > s.closed_at OR (o.closed_at = s.closed_at AND o.rowid > s.rowid))))`

// closedSessionsMatch is the OR the search is built around, as SQL: every word
// in the name half, or the conversation in the index. An empty term is no
// condition at all beyond the row being parked.
//
// The match is MATERIALIZED, and not a subquery inlined into the join: without
// it SQLite runs the full-text search again for every parked row it walks,
// which on a workspace of two hundred sessions costs tens of seconds instead of
// tens of milliseconds.
func closedSessionsMatch(term string) sessionMatch {
	m := sessionMatch{where: "WHERE s.is_open = 0 AND " + newestOfItsConversation, said: "0", args: []any{}}
	if fts := ftsQuery(term); fts != "" {
		m.with = `WITH said AS MATERIALIZED (
		          SELECT t.session_id FROM session_texts t
		            JOIN session_texts_fts f ON f.rowid = t.rowid
		           WHERE session_texts_fts MATCH ?)`
		m.join = "LEFT JOIN said m ON m.session_id = s.id"
		m.said = "m.session_id IS NOT NULL"
		m.args = append(m.args, fts)
	}
	names := strings.Fields(term)
	if len(names) == 0 {
		return m
	}
	clauses := make([]string, 0, len(names))
	for _, word := range names {
		clauses = append(clauses,
			"(s.label || ' ' || p.name || ' ' || s.path || ' ' || s.parked_branch)"+
				" LIKE ? ESCAPE '"+likeEscape+"'")
		m.args = append(m.args, "%"+escapeLike(word)+"%")
	}
	matched := "(" + strings.Join(clauses, " AND ") + ")"
	if m.join != "" {
		matched += " OR m.session_id IS NOT NULL"
	}
	m.where += " AND (" + matched + ")"
	return m
}
