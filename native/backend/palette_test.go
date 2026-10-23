package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestPaletteCallsSendPositionalArgs(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name, method, args string
		result             any
		call               func(c *Client) error
	}{
		{"RecentProjectsMatching", "store.RecentProjects", `{"term":"plug"}`, []RecentProject{}, func(c *Client) error {
			_, err := c.RecentProjectsMatching(ctx, "plug")
			return err
		}},
		{"ClosedProjectCountMatching", "store.ClosedProjectCount", `{"term":"plug"}`, 2, func(c *Client) error {
			_, err := c.ClosedProjectCountMatching(ctx, "plug")
			return err
		}},
		{"ClosedSessions", "store.ClosedSessions", `{"term":"relay"}`, ClosedHistory{}, func(c *Client) error {
			_, err := c.ClosedSessions(ctx, "relay")
			return err
		}},
		{"ExternalSessions", "store.ExternalSessions", ``, []ExternalSession{}, func(c *Client) error {
			_, err := c.ExternalSessions(ctx)
			return err
		}},
		{"AdoptExternalSession", "store.AdoptExternalSession", `{"projectId":"p","sessionId":"s1","kind":"codex","path":"/wt","providerSessionId":"c1","label":"Port"}`, nil, func(c *Client) error {
			return c.AdoptExternalSession(ctx, "p", "s1", "codex", "/wt", "c1", "Port")
		}},
		{"ForgetSession", "store.ForgetSession", `{"id":"h1"}`, nil, func(c *Client) error {
			return c.ForgetSession(ctx, "h1")
		}},
		{"SearchTranscripts", "terminal.SearchTranscripts", `{"ids":["s1","s2"],"query":"flaky"}`, []TranscriptMatch{}, func(c *Client) error {
			_, err := c.SearchTranscripts(ctx, []string{"s1", "s2"}, "flaky")
			return err
		}},
		{"BranchesOf", "project.BranchesOf", `{"paths":["/a"]}`, map[string]string{}, func(c *Client) error {
			_, err := c.BranchesOf(ctx, []string{"/a"})
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := fakeBackend(t, func(method string, args []json.RawMessage) (any, int) {
				if method != tc.method || rawArgs(args) != tc.args {
					t.Errorf("got %s(%s), want %s(%s)", method, rawArgs(args), tc.method, tc.args)
				}
				return tc.result, http.StatusOK
			})
			if err := tc.call(c); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The wire names are store.ClosedSession's and store.ExternalSession's own
// json tags, which the History tab reads field for field.
func TestPaletteHistoryDecodes(t *testing.T) {
	const wire = `{"sessions":[{"id":"h1","projectId":"p1","projectName":"lich","projectPath":"/src/lich","label":"Relay","kind":"claude","path":"/wt/relay","parkedBranch":"feat/relay","closedAt":1700000200,"matchedConversation":true,"snippet":"…relay…","truncated":true}],"total":143,"indexing":12}`
	var h ClosedHistory
	if err := json.Unmarshal([]byte(wire), &h); err != nil {
		t.Fatal(err)
	}
	want := ClosedSession{ID: "h1", ProjectID: "p1", ProjectName: "lich", ProjectPath: "/src/lich", Label: "Relay", Kind: "claude",
		Path: "/wt/relay", ParkedBranch: "feat/relay", ClosedAt: 1700000200, MatchedConversation: true, Snippet: "…relay…", Truncated: true}
	if h.Total != 143 || h.Indexing != 12 || len(h.Sessions) != 1 || h.Sessions[0] != want {
		t.Errorf("decoded %+v, want one %+v of 143 with 12 indexing", h, want)
	}
}

func TestPaletteSearchesDecode(t *testing.T) {
	ctx := context.Background()
	c := fakeBackend(t, func(method string, _ []json.RawMessage) (any, int) {
		switch method {
		case "terminal.SearchTranscripts":
			return []TranscriptMatch{{ID: "s3", Snippet: "wire the auth", Count: 2}}, http.StatusOK
		case "project.BranchesOf":
			return map[string]string{"/wt/a": "feat/a"}, http.StatusOK
		}
		return []ExternalSession{{Kind: "kiro", ProviderSessionID: "k1", ProjectID: "p2", Profile: "work"}}, http.StatusOK
	})
	found, err := c.SearchTranscripts(ctx, []string{"s3"}, "auth")
	if err != nil || len(found) != 1 || found[0] != (TranscriptMatch{ID: "s3", Snippet: "wire the auth", Count: 2}) {
		t.Fatalf("SearchTranscripts = %+v, %v", found, err)
	}
	named, err := c.BranchesOf(ctx, []string{"/wt/a"})
	if err != nil || named["/wt/a"] != "feat/a" {
		t.Fatalf("BranchesOf = %v, %v", named, err)
	}
	external, err := c.ExternalSessions(ctx)
	if err != nil || len(external) != 1 || external[0].Profile != "work" || external[0].Kind != "kiro" {
		t.Fatalf("ExternalSessions = %+v, %v", external, err)
	}
}
