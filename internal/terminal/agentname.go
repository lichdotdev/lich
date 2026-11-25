package terminal

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
)

// The name records Claude Code writes into a conversation's transcript. Both
// carry the same string and both are written at spawn — `custom-title` first,
// `agent-name` right behind it — and `agent-name` is rewritten from then on,
// once per turn and again on every `/rename` (measured on 2.1.263). The last
// one on record is therefore the name the session answers to now, whichever of
// the two it is.
//
// They are matched as bytes before anything is parsed: a transcript line is
// mostly tool output, and decoding every one of them to find the handful that
// name anything would cost the read what the tail bound saves it.
//
// Taking the last of either kind is also what reads a fork correctly. A fork's
// transcript opens as a copy of the parent's, records and all, and the name
// lich hands it lands behind them — a reader preferring one kind over the other
// could pick a name out of the copied history and put two cards under it.
var (
	agentNameRecord   = []byte(`"agent-name"`)
	customTitleRecord = []byte(`"custom-title"`)
)

// AgentName is the name this session's agent answers to in its provider's peer
// roster right now — the one lich passed at spawn, or whatever a `/rename`
// typed inside the session changed it to. Empty when there is nothing on
// record, which is what leaves the caller with lich's own derived name
// (relay.RosterNameOf).
//
// Claude Code is the only provider read here, and the only one that has
// anything to read: it is the only one lich names at spawn (command.go,
// nameArgs) and the only one whose sessions can rename themselves. Codex,
// Antigravity, opencode, oh-my-pi, Crush, Cursor CLI and Kiro CLI are never
// handed a roster name at all — the name lich addresses their sessions by is
// lich's own, it never crosses into the harness, and there is no `/rename` in
// any of them for it to fall out of step with.
//
// The read is the transcript rather than the `sessions/<pid>.json` Claude Code
// keeps beside it, which is one small file against a bounded tail of a large
// one. That file is keyed by the pid of the `claude` process, and a confined
// session's runs in a PID namespace of its own (internal/sandbox, --unshare-pid)
// whose numbers lich does not share — so the sessions most in need of the read
// are the ones it could not name. The transcript is the same file for both, and
// the one this package already resolves for cost and for the last-turn recap.
func (s *Service) AgentName(id string) string {
	providerSessionID, err := s.store.ProviderSession(id)
	if err != nil || providerSessionID == "" {
		return ""
	}
	path, ok := claudeTranscriptPath(providerSessionID)
	if !ok {
		return ""
	}
	return s.names.agentName(id, path)
}

// nameCursors is where each session's transcript was last read to for its
// agent's name (see transcriptCursors). The roster asks for every session on
// every call, and the tray calls it every few seconds: a 4MB tail re-read per
// session per call costs a whole core with a few dozen sessions open.
type nameCursors struct {
	cursors transcriptCursors[string]
}

// agentName reads what the transcript at path has grown since this session was
// last asked, and keeps the last name on record. The name already on record
// stands when those new bytes hold none, which is every turn but the one a
// /rename lands in.
func (c *nameCursors) agentName(id, path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return ""
	}
	var name string
	c.cursors.under(id, info, func(cur *transcriptCursor[string], _ bool) {
		lines, end := readLines(f, cur.offset, info.Size())
		cur.info, cur.offset = info, end
		if found := claudeAgentName(lines); found != "" {
			cur.value = found
		}
		name = cur.value
	})
	return name
}

// claudeAgentName reads the last name record out of a Claude transcript tail,
// empty when it holds none. A tail cut mid-line fails to parse like any
// malformed one, which is why nothing here distinguishes the two.
//
// Ceiling: a session's first read starts a bounded tail (searchTailBytes) from
// the end, so a resumed session whose last record sits further back than that
// reads empty until its next turn writes another one. The caller falls back to
// the derived name for that window, which is the name a session that was never
// renamed answers to anyway.
func claudeAgentName(tail []byte) string {
	name := ""
	for line := range bytes.SplitSeq(tail, []byte("\n")) {
		if !bytes.Contains(line, agentNameRecord) && !bytes.Contains(line, customTitleRecord) {
			continue
		}
		var record struct {
			Type        string `json:"type"`
			AgentName   string `json:"agentName"`
			CustomTitle string `json:"customTitle"`
		}
		if json.Unmarshal(line, &record) != nil {
			continue
		}
		switch record.Type {
		case "agent-name":
			if record.AgentName != "" {
				name = record.AgentName
			}
		case "custom-title":
			if record.CustomTitle != "" {
				name = record.CustomTitle
			}
		}
	}
	return strings.TrimSpace(name)
}
