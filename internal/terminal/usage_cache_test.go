// Same tag as usage_tokenonly_test.go, whose fixtures and stubBins this reuses.
//go:build !windows

package terminal

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/providers"
)

// ompCachedAssistant is an oh-my-pi assistant line with a cache split and a cost.
func ompCachedAssistant(id string, input, read, write int) string {
	return fmt.Sprintf(
		`{"type":"message","id":%q,"message":{"role":"assistant","model":"rig/one",`+
			`"usage":{"input":%d,"output":26,"cacheRead":%d,"cacheWrite":%d,"cost":{"total":0.01}}}}`,
		id, input, read, write,
	)
}

func TestOMPLastTurnCacheReadsTheNewestAssistantLine(t *testing.T) {
	writeOMPSession(t, "omp-1",
		ompCachedAssistant("a1", 9, 9, 9),
		ompCachedAssistant("a2", 621, 36352, 12),
		`{"type":"message","id":"u1","message":{"role":"user","content":"next"}}`,
	)
	src, ok := usageSourceFor("omp-1", "")
	if !ok || src.kind != providers.OMP {
		t.Fatalf("usageSourceFor = %+v, %v", src, ok)
	}
	got := lastTurnCache(src)
	want := api.SessionCache{Read: 36352, Write: 12, Uncached: 621}
	if got == nil || *got != want {
		t.Errorf("cache = %+v, want %+v", got, want)
	}
}

func TestOMPLastTurnCacheMisses(t *testing.T) {
	writeOMPSession(t, "omp-1", `{"type":"message","id":"u1","message":{"role":"user","content":"hi"}}`)
	src, _ := usageSourceFor("omp-1", "")
	if got := lastTurnCache(src); got != nil {
		t.Errorf("no assistant line: cache = %+v, want nil", got)
	}
	if got := ompLastTurnCache("/does/not/exist.jsonl"); got != nil {
		t.Errorf("missing file: cache = %+v, want nil", got)
	}
}

// writeOpenCodeMessages adds opencode's message table beside the session rows,
// each message as the JSON opencode keeps in its data column.
func writeOpenCodeMessages(t *testing.T, rows ...[3]string) {
	t.Helper()
	path, _ := opencodeSessionDB()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT, time_created INTEGER, data TEXT)`); err != nil {
		t.Fatal(err)
	}
	for i, r := range rows {
		if _, err := db.Exec(`INSERT INTO message VALUES (?, ?, ?, ?)`, r[0], r[1], i, r[2]); err != nil {
			t.Fatal(err)
		}
	}
}

func opencodeAssistant(input, read, write int, completed bool) string {
	done := ""
	if completed {
		done = `,"completed":2`
	}
	return fmt.Sprintf(`{"role":"assistant","time":{"created":1%s},`+
		`"tokens":{"input":%d,"output":26,"cache":{"read":%d,"write":%d}}}`, done, input, read, write)
}

func TestOpenCodeLastTurnCacheReadsTheNewestFinishedTurn(t *testing.T) {
	writeOpenCodeDB(t, sessionRow{id: "ses_one", cost: 0.02}, sessionRow{id: "ses_two", parent: "ses_one"})
	writeOpenCodeMessages(t,
		[3]string{"m1", "ses_one", opencodeAssistant(1, 1, 1, true)},
		[3]string{"m2", "ses_one", opencodeAssistant(621, 36352, 0, true)},
		[3]string{"m3", "ses_one", `{"role":"user","time":{"created":3}}`},
		[3]string{"m4", "ses_two", opencodeAssistant(5, 5, 5, true)}, // a sub-agent's own window
		[3]string{"m5", "ses_one", opencodeAssistant(0, 0, 0, false)},
	)
	src, ok := usageSourceFor("ses_one", "")
	if !ok || src.kind != providers.OpenCode {
		t.Fatalf("usageSourceFor = %+v, %v", src, ok)
	}
	got := lastTurnCache(src)
	want := api.SessionCache{Read: 36352, Uncached: 621}
	if got == nil || *got != want {
		t.Errorf("cache = %+v, want %+v", got, want)
	}
	if got := opencodeLastTurnCache(src.path, ""); got != nil {
		t.Errorf("no id: cache = %+v, want nil", got)
	}
}

// A database without the message table is a schema that moved: no line, and
// the cost beside it still reports.
func TestOpenCodeLastTurnCacheWithoutAMessageTable(t *testing.T) {
	writeOpenCodeDB(t, sessionRow{id: "ses_one", cost: 0.02})
	src, _ := usageSourceFor("ses_one", "")
	if got := lastTurnCache(src); got != nil {
		t.Errorf("cache = %+v, want nil", got)
	}
}

func TestTheCostOnlyRungCarriesTheCache(t *testing.T) {
	writeOMPSession(t, "omp-1", ompCachedAssistant("a1", 412, 118020, 2310))
	svc := New(newCostStore("omp-1"), nil, events.New())
	svc.prices = testRate
	svc.spawns.Store("s1", spawn{kind: providers.OMP})

	got, ok := svc.sessionUsage("s1")
	if !ok || got.Window != 0 {
		t.Fatalf("sessionUsage = %+v, %v; want a cost-only report", got, ok)
	}
	want := api.SessionCache{Read: 118020, Write: 2310, Uncached: 412}
	if got.Cache == nil || *got.Cache != want {
		t.Errorf("Cache = %+v, want %+v", got.Cache, want)
	}
}
