package cli

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

const twoTabs = `[{"id":"T0","url":"https://example.com/a","title":"A"},{"id":"T1","url":"https://example.com/b","title":"B","front":true}]`

func tabsLich(t *testing.T) *fakeLich {
	t.Helper()
	f := newFakeLich(t, `{"url":"https://example.com/a","title":"A"}`)
	f.answers = map[string]answer{api.BrowserTabs.Name: {status: 200, body: twoTabs}}
	return f
}

func TestMCPBrowserTabSelectsTheTabAtItsIndex(t *testing.T) {
	f := tabsLich(t)
	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"browser_tab","arguments":{"action":"select","index":0}}}`)
	text, failed := textOf(t, replies[0])
	if failed || !strings.Contains(text, "title: A") {
		t.Fatalf("tool answered %q (failed %v)", text, failed)
	}
	last := f.calls[len(f.calls)-1]
	opts := optionsOf[api.BrowserTabOptions](t, last)
	if last.method != api.BrowserTabAction.Name || opts.ID != "s1" || opts.Action != api.TabSelect || opts.Tab != "T0" {
		t.Errorf("call %s options = %+v, want select of T0", last.method, opts)
	}
}

func TestMCPBrowserTabRefusesAnIndexPastTheTabs(t *testing.T) {
	f := tabsLich(t)
	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"browser_tab","arguments":{"action":"close","index":2}}}`)
	text, failed := textOf(t, replies[0])
	if !failed || !strings.Contains(text, "no tab at index 2") {
		t.Fatalf("tool answered %q (failed %v), want the index refused", text, failed)
	}
	for _, call := range f.calls {
		if call.method == api.BrowserTabAction.Name {
			t.Fatal("a tab action went out for an index past the tabs")
		}
	}
}

func TestMCPBrowserTabsListsByIndex(t *testing.T) {
	f := tabsLich(t)
	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"browser_tabs","arguments":{}}}`)
	text, failed := textOf(t, replies[0])
	want := `[{"index":0,"title":"A","url":"https://example.com/a"},{"index":1,"title":"B","url":"https://example.com/b","front":true}]`
	if failed || text != want {
		t.Fatalf("tool answered %q (failed %v), want %q", text, failed, want)
	}
}

func TestBrowserTabNewLoadsItsURLWithoutAsking(t *testing.T) {
	f := tabsLich(t)
	code, _, stderr := run(t, f, "browser", "tab", "new", "https://example.com/c")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	opts := optionsOf[api.BrowserTabOptions](t, f.only(t))
	if opts.Action != api.TabNew || opts.URL != "https://example.com/c" || opts.Tab != "" {
		t.Errorf("options = %+v", opts)
	}
}

func TestBrowserTabWantsANumberedIndex(t *testing.T) {
	f := tabsLich(t)
	if code, _, _ := run(t, f, "browser", "tab", "select", "first"); code == 0 {
		t.Fatal("tab select first was taken")
	}
	if code, _, _ := run(t, f, "browser", "tab"); code == 0 {
		t.Fatal("tab with no action was taken")
	}
}
