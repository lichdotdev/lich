//go:build !windows

package chat

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// TestAShellCommandCarriesTheFilesItChanged: a call that says nothing of the
// files it writes gets them from git once it ends, each with its diff, and
// the call after it, which changed nothing, gets none.
func TestAShellCommandCarriesTheFilesItChanged(t *testing.T) {
	r := newRig(t, providers.Claude)
	r.gitCheckout()
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("sed")
	r.turnEnd(0)
	var shell, read api.ChatItem
	r.eventually("the shell command's files", func() bool {
		for _, it := range r.history() {
			if it.ID == "toolu_sed" {
				shell = it
			}
			if it.ID == "toolu_read" {
				read = it
			}
		}
		return shell.Tool != nil && len(shell.Tool.Files) > 0
	})
	files := shell.Tool.Files
	if len(files) != 2 || files[0].Path != "a.txt" || files[1].Path != "c.txt" {
		t.Fatalf("the shell command changed %+v, want a.txt and c.txt", files)
	}
	if !strings.Contains(files[0].Diff, "-x") || !strings.Contains(files[0].Diff, "+z") || !strings.Contains(files[1].Diff, "+new") {
		t.Errorf("diffs %q and %q, want a.txt's x to z and c.txt's new line", files[0].Diff, files[1].Diff)
	}
	if read.Tool == nil || len(read.Tool.Files) != 0 {
		t.Errorf("the Read after it carries %+v, want no files", read.Tool)
	}
}
