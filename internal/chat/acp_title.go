package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/lichdotdev/lich/internal/providers"
)

// titleArgs is the print mode of the two ACP agents whose call was measured:
// oh-my-pi's `-p`, which saves no session, and opencode 1.19.4's `run`, which
// always saves one (titleReply deletes it) and takes the title agent, the one
// that runs no tool, so it keeps the default agent's tools off. Antigravity,
// Cursor CLI and Kiro CLI always leave a conversation with no way to delete it
// that lich can use, and Cursor's --model rewrites the user's default model
// (docs/ceilings.md).
func (d *acpDriver) titleArgs(prompt string) []string {
	switch d.c.kind {
	case providers.OMP:
		return []string{"-p", "--no-session", "--no-tools", "--thinking", "low", "--no-title", prompt}
	case providers.OpenCode:
		args := []string{"run", "--format", "json", "--agent", "title", "--variant", "low"}
		if model := d.openCodeModel(); model != "" {
			args = append(args, "-m", model)
		}
		return append(args, prompt)
	}
	return nil
}

// openCodeModel is the session's model as `-m` takes it, provider/model:
// without it the call runs on opencode's default, which a free tier can leave
// hanging until the timeout. The id's variant, if it has one, is dropped.
func (d *acpDriver) openCodeModel() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.settings.models == nil {
		return ""
	}
	parts := strings.SplitN(d.settings.models.Current, "/", 3)
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

// titleReply reads the reply out of opencode's JSON events, one per line, and
// deletes the session the call created, which opencode keeps in its history
// and resume list.
func (d *acpDriver) titleReply(out []byte) string {
	if d.c.kind != providers.OpenCode {
		return string(out)
	}
	var reply strings.Builder
	var session string
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		var e struct {
			Type      string `json:"type"`
			SessionID string `json:"sessionID"`
			Part      struct {
				Text string `json:"text"`
			} `json:"part"`
		}
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if e.SessionID != "" {
			session = e.SessionID
		}
		if e.Type == "text" {
			reply.WriteString(e.Part.Text)
		}
	}
	if session != "" {
		d.forgetTitleSession(session)
	}
	return reply.String()
}

// forgetTitleSession runs on a timeout of its own: the call it follows may have been
// killed by its timeout, which is when a session is most likely left behind.
func (d *acpDriver) forgetTitleSession(session string) {
	ctx, cancel := context.WithTimeout(d.c.svc.quit, callTimeout)
	defer cancel()
	cmd, err := d.c.proc.Command(ctx, "session", "delete", session)
	if err == nil {
		cmd.Dir = d.c.cwd
		cmd.Env = titleEnv(cmd.Env)
		err = cmd.Run()
	}
	if err != nil {
		slog.Debug("chat: delete opencode's title session", "session", d.c.id, "err", err)
	}
}
