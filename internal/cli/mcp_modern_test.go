package cli

import (
	"encoding/json"
	"testing"
)

// modernMeta is the _meta a 2026-07-28 client puts on every request.
const modernMeta = `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}`

// modernResult is what every modern result carries beside its own fields.
type modernResult struct {
	ResultType string `json:"resultType"`
	Meta       struct {
		ServerInfo *struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"io.modelcontextprotocol/serverInfo"`
	} `json:"_meta"`
	TTLMs      *int64 `json:"ttlMs"`
	CacheScope string `json:"cacheScope"`
}

// check asserts the modern fields; an empty cacheScope means the result is not
// cacheable and must carry neither ttlMs nor cacheScope.
func (r modernResult) check(t *testing.T, cacheScope string) {
	t.Helper()
	if r.ResultType != "complete" {
		t.Errorf("resultType = %q, want complete", r.ResultType)
	}
	if r.Meta.ServerInfo == nil || r.Meta.ServerInfo.Name != "lich" {
		t.Errorf("_meta serverInfo = %+v, want lich", r.Meta.ServerInfo)
	}
	if cacheScope == "" {
		if r.TTLMs != nil || r.CacheScope != "" {
			t.Errorf("a result that is not cacheable carries ttlMs %v, cacheScope %q", r.TTLMs, r.CacheScope)
		}
		return
	}
	if r.TTLMs == nil || *r.TTLMs <= 0 {
		t.Errorf("ttlMs = %v, want a positive freshness", r.TTLMs)
	}
	if r.CacheScope != cacheScope {
		t.Errorf("cacheScope = %q, want %s", r.CacheScope, cacheScope)
	}
}

func TestMCPDiscoverDescribesTheServer(t *testing.T) {
	t.Parallel()
	f := newFakeLich(t, `null`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{`+modernMeta+`}}`)
	var result struct {
		modernResult
		SupportedVersions []string `json:"supportedVersions"`
		Capabilities      struct {
			Tools *struct{} `json:"tools"`
		} `json:"capabilities"`
		Instructions string `json:"instructions"`
	}
	resultOf(t, replies[0], &result)

	result.check(t, "private")
	if len(result.SupportedVersions) != 1 || result.SupportedVersions[0] != "2026-07-28" {
		t.Errorf("supportedVersions = %v", result.SupportedVersions)
	}
	if result.Capabilities.Tools == nil {
		t.Error("the server did not advertise tools")
	}
	if result.Instructions == "" {
		t.Error("discover carries no instructions, which initialize does")
	}
}

func TestMCPModernToolsListIsCacheable(t *testing.T) {
	t.Parallel()
	f := newFakeLich(t, `null`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{`+modernMeta+`}}`)
	var result struct {
		modernResult
		Tools []json.RawMessage `json:"tools"`
	}
	resultOf(t, replies[0], &result)

	result.check(t, "public")
	if len(result.Tools) == 0 {
		t.Error("the modern tools/list lists no tools")
	}
}

func TestMCPModernToolCallIsComplete(t *testing.T) {
	t.Parallel()
	f := newFakeLich(t, `[]`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{`+modernMeta+`,"name":"list_worktrees","arguments":{}}}`)
	var result modernResult
	resultOf(t, replies[0], &result)

	result.check(t, "")
	if _, failed := textOf(t, replies[0]); failed {
		t.Error("tool reported a failure")
	}
}

func TestMCPRefusesAnUnsupportedModernVersion(t *testing.T) {
	t.Parallel()
	f := newFakeLich(t, `null`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/list",
		"params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"1900-01-01"}}}`)
	reply := replies[0]
	if reply.Result != nil {
		t.Errorf("an unsupported version got a result: %v", reply.Result)
	}
	if reply.Error == nil || reply.Error.Code != -32022 || reply.Error.Message != "Unsupported protocol version" {
		t.Fatalf("error = %+v, want -32022 Unsupported protocol version", reply.Error)
	}
	raw, err := json.Marshal(reply.Error.Data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	if want := `{"requested":"1900-01-01","supported":["2026-07-28"]}`; string(raw) != want {
		t.Errorf("data = %s, want %s", raw, want)
	}
}

// initialize does not exist in the modern era, so a request that names a modern
// version cannot open a legacy session with it.
func TestMCPModernInitializeIsUnknown(t *testing.T) {
	t.Parallel()
	f := newFakeLich(t, `null`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{`+modernMeta+`}}`)
	reply := replies[0]
	if reply.Result != nil {
		t.Errorf("a modern initialize got a result: %v", reply.Result)
	}
	if reply.Error == nil || reply.Error.Code != -32601 {
		t.Errorf("error = %+v, want -32601", reply.Error)
	}
}

// A legacy client never asked for the modern fields and has never been measured
// against them, so it must not see them.
func TestMCPLegacyResultsCarryNoModernFields(t *testing.T) {
	t.Parallel()
	f := newFakeLich(t, `[]`)

	replies := speak(t, f,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_worktrees","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"ping"}`,
	)
	if len(replies) != 4 {
		t.Fatalf("got %d replies, want 4", len(replies))
	}
	for _, reply := range replies {
		var result map[string]json.RawMessage
		resultOf(t, reply, &result)
		for _, field := range []string{"resultType", "_meta", "ttlMs", "cacheScope"} {
			if _, ok := result[field]; ok {
				t.Errorf("legacy reply %s carries %s", reply.ID, field)
			}
		}
	}
}
