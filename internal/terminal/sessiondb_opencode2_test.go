package terminal

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/store"
)

// plantOpencode2DB is the database opencode 2.2.5 leaves after running over a
// 1.19.4 one: the 1.x tables frozen as they were, and session_v2 beside them
// holding a copy of the 1.x conversation it continued ('ses_both'), a
// conversation of its own ('ses_new') with a sub-agent under it, and nothing
// for the 1.x conversation it never touched ('ses_old'). Each 1.x row the copy
// shadows says something different from the copy, so a read that picks the
// wrong table shows.
func plantOpencode2DB(t *testing.T) string {
	t.Helper()
	home := conversationHome(t)
	path := filepath.Join(home, ".local", "share", "opencode", opencodeDBFile)
	plantDB(t, path,
		`CREATE TABLE session (id TEXT PRIMARY KEY, parent_id TEXT, directory TEXT, title TEXT,
		   time_updated INTEGER, time_archived INTEGER, cost REAL)`,
		`CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT, time_created INTEGER, data TEXT)`,
		`CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT, session_id TEXT, time_created INTEGER, data TEXT)`,
		`INSERT INTO session VALUES
		   ('ses_both', NULL, '/work/alpha', '1.x title', 1000, NULL, 9),
		   ('ses_old', NULL, '/work/beta', 'Old chat', 2000, NULL, 0.5)`,
		`INSERT INTO message VALUES ('m1', 'ses_old', 1, '{"role":"assistant"}'),
		   ('m2', 'ses_both', 2, '{"role":"assistant"}')`,
		`INSERT INTO part VALUES ('p1', 'm1', 'ses_old', 1, '{"type":"text","text":"frozen 1.x words"}'),
		   ('p2', 'm2', 'ses_both', 2, '{"type":"text","text":"stale 1.x words"}')`,
		`CREATE TABLE session_v2 (id TEXT PRIMARY KEY, parent_id TEXT, directory TEXT, title TEXT,
		   cost REAL, time_updated INTEGER, time_archived INTEGER)`,
		`CREATE TABLE session_message (id TEXT PRIMARY KEY, session_id TEXT, type TEXT, seq INTEGER,
		   time_created INTEGER, data TEXT)`,
		`INSERT INTO session_v2 VALUES
		   ('ses_both', NULL, '/work/alpha', 'Simple hi reply', 0.25, 3000, NULL),
		   ('ses_new', NULL, '/work/alpha', NULL, 0.5, 4000, NULL),
		   ('ses_sub', 'ses_new', '/work/alpha', 'task', 0.125, 4000, NULL),
		   ('ses_gone', NULL, '/work/alpha', 'archived', 0, 4000, 5)`,
		`INSERT INTO session_message VALUES
		   ('s1', 'ses_new', 'user', 0, 1, '{"text":"port the worktree hash"}'),
		   ('s2', 'ses_new', 'assistant', 1, 2, '{"content":[{"type":"reasoning","text":"thinking about the worktree"},
		     {"type":"text","text":"An earlier worktree answer."},{"type":"tool","name":"read"}],
		     "time":{"completed":3},"tokens":{"input":9,"cache":{"read":9,"write":9}}}'),
		   ('s3', 'ses_new', 'agent-switched', 2, 3, '{"agent":"build"}'),
		   ('s4', 'ses_new', 'assistant', 3, 4, '{"content":[{"type":"text","text":"Hashed."},
		     {"type":"text","text":"The worktree port is a hash."}],
		     "time":{"completed":5},"tokens":{"input":182,"cache":{"read":5760,"write":0}}}'),
		   ('s5', 'ses_new', 'assistant', 4, 5, '{"content":[],"time":{"created":5}}'),
		   ('s6', 'ses_sub', 'assistant', 0, 6, '{"content":[{"type":"text","text":"a sub-agent worktree line"}]}'),
		   ('s7', 'ses_both', 'assistant', 0, 7, '{"content":[{"type":"text","text":"bye"}]}')`,
	)
	return path
}

func TestOpencode2Conversations(t *testing.T) {
	plantOpencode2DB(t)
	assertConversations(t, opencodeConversations(),
		store.Conversation{Kind: "opencode", ID: "ses_both", Title: "Simple hi reply", Cwd: "/work/alpha", Profile: "personal"},
		store.Conversation{Kind: "opencode", ID: "ses_new", Title: "", Cwd: "/work/alpha", Profile: "personal"},
		store.Conversation{Kind: "opencode", ID: "ses_old", Title: "Old chat", Cwd: "/work/beta", Profile: "personal"},
	)
}

func opencode2Source(t *testing.T, id string) usageSource {
	t.Helper()
	src, ok := usageSourceFor(id, "")
	if !ok || src.kind != providers.OpenCode {
		t.Fatalf("usageSourceFor(%s) = %+v, %v", id, src, ok)
	}
	return src
}

func TestOpencode2ReadsEachConversationFromItsGeneration(t *testing.T) {
	plantOpencode2DB(t)
	cases := []struct {
		id   string
		cost float64
		said string
	}{
		// The sub-agent's spend is walked into its parent, as on 1.x.
		{"ses_new", 0.625, "The worktree port is a hash."},
		{"ses_both", 0.25, "bye"},
		{"ses_old", 0.5, "frozen 1.x words"},
	}
	for _, tc := range cases {
		src := opencode2Source(t, tc.id)
		if cost, ok := sessionDBCost(src.path, queriesFor(src).cost, src.id); !ok || cost != tc.cost {
			t.Errorf("%s: cost = %v, %v, want %v", tc.id, cost, ok, tc.cost)
		}
		if got := sessionDBTexts(src.path, queriesFor(src).said, src.id); len(got) != 1 || got[0] != tc.said {
			t.Errorf("%s: said = %q, want %q", tc.id, got, tc.said)
		}
	}
}

func TestOpencode2SearchReadsBothSidesInOrder(t *testing.T) {
	plantOpencode2DB(t)
	src := opencode2Source(t, "ses_new")
	got := sessionDBTexts(src.path, queriesFor(src).search, src.id)
	want := []string{"port the worktree hash", "An earlier worktree answer.", "Hashed.", "The worktree port is a hash."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("search texts = %q, want %q", got, want)
	}
}

func TestOpencode2LastTurnCache(t *testing.T) {
	plantOpencode2DB(t)
	got := lastTurnCache(opencode2Source(t, "ses_new"))
	want := api.SessionCache{Read: 5760, Uncached: 182}
	if got == nil || *got != want {
		t.Errorf("cache = %+v, want %+v", got, want)
	}
}
