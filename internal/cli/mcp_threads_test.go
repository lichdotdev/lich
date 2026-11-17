package cli

import (
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestMCPMarkThreadFixedPassesTheThreadAndItsCommit(t *testing.T) {
	f := newFakeLich(t, `null`)
	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"mark_thread_fixed","arguments":{"thread_id":"PRRT_1","commit":"abc1234"}}}`)
	if text, failed := textOf(t, replies[0]); failed {
		t.Fatalf("tool failed: %q", text)
	}
	last := f.calls[len(f.calls)-1]
	opts := optionsOf[api.MarkThreadFixedOptions](t, last)
	if last.method != api.ProjectMarkThreadFixed.Name || opts != (api.MarkThreadFixedOptions{ThreadID: "PRRT_1", Commit: "abc1234"}) {
		t.Errorf("call %s options = %+v, want PRRT_1 at abc1234", last.method, opts)
	}
}
