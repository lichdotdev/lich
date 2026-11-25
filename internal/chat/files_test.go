package chat

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/project"
)

func TestChangedFiles(t *testing.T) {
	read := func(head string, files map[string]project.FileLines) *project.Lines {
		return &project.Lines{Head: head, Files: files}
	}
	before := read("h", map[string]project.FileLines{"a": {Added: 1}, "gone": {Removed: 2}, "same": {Added: 3}})
	for _, tc := range []struct {
		name          string
		before, after *project.Lines
		want          []string
	}{
		{"changed, new and reverted", before, read("h", map[string]project.FileLines{"a": {Added: 2}, "new": {Added: 1}, "same": {Added: 3}}), []string{"a", "gone", "new"}},
		{"nothing changed", before, before, nil},
		{"no reading before", nil, before, nil},
		{"no reading after", before, nil, nil},
		{"another commit", before, read("h2", nil), nil},
	} {
		if got := changedFiles(tc.before, tc.after); !slices.Equal(got, tc.want) {
			t.Errorf("%s: changedFiles = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A call is heard ending once, live, and the files lich found it changed
// survive the provider sending the call again.
func TestConversationHearsACallEndOnce(t *testing.T) {
	c := newConversation(func(api.ChatItem) {})
	var heard []string
	c.ended = func(it api.ChatItem) { heard = append(heard, it.ID) }
	call := func(state string) api.ChatItem {
		return api.ChatItem{ID: "b", Kind: api.ItemTool, Tool: &api.ChatToolCall{Name: "Bash", State: state}}
	}
	c.put(call(api.ToolRunning), true)
	c.put(call(api.ToolDone), true)
	c.put(call(api.ToolDone), true)
	if !slices.Equal(heard, []string{"b"}) {
		t.Fatalf("heard %v, want b once", heard)
	}
	files := []api.ChatFileChange{{Path: "a.txt", Diff: "+z"}}
	c.attachFiles("b", files)
	c.put(call(api.ToolDone), true)
	if it, _ := c.get("b"); it.Tool == nil || !slices.Equal(it.Tool.Files, files) {
		t.Errorf("the call sent again carries %+v, want its files kept", it.Tool)
	}
	c.attachFiles("missing", files)
	if _, ok := c.get("missing"); ok {
		t.Error("files for a call not in the conversation made an item")
	}
	if len(heard) != 1 {
		t.Errorf("attaching files was heard as a call ending: %v", heard)
	}
	c.setQuiet(true)
	c.put(api.ChatItem{ID: "r", Kind: api.ItemTool, Tool: &api.ChatToolCall{Name: "Read", State: api.ToolDone}}, true)
	if len(heard) != 1 {
		t.Errorf("a replayed call was heard ending: %v", heard)
	}
}

// A call's reading waits for the call that ended before it: run off the
// stream in goroutines, a later call's reading could otherwise take the files
// an earlier one changed (TestAShellCommandCarriesTheFilesItChanged lost the
// sed's files to the read after it on the macOS runners).
func TestCallReadingsRunInTheOrderTheCallsEnded(t *testing.T) {
	var w fileWatch
	first, firstDone := w.queue()
	if first != nil {
		t.Fatal("the turn's first call waits on nothing")
	}
	second, secondDone := w.queue()
	third, _ := w.queue()
	select {
	case <-second:
		t.Fatal("the second call's reading ran before the first's ended")
	default:
	}
	close(firstDone)
	<-second
	select {
	case <-third:
		t.Fatal("the third call's reading ran before the second's ended")
	default:
	}
	close(secondDone)
	<-third
}
