package terminal

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestScanCodexContextUsage(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	body := `{"type":"turn_context","payload":{"model":"gpt-5.6-sol","effort":"high"}}` + "\n" +
		`{"type":"response_item","payload":{"text":"` + strings.Repeat("x", 2*int(codexScanBlockBytes)) + `"}}` + "\n" +
		`not json` + "\n" +
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":299000,"cached_input_tokens":250000,"cache_write_input_tokens":0,"output_tokens":1000,"total_tokens":300000},"model_context_window":258400}}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"token_count","info":null}}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	got, ok := scanCodexContextUsage(path)
	if !ok {
		t.Fatal("scanCodexContextUsage: want ok, got false")
	}
	want := contextUsage{
		tokens: 300000, percent: 100, window: 258400, model: "gpt-5.6-sol", effort: "high",
		cache: &api.SessionCache{Read: 250000, Write: 0, Uncached: 49000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("usage = %+v, want %+v", got, want)
	}
}

// Both cache counters are part of input_tokens, so the uncached share is what
// is left of it after the two.
func TestCodexCacheSplit(t *testing.T) {
	t.Parallel()
	got := codexCache(codexTokenUsage{Input: 100, Cached: 40, CacheWrite: 60})
	want := &api.SessionCache{Read: 40, Write: 60, Uncached: 0}
	if *got != *want {
		t.Errorf("cache = %+v, want %+v", *got, *want)
	}
	got = codexCache(codexTokenUsage{Input: 1000, Cached: 400, CacheWrite: 25})
	if got.Uncached != 575 {
		t.Errorf("uncached = %d, want 575", got.Uncached)
	}
	if got := codexCache(codexTokenUsage{Input: 10, Cached: 20}); got.Uncached != 0 {
		t.Errorf("cached past input: uncached = %d, want 0", got.Uncached)
	}
}

func TestCodexContextUsageMisses(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`{"type":"event_msg","payload":{"type":"token_count","info":null}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"model_context_window":258400}}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":-1},"model_context_window":258400}}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":1},"model_context_window":0}}}`,
	} {
		path := filepath.Join(t.TempDir(), "rollout.jsonl")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, ok := scanCodexContextUsage(path); ok {
			t.Errorf("scanCodexContextUsage(%s): want not-ok", body)
		}
	}
	if _, ok := scanCodexContextUsage(filepath.Join(t.TempDir(), "missing.jsonl")); ok {
		t.Error("a missing rollout should be not-ok")
	}
}
