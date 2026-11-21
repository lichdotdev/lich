//go:build !windows

package chat

import (
	"encoding/base64"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// promptWith sends text with files attached, and waits for its turn to end.
func (r *rig) promptWith(text string, files ...string) {
	r.t.Helper()
	att := make([]api.DropAttachment, len(files))
	for i, f := range files {
		att[i] = api.DropAttachment{Path: f}
	}
	if _, err := r.svc.Prompt(r.t.Context(), api.ChatPromptOptions{ID: "s1", Text: text, Attachments: att}); err != nil {
		r.t.Fatalf("Prompt: %v", err)
	}
	r.turnEnd(0)
}

// The person's message shows the files it attached, whatever went to the
// agent.
func (r *rig) wantUserFiles(files ...string) {
	r.t.Helper()
	if it := r.mustItem(api.ItemUser, "hi"); it.Text != "hi" || !slices.Equal(it.Files, files) {
		r.t.Errorf("person's message = %+v, want hi with %v", it, files)
	}
}

// Claude Code gets an image as an image block beside the text.
func TestClaudeChatSendsAnImageBlock(t *testing.T) {
	r := newChat(t, providers.Claude)
	shot := writeAttachment(t, "shot.png", pngHead)
	r.promptWith("hi", shot)
	log := r.fakeLog()
	if !strings.Contains(log, `{"text":"hi","type":"text"}`) || !strings.Contains(log, `"media_type":"image/png"`) ||
		!strings.Contains(log, base64.StdEncoding.EncodeToString(pngHead)) {
		t.Errorf("no text and image blocks reached Claude Code: %q", log)
	}
	r.wantUserFiles(shot)
}

// Codex gets an image as a localImage item it reads itself.
func TestCodexChatSendsALocalImage(t *testing.T) {
	r := newCodexChat(t)
	shot := writeAttachment(t, "shot.png", pngHead)
	r.promptWith("hi", shot)
	starts := r.turnStarts()
	if len(starts) != 1 || !strings.Contains(starts[0], `{"path":"`+shot+`","type":"localImage"}`) {
		t.Errorf("turn/start = %q, want a localImage of %s", starts, shot)
	}
	r.wantUserFiles(shot)
}

// An ACP agent whose initialize says nothing of images gets an image as its
// path in the text.
func TestACPChatWithoutImagesSendsThePath(t *testing.T) {
	r := newChat(t, providers.OMP)
	shot := writeAttachment(t, "shot.png", pngHead)
	r.promptWith("hi", shot)
	if log := r.fakeLog(); !strings.Contains(log, `"text":"hi\n\n`+shot+`"`) || strings.Contains(log, `"type":"image"`) {
		t.Errorf("session/prompt = %q, want the path in the text", log)
	}
	r.wantUserFiles(shot)
}
