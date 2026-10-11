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
