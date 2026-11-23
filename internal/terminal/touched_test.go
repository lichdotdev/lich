package terminal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// postTouched posts body to a transport whose touches are counted and whose
// note is answer, and returns the status and the decoded body.
func postTouched(t *testing.T, body, answer string) (int, api.TouchedAnswer, int, api.TouchedRequest) {
	t.Helper()
	refreshes := 0
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, func(string) { refreshes++ })
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.close() })
	var asked api.TouchedRequest
	tr.editNote = func(req api.TouchedRequest) string { asked = req; return answer }
	url := fmt.Sprintf("http://127.0.0.1:%d/session-touched?token=%s", tr.port, tr.token)
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var got api.TouchedAnswer
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("decode %q: %v", raw, err)
		}
	}
	return resp.StatusCode, got, refreshes, asked
}

func TestSessionTouchedAnswersTheNote(t *testing.T) {
	t.Parallel()
	status, got, refreshes, asked := postTouched(t, `{"session_id":"s1","paths":["/repo/a.go"]}`, "<lich-notification/>")
	if status != http.StatusOK || got.Note != "<lich-notification/>" {
		t.Fatalf("status %d, note %q; want 200 and the note", status, got.Note)
	}
	if refreshes != 1 || asked.SessionID != "s1" || len(asked.Paths) != 1 {
		t.Fatalf("refreshes %d, asked %+v; want one refresh and the request handed over", refreshes, asked)
	}
}

func TestSessionTouchedWithoutANoteIs204(t *testing.T) {
	t.Parallel()
	if status, _, refreshes, _ := postTouched(t, `{"session_id":"s1","paths":["/repo/a.go"],"hold":true}`, ""); status != http.StatusNoContent || refreshes != 1 {
		t.Fatalf("status %d, refreshes %d; want 204 and one refresh", status, refreshes)
	}
}

// A collect fires before every model call on Antigravity: a git refresh each
// time would cost more than the poll it is meant to beat.
func TestSessionTouchedCollectRefreshesNothing(t *testing.T) {
	t.Parallel()
	status, got, refreshes, asked := postTouched(t, `{"session_id":"s1","collect":true}`, "note")
	if status != http.StatusOK || got.Note != "note" || refreshes != 0 || !asked.Collect {
		t.Fatalf("status %d, note %q, refreshes %d, collect %v; want 200, the note, no refresh", status, got.Note, refreshes, asked.Collect)
	}
}

// One apply_patch names every file it changed, past hookBodyLimit.
func TestSessionTouchedTakesALongPathList(t *testing.T) {
	t.Parallel()
	paths := make([]string, 200)
	for i := range paths {
		paths[i] = fmt.Sprintf("/home/someone/projects/repository/internal/package/file_%03d.go", i)
	}
	body, err := json.Marshal(api.TouchedRequest{SessionID: "s1", Paths: paths})
	if err != nil {
		t.Fatal(err)
	}
	if len(body) <= hookBodyLimit {
		t.Fatalf("body is %d bytes, not past hookBodyLimit", len(body))
	}
	if status, _, _, asked := postTouched(t, string(body), ""); status != http.StatusNoContent || len(asked.Paths) != 200 {
		t.Fatalf("status %d with %d paths, want 204 with 200", status, len(asked.Paths))
	}
}
