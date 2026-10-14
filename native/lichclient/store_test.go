package lichclient

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
)

func TestStoreCallsSendPositionalArgs(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name, method, args string
		result             any
		call               func(c *Client) error
	}{
		{"ReorderSessions", "store.ReorderSessions", `"p",["b","a"]`, nil, func(c *Client) error {
			return c.ReorderSessions(ctx, "p", []string{"b", "a"})
		}},
		{"SetSessionFolder", "store.SetSessionFolder", `"s1","Apps"`, nil, func(c *Client) error {
			return c.SetSessionFolder(ctx, "s1", "Apps")
		}},
		{"RenameFolder", "store.RenameFolder", `"p","Apps","Ops"`, []string{"s1"}, func(c *Client) error {
			_, err := c.RenameFolder(ctx, "p", "Apps", "Ops")
			return err
		}},
		{"AddSessionInFolder", "store.AddSessionInFolder", `"p","s9","Claude 9","claude","/wt/a",9,"","Apps"`, nil, func(c *Client) error {
			return c.AddSessionInFolder(ctx, "p", "s9", "Claude 9", "claude", "/wt/a", 9, "Apps")
		}},
		{"DetectProviders", "providers.Detect", ``, []DetectedProvider{}, func(c *Client) error {
			_, err := c.DetectProviders(ctx)
			return err
		}},
		{"ProviderVersion", "providers.Version", `"codex"`, "0.162.1", func(c *Client) error {
			_, err := c.ProviderVersion(ctx, "codex")
			return err
		}},
		{"ProviderBinaries", "providers.OnPath", `"codex"`, []ProviderBinary{{Path: "/usr/bin/codex", Version: "0.147.0"}}, func(c *Client) error {
			_, err := c.ProviderBinaries(ctx, "codex")
			return err
		}},
		{"CloseProject", "store.CloseProject", `"p"`, nil, func(c *Client) error {
			return c.CloseProject(ctx, "p")
		}},
		{"ReorderProjects", "store.ReorderProjects", `["b","a"]`, nil, func(c *Client) error {
			return c.ReorderProjects(ctx, []string{"b", "a"})
		}},
		{"RecentProjects", "store.RecentProjects", `""`, []RecentProject{}, func(c *Client) error {
			_, err := c.RecentProjects(ctx)
			return err
		}},
		{"ClosedProjectCount", "store.ClosedProjectCount", `""`, 3, func(c *Client) error {
			_, err := c.ClosedProjectCount(ctx)
			return err
		}},
		{"PickProject", "project.Open", ``, nil, func(c *Client) error {
			_, err := c.PickProject(ctx)
			return err
		}},
		{"RelocateProject", "project.Relocate", `"p"`, nil, func(c *Client) error {
			_, err := c.RelocateProject(ctx, "p")
			return err
		}},
		{"PathExists", "project.Exists", `"/a"`, true, func(c *Client) error {
			_, err := c.PathExists(ctx, "/a")
			return err
		}},
		{"MissingPaths", "project.Missing", `["/a","/b"]`, []string{"/b"}, func(c *Client) error {
			_, err := c.MissingPaths(ctx, []string{"/a", "/b"})
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

func TestRenameFolderAndDetectDecode(t *testing.T) {
	ctx := context.Background()
	c := fakeBackend(t, func(method string, _ []json.RawMessage) (any, int) {
		if method == "store.RenameFolder" {
			return []string{"s1", "s3"}, http.StatusOK
		}
		return []DetectedProvider{{ID: "codex", Name: "Codex", Installed: true}}, http.StatusOK
	})
	moved, err := c.RenameFolder(ctx, "p", "Apps", "")
	if err != nil || !slices.Equal(moved, []string{"s1", "s3"}) {
		t.Fatalf("RenameFolder = %v, %v; want [s1 s3]", moved, err)
	}
	found, err := c.DetectProviders(ctx)
	if err != nil || len(found) != 1 || found[0] != (DetectedProvider{ID: "codex", Name: "Codex", Installed: true}) {
		t.Fatalf("DetectProviders = %+v, %v", found, err)
	}
}

// The wire names are providers.Detected's own json tags; a Settings row reads
// the executable, where it was found and the page that documents installing it.
func TestDetectedProviderReadsBinarySourceAndDocs(t *testing.T) {
	const wire = `{"id":"codex","name":"Codex","binary":"codex","installed":true,"path":"/opt/codex","source":"setting","docs":"https://example.com/codex"}`
	var d DetectedProvider
	if err := json.Unmarshal([]byte(wire), &d); err != nil {
		t.Fatal(err)
	}
	want := DetectedProvider{ID: "codex", Name: "Codex", Binary: "codex", Installed: true, Source: "setting", Docs: "https://example.com/codex"}
	if d != want {
		t.Errorf("decoded %+v, want %+v", d, want)
	}
}
