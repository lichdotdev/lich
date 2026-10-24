package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestServiceCallsSendTheirOptions(t *testing.T) {
	ctx := context.Background()
	one := `{"id":"s1"}`
	cases := []struct {
		name, method, args string
		result             any
		call               func(c *Client) error
	}{
		{"Services", "services.List", ``, []Service{}, func(c *Client) error { _, err := c.Services(ctx); return err }},
		{"SaveService", "services.Save", `{"id":"","projectId":"p","name":"db","command":"pg","startWithLich":true}`, Service{}, func(c *Client) error {
			_, err := c.SaveService(ctx, api.SaveServiceOptions{ProjectID: "p", Name: "db", Command: "pg", StartWithLich: true})
			return err
		}},
		{"DeleteService", "services.Delete", one, nil, func(c *Client) error { return c.DeleteService(ctx, "s1") }},
		{"StartService", "services.Start", one, nil, func(c *Client) error { return c.StartService(ctx, "s1") }},
		{"StopService", "services.Stop", one, nil, func(c *Client) error { return c.StopService(ctx, "s1") }},
		{"ForceStopService", "services.ForceStop", one, nil, func(c *Client) error { return c.ForceStopService(ctx, "s1") }},
		{"RestartService", "services.Restart", one, nil, func(c *Client) error { return c.RestartService(ctx, "s1") }},
		{"StartAllServices", "services.StartAll", `{"id":"p"}`, nil, func(c *Client) error { return c.StartAllServices(ctx, "p") }},
		{"StopAllServices", "services.StopAll", `{"id":"p"}`, nil, func(c *Client) error { return c.StopAllServices(ctx, "p") }},
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

func TestServiceOutputDecodesTheBytes(t *testing.T) {
	c := fakeBackend(t, func(string, []json.RawMessage) (any, int) { return []byte("exit 3\r\n"), http.StatusOK })
	out, err := c.ServiceOutput(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, []byte("exit 3\r\n")) {
		t.Errorf("ServiceOutput = %q", out)
	}
}
